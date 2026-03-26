package route

import (
	"github.com/labstack/echo/v5"
	"github.com/mikyk10/wisp-ai/handler"
)

func Configure(e *echo.Echo, ph *handler.PipelineHandler) *echo.Echo {
	e.GET("/health", handler.HealthHandler{}.GetHealth)
	e.GET("/test.png", handler.HealthHandler{}.GetTestImage)
	e.GET("/pipeline/:name", ph.Run)
	e.POST("/pipeline/:name", ph.Run)
	return e
}
