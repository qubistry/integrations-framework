package volume

import (
	"context"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/skudasov/ethlog/ethlog"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/contracts/ethereum"
	"github.com/smartcontractkit/integrations-framework/suite"
	"github.com/smartcontractkit/integrations-framework/tools"
	"golang.org/x/sync/errgroup"
	"math/big"
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
	//until all confirmations found on-chain (block_timestamp)
	roundsDurationData []float64
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

// roundVals structure only for on-chain debug
type roundVals struct {
	RoundID int64
	Val     int64
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
	s.Client.(*client.EthereumClient).BorrowedNonces(true)
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
			EthLog:                  ethlog.NewEthLog(s.Client.(*client.EthereumClient).Client, zerolog.InfoLevel),
		},
		FluxInstances:      &fluxInstances,
		ContractsToJobsMap: contractToJobsMap,
		NodesByHostPort:    nodesByHostPort,
		roundsDurationData: make([]float64, 0),
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

// runInstanceKey to record start timestamp of every run + instance
func (vt *FluxTest) runInstanceKey(inst string, runID string) string {
	return fmt.Sprintf("%s:%s", inst, runID)
}

// roundsStartTimes gets run start time for every contract, adds to already seen runs
func (vt *FluxTest) roundsStartTimes() (map[string]int64, error) {
	contactsStartTimes := make(map[string]int64)
	for contractAddr, jobs := range vt.ContractsToJobsMap {
		startTimesForContract := make([]int64, 0)
		for _, j := range jobs {
			node := vt.NodesByHostPort[j.Instance]
			runs, err := node.ReadRunsByJob(j.ID)
			if err != nil {
				return nil, err
			}
			for _, r := range runs.Data {
				key := vt.runInstanceKey(j.Instance, r.ID)
				if _, ok := vt.AlreadySeenRuns[key]; ok {
					continue
				}
				vt.AlreadySeenRuns[key] = true
				// milliseconds
				startTimesForContract = append(startTimesForContract, r.Attributes.CreatedAt.UnixNano()/1e6)
			}
		}
		contactsStartTimes[contractAddr] = minInt64Slice(startTimesForContract)
	}
	log.Debug().Interface("Round start times", contactsStartTimes).Send()
	return contactsStartTimes, nil
}

// roundsMetrics get start times from runs via API, count submissions on-chain and get block time when round ends
func (vt *FluxTest) roundsMetrics(fromBlock *big.Int, toBlock *big.Int, roundID int, submissions int) error {
	startTimes, err := vt.roundsStartTimes()
	if err != nil {
		return err
	}
	hr, err := vt.getOnChainLogs(fromBlock, toBlock)
	if err != nil {
		return err
	}
	endTimes, err := vt.roundEndTimes(hr, roundID, submissions)
	if err != nil {
		return err
	}
	for contract := range startTimes {
		duration := endTimes[contract] - startTimes[contract]
		vt.roundsDurationData = append(vt.roundsDurationData, float64(duration))
		log.Info().Str("Contract", contract).Int64("Round duration ms", duration).Send()
	}
	return nil
}

func (vt *FluxTest) awaitRoundFinishedOnChain(roundID int, newVal int) error {
	if err := retry.Do(func() error {
		finished, err := vt.checkRoundFinishedOnChain(roundID, newVal)
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

func (vt *FluxTest) debugRoundTuple(rounds []*contracts.FluxAggregatorData) {
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
	log.Debug().Interface("Rounds values on chain", roundValsArr).Msg("Last rounds on chain")
}

func (vt *FluxTest) checkRoundFinishedOnChain(roundID int, newVal int) (bool, error) {
	log.Debug().Int("Round ID", roundID).Msg("Checking round completion on chain")
	var rounds []*contracts.FluxAggregatorData
	for _, flux := range *vt.FluxInstances {
		cd, err := flux.GetContractData(context.Background())
		if err != nil {
			return false, err
		}
		rounds = append(rounds, cd)
	}
	vt.debugRoundTuple(rounds)
	for _, r := range rounds {
		if r.LatestRoundData.RoundId.Int64() != int64(roundID) || r.LatestRoundData.Answer.Int64() != int64(newVal) {
			return false, nil
		}
	}
	return true, nil
}

// roundEndTimes counts submissions for round, when all required submission is found, saves last block_timestamp for contract
func (vt *FluxTest) roundEndTimes(hr *ethlog.HistoryResult, expectedRound int, submitsRequired int) (map[string]int64, error) {
	endTimesMap := make(map[string]int64)
	submitsMap := make(map[string]int)
	for _, block := range hr.History {
		if len(block["transactions"].([]ethlog.ParsedTx)) == 0 {
			continue
		}
		// for every tx search event with matching round id, happened in particular contract
		for _, tx := range block["transactions"].([]ethlog.ParsedTx) {
			for _, event := range tx["events"].([]ethlog.ParsedEvent) {
				data := event["event_data"].(ethlog.RawParsedEventData)
				if data["round"] == nil {
					continue
				}
				addr := event["event_address"].(common.Address)
				// identifying submission by "round" field
				if int(data["round"].(uint32)) == expectedRound {
					submitsMap[addr.Hex()] += 1
				}
				if submitsMap[addr.Hex()] > submitsRequired {
					return nil, errors.New(fmt.Sprintf("more that required submits found for contract: %s", addr.Hex()))
				}
				if submitsMap[addr.Hex()] == submitsRequired {
					// milliseconds
					endTimesMap[addr.Hex()] = int64(block["block_time"].(uint64) * 1000)
				}
			}
		}
	}
	log.Debug().Interface("Round end block times", endTimesMap).Send()
	return endTimesMap, nil
}

// getOnChainLogs get all block data for test interval
func (vt *FluxTest) getOnChainLogs(from *big.Int, to *big.Int) (*ethlog.HistoryResult, error) {
	contractSearchData := make([]ethlog.ContractData, 0)
	for contractAddr := range vt.ContractsToJobsMap {
		contractSearchData = append(contractSearchData, ethlog.ContractData{
			Name:    "flux_aggregator",
			ABI:     ethereum.FluxAggregatorABI,
			Address: common.HexToAddress(contractAddr),
		})
	}
	bhConfig := &ethlog.BlockHistoryConfig{
		FromBlock:     from,
		ToBlock:       to,
		Format:        ethlog.FormatYAML,
		Rewrite:       true,
		ContractsData: contractSearchData,
	}
	blocksHistory, err := vt.EthLog.RequestBlocksHistory(bhConfig)
	if err != nil {
		return nil, err
	}
	return blocksHistory, nil
}
