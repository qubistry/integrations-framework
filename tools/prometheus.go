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
	QueryNodesLastReportedRounds              = `flux_monitor_reported_round{job="%s",job_spec_id=~".*"}`
	QueryAllCPUBusyPercentage                 = `100 - (avg by (instance) (irate(node_cpu_seconds_total{job="%s",mode="idle"}[%s])) * 100)`
)

type FluxRoundMetrics struct {
	Instance string
	JobID    int
	Value    int
}

type FluxRoundSummary struct {
	PipelineExecutionTimeAvgOverIntervalMilliseconds int64
	PipelineErrorsSum                                int64
	CPUPercentage                                    float64
}

type PromChecker struct {
	API v1.API
	Cfg *config.PrometheusClientConfig
}

func NewPrometheusClient(cfg *config.PrometheusClientConfig) *PromChecker {
	client, err := api.NewClient(api.Config{
		Address: cfg.Url,
	})
	if err != nil {
		log.Fatal().Err(err)
	}
	return &PromChecker{
		API: v1.NewAPI(client),
		Cfg: cfg,
	}
}

func (p *PromChecker) debugRoundMetrics(vec model.Vector) {
	var metrics []FluxRoundMetrics
	for _, m := range vec {
		metrics = append(metrics, FluxRoundMetrics{
			Instance: string(m.Metric["instance"]),
			Value:    int(m.Value),
		})
	}
	log.Debug().Interface("last_round_metrics", metrics).Msg("new round reached")
}

// AwaitRoundFinishedAcrossNodes waits for all nodes to report next round
func (p *PromChecker) AwaitRoundFinishedAcrossNodes(ctx context.Context, roundID int) (bool, error) {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("timeout awaiting new round on all nodes")
			return false, nil
		case <-ticker.C:
			lastReportedRounds, err := p.NodesLastReportedRounds()
			if err != nil {
				return false, err
			}
			tryAgain := false
			log.Info().Int("round_id", roundID).Msg("awaiting prometheus metrics")
			for _, v := range lastReportedRounds {
				if int(v.Value) != roundID {
					tryAgain = true
					break
				}
			}
			if tryAgain {
				continue
			}
			p.debugRoundMetrics(lastReportedRounds)
			return true, nil
		}
	}
}

func (p *PromChecker) NodesLastReportedRounds() (model.Vector, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	// TODO: no job_name, ex. "flux monitor in labels", need to add or aggregate over job ids
	q := fmt.Sprintf(QueryNodesLastReportedRounds, p.Cfg.ScrapeJobName)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return nil, err
	}
	p.printWarns(warns)
	if !p.validateNotEmptyVec(q, val) {
		return nil, nil
	}
	return val.(model.Vector), nil
}

func (p *PromChecker) toMs(val model.SampleValue) int64 {
	return time.Duration(int64(val)).Milliseconds()
}

func (p *PromChecker) printWarns(warns v1.Warnings) {
	if len(warns) > 0 {
		log.Info().Interface("warnings", warns).Msg("warnings found when performing prometheus query")
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

// PipelineErrorsSum sum all errors across nodes and jobs
func (p *PromChecker) PipelineErrorsSum(jobPrefix string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryPipelineSumErrors, p.Cfg.ScrapeJobName, jobPrefix)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	p.printWarns(warns)
	if len(val.(model.Vector)) == 0 {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return int64(scalarVal), nil
}

// PipelineExecutionTimeAvgNow average of total execution time over all pipelines now
func (p *PromChecker) PipelineExecutionTimeAvgNow(jobPrefix string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryPipelineExecutionTimeAvgNow, p.Cfg.ScrapeJobName, jobPrefix)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	p.printWarns(warns)
	if !p.validateNotEmptyVec(q, val) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return p.toMs(scalarVal), nil
}

func (p *PromChecker) FluxRoundSummary(jobPrefix string) (*FluxRoundSummary, error) {
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
func (p *PromChecker) PipelineExecutionTimeAvgOverInterval(jobPrefix string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Cfg.QueryTimeout)
	defer cancel()
	q := fmt.Sprintf(QueryPipelineExecutionTimeAvgOverInterval, p.Cfg.ScrapeJobName, jobPrefix, p.Cfg.TestAggregationInterval)
	val, warns, err := p.API.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	p.printWarns(warns)
	if !p.validateNotEmptyVec(q, val) {
		return 0, nil
	}
	scalarVal := val.(model.Vector)[0].Value
	return p.toMs(scalarVal), nil
}
