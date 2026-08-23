package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/handler"
	"github.com/mikyk10/phosphor/metrics"
	"github.com/stretchr/testify/require"
)

func TestMetricsEndpointExposesPipelineMetrics(t *testing.T) {
	metrics.ObservePipeline("demo", metrics.StatusSuccess, 1.5)
	metrics.ObservePipeline("demo", metrics.StatusFailed, 0.25)
	metrics.ObserveStageFailure("render", "execution_failed")

	e := echo.New()
	Configure(e, handler.NewPipelineHandler(&stubUsecase{}, 1<<20), "")

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	require.Contains(t, body, `phosphor_pipeline_executions_total{pipeline="demo",status="success"} 1`)
	require.Contains(t, body, `phosphor_pipeline_executions_total{pipeline="demo",status="failed"} 1`)
	require.Contains(t, body, `phosphor_pipeline_duration_seconds_count{pipeline="demo"} 2`)
	require.Contains(t, body, `phosphor_stage_failures_total{error_code="execution_failed",stage="render"} 1`)

	// The default registry also carries the Go runtime collectors.
	require.Contains(t, body, "go_goroutines")
}

// /metrics is an operational endpoint: a scraper has no pipeline token, so it
// must stay reachable when auth is enabled.
func TestMetricsEndpointIsNotBehindAuth(t *testing.T) {
	e := echo.New()
	Configure(e, handler.NewPipelineHandler(&stubUsecase{}, 1<<20), "s3cret")

	for _, path := range []string{"/metrics", "/health"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, "%s must not require the token", path)
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pipeline/demo", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code, "pipeline routes stay protected")
}
