package volume

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/actions"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/suite"
	suitecommon "github.com/smartcontractkit/integrations-framework/suite/common"
	"github.com/smartcontractkit/integrations-framework/tools"
	"golang.org/x/sync/errgroup"
	"math/big"
	"sync"
	"time"
)

// FluxTestSpec flux aggregator volume test spec
type FluxTestSpec struct {
	suitecommon.TestSpec
	AggregatorsNum              int
	RequiredSubmissions         int
	RestartDelayRounds          int
	JobPrefix                   string
	ObservedValueChangeInterval time.Duration
	NodePollTimePeriod          time.Duration
	FluxOptions                 contracts.FluxAggregatorOptions
}

// FluxTest flux test runtime data
type FluxTest struct {
	suitecommon.Test
	// round durations, calculated as a difference from earliest chainlink run for contract,
	// until all confirmations found on-chain (block_timestamp)
	roundsDurationData []time.Duration
	FluxInstances      *[]contracts.FluxAggregator
	ContractsToJobsMap map[string][]contracts.JobByInstance
	NodesByHostPort    map[string]client.Chainlink
	AlreadySeenRuns    map[string]bool
}

// FluxInstanceDeployment data required by flux instance to calculate per round metrics
type FluxInstanceDeployment struct {
	suitecommon.InstanceDeployment
	Spec              *FluxTestSpec
	FluxInstancesMu   *sync.Mutex
	FluxInstances     *[]contracts.FluxAggregator
	ContractToJobsMap map[string][]contracts.JobByInstance
	NodesByHostPort   map[string]client.Chainlink
}

// NewFluxTest deploys AggregatorsNum flux aggregators concurrently
func NewFluxTest(spec *FluxTestSpec) (*FluxTest, error) {
	s, err := suite.DefaultLocalSetup(spec.InitFunc)
	if err != nil {
		return nil, err
	}
	clNodes, nodeAddrs, err := suite.ConnectToTemplateNodes()
	if err != nil {
		return nil, err
	}
	err = suite.FundTemplateNodes(s.Client, s.Wallets, clNodes, 9e18, 0)
	if err != nil {
		return nil, err
	}
	adapter, err := tools.NewExternalAdapter()
	if err != nil {
		return nil, err
	}

	fluxInstances := make([]contracts.FluxAggregator, 0)
	nodesByHostPort := make(map[string]client.Chainlink)
	contractToJobsMap := make(map[string][]contracts.JobByInstance)
	mu := &sync.Mutex{}
	//g := &errgroup.Group{}
	for i := 0; i < spec.AggregatorsNum; i++ {
		d := &FluxInstanceDeployment{
			InstanceDeployment: suitecommon.InstanceDeployment{
				Index:   i,
				Suite:   s,
				Oracles: nodeAddrs,
				Nodes:   clNodes,
				Adapter: adapter,
			},
			Spec:              spec,
			FluxInstancesMu:   mu,
			NodesByHostPort:   nodesByHostPort,
			FluxInstances:     &fluxInstances,
			ContractToJobsMap: contractToJobsMap,
		}
		log.Info().Int("Instance ID", d.Index).Msg("Deploying contracts instance")
		fluxInstance, err := d.Suite.Deployer.DeployFluxAggregatorContract(d.Suite.Wallets.Default(), d.Spec.FluxOptions)
		if err != nil {
			return nil, err
		}
		err = fluxInstance.Fund(d.Suite.Wallets.Default(), big.NewInt(0), big.NewInt(1e18))
		if err != nil {
			return nil, err
		}
		err = fluxInstance.UpdateAvailableFunds(context.Background(), d.Suite.Wallets.Default())
		if err != nil {
			return nil, err
		}
		// set oracles and submissions
		err = fluxInstance.SetOracles(d.Suite.Wallets.Default(),
			contracts.FluxAggregatorSetOraclesOptions{
				AddList:            d.Oracles,
				RemoveList:         []common.Address{},
				AdminList:          d.Oracles,
				MinSubmissions:     uint32(d.Spec.RequiredSubmissions),
				MaxSubmissions:     uint32(d.Spec.RequiredSubmissions),
				RestartDelayRounds: uint32(d.Spec.RestartDelayRounds),
			})
		if err != nil {
			return nil, err
		}
		for _, n := range d.Nodes {
			fluxSpec := &client.FluxMonitorJobSpec{
				Name:            fmt.Sprintf("%s_%d", d.Spec.JobPrefix, d.Index),
				ContractAddress: fluxInstance.Address(),
				PollTimerPeriod: d.Spec.NodePollTimePeriod,
				// it's crucial not to skew rounds schedule for that particular volume test
				IdleTimerDisabled: true,
				PollTimerDisabled: false,
				ObservationSource: client.ObservationSourceSpec(d.Adapter.InsideDockerAddr + "/variable"),
			}
			job, err := n.CreateJob(fluxSpec)
			if err != nil {
				return nil, err
			}
			d.FluxInstancesMu.Lock()
			d.NodesByHostPort[n.URL()] = n
			d.ContractToJobsMap[fluxInstance.Address()] = append(d.ContractToJobsMap[fluxInstance.Address()],
				contracts.JobByInstance{
					ID:       job.Data.ID,
					Instance: n.URL(),
				})
			d.FluxInstancesMu.Unlock()
		}
		d.FluxInstancesMu.Lock()
		*d.FluxInstances = append(*d.FluxInstances, fluxInstance)
		d.FluxInstancesMu.Unlock()
	}
	//err = g.Wait()
	//if err != nil {
	//	return nil, err
	//}
	prom, err := tools.NewPrometheusClient(s.Config.Prometheus)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("Contracts to jobs", contractToJobsMap).Msg("Debug data for per round metrics")
	return &FluxTest{
		Test: suitecommon.Test{
			DefaultSetup:            s,
			OnChainCheckAttemptsOpt: spec.OnChainCheckAttemptsOpt,
			Nodes:                   clNodes,
			Adapter:                 adapter,
			Prom:                    prom,
		},
		FluxInstances:      &fluxInstances,
		ContractsToJobsMap: contractToJobsMap,
		NodesByHostPort:    nodesByHostPort,
		roundsDurationData: make([]time.Duration, 0),
		AlreadySeenRuns:    make(map[string]bool),
	}, nil
}

