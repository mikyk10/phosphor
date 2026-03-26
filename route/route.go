package route

import (
	"github.com/labstack/echo/v5"
	"github.com/mikyk10/wisp-ai/handler"
)

func Configure(e *echo.Echo) *echo.Echo {
	e.GET("/health", handler.HealthHandler{}.GetHealth)

	// TODO: Phase 3 — image and tag handlers
	// e.GET("/image", imageHandler.Generate)
	// e.POST("/image", imageHandler.Remix)
	// e.POST("/tag", tagHandler.Tag)

	return e
}
