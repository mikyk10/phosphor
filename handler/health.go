package handler

import (
	"image"
	"image/color"
	"image/png"
	"net/http"

	"github.com/labstack/echo/v5"
)

type HealthHandler struct{}

func (h HealthHandler) GetHealth(c *echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

// GetTestImage returns a small test PNG to verify image response works.
func (h HealthHandler) GetTestImage(c *echo.Context) error {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for x := range 100 {
		for y := range 100 {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	c.Response().Header().Set("Content-Type", "image/png")
	c.Response().WriteHeader(http.StatusOK)
	return png.Encode(c.Response(), img)
}
