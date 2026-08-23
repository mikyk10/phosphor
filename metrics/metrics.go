// Package metrics holds the Prometheus instrumentation for pipeline
// execution. Everything is registered on the default registry, so
// promhttp.Handler() exposes it.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "phosphor"

var (
	// PipelineExecutions counts finished pipeline runs by outcome.
	PipelineExecutions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "pipeline_executions_total",
		Help:      "Total number of pipeline executions, by pipeline and outcome.",
	}, []string{"pipeline", "status"})

	// PipelineDuration observes how long a pipeline took end to end.
	// A run chains several LLM calls, so the range spans sub-second
	// failures to multi-minute image generation.
	PipelineDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "pipeline_duration_seconds",
		Help:      "Pipeline execution duration in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 12), // 0.1s .. ~3.4min
	}, []string{"pipeline"})

	// StageFailures counts individual stage failures by cause.
	StageFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "stage_failures_total",
		Help:      "Total number of stage failures, by stage and error code.",
	}, []string{"stage", "error_code"})
)

// Status label values for PipelineExecutions.
const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// ObservePipeline records the outcome and duration of one pipeline run.
func ObservePipeline(pipeline, status string, seconds float64) {
	PipelineExecutions.WithLabelValues(pipeline, status).Inc()
	PipelineDuration.WithLabelValues(pipeline).Observe(seconds)
}

// ObserveStageFailure records one failed stage.
func ObserveStageFailure(stage, errorCode string) {
	StageFailures.WithLabelValues(stage, errorCode).Inc()
}
