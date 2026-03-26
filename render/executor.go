package render

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"github.com/mikyk10/wisp-ai/pipeline"
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
	html, err := renderHTML(prompt)
	if err != nil {
		return nil, fmt.Errorf("render html: %w", err)
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

// renderHTML loads HTML from a file path or uses the content directly.
func renderHTML(content string) (string, error) {
	if !strings.Contains(content, "<") {
		data, err := os.ReadFile(content)
		if err != nil {
			return "", fmt.Errorf("load template %s: %w", content, err)
		}
		content = string(data)
	}

	if _, err := template.New("page").Parse(content); err != nil {
		return "", fmt.Errorf("parse html template: %w", err)
	}

	return content, nil
}

func (e *Executor) capture(ctx context.Context, html string) ([]byte, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(e.width, e.height),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("force-device-scale-factor", "1"),
	)

	// Use CHROME_PATH if set (e.g. /opt/chrome/chrome-headless-shell in Docker).
	if p := os.Getenv("CHROME_PATH"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	taskCtx, timeoutCancel := context.WithTimeout(taskCtx, 30*time.Second)
	defer timeoutCancel()

	var buf []byte
	if err := chromedp.Run(taskCtx,
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
