package suite

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/suite"
	"github.com/smartcontractkit/integrations-framework/tools"
	"math/big"
	"sync"
	"time"
)

type VolumeTestSpec struct {
	AggregatorsNum              int
	JobPrefix                   string
	ObservedValueChangeInterval time.Duration
	NodePollTimePeriod          time.Duration
	InitFunc                    client.BlockchainNetworkInit
	FluxOptions                 contracts.FluxAggregatorOptions
}

type VolumeFluxInstanceDeployment struct {
	Wg              *sync.WaitGroup
	Index           int
	Suite           *suite.DefaultSuiteSetup
	Spec            *VolumeTestSpec
	Oracles         []common.Address
	Nodes           []client.Chainlink
	Adapter         tools.ExternalAdapter
	FluxInstancesMu *sync.Mutex
	FluxInstances   *[]contracts.FluxAggregator
	FluxJobs        *[]*client.Job
}

type VolumeFluxTest struct {
	DefaultSetup  *suite.DefaultSuiteSetup
	FluxInstances *[]contracts.FluxAggregator
	FluxJobs      *[]*client.Job
	Adapter       tools.ExternalAdapter
	Nodes         []client.Chainlink
	Prom          *tools.PromQueries
}

// roundVals structure only for debug on-chain check
type roundVals struct {
	RoundID int64
	Val     int64
}

// deployFluxInstance deploy one flux instance concurrently, add jobs to all the nodes
func deployFluxInstance(d *VolumeFluxInstanceDeployment) {
	go func() {
		defer d.Wg.Done()
		log.Info().Int("instance_id", d.Index).Msg("deploying contracts instance")
		fluxInstance, err := d.Suite.Deployer.DeployFluxAggregatorContract(d.Suite.Wallets.Default(), d.Spec.FluxOptions)
		Expect(err).ShouldNot(HaveOccurred())
		err = fluxInstance.Fund(d.Suite.Wallets.Default(), big.NewInt(0), big.NewInt(1e18))
		Expect(err).ShouldNot(HaveOccurred())
		err = fluxInstance.UpdateAvailableFunds(context.Background(), d.Suite.Wallets.Default())
		Expect(err).ShouldNot(HaveOccurred())

		// set oracles and submissions
		err = fluxInstance.SetOracles(d.Suite.Wallets.Default(),
			contracts.SetOraclesOptions{
				AddList:            d.Oracles,
				RemoveList:         []common.Address{},
				AdminList:          d.Oracles,
				MinSubmissions:     5,
				MaxSubmissions:     5,
				RestartDelayRounds: 0,
			})
		Expect(err).ShouldNot(HaveOccurred())
		for _, n := range d.Nodes {
			fluxSpec := &client.FluxMonitorJobSpec{
				Name:              fmt.Sprintf("%s_%d", d.Spec.JobPrefix, d.Index),
				ContractAddress:   fluxInstance.Address(),
				PollTimerPeriod:   d.Spec.NodePollTimePeriod,
				PollTimerDisabled: false,
				ObservationSource: client.ObservationSourceSpec(d.Adapter.InsideDockerAddr + "/variable"),
			}
			job, err := n.CreateJob(fluxSpec)
			Expect(err).ShouldNot(HaveOccurred())
			d.FluxInstancesMu.Lock()
			*d.FluxJobs = append(*d.FluxJobs, job)
			d.FluxInstancesMu.Unlock()
		}
		d.FluxInstancesMu.Lock()
		*d.FluxInstances = append(*d.FluxInstances, fluxInstance)
		d.FluxInstancesMu.Unlock()
	}()
}

