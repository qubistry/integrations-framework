package volume

import (
	"context"
	"github.com/avast/retry-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/tools"
	"math/big"
	"time"
)

var _ = Describe("Flux monitor volume tests", func() {
	jobPrefix := "flux_monitor"
	spec := &FluxTestSpec{
		TestSpec: TestSpec{
			InitFunc:                client.NewHardhatNetwork,
			OnChainCheckAttemptsOpt: retry.Attempts(120),
		},
		AggregatorsNum:      20,
		RequiredSubmissions: 5,
		RestartDelayRounds:  0,
		JobPrefix:           jobPrefix,
		NodePollTimePeriod:  15 * time.Second,
		FluxOptions:         contracts.DefaultFluxAggregatorOptions(),
	}
	ft, err := NewFluxTest(spec)
	Expect(err).ShouldNot(HaveOccurred())
	Describe("round completion times", func() {
		currentRound := 1
		rounds := 5
		// just wait for the first round to settle about initial data
		err := ft.awaitRoundFinishedOnChain(currentRound, tools.VariableData)
		Expect(err).ShouldNot(HaveOccurred())
		_, err = ft.roundsStartTimes()
		Expect(err).ShouldNot(HaveOccurred())

		Measure("Should process rounds without errors", func(b Benchmarker) {
			blockStart, err := ft.DefaultSetup.Client.BlockNumber(context.Background())
			Expect(err).ShouldNot(HaveOccurred())

			newVal, err := ft.Adapter.TriggerValueChange(currentRound)
			Expect(err).ShouldNot(HaveOccurred())

			err = ft.awaitRoundFinishedOnChain(currentRound+1, newVal)
			Expect(err).ShouldNot(HaveOccurred())

			blockEnd, err := ft.DefaultSetup.Client.BlockNumber(context.Background())
			Expect(err).ShouldNot(HaveOccurred())

			err = ft.roundsMetrics(
				big.NewInt(int64(blockStart)),
				big.NewInt(int64(blockEnd+1)),
				currentRound+1,
				spec.RequiredSubmissions,
			)
			Expect(err).ShouldNot(HaveOccurred())
			summary, err := ft.Prom.FluxRoundSummary(jobPrefix)
			Expect(err).ShouldNot(HaveOccurred())
			// record benchmark metrics
			b.RecordValue("sum cpu", summary.CPUPercentage)
			b.RecordValue("sum pipeline execution time over interval",
				float64(summary.PipelineExecutionTimeAvgOverIntervalMilliseconds))
			b.RecordValue("sum execution errors", float64(summary.PipelineErrorsSum))
			currentRound += 1
		}, rounds)
		AfterSuite(func() {
			percs, err := ft.CalculatePercentiles(ft.roundsDurationData)
			Expect(err).ShouldNot(HaveOccurred())
			ft.reportMetrics(percs)
		})
	})
})
