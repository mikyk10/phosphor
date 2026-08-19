package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"

	anyllmerrors "github.com/mozilla-ai/any-llm-go/errors"
	"github.com/openai/openai-go"
	"github.com/stretchr/testify/require"
)

func TestStatusErrorMessage(t *testing.T) {
	err := fmt.Errorf("chat completion %w", &StatusError{StatusCode: 400, Body: `{"error":"bad prompt"}`})
	require.Equal(t, `chat completion returned 400: {"error":"bad prompt"}`, err.Error())

	var se *StatusError
	require.True(t, errors.As(err, &se))
	require.Equal(t, 400, se.StatusCode)
}

func TestIsRetryable_StatusError(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{http.StatusBadRequest, false},
		{http.StatusUnauthorized, false},
		{http.StatusForbidden, false},
		{http.StatusNotFound, false},
		{http.StatusUnprocessableEntity, false},
		{http.StatusRequestTimeout, true},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			err := fmt.Errorf("image edit %w", &StatusError{StatusCode: tt.status, Body: "body"})
			require.Equal(t, tt.want, IsRetryable(err))
		})
	}
}

func TestIsRetryable_OpenAIError(t *testing.T) {
	require.False(t, IsRetryable(&openai.Error{StatusCode: http.StatusBadRequest}))
	require.False(t, IsRetryable(fmt.Errorf("wrapped: %w", &openai.Error{StatusCode: http.StatusUnauthorized})))
	require.True(t, IsRetryable(&openai.Error{StatusCode: http.StatusTooManyRequests}))
	require.True(t, IsRetryable(&openai.Error{StatusCode: http.StatusInternalServerError}))
}

func TestIsRetryable_AnyLLMClassifiedErrors(t *testing.T) {
	// any-llm wraps the SDK error, so the HTTP status still decides.
	wrapped := anyllmerrors.NewInvalidRequestError("openai", &openai.Error{StatusCode: http.StatusBadRequest})
	require.False(t, IsRetryable(fmt.Errorf("chat completion failed: %w", wrapped)))

	rateLimited := anyllmerrors.NewRateLimitError("openai", &openai.Error{StatusCode: http.StatusTooManyRequests})
	require.True(t, IsRetryable(fmt.Errorf("chat completion failed: %w", rateLimited)))

	// Classifications that carry no HTTP status of their own.
	require.False(t, IsRetryable(anyllmerrors.NewAuthenticationError("openai", errors.New("bad key"))))
	require.False(t, IsRetryable(anyllmerrors.NewContentFilterError("openai", errors.New("blocked"))))
	require.False(t, IsRetryable(anyllmerrors.NewMissingAPIKeyError("openai", "OPENAI_API_KEY")))
	require.True(t, IsRetryable(anyllmerrors.NewRateLimitError("openai", errors.New("slow down"))))
}

func TestIsRetryable_ContextErrors(t *testing.T) {
	// A single attempt timing out; the next attempt gets a fresh deadline.
	require.True(t, IsRetryable(context.DeadlineExceeded))
	require.True(t, IsRetryable(fmt.Errorf("chat completion failed: %w", context.DeadlineExceeded)))

	// The caller gave up — retrying is pointless.
	require.False(t, IsRetryable(context.Canceled))
	require.False(t, IsRetryable(fmt.Errorf("chat completion failed: %w", context.Canceled)))
}

func TestIsRetryable_NetworkErrors(t *testing.T) {
	netErr := &url.Error{Op: "Post", URL: "https://api.example.com", Err: &net.DNSError{IsTimeout: true}}
	require.True(t, IsRetryable(netErr))
	require.True(t, IsRetryable(fmt.Errorf("request failed: %w", netErr)))
}

func TestIsRetryable_UnknownAndNil(t *testing.T) {
	require.False(t, IsRetryable(nil))
	// Unclassifiable errors keep the pre-classification behaviour: retry.
	require.True(t, IsRetryable(errors.New("something odd happened")))
}
