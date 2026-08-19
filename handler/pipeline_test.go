package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/usecase"
	"github.com/stretchr/testify/require"
)

type stubUsecase struct {
	out *usecase.PipelineOutput
	err error
}

func (s *stubUsecase) Run(context.Context, usecase.PipelineInput) (*usecase.PipelineOutput, error) {
	return s.out, s.err
}

func get(t *testing.T, uc usecase.PipelineUsecase) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.GET("/pipeline/:name", NewPipelineHandler(uc).Run)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pipeline/demo", nil))
	return rec
}

func TestRun_PipelineNotFoundIs404(t *testing.T) {
	err := fmt.Errorf("pipeline %q: %w", "demo", usecase.ErrPipelineNotFound)
	rec := get(t, &stubUsecase{err: err})

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"error":"pipeline not found"}`, rec.Body.String())
}

func TestRun_InternalErrorLeaksNothing(t *testing.T) {
	secret := `chat completion returned 401: {"error":{"message":"Incorrect API key sk-abc123"}} at /home/user/config/prompts/x.md`
	rec := get(t, &stubUsecase{err: fmt.Errorf("stage %q failed: %s", "gen", secret)})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "sk-abc123")
	require.NotContains(t, rec.Body.String(), "/home/user")

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "pipeline execution failed", body["error"])
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}$`), body["error_id"])
}

func TestRun_UnknownOutputTypeIsOpaque(t *testing.T) {
	rec := get(t, &stubUsecase{out: &usecase.PipelineOutput{OutputType: "video"}})

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "pipeline execution failed", body["error"])
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}$`), body["error_id"])
}

func TestRun_ErrorIDsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		id := newErrorID()
		require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}$`), id)
		seen[id] = true
	}
	require.Greater(t, len(seen), 15, "error IDs should not repeat")
}

func TestRun_InvalidMaxTagsStays400(t *testing.T) {
	e := echo.New()
	e.GET("/pipeline/:name", NewPipelineHandler(&stubUsecase{}).Run)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pipeline/demo?max_tags=abc", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid max_tags")
}
