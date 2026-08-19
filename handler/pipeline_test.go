package handler

import (
	"bytes"
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
	out      *usecase.PipelineOutput
	err      error
	calls    int
	lastBody []byte
}

func (s *stubUsecase) Run(_ context.Context, in usecase.PipelineInput) (*usecase.PipelineOutput, error) {
	s.calls++
	s.lastBody = in.SourceImage
	if s.err != nil {
		return nil, s.err
	}
	if s.out != nil {
		return s.out, nil
	}
	return &usecase.PipelineOutput{OutputType: "text", Text: "ok"}, nil
}

func serve(t *testing.T, uc usecase.PipelineUsecase, limit int64, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	h := NewPipelineHandler(uc, limit)
	e.GET("/pipeline/:name", h.Run)
	e.POST("/pipeline/:name", h.Run)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func post(t *testing.T, limit int64, body []byte) (*httptest.ResponseRecorder, *stubUsecase) {
	t.Helper()
	uc := &stubUsecase{}
	rec := serve(t, uc, limit, httptest.NewRequest(http.MethodPost, "/pipeline/demo", bytes.NewReader(body)))
	return rec, uc
}

func get(t *testing.T, uc usecase.PipelineUsecase) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, uc, 1<<20, httptest.NewRequest(http.MethodGet, "/pipeline/demo", nil))
}

// ---------------------------------------------------------------------------
// Request body limit
// ---------------------------------------------------------------------------

func TestRun_BodyWithinLimit(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 1024)
	rec, uc := post(t, 4096, body)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, uc.calls)
	require.Equal(t, body, uc.lastBody)
}

func TestRun_BodyOverLimitRejected(t *testing.T) {
	rec, uc := post(t, 1024, bytes.Repeat([]byte("a"), 4096))

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.JSONEq(t, `{"error":"request body too large"}`, rec.Body.String())
	require.Zero(t, uc.calls, "the pipeline must not run")
}

func TestRun_ZeroLimitMeansUnlimited(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 1<<20)
	rec, uc := post(t, 0, body)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, uc.lastBody, len(body))
}

func TestRun_EmptyBodyLeavesSourceImageNil(t *testing.T) {
	rec, uc := post(t, 1024, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, uc.lastBody)
}

// ---------------------------------------------------------------------------
// Error responses
// ---------------------------------------------------------------------------

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
	rec := serve(t, &stubUsecase{}, 1<<20,
		httptest.NewRequest(http.MethodGet, "/pipeline/demo?max_tags=abc", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid max_tags")
}
