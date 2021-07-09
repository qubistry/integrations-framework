package volume

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
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
	OnChainCheckAttemptsOpt     func(config *retry.Config)
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
	DefaultSetup            *suite.DefaultSuiteSetup
	FluxInstances           *[]contracts.FluxAggregator
	FluxJobs                *[]*client.Job
	OnChainCheckAttemptsOpt func(config *retry.Config)
	Adapter                 tools.ExternalAdapter
	Nodes                   []client.Chainlink
	Prom                    *tools.PromChecker
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
				MinSubmissions:     uint32(len(d.Oracles)),
				MaxSubmissions:     uint32(len(d.Oracles)),
				RestartDelayRounds: 0,
			})
		Expect(err).ShouldNot(HaveOccurred())
		for _, n := range d.Nodes {
			fluxSpec := &client.FluxMonitorJobSpec{
				Name:            fmt.Sprintf("%s_%d", d.Spec.JobPrefix, d.Index),
				ContractAddress: fluxInstance.Address(),
				PollTimerPeriod: d.Spec.NodePollTimePeriod,
				// it's crucial not to skew rounds schedule for volume tests
				IdleTimerDisabled: true,
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
	err = suite.FundTemplateNodes(s.Client, s.Wallets, clNodes, 9e18, 0)
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
		DefaultSetup:            s,
		FluxInstances:           &fluxInstances,
		FluxJobs:                &fluxJobs,
		Adapter:                 adapter,
		Nodes:                   clNodes,
		Prom:                    prom,
		OnChainCheckAttemptsOpt: spec.OnChainCheckAttemptsOpt,
	}
}

func (vt *VolumeFluxTest) AwaitRoundFinishedOnChain(roundID int, newVal int) {
	if err := retry.Do(func() error {
		if !vt.checkRoundFinishedOnChain(roundID, newVal) {
			return errors.New("round is not finished")
		}
		return nil
	}, vt.OnChainCheckAttemptsOpt); err != nil {
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
