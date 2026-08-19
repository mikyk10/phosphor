package render

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/mikyk10/phosphor/pipeline"
)

// Executor renders an HTML template and captures it as a PNG screenshot
// using a headless Chrome instance.
type Executor struct {
	width  int
	height int
}

// NewExecutor creates a render executor with the given viewport size.
func NewExecutor(size string) (*Executor, error) {
	w, h, err := parseSize(size)
	if err != nil {
		return nil, err
	}
	return &Executor{width: w, height: h}, nil
}

// Execute renders the prompt as HTML content and captures a full-page screenshot.
// If the prompt string does not contain HTML tags, it is treated as a file path.
func (e *Executor) Execute(ctx context.Context, prompt string, _ [][]byte) (*pipeline.StageResult, error) {
	html, err := loadHTML(prompt)
	if err != nil {
		return nil, fmt.Errorf("load html: %w", err)
	}

	png, err := e.capture(ctx, html)
	if err != nil {
		return nil, fmt.Errorf("capture screenshot: %w", err)
	}

	slog.Info("render: captured screenshot", "size", fmt.Sprintf("%dx%d", e.width, e.height), "bytes", len(png))

	return &pipeline.StageResult{
		OutputType:  "image",
		ImageData:   png,
		ContentType: "image/png",
	}, nil
}

// loadHTML returns the HTML to capture. If the content contains no "<" it is
// treated as a path to an HTML file, otherwise it is the markup itself.
//
// The content is used verbatim: the pipeline runner already expanded it as a
// text/template before the stage ran, so template syntax has been resolved by
// the time it gets here.
func loadHTML(content string) (string, error) {
	if strings.Contains(content, "<") {
		return content, nil
	}

	raw, err := os.ReadFile(content)
	if err != nil {
		return "", fmt.Errorf("load template %s: %w", content, err)
	}
	return string(raw), nil
}

// A single headless Chrome process is shared by the whole service: starting
// one costs 1-2 seconds and several tens of MB, which is far too expensive to
// pay per request. Each capture only opens a new tab on the shared browser.
var (
	browserOnce sync.Once
	browserCtx  context.Context
	browserErr  error

	// Cancels are deliberately retained (never called) so the allocator and
	// the browser outlive individual requests. They are released when the
	// process exits.
	browserAllocCancel context.CancelFunc
	browserCancel      context.CancelFunc
)

// sharedBrowser returns the process-wide chromedp browser context, starting
// Chrome on first use.
func sharedBrowser() (context.Context, error) {
	browserOnce.Do(func() {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("hide-scrollbars", true),
			chromedp.Flag("force-device-scale-factor", "1"),
		)

		// Use CHROME_PATH if set (e.g. /opt/chrome/chrome-headless-shell in Docker).
		if p := os.Getenv("CHROME_PATH"); p != "" {
			opts = append(opts, chromedp.ExecPath(p))
		}

		var allocCtx context.Context
		allocCtx, browserAllocCancel = chromedp.NewExecAllocator(context.Background(), opts...)

		var ctx context.Context
		ctx, browserCancel = chromedp.NewContext(allocCtx)

		// Run with no actions boots the browser so the first real capture
		// does not pay the startup cost.
		if err := chromedp.Run(ctx); err != nil {
			browserErr = fmt.Errorf("start headless chrome: %w", err)
			return
		}
		browserCtx = ctx
		slog.Info("render: headless chrome started", "max_concurrent", maxConcurrent())
	})
	return browserCtx, browserErr
}

// defaultMaxConcurrent is the number of simultaneous renders allowed when
// RENDER_MAX_CONCURRENT is unset. Each concurrent render is a live Chrome tab.
const defaultMaxConcurrent = 2

var (
	semOnce sync.Once
	sem     chan struct{}
)

func maxConcurrent() int {
	if v := os.Getenv("RENDER_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		slog.Warn("render: ignoring invalid RENDER_MAX_CONCURRENT", "value", v)
	}
	return defaultMaxConcurrent
}

// acquireSlot blocks until a render slot is free, returning a release func.
// It gives up if ctx is cancelled while waiting.
func acquireSlot(ctx context.Context) (func(), error) {
	semOnce.Do(func() { sem = make(chan struct{}, maxConcurrent()) })

	select {
	case sem <- struct{}{}:
		return func() { <-sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (e *Executor) capture(ctx context.Context, html string) ([]byte, error) {
	release, err := acquireSlot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	browser, err := sharedBrowser()
	if err != nil {
		return nil, err
	}

	// A new tab on the shared browser, not a new browser.
	tabCtx, tabCancel := chromedp.NewContext(browser)
	defer tabCancel()

	// The tab hangs off the browser rather than off ctx, so propagate
	// cancellation of the request explicitly.
	stopWatch := context.AfterFunc(ctx, tabCancel)
	defer stopWatch()

	tabCtx, timeoutCancel := context.WithTimeout(tabCtx, 30*time.Second)
	defer timeoutCancel()

	var buf []byte
	if err := chromedp.Run(tabCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return emulation.SetDeviceMetricsOverride(int64(e.width), int64(e.height), 1.0, false).Do(ctx)
		}),
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(`document.open(); document.write(`+quote(html)+`); document.close();`, nil).Do(ctx)
		}),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, err = page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatPng).
				WithCaptureBeyondViewport(false).
				Do(ctx)
			return err
		}),
	); err != nil {
		return nil, err
	}

	return buf, nil
}

func parseSize(size string) (int, int, error) {
	var w, h int
	if _, err := fmt.Sscanf(size, "%dx%d", &w, &h); err != nil {
		return 0, 0, fmt.Errorf("invalid size %q: expected WIDTHxHEIGHT", size)
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid size %q: width and height must be positive", size)
	}
	return w, h, nil
}

func quote(s string) string {
	var buf bytes.Buffer
	buf.WriteByte('`')
	buf.WriteString(strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "`", "\\`"))
	buf.WriteByte('`')
	return buf.String()
}
