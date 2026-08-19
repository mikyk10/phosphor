package route

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/handler"
)

// Configure registers the HTTP routes. When token is non-empty the pipeline
// endpoints require it; /health stays open so container health checks and
// load balancers keep working.
func Configure(e *echo.Echo, ph *handler.PipelineHandler, token string) *echo.Echo {
	e.GET("/health", handler.HealthHandler{}.GetHealth)

	var mw []echo.MiddlewareFunc
	if token != "" {
		mw = append(mw, requireToken(token))
		slog.Info("route: pipeline endpoints require a shared token")
	} else {
		slog.Warn("route: pipeline endpoints are unauthenticated (auth.token is not set)")
	}

	e.GET("/pipeline/:name", ph.Run, mw...)
	e.POST("/pipeline/:name", ph.Run, mw...)
	return e
}

// requireToken accepts the shared token either as an
// "Authorization: Bearer <token>" header or as a ?key=<token> query
// parameter, so clients that can only configure a URL (e.g. WiSP's HTTP
// catalog) can still authenticate.
func requireToken(token string) echo.MiddlewareFunc {
	want := []byte(token)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if tokenMatches(want, presentedToken(c)) {
				return next(c)
			}
			slog.Warn("route: unauthorized pipeline request",
				"path", c.Request().URL.Path, "remote", c.Request().RemoteAddr)
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
	}
}

func presentedToken(c *echo.Context) string {
	if h := c.Request().Header.Get("Authorization"); h != "" {
		if after, ok := strings.CutPrefix(h, "Bearer "); ok {
			return after
		}
		return ""
	}
	return c.QueryParam("key")
}

func tokenMatches(want []byte, got string) bool {
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare(want, []byte(got)) == 1
}
