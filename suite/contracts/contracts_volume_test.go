package suite

//
//import (
//	"context"
//	"fmt"
//	"github.com/ethereum/go-ethereum/common"
//	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
//	"github.com/rs/zerolog/log"
//	"github.com/smartcontractkit/integrations-framework/client"
//	"github.com/smartcontractkit/integrations-framework/contracts"
//	"github.com/smartcontractkit/integrations-framework/suite"
//	"github.com/smartcontractkit/integrations-framework/tools"
//	"math/big"
//	"strings"
//	"sync"
//	"time"
//)
//
//type VolumeSpec struct {
//	AggregatorsNum              int
//	JobPrefix                   string
//	ObservedValueChangeInterval time.Duration
//	NodePollTimePeriod          time.Duration
//	InitFunc                    client.BlockchainNetworkInit
//	FluxOptions                 contracts.FluxAggregatorOptions
//}
//
//type InstanceDeployment struct {
//	Wg              *sync.WaitGroup
//	Index           int
//	Suite           *suite.DefaultSuiteSetup
//	Spec            *VolumeSpec
//	Oracles         []common.Address
//	Nodes           []client.Chainlink
//	Adapter         tools.ExternalAdapter
//	FluxInstancesMu *sync.Mutex
//	FluxInstances   []contracts.FluxAggregator
//}
//
//func deployInstance(d *InstanceDeployment) {
//	go func() {
//		defer d.Wg.Done()
//		log.Info().Int("instance_id", d.Index).Msg("deploying contracts instance")
//		fluxInstance, err := d.Suite.Deployer.DeployFluxAggregatorContract(d.Suite.Wallets.Default(), d.Spec.FluxOptions)
//		log.Error().Err(err).Msg("fatal error parallel deployment")
//		Expect(err).ShouldNot(HaveOccurred())
//		err = fluxInstance.Fund(d.Suite.Wallets.Default(), big.NewInt(0), big.NewInt(1e18))
//		Expect(err).ShouldNot(HaveOccurred())
//		err = fluxInstance.UpdateAvailableFunds(context.Background(), d.Suite.Wallets.Default())
//		Expect(err).ShouldNot(HaveOccurred())
//
//		// set oracles and submissions
//		err = fluxInstance.SetOracles(d.Suite.Wallets.Default(),
//			contracts.SetOraclesOptions{
//				AddList:            d.Oracles,
//				RemoveList:         []common.Address{},
//				AdminList:          d.Oracles,
//				MinSubmissions:     3,
//				MaxSubmissions:     3,
//				RestartDelayRounds: 0,
//			})
//		Expect(err).ShouldNot(HaveOccurred())
//		oracles, err := fluxInstance.GetOracles(context.Background())
//		Expect(err).ShouldNot(HaveOccurred())
//		log.Info().Str("Oracles", strings.Join(oracles, ",")).Msg("oracles set")
//		for _, n := range d.Nodes {
//			fluxSpec := &client.FluxMonitorJobSpec{
//				Name:              fmt.Sprintf("%s_%d", d.Spec.JobPrefix, d.Index),
//				ContractAddress:   fluxInstance.Address(),
//				PollTimerPeriod:   d.Spec.NodePollTimePeriod,
//				PollTimerDisabled: false,
//				ObservationSource: client.ObservationSourceSpec(d.Adapter.InsideDockerAddr + "/variable"),
//			}
//			_, err = n.CreateJob(fluxSpec)
//			Expect(err).ShouldNot(HaveOccurred())
//		}
//		d.FluxInstancesMu.Lock()
//		d.FluxInstances = append(d.FluxInstances, fluxInstance)
//		d.FluxInstancesMu.Unlock()
//	}()
//}
//
//// DeployVolumeFlux deploys AggregatorsNum aggregators, creates jobs on all nodes for every aggregator
//func DeployVolumeFlux(spec *VolumeSpec) (*suite.DefaultSuiteSetup, []contracts.FluxAggregator, tools.ExternalAdapter) {
//	s, err := suite.DefaultLocalSetup(spec.InitFunc)
//	Expect(err).ShouldNot(HaveOccurred())
//
//	clNodes, nodeAddrs, err := suite.ConnectToTemplateNodes()
//	// TODO: just keep 3 before we have k8s
//	oraclesAtTest := nodeAddrs[:3]
//	clNodesAtTest := clNodes[:3]
//	Expect(err).ShouldNot(HaveOccurred())
//	err = suite.FundTemplateNodes(s.Client, s.Wallets, clNodes, 2e18, 0)
//	Expect(err).ShouldNot(HaveOccurred())
//
//	s.Client.(*client.EthereumClient).BorrowedNonces(true)
//
//	adapter := tools.NewExternalAdapter()
//
//	var fluxInstances []contracts.FluxAggregator
//	mu := &sync.Mutex{}
//	wg := &sync.WaitGroup{}
//	for i := 0; i < spec.AggregatorsNum; i++ {
//		wg.Add(1)
//		deployInstance(&InstanceDeployment{
//			Wg:              wg,
//			Index:           i,
//			Suite:           s,
//			Spec:            spec,
//			Oracles:         oraclesAtTest,
//			Nodes:           clNodesAtTest,
//			Adapter:         adapter,
//			FluxInstancesMu: mu,
//			FluxInstances:   fluxInstances,
//		})
//	}
//	wg.Wait()
//	return s, fluxInstances, adapter
//}
//
//var _ = FDescribe("Volume tests", func() {
//	var iteration = 1
//	jobPrefix := "flux_monitor"
//	s, _, adapter := DeployVolumeFlux(&VolumeSpec{
//		AggregatorsNum:              20,
//		JobPrefix:                   jobPrefix,
//		NodePollTimePeriod:          15 * time.Second,
//		InitFunc:                    client.NewHardhatNetwork,
//		FluxOptions:                 contracts.DefaultFluxAggregatorOptions(),
//	})
//	prom := tools.NewPrometheusClient(s.Config.Prometheus)
//	Measure("Should process one round with 100 contracts, 100 submits per node", func(b Benchmarker) {
//		// all contracts/nodes/jobs are setup at that point, triggering new round,
//		// waiting for all contracts to complete one round and get metrics
//		err := adapter.TriggerValueChange(iteration)
//		if err != nil {
//			log.Fatal().Err(err)
//		}
//		// TODO: check all contracts have one round completed, get interval for prom aggregation
//		time.Sleep(20 * time.Second)
//		summary, err := prom.FluxRoundSummary(jobPrefix)
//		if err != nil {
//			log.Fatal().Err(err).Msg("failed to get prom summary")
//		}
//		b.RecordValue("sum cpu", summary.CPUPercentage)
//		b.RecordValue("sum pipeline execution time over interval",
//			float64(summary.PipelineExecutionTimeAvgOverIntervalMilliseconds))
//		b.RecordValue("sum execution errors", float64(summary.PipelineErrorsSum))
//		iteration += 1
//	}, 5)
//})
