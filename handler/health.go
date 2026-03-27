package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type HealthHandler struct{}

func (h HealthHandler) GetHealth(c *echo.Context) error {
	return c.String(http.StatusOK, "OK")
}
