package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// reasoningTransport injects extra JSON fields into POST request bodies.
// Used to force reasoning_effort=none for models like Qwen3.5 that default
// to thinking mode when the field is absent.
type reasoningTransport struct {
	base        http.RoundTripper
	extraFields map[string]any
}

func newReasoningTransport(base http.RoundTripper, fields map[string]any) *reasoningTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &reasoningTransport{base: base, extraFields: fields}
}

func (t *reasoningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodPost || req.Body == nil || len(t.extraFields) == 0 {
		return t.base.RoundTrip(req)
	}

	// Skip injection for OpenAI — it rejects unknown fields like reasoning_effort.
	if req.URL != nil && strings.Contains(req.URL.Host, "openai.com") {
		slog.Debug("transport: skipping field injection (openai)", "host", req.URL.Host)
		return t.base.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close() //nolint:errcheck
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return t.base.RoundTrip(req)
	}

	for k, v := range t.extraFields {
		m[k] = v
	}

	modified, err := json.Marshal(m)
	if err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return t.base.RoundTrip(req)
	}

	req.Body = io.NopCloser(bytes.NewReader(modified))
	req.ContentLength = int64(len(modified))
	slog.Debug("transport: injected fields", "host", req.URL.Host, "fields", t.extraFields)
	return t.base.RoundTrip(req)
}
