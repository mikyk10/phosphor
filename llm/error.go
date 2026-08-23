package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	anyllmerrors "github.com/mozilla-ai/any-llm-go/errors"
	"github.com/openai/openai-go"
)

// StatusError reports a non-2xx HTTP response from a provider endpoint.
// It is returned instead of a plain error so retry logic can tell a
// transient failure (429, 5xx) from a permanent one (bad prompt, policy
// violation, bad credentials).
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("returned %d: %s", e.StatusCode, e.Body)
}

// IsRetryable reports whether retrying the same request could plausibly
// succeed. Image APIs are expensive, so a permanent rejection must not be
// paid for four times over.
//
// Errors that cannot be classified are treated as retryable, preserving the
// behaviour that existed before classification was introduced.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// The caller gave up; nothing to retry.
	if errors.Is(err, context.Canceled) {
		return false
	}
	// A per-attempt timeout — the next attempt gets a fresh deadline.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return retryableStatus(statusErr.StatusCode)
	}

	// any-llm wraps the underlying SDK error, so this also covers errors
	// surfaced through the AnyLLM chat provider.
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return retryableStatus(apiErr.StatusCode)
	}

	// any-llm classifications that carry no HTTP status of their own.
	if errors.Is(err, anyllmerrors.ErrRateLimit) {
		return true
	}
	for _, permanent := range []error{
		anyllmerrors.ErrAuthentication,
		anyllmerrors.ErrInvalidRequest,
		anyllmerrors.ErrContextLength,
		anyllmerrors.ErrContentFilter,
		anyllmerrors.ErrModelNotFound,
		anyllmerrors.ErrMissingAPIKey,
		anyllmerrors.ErrUnsupportedProvider,
		anyllmerrors.ErrUnsupportedParam,
	} {
		if errors.Is(err, permanent) {
			return false
		}
	}

	// Connection resets, DNS hiccups, i/o timeouts.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return true
}

func retryableStatus(code int) bool {
	switch {
	case code == http.StatusTooManyRequests, code == http.StatusRequestTimeout:
		return true
	case code >= 500:
		return true
	case code >= 400:
		return false
	default:
		return true
	}
}
