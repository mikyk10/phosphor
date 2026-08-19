package route

import (
	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/handler"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Configure(e *echo.Echo, ph *handler.PipelineHandler) *echo.Echo {
	e.GET("/health", handler.HealthHandler{}.GetHealth)
	e.GET("/metrics", metricsHandler())
	e.GET("/pipeline/:name", ph.Run)
	e.POST("/pipeline/:name", ph.Run)
	return e
}

// metricsHandler exposes the Prometheus default registry. Like /health it is
// an operational endpoint and carries no pipeline data.
func metricsHandler() echo.HandlerFunc {
	h := promhttp.Handler()
	return func(c *echo.Context) error {
		h.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