// NewVolumeFluxTest deploys AggregatorsNum flux aggregators concurrently
func NewVolumeFluxTest(spec *VolumeTestSpec) *VolumeFluxTest {
	s, err := suite.DefaultLocalSetup(spec.InitFunc)
	Expect(err).ShouldNot(HaveOccurred())

	clNodes, nodeAddrs, err := suite.ConnectToTemplateNodes()
	Expect(err).ShouldNot(HaveOccurred())
	err = suite.FundTemplateNodes(s.Client, s.Wallets, clNodes, 2e18, 0)
	Expect(err).ShouldNot(HaveOccurred())

	s.Client.(*client.EthereumClient).BorrowedNonces(true)

	adapter := tools.NewExternalAdapter()

	fluxInstances := make([]contracts.FluxAggregator, 0)
	fluxJobs := make([]*client.Job, 0)
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	for i := 0; i < spec.AggregatorsNum; i++ {
		wg.Add(1)
		deployFluxInstance(&VolumeFluxInstanceDeployment{
			Wg:              wg,
			Index:           i,
			Suite:           s,
			Spec:            spec,
			Oracles:         nodeAddrs,
			Nodes:           clNodes,
			Adapter:         adapter,
			FluxInstancesMu: mu,
			FluxInstances:   &fluxInstances,
			FluxJobs:        &fluxJobs,
		})
	}
	wg.Wait()
	prom := tools.NewPrometheusClient(s.Config.Prometheus)
	return &VolumeFluxTest{
		DefaultSetup:  s,
		FluxInstances: &fluxInstances,
		FluxJobs:      &fluxJobs,
		Adapter:       adapter,
		Nodes:         clNodes,
		Prom:          prom,
	}
}

func (vt *VolumeFluxTest) AwaitRoundFinishedOnChain(roundID int, newVal int) {
	if err := retry.Do(func() error {
		if !vt.checkRoundFinishedOnChain(roundID, newVal) {
			return errors.New("round is not finished")
		}
		return nil
	}); err != nil {
		log.Fatal().Msg("round is not fully finished on chain")
	}
}

func (vt *VolumeFluxTest) debugRoundTuple(rounds []*contracts.FluxAggregatorData) {
	roundValsArr := make([]roundVals, 0)
	for _, r := range rounds {
		if r == nil {
			continue
		}
		roundValsArr = append(roundValsArr, roundVals{
			RoundID: r.LatestRoundData.RoundId.Int64(),
			Val:     r.LatestRoundData.Answer.Int64(),
		})
	}
	log.Debug().Interface("rounds_on_chain", roundValsArr).Msg("last rounds on chain")
}

func (vt *VolumeFluxTest) checkRoundFinishedOnChain(roundID int, newVal int) bool {
	log.Debug().Int("round_id", roundID).Msg("checking round completion on chain")
	var rounds []*contracts.FluxAggregatorData
	for _, flux := range *vt.FluxInstances {
		cd, err := flux.GetContractData(context.Background())
		if err != nil {
			log.Err(err).Msg("error checking flux last round")
			return false
		}
		rounds = append(rounds, cd)
	}
	vt.debugRoundTuple(rounds)
	for _, r := range rounds {
		if r.LatestRoundData.RoundId.Int64() != int64(roundID) || r.LatestRoundData.Answer.Int64() != int64(newVal) {
			return false
		}
	}
	return true
}

var _ = Describe("Flux monitor volume tests", func() {
	jobPrefix := "flux_monitor"
	vt := NewVolumeFluxTest(&VolumeTestSpec{
		AggregatorsNum:     100,
		JobPrefix:          jobPrefix,
		NodePollTimePeriod: 15 * time.Second,
		InitFunc:           client.NewHardhatNetwork,
		FluxOptions:        contracts.DefaultFluxAggregatorOptions(),
	})
	Describe("consistency test", func() {
		// start from one so currentSample = currentRound
		promRoundTimeout := 60 * time.Second
		currentSample := 1
		samples := 5

		Measure("Should process rounds without errors", func(b Benchmarker) {
			// all contracts/nodes/jobs are setup at that point, triggering new round,
			// waiting for all contracts to complete one round and get metrics
			newVal, err := vt.Adapter.TriggerValueChange(currentSample)
			if err != nil {
				log.Fatal().Err(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), promRoundTimeout)
			defer cancel()

			// await all nodes report round was finished
			roundFinished, err := vt.Prom.AwaitRoundFinishedAcrossNodes(ctx, currentSample+1)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to check round consistency")
			}
			if !roundFinished {
				log.Fatal().Msg("round was not finished in time")
			}

			// await all round data is on chain
			vt.AwaitRoundFinishedOnChain(currentSample+1, newVal)
			summary, err := vt.Prom.FluxRoundSummary(jobPrefix)
			if err != nil {
				log.Error().Err(err).Msg("failed to get prom summary")
				return
			}

			// record benchmark metrics
			b.RecordValue("sum cpu", summary.CPUPercentage)
			b.RecordValue("sum pipeline execution time over interval",
				float64(summary.PipelineExecutionTimeAvgOverIntervalMilliseconds))
			b.RecordValue("sum execution errors", float64(summary.PipelineErrorsSum))
			currentSample += 1
		}, samples)
	})
})