// roundsStartTimes gets run start time for every contract, ns
func (vt *FluxTest) roundsStartTimes() (map[string]int64, error) {
	mu := &sync.Mutex{}
	g := &errgroup.Group{}
	contractsStartTimes := make(map[string]int64)
	for contractAddr, jobs := range vt.ContractsToJobsMap {
		contractAddr := contractAddr
		jobs := jobs
		g.Go(actions.GetRoundStartTimesAcrossNodes(contractAddr, jobs, vt.NodesByHostPort, mu, contractsStartTimes))
	}
	err := g.Wait()
	if err != nil {
		return nil, err
	}
	for contract, rst := range contractsStartTimes {
		log.Debug().Str("Contract", contract).Int64("Start time", rst/1e9).Send()
	}
	log.Debug().Interface("Round start times", contractsStartTimes).Send()
	return contractsStartTimes, nil
}

// checkRoundSubmissionsEvents checks whether all submissions is found, gets last event block as round end time
func (vt *FluxTest) checkRoundSubmissionsEvents(ctx context.Context, roundID int, submissions int, submissionVal *big.Int) (map[string]int64, error) {
	g := errgroup.Group{}
	mu := &sync.Mutex{}
	endTimes := make(map[string]int64)
	for _, fi := range *vt.FluxInstances {
		fi := fi
		g.Go(actions.GetRoundCompleteTimestamps(fi, roundID, submissions, submissionVal, mu, endTimes, vt.DefaultSetup.Client))
	}
	err := g.Wait()
	if err != nil {
		return nil, err
	}
	return endTimes, nil
}

// roundsMetrics gets start times from runs via API, awaits submissions for each contract and get round end times
func (vt *FluxTest) roundsMetrics(roundID int, submissions int, submissionVal *big.Int) error {
	startTimes, err := vt.roundsStartTimes()
	if err != nil {
		return err
	}
	endTimes, err := vt.checkRoundSubmissionsEvents(context.Background(), roundID, submissions, submissionVal)
	if err != nil {
		return err
	}
	for contract := range startTimes {
		duration := time.Duration(endTimes[contract] - startTimes[contract])
		vt.roundsDurationData = append(vt.roundsDurationData, duration)
		log.Info().Str("Contract", contract).Str("Round duration", duration.String()).Send()
	}
	return nil
}

// checkRoundDataOnChain check that ContractData about last round is correct
func (vt *FluxTest) checkRoundDataOnChain(roundID int, newVal int) error {
	if err := retry.Do(func() error {
		finished, err := vt.checkContractData(roundID, newVal)
		if err != nil {
			return err
		}
		if !finished {
			return errors.New("round is not finished")
		}
		return nil
	}, vt.OnChainCheckAttemptsOpt); err != nil {
		return errors.Wrap(err, "round is not fully finished on chain")
	}
	return nil
}

func (vt *FluxTest) checkContractData(roundID int, newVal int) (bool, error) {
	log.Debug().Int("Round ID", roundID).Msg("Checking round completion on chain")
	var rounds []*contracts.FluxAggregatorData
	for _, flux := range *vt.FluxInstances {
		cd, err := flux.GetContractData(context.Background())
		if err != nil {
			return false, err
		}
		rounds = append(rounds, cd)
	}
	for _, r := range rounds {
		if r.LatestRoundData.RoundId.Int64() != int64(roundID) || r.LatestRoundData.Answer.Int64() != int64(newVal) {
			return false, nil
		}
	}
	return true, nil
}
