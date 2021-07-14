package volume

import (
	"github.com/avast/retry-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/montanaflynn/stats"
	"github.com/rs/zerolog/log"
	"github.com/skudasov/ethlog/ethlog"
	"github.com/smartcontractkit/integrations-framework/client"
	"github.com/smartcontractkit/integrations-framework/suite"
	"github.com/smartcontractkit/integrations-framework/tools"
	"sync"
)

// TestSpec common volume test spec
type TestSpec struct {
	InitFunc                client.BlockchainNetworkInit
	OnChainCheckAttemptsOpt func(config *retry.Config)
}

// InstanceDeployment common deploy data for one contract deployed concurrently
type InstanceDeployment struct {
	Wg      *sync.WaitGroup
	Index   int
	Suite   *suite.DefaultSuiteSetup
	Spec    *FluxTestSpec
	Oracles []common.Address
	Nodes   []client.Chainlink
	Adapter tools.ExternalAdapter
}

// Test common test runtime instance
type Test struct {
	DefaultSetup            *suite.DefaultSuiteSetup
	OnChainCheckAttemptsOpt func(config *retry.Config)
	Nodes                   []client.Chainlink
	Adapter                 tools.ExternalAdapter
	Prom                    *tools.PromChecker
	EthLog                  *ethlog.EthLog
}

// PercentileReport common percentile report
type PercentileReport struct {
	StdDev float64
	Max    float64
	P99    float64
	P95    float64
	P90    float64
	P50    float64
}

func (t *Test) reportMetrics(m *PercentileReport) {
	log.Info().Float64("round_duration_ms_MAX", m.Max).Send()
	log.Info().Float64("round_duration_ms_P99", m.P99).Send()
	log.Info().Float64("round_duration_ms_P95", m.P95).Send()
	log.Info().Float64("round_duration_ms_P90", m.P90).Send()
	log.Info().Float64("round_duration_ms_P50", m.P50).Send()
	log.Info().Float64("round_duration_ms_std_dev", m.StdDev).Send()
}

// CalculatePercentiles calculates percentiles for arbitrary float64 data
func (t *Test) CalculatePercentiles(data []float64) (*PercentileReport, error) {
	perc99, err := stats.Percentile(data, 99)
	if err != nil {
		return nil, err
	}
	perc95, err := stats.Percentile(data, 95)
	if err != nil {
		return nil, err
	}
	perc90, err := stats.Percentile(data, 90)
	if err != nil {
		return nil, err
	}
	perc50, err := stats.Percentile(data, 50)
	if err != nil {
		return nil, err
	}
	max, err := stats.Max(data)
	if err != nil {
		return nil, err
	}
	stdDev, err := stats.StandardDeviation(data)
	if err != nil {
		return nil, err
	}
	return &PercentileReport{P99: perc99, P95: perc95, P90: perc90, P50: perc50, Max: max, StdDev: stdDev}, nil
}

func minInt64Slice(v []int64) (m int64) {
	if len(v) > 0 {
		m = v[0]
	}
	for i := 1; i < len(v); i++ {
		if v[i] < m {
			m = v[i]
		}
	}
	return
}
