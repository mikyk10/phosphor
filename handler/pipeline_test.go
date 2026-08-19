package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/usecase"
	"github.com/stretchr/testify/require"
)

type stubUsecase struct {
	calls    int
	lastBody []byte
}

func (s *stubUsecase) Run(_ context.Context, in usecase.PipelineInput) (*usecase.PipelineOutput, error) {
	s.calls++
	s.lastBody = in.SourceImage
	return &usecase.PipelineOutput{OutputType: "text", Text: "ok"}, nil
}

func post(t *testing.T, limit int64, body []byte) (*httptest.ResponseRecorder, *stubUsecase) {
	t.Helper()
	uc := &stubUsecase{}
	e := echo.New()
	e.POST("/pipeline/:name", NewPipelineHandler(uc, limit).Run)

	req := httptest.NewRequest(http.MethodPost, "/pipeline/demo", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec, uc
}

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
