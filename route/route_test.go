package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/handler"
	"github.com/mikyk10/phosphor/usecase"
	"github.com/stretchr/testify/require"
)

type stubUsecase struct{ calls int }

func (s *stubUsecase) Run(context.Context, usecase.PipelineInput) (*usecase.PipelineOutput, error) {
	s.calls++
	return &usecase.PipelineOutput{OutputType: "text", Text: "ok"}, nil
}

func newTestServer(t *testing.T, token string) (*echo.Echo, *stubUsecase) {
	t.Helper()
	uc := &stubUsecase{}
	e := echo.New()
	Configure(e, handler.NewPipelineHandler(uc, 1<<20), token)
	return e, uc
}

func do(e *echo.Echo, target string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestNoTokenConfigured_EndpointsStayOpen(t *testing.T) {
	e, uc := newTestServer(t, "")

	require.Equal(t, http.StatusOK, do(e, "/pipeline/demo", nil).Code)
	require.Equal(t, 1, uc.calls)
}

func TestTokenConfigured(t *testing.T) {
	const token = "s3cret"

	t.Run("bearer header accepted", func(t *testing.T) {
		e, uc := newTestServer(t, token)
		rec := do(e, "/pipeline/demo", map[string]string{"Authorization": "Bearer " + token})
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 1, uc.calls)
	})

	t.Run("query parameter accepted", func(t *testing.T) {
		e, uc := newTestServer(t, token)
		rec := do(e, "/pipeline/demo?key="+token, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 1, uc.calls)
	})

	t.Run("missing credentials rejected", func(t *testing.T) {
		e, uc := newTestServer(t, token)
		rec := do(e, "/pipeline/demo", nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.JSONEq(t, `{"error":"unauthorized"}`, rec.Body.String())
		require.Zero(t, uc.calls, "the pipeline must not run")
	})

	t.Run("wrong token rejected", func(t *testing.T) {
		e, uc := newTestServer(t, token)
		for _, tc := range []struct {
			name   string
			target string
			header map[string]string
		}{
			{"bad bearer", "/pipeline/demo", map[string]string{"Authorization": "Bearer nope"}},
			{"bad query", "/pipeline/demo?key=nope", nil},
			{"prefix of token", "/pipeline/demo?key=s3cre", nil},
			{"non-bearer scheme", "/pipeline/demo", map[string]string{"Authorization": "Basic " + token}},
			{"header wins over query", "/pipeline/demo?key=" + token, map[string]string{"Authorization": "Bearer nope"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				require.Equal(t, http.StatusUnauthorized, do(e, tc.target, tc.header).Code)
			})
		}
		require.Zero(t, uc.calls)
	})

	t.Run("health stays open", func(t *testing.T) {
		e, _ := newTestServer(t, token)
		require.Equal(t, http.StatusOK, do(e, "/health", nil).Code)
	})
}
