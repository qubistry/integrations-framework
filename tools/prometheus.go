package tools

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/integrations-framework/config"
	"time"
)

const (
	QueryMemoryUsage          = `100 * (1 - ((avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s])) / avg_over_time(node_memory_MemTotal_bytes[%s])))`
	QueryAllCPUBusyPercentage = `100 - (avg by (instance) (irate(node_cpu_seconds_total{job="%s",mode="idle"}[%s])) * 100)`
)

type ResourcesSummary struct {
	MemoryUsage   float64
	CPUPercentage float64
}

type PromChecker struct {
	API v1.API
	Cfg *config.PrometheusClientConfig
}

func NewPrometheusClient(cfg *config.PrometheusClientConfig) (*PromChecker, error) {
	client, err := api.NewClient(api.Config{
		Address: cfg.Url,
	})
	if err != nil {
		return nil, err
	}
	return &PromChecker{
		API: v1.NewAPI(client),
		Cfg: cfg,
	}, nil
}

func (p *PromChecker) printWarns(warns v1.Warnings) {
	if len(warns) > 0 {
		log.Info().Interface("Warnings", warns).Msg("Warnings found when performing prometheus query")
	}
}

func (p *PromChecker) validateNotEmptyVec(q string, val model.Value) bool {
	if len(val.(model.Vector)) == 0 {
		log.Warn().Str("query", q).Msg("empty response for prometheus query")
		return false
	}
	return true
}

// CPUBusyPercentage host CPU busy percentage
func (p *PromChecker) CPUBusyPercentage() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryAllCPUBusyPercentage, p.Cfg.ScrapeJobName, p.Cfg.TestAggregationInterval)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	p.printWarns(warns)
	if !p.validateNotEmptyVec(q, val) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return float64(scalarVal), nil
}

// MemoryUsage total memory used by interval
func (p *PromChecker) MemoryUsage() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	i := p.Cfg.TestAggregationInterval
	q := fmt.Sprintf(QueryMemoryUsage, i, i, i, i)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	p.printWarns(warns)
	if !p.validateNotEmptyVec(q, val) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return float64(scalarVal), nil
}

func (p *PromChecker) ResourcesSummary() (*ResourcesSummary, error) {
	cpu, err := p.CPUBusyPercentage()
	if err != nil {
		return nil, err
	}
	mem, err := p.MemoryUsage()
	if err != nil {
		return nil, err
	}
	return &ResourcesSummary{
		CPUPercentage: cpu,
		MemoryUsage:   mem,
	}, nil
}
