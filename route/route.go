package route

import (
	"github.com/labstack/echo/v5"
	"github.com/mikyk10/wisp-ai/handler"
)

func Configure(e *echo.Echo, imgHandler *handler.ImageHandler, tagHandler *handler.TagHandler) *echo.Echo {
	e.GET("/health", handler.HealthHandler{}.GetHealth)
	e.GET("/test.png", handler.HealthHandler{}.GetTestImage)
	e.GET("/image", imgHandler.Generate)
	e.POST("/image", imgHandler.Remix)
	e.POST("/tag", tagHandler.Tag)
	return e
}
