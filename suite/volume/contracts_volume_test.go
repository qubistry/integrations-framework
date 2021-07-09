package volume

import (
	"context"
	"github.com/avast/retry-go"
	. "github.com/onsi/ginkgo"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/contracts"
	"github.com/smartcontractkit/integrations-framework/tools"
	"time"
)

var _ = Describe("Flux monitor volume tests", func() {
	jobPrefix := "flux_monitor"
	vt := NewVolumeFluxTest(&VolumeTestSpec{
		AggregatorsNum:          100,
		JobPrefix:               jobPrefix,
		NodePollTimePeriod:      30 * time.Second,
		InitFunc:                client.NewHardhatNetwork,
		FluxOptions:             contracts.DefaultFluxAggregatorOptions(),
		OnChainCheckAttemptsOpt: retry.Attempts(120),
	})
	Describe("round completion times", func() {
		promRoundTimeout := 120 * time.Second
		currentRound := 1
		rounds := 30
		// just wait for the first round to settle about initial data
		vt.AwaitRoundFinishedOnChain(currentRound, tools.VariableData)

		Measure("Should process rounds without errors", func(b Benchmarker) {
			// all contracts/nodes/jobs are setup at that point, triggering new round,
			// waiting for all contracts to complete one round and get metrics
			newVal, err := vt.Adapter.TriggerValueChange(currentRound)
			if err != nil {
				log.Fatal().Err(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), promRoundTimeout)
			defer cancel()

			// await all nodes report round was finished
			roundFinished, err := vt.Prom.AwaitRoundFinishedAcrossNodes(ctx, currentRound+1)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to check round consistency")
			}
			if !roundFinished {
				log.Fatal().Msg("round was not finished in time")
			}

			// await all round data is on chain
			vt.AwaitRoundFinishedOnChain(currentRound+1, newVal)
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
			currentRound += 1
		}, rounds)
	})
})
