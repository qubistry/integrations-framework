package volume

import (
	"github.com/avast/retry-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/suite/common"
	"github.com/smartcontractkit/integrations-framework/tools"
	"math/big"
	"time"
)

var _ = Describe("Flux monitor volume tests", func() {
	Describe("round completion times", func() {
		jobPrefix := "flux_monitor"
		spec := &FluxTestSpec{
			TestSpec: common.TestSpec{
				InitFunc:                client.NewHardhatNetwork,
				OnChainCheckAttemptsOpt: retry.Attempts(120),
			},
			AggregatorsNum:      1,
			RequiredSubmissions: 5,
			RestartDelayRounds:  0,
			JobPrefix:           jobPrefix,
			NodePollTimePeriod:  15 * time.Second,
			FluxOptions:         contracts.DefaultFluxAggregatorOptions(),
		}
		ft, err := NewFluxTest(spec)
		Expect(err).ShouldNot(HaveOccurred())

		currentRound := 1
		rounds := 5

		err = ft.checkRoundDataOnChain(currentRound, tools.VariableData)
		Expect(err).ShouldNot(HaveOccurred())

		Measure("Round completion time percentiles", func(b Benchmarker) {
			newVal, err := ft.Adapter.TriggerValueChange(currentRound)
			Expect(err).ShouldNot(HaveOccurred())

			err = ft.checkRoundDataOnChain(currentRound+1, newVal)
			Expect(err).ShouldNot(HaveOccurred())

			err = ft.roundsMetrics(
				currentRound+1,
				spec.RequiredSubmissions,
				big.NewInt(int64(newVal)),
			)
			Expect(err).ShouldNot(HaveOccurred())

			cpu, mem, err := ft.Prom.ResourcesSummary()
			Expect(err).ShouldNot(HaveOccurred())
			b.RecordValue("Sum CPU", cpu)
			b.RecordValue("Sum MEM", mem)
			currentRound += 1
		}, rounds)

		AfterSuite(func() {
			percs, err := ft.CalculatePercentiles(ft.roundsDurationData)
			Expect(err).ShouldNot(HaveOccurred())
			ft.PrintMetrics(percs)
		})
	})
})
