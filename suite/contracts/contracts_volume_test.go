package suite

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/suite"
	"github.com/smartcontractkit/integrations-framework/tools"
	"math/big"
	"strings"
	"sync"
	"time"
)

type VolumeSpec struct {
	AggregatorsNum              int
	JobPrefix                   string
	ObservedValueChangeInterval time.Duration
	NodePollTimePeriod          time.Duration
	InitFunc                    client.BlockchainNetworkInit
	FluxOptions                 contracts.FluxAggregatorOptions
}

func deployInstance(
	index int,
	s *suite.DefaultSuiteSetup,
	spec *VolumeSpec,
	fluxInstances []contracts.FluxAggregator,
	adapter tools.ExternalAdapter,
	mu *sync.Mutex,
	wg *sync.WaitGroup,
) {
	// TODO: wrap with go when concurrent nonce refactoring is ready
	defer wg.Done()
	fluxInstance, err := s.Deployer.DeployFluxAggregatorContract(s.Wallets.Default(), spec.FluxOptions)
	Expect(err).ShouldNot(HaveOccurred())
	err = fluxInstance.Fund(s.Wallets.Default(), big.NewInt(0), big.NewInt(1e18))
	Expect(err).ShouldNot(HaveOccurred())
	err = fluxInstance.UpdateAvailableFunds(context.Background(), s.Wallets.Default())
	Expect(err).ShouldNot(HaveOccurred())
	clNodes, nodeAddrs, err := suite.ConnectToTemplateNodes()
	// TODO: just keep 3 before we have k8s
	oraclesAtTest := nodeAddrs[:3]
	clNodesAtTest := clNodes[:3]
	Expect(err).ShouldNot(HaveOccurred())
	err = suite.FundTemplateNodes(s.Client, s.Wallets, clNodes, 2e18, 0)
	Expect(err).ShouldNot(HaveOccurred())

	// set oracles and submissions
	err = fluxInstance.SetOracles(s.Wallets.Default(),
		contracts.SetOraclesOptions{
			AddList:            oraclesAtTest,
			RemoveList:         []common.Address{},
			AdminList:          oraclesAtTest,
			MinSubmissions:     3,
			MaxSubmissions:     3,
			RestartDelayRounds: 0,
		})
	Expect(err).ShouldNot(HaveOccurred())
	oracles, err := fluxInstance.GetOracles(context.Background())
	Expect(err).ShouldNot(HaveOccurred())
	log.Info().Str("Oracles", strings.Join(oracles, ",")).Msg("oracles set")
	for _, n := range clNodesAtTest {
		fluxSpec := &client.FluxMonitorJobSpec{
			Name:              fmt.Sprintf("%s_%d", spec.JobPrefix, index),
			ContractAddress:   fluxInstance.Address(),
			PollTimerPeriod:   spec.NodePollTimePeriod,
			PollTimerDisabled: false,
			ObservationSource: client.ObservationSourceSpec(adapter.InsideDockerAddr + "/variable"),
		}
		_, err = n.CreateJob(fluxSpec)
		Expect(err).ShouldNot(HaveOccurred())
	}
	mu.Lock()
	fluxInstances = append(fluxInstances, fluxInstance)
	mu.Unlock()
}

// DeployVolumeFlux
// deploys AggregatorsNum aggregators, creates jobs on all nodes for every aggregator
func DeployVolumeFlux(spec *VolumeSpec) (*suite.DefaultSuiteSetup, []contracts.FluxAggregator, tools.ExternalAdapter) {
	s, err := suite.DefaultLocalSetup(spec.InitFunc)
	Expect(err).ShouldNot(HaveOccurred())

	adapter := tools.NewExternalAdapter()

	var fluxInstances []contracts.FluxAggregator
	mu := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	for i := 0; i < spec.AggregatorsNum; i++ {
		wg.Add(1)
		deployInstance(i, s, spec, fluxInstances, adapter, mu, wg)
	}
	wg.Wait()
	return s, fluxInstances, adapter
}

var _ = FDescribe("Volume tests", func() {
	var iteration = 1
	jobPrefix := "flux_monitor"
	s, _, adapter := DeployVolumeFlux(&VolumeSpec{
		AggregatorsNum:              5,
		JobPrefix:                   jobPrefix,
		ObservedValueChangeInterval: 5 * time.Second,
		NodePollTimePeriod:          15 * time.Second,
		InitFunc:                    client.NewHardhatNetwork,
		FluxOptions:                 contracts.DefaultFluxAggregatorOptions(),
	})
	prom := tools.NewPrometheusClient(s.Config.Prometheus)
	Measure("Should respond to N jobs/contracts", func(b Benchmarker) {
		err := adapter.TriggerValueChange(iteration)
		if err != nil {
			log.Fatal().Err(err)
		}
		// TODO: check all contracts have one round completed
		time.Sleep(20 * time.Second)
		summary, err := prom.Summary(jobPrefix)
		if err != nil {
			log.Fatal().Err(err)
		}
		b.RecordValue("sum cpu", summary.CPUPercentage)
		b.RecordValue("sum execution over interval", float64(summary.PipelineExecutionTimeAvgOverIntervalMilliseconds))
		b.RecordValue("sum execution errors", float64(summary.PipelineErrorsSum))
		iteration += 1
	}, 5)
})
