package lua

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A payload far larger than any single Read buffer: the old stringReader
// returned io.EOF on its first Read, so everything past the first buffer was
// silently dropped.
func TestHTTPPost_SendsLargePayloadInFull(t *testing.T) {
	const payloadSize = 512 * 1024

	var (
		gotBody        []byte
		gotContentType string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gotBody = body
		fmt.Fprintf(w, "received %d", len(body))
	}))
	defer srv.Close()

	payload := strings.Repeat("0123456789abcdef", payloadSize/16)
	require.Len(t, payload, payloadSize)

	script := fmt.Sprintf(`
local http = require("http")
local payload = string.rep("0123456789abcdef", %d)
return http.post(%q, "application/json", payload)
`, payloadSize/16, srv.URL)

	res, err := NewExecutor(10*time.Second).Execute(context.Background(), script, nil)
	require.NoError(t, err)

	require.Len(t, gotBody, payloadSize, "the server must receive the whole payload")
	require.Equal(t, payload, string(gotBody))
	require.Equal(t, "application/json", gotContentType)
	require.Equal(t, fmt.Sprintf("received %d", payloadSize), res.Text)
}

func TestHTTPPost_SmallPayload(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	script := fmt.Sprintf(`
local http = require("http")
return http.post(%q, "application/json", '{"key":"value"}')
`, srv.URL)

	res, err := NewExecutor(10*time.Second).Execute(context.Background(), script, nil)
	require.NoError(t, err)
	require.Equal(t, `{"key":"value"}`, gotBody)
	require.Equal(t, "ok", res.Text)
}

func TestHTTPGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"temperature":"24"}`))
	}))
	defer srv.Close()

	script := fmt.Sprintf(`
local http = require("http")
local json = require("json")
local raw = http.get(%q)
local data = json.decode(raw)
return data.temperature
`, srv.URL)

	res, err := NewExecutor(10*time.Second).Execute(context.Background(), script, nil)
	require.NoError(t, err)
	require.Equal(t, "text", res.OutputType)
	require.Equal(t, "24", res.Text)
}

func TestHTTPPost_NonSuccessStatusRaises(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	script := fmt.Sprintf(`
local http = require("http")
return http.post(%q, "application/json", "{}")
`, srv.URL)

	_, err := NewExecutor(10*time.Second).Execute(context.Background(), script, nil)
	require.ErrorContains(t, err, "status 500")
}
