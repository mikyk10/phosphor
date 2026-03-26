package lua

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mikyk10/wisp-ai/pipeline"

	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"
)

// Executor runs a Lua script and returns its output as text.
type Executor struct {
	timeout time.Duration
}

func NewExecutor(timeout time.Duration) *Executor {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Executor{timeout: timeout}
}

func (e *Executor) Execute(ctx context.Context, script string, _ [][]byte) (*pipeline.StageResult, error) {
	L := lua.NewState()
	defer L.Close()

	// Register modules.
	luajson.Preload(L)
	L.PreloadModule("http", httpLoader)

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	L.SetContext(ctx)

	if err := L.DoString(script); err != nil {
		return nil, fmt.Errorf("lua exec: %w", err)
	}

	// Capture return value from the script.
	ret := L.Get(-1)
	var output string
	if ret != lua.LNil {
		output = lua.LVAsString(ret)
	}

	return &pipeline.StageResult{
		OutputType:  "text",
		Text:        output,
		ContentType: "text/plain",
	}, nil
}

// httpLoader provides a minimal HTTP module for Lua scripts.
//
//	local http = require("http")
//	local body = http.get("https://example.com/api")
//	local body = http.post("https://example.com/api", "application/json", '{"key":"val"}')
func httpLoader(L *lua.LState) int {
	mod := L.NewTable()

	mod.RawSetString("get", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		body, err := doHTTP(L.Context(), http.MethodGet, url, "", "")
		if err != nil {
			L.RaiseError("http.get: %s", err)
			return 0
		}
		L.Push(lua.LString(body))
		return 1
	}))

	mod.RawSetString("post", L.NewFunction(func(L *lua.LState) int {
		url := L.CheckString(1)
		ct := L.CheckString(2)
		payload := L.CheckString(3)
		body, err := doHTTP(L.Context(), http.MethodPost, url, ct, payload)
		if err != nil {
			L.RaiseError("http.post: %s", err)
			return 0
		}
		L.Push(lua.LString(body))
		return 1
	}))

	L.Push(mod)
	return 1
}

func doHTTP(ctx context.Context, method, url, contentType, payload string) (string, error) {
	var body io.Reader
	if payload != "" {
		body = io.NopCloser(io.Reader(stringReader(payload)))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return "", err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type stringReader string

func (s stringReader) Read(p []byte) (int, error) {
	n := copy(p, s)
	return n, io.EOF
}
