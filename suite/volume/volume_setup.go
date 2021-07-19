package volume

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/suite"
	"github.com/smartcontractkit/integrations-framework/tools"
	"golang.org/x/sync/errgroup"
	"math/big"
	"sort"
	"sync"
	"time"
)

// FluxTestSpec flux aggregator volume test spec
type FluxTestSpec struct {
	TestSpec
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
	Test
	// round durations, calculated as a difference from earliest chainlink run for contract,
	// until all confirmations found on-chain (block_timestamp)
	roundsDurationData []time.Duration
	FluxInstances      *[]contracts.FluxAggregator
	ContractsToJobsMap map[string][]JobByInstance
	NodesByHostPort    map[string]client.Chainlink
	AlreadySeenRuns    map[string]bool
}

// FluxInstanceDeployment data required by flux instance to calculate per round metrics
type FluxInstanceDeployment struct {
	InstanceDeployment
	FluxInstancesMu   *sync.Mutex
	FluxInstances     *[]contracts.FluxAggregator
	ContractToJobsMap map[string][]JobByInstance
	NodesByHostPort   map[string]client.Chainlink
}

// JobByInstance helper struct to match job + instance ID against prom metrics
type JobByInstance struct {
	ID       string
	Instance string
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
	contractToJobsMap := make(map[string][]JobByInstance)
	mu := &sync.Mutex{}
	g := &errgroup.Group{}
	for i := 0; i < spec.AggregatorsNum; i++ {
		deployFluxInstance(&FluxInstanceDeployment{
			InstanceDeployment: InstanceDeployment{
				Index:   i,
				Suite:   s,
				Spec:    spec,
				Oracles: nodeAddrs,
				Nodes:   clNodes,
				Adapter: adapter,
			},
			FluxInstancesMu:   mu,
			NodesByHostPort:   nodesByHostPort,
			FluxInstances:     &fluxInstances,
			ContractToJobsMap: contractToJobsMap,
		}, g)
	}
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	prom, err := tools.NewPrometheusClient(s.Config.Prometheus)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("Contracts to jobs", contractToJobsMap).Msg("Debug data for per round metrics")
	return &FluxTest{
		Test: Test{
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

// deployFluxInstance deploy one flux instance concurrently, add jobs to all the nodes
func deployFluxInstance(d *FluxInstanceDeployment, g *errgroup.Group) {
	g.Go(func() error {
		log.Info().Int("Instance ID", d.Index).Msg("Deploying contracts instance")
		fluxInstance, err := d.Suite.Deployer.DeployFluxAggregatorContract(d.Suite.Wallets.Default(), d.Spec.FluxOptions)
		if err != nil {
			return err
		}
		err = fluxInstance.Fund(d.Suite.Wallets.Default(), big.NewInt(0), big.NewInt(1e18))
		if err != nil {
			return err
		}
		err = fluxInstance.UpdateAvailableFunds(context.Background(), d.Suite.Wallets.Default())
		if err != nil {
			return err
		}
		// set oracles and submissions
		err = fluxInstance.SetOracles(d.Suite.Wallets.Default(),
			contracts.SetOraclesOptions{
				AddList:            d.Oracles,
				RemoveList:         []common.Address{},
				AdminList:          d.Oracles,
				MinSubmissions:     uint32(d.Spec.RequiredSubmissions),
				MaxSubmissions:     uint32(d.Spec.RequiredSubmissions),
				RestartDelayRounds: uint32(d.Spec.RestartDelayRounds),
			})
		if err != nil {
			return err
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
				return err
			}
			d.FluxInstancesMu.Lock()
			d.NodesByHostPort[n.URL()] = n
			d.ContractToJobsMap[fluxInstance.Address()] = append(d.ContractToJobsMap[fluxInstance.Address()],
				JobByInstance{
					ID:       job.Data.ID,
					Instance: n.URL(),
				})
			d.FluxInstancesMu.Unlock()
		}
		d.FluxInstancesMu.Lock()
		*d.FluxInstances = append(*d.FluxInstances, fluxInstance)
		d.FluxInstancesMu.Unlock()
		return nil
	})
}

// roundsStartTimes gets run start time for every contract, ns
func (vt *FluxTest) roundsStartTimes() (map[string]float64, error) {
	mu := &sync.Mutex{}
	g := &errgroup.Group{}
	contactsStartTimes := make(map[string]float64)
	for contractAddr, jobs := range vt.ContractsToJobsMap {
		contractAddr := contractAddr
		jobs := jobs
		g.Go(func() error {
			startTimesAcrossNodes := make([]int64, 0)
			for _, j := range jobs {
				// get node for a job
				node := vt.NodesByHostPort[j.Instance]
				runs, err := node.ReadRunsByJob(j.ID)
				if err != nil {
					return err
				}
				runsStartTimes := make([]int64, 0)
				for _, r := range runs.Data {
					runsStartTimes = append(runsStartTimes, r.Attributes.CreatedAt.UnixNano()) // milliseconds
				}
				sort.SliceStable(runsStartTimes, func(i, j int) bool {
					return runsStartTimes[i] > runsStartTimes[j]
				})
				lastRunStartTime := runsStartTimes[0]
				startTimesAcrossNodes = append(startTimesAcrossNodes, lastRunStartTime)
			}
			mu.Lock()
			defer mu.Unlock()
			// earliest start across nodes for contract
			sort.SliceStable(startTimesAcrossNodes, func(i, j int) bool {
				return startTimesAcrossNodes[i] < startTimesAcrossNodes[j]
			})
			contactsStartTimes[contractAddr] = float64(startTimesAcrossNodes[0])
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("Round start times", contactsStartTimes).Send()
	return contactsStartTimes, nil
}

// checkRoundSubmissionsEvents checks whether all submissions is found, gets last event block as round end time
func (vt *FluxTest) checkRoundSubmissionsEvents(ctx context.Context, roundID int, submissions int, submissionVal *big.Int) (map[string]float64, error) {
	g, gctx := errgroup.WithContext(ctx)

	mu := &sync.Mutex{}
	endTimes := make(map[string]float64)
	for _, fi := range *vt.FluxInstances {
		fi := fi
		g.Go(func() error {
			events, err := fi.FilterRoundSubmissions(gctx, submissionVal, roundID)
			if err != nil {
				return err
			}
			if len(events) == submissions {
				lastEvent := events[len(events)-1]
				hTime, err := vt.DefaultSetup.Client.HeaderTimestampByNumber(ctx, big.NewInt(lastEvent.BlockNumber()))
				if err != nil {
					return err
				}
				log.Debug().
					Str("Contract", fi.Address()).
					Uint64("Header timestamp ns", hTime*1e9).
					Msg("All submissions found")
				mu.Lock()
				defer mu.Unlock()
				// milliseconds
				endTimes[fi.Address()] = float64(hTime * 1e9)
				return nil
			}
			return errors.New(fmt.Sprintf("not all submissions found for contract: %s", fi.Address()))
		})
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
