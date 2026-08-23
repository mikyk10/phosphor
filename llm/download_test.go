package llm

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolveImageURL_DataURLNeedsNoNetwork(t *testing.T) {
	raw := []byte{0x89, 'P', 'N', 'G'}
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)

	// An already-cancelled context proves nothing is fetched.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	data, ct, err := resolveImageURL(ctx, url)
	require.NoError(t, err)
	require.Equal(t, raw, data)
	require.Equal(t, "image/png", ct)
}

func TestResolveImageURL_DownloadsOverHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpegbytes"))
	}))
	defer srv.Close()

	data, ct, err := resolveImageURL(context.Background(), srv.URL+"/img.jpg")
	require.NoError(t, err)
	require.Equal(t, []byte("jpegbytes"), data)
	require.Equal(t, "image/jpeg", ct)
}

// A server that accepts the request and then never answers must not pin the
// caller: the context deadline has to end the download.
func TestResolveImageURL_HonoursContextDeadline(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-blocked:
		case <-r.Context().Done():
		}
	}))
	defer func() {
		close(blocked)
		srv.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, err := resolveImageURL(ctx, srv.URL+"/stalled.png")
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestExtractImageFromChatResponse_PropagatesContextToDownload(t *testing.T) {
	// The handler is still parked when the assertions run, so the flag has to
	// be written and read atomically.
	var gotRequest atomic.Bool
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotRequest.Store(true)
		select {
		case <-blocked:
		case <-r.Context().Done():
		}
	}))
	defer func() {
		close(blocked)
		srv.Close()
	}()

	body := []byte(`{"choices":[{"message":{"content":[{"type":"image_url","image_url":{"url":"` + srv.URL + `/a.png"}}]}}]}`)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := extractImageFromChatResponse(ctx, body)
	require.Error(t, err)
	require.True(t, gotRequest.Load())
	require.True(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)
}
