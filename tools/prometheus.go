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
	QueryPipelineExecutionTimeAvgNow          = `avg(pipeline_run_total_time_to_completion{job="%s", job_name=~"%s.*"})`
	QueryPipelineExecutionTimeAvgOverInterval = `avg(avg_over_time(pipeline_run_total_time_to_completion{job="%s", job_name=~"%s.*"}[%s]))`
	QueryPipelineSumErrors                    = `sum(pipeline_run_errors{job="%s", job_name=~"%s.*"})`
	QueryAllCPUBusyPercentage                 = `100 - (avg by (instance) (irate(node_cpu_seconds_total{job="%s",mode="idle"}[%s])) * 100)`
)

type FluxRoundSummary struct {
	PipelineExecutionTimeAvgOverIntervalMilliseconds int64
	PipelineErrorsSum                                int64
	CPUPercentage                                    float64
}

type PromQueries struct {
	API v1.API
	Cfg *config.PrometheusClientConfig
}

func NewPrometheusClient(cfg *config.PrometheusClientConfig) *PromQueries {
	client, err := api.NewClient(api.Config{
		Address: cfg.Url,
	})
	if err != nil {
		log.Fatal().Err(err)
	}
	return &PromQueries{
		API: v1.NewAPI(client),
		Cfg: cfg,
	}
}

func (p *PromQueries) toMs(val model.SampleValue) int64 {
	return time.Duration(int64(val)).Milliseconds()
}

func (p *PromQueries) validate(q string, val model.Value, warns v1.Warnings) bool {
	if len(warns) > 0 {
		log.Info().Interface("warnings", warns).Msg("warnings found when performing prometheus query")
	}
	if len(val.(model.Vector)) == 0 {
		log.Warn().Str("query", q).Msg("empty response for prometheus query")
		return false
	}
	return true
}

// CPUBusyPercentage host CPU busy percentage
func (p *PromQueries) CPUBusyPercentage() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryAllCPUBusyPercentage, p.Cfg.ScrapeJobName, p.Cfg.TestAggregationInterval)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	if !p.validate(q, val, warns) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return float64(scalarVal), nil
}

// PipelineErrorsSum sum all errors across nodes and jobs
func (p *PromQueries) PipelineErrorsSum(jobPrefix string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryPipelineSumErrors, p.Cfg.ScrapeJobName, jobPrefix)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	if !p.validate(q, val, warns) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return int64(scalarVal), nil
}

// PipelineExecutionTimeAvgNow average of total execution time over all pipelines now
func (p *PromQueries) PipelineExecutionTimeAvgNow(jobPrefix string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryPipelineExecutionTimeAvgNow, p.Cfg.ScrapeJobName, jobPrefix)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	if !p.validate(q, val, warns) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return p.toMs(scalarVal), nil
}

func (p *PromQueries) FluxRoundSummary(jobPrefix string) (*FluxRoundSummary, error) {
	cpu, err := p.CPUBusyPercentage()
	if err != nil {
		return nil, err
	}
	avgExec, err := p.PipelineExecutionTimeAvgOverInterval(jobPrefix)
	if err != nil {
		return nil, err
	}
	avgErr, err := p.PipelineErrorsSum(jobPrefix)
	if err != nil {
		return nil, err
	}
	return &FluxRoundSummary{
		PipelineExecutionTimeAvgOverIntervalMilliseconds: avgExec,
		PipelineErrorsSum: avgErr,
		CPUPercentage:     cpu,
	}, nil
}

// PipelineExecutionTimeAvgOverInterval average of total execution time over all pipelines in aggregation (test) interval
func (p *PromQueries) PipelineExecutionTimeAvgOverInterval(jobPrefix string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryPipelineExecutionTimeAvgOverInterval, p.Cfg.ScrapeJobName, jobPrefix, p.Cfg.TestAggregationInterval)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	if !p.validate(q, val, warns) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return p.toMs(scalarVal), nil
}
