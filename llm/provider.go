package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/mikyk10/phosphor/config"
	luaexec "github.com/mikyk10/phosphor/lua"
	"github.com/mikyk10/phosphor/pipeline"
	"github.com/mikyk10/phosphor/render"

	anyllm "github.com/mozilla-ai/any-llm-go"
	openaicompat "github.com/mozilla-ai/any-llm-go/providers/openai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// NewStageExecutor creates the appropriate StageExecutor based on output type and api_type.
// ExecutionParams override prompt frontmatter values when non-empty.
func NewStageExecutor(providers map[string]config.ProviderConfig, meta PromptMeta, outputType string, timeout time.Duration, params pipeline.ExecutionParams) (pipeline.StageExecutor, error) {
	// Apply runtime overrides.
	if params.Size != "" {
		meta.Size = params.Size
	}
	if params.Quality != "" {
		meta.Quality = params.Quality
	}
	if meta.ApiType == ApiTypeLua {
		return luaexec.NewExecutor(timeout), nil
	}
	if meta.ApiType == ApiTypeRender {
		size := meta.Size
		if params.Size != "" {
			size = params.Size
		}
		if size == "" {
			size = "800x600"
		}
		return render.NewExecutor(size)
	}
	if outputType == "image" {
		switch meta.ApiType {
		case ApiTypeImageGeneration, "":
			return newImageGenExecutor(providers, meta, timeout)
		case ApiTypeImageEdit:
			return newImageEditExecutor(providers, meta, timeout)
		case ApiTypeChat:
			return newChatImageExecutor(providers, meta, timeout)
		default:
			return nil, fmt.Errorf("unknown api_type %q", meta.ApiType)
		}
	}
	return newChatTextExecutor(providers, meta, timeout)
}

// ---------------------------------------------------------------------------
// Chat text executor — AnyLLM (output: text)
// ---------------------------------------------------------------------------

func newChatTextExecutor(providers map[string]config.ProviderConfig, meta PromptMeta, timeout time.Duration) (pipeline.StageExecutor, error) {
	prov, ok := providers[meta.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown AI provider %q", meta.Provider)
	}

	provider, err := openaicompat.NewCompatible(
		openaicompat.CompatibleConfig{
			Name:           meta.Provider,
			DefaultBaseURL: prov.Endpoint,
			RequireAPIKey:  false,
			DefaultAPIKey:  "none",
		},
		anyllm.WithBaseURL(prov.Endpoint),
		anyllm.WithAPIKey(providerAPIKey(prov.APIKey)),
		anyllm.WithHTTPClient(&http.Client{
			Timeout:   timeout,
			Transport: newReasoningTransport(http.DefaultTransport, map[string]any{
				"reasoning_effort": "none",
				"think":           false,
			}),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("create AnyLLM provider: %w", err)
	}

	return &chatTextExecutor{provider: provider, meta: meta, timeout: timeout}, nil
}

type chatTextExecutor struct {
	provider *openaicompat.CompatibleProvider
	meta     PromptMeta
	timeout  time.Duration
}

func (e *chatTextExecutor) Execute(ctx context.Context, prompt string, images [][]byte) (*pipeline.StageResult, error) {
	var parts []anyllm.ContentPart
	parts = append(parts, anyllm.ContentPart{Type: "text", Text: prompt})
	for _, img := range images {
		b64 := base64.StdEncoding.EncodeToString(img)
		parts = append(parts, anyllm.ContentPart{
			Type:     "image_url",
			ImageURL: &anyllm.ImageURL{URL: "data:image/jpeg;base64," + b64},
		})
	}

	params := anyllm.CompletionParams{
		Model: e.meta.Model,
		Messages: []anyllm.Message{
			{Role: anyllm.RoleUser, Content: parts},
		},
		ReasoningEffort: anyllm.ReasoningEffortNone,
	}
	if e.meta.MaxTokens > 0 {
		params.MaxTokens = &e.meta.MaxTokens
	}
	if e.meta.Temperature > 0 {
		params.Temperature = &e.meta.Temperature
	}

	slog.Debug("llm: chat text request", "model", e.meta.Model, "provider", e.meta.Provider, "images", len(images), "prompt_len", len(prompt), "reasoning_effort", "none")
	callCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	resp, err := e.provider.Completion(callCtx, params)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("chat completion returned no choices")
	}

	return &pipeline.StageResult{
		OutputType: "text",
		Text:       strings.TrimSpace(resp.Choices[0].Message.ContentString()),
	}, nil
}

// ---------------------------------------------------------------------------
// Chat image executor — Raw HTTP (output: image, api_type: chat)
// ---------------------------------------------------------------------------

func newChatImageExecutor(providers map[string]config.ProviderConfig, meta PromptMeta, timeout time.Duration) (pipeline.StageExecutor, error) {
	prov, ok := providers[meta.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown AI provider %q", meta.Provider)
	}
	return &chatImageExecutor{
		endpoint: strings.TrimSuffix(prov.Endpoint, "/") + "/chat/completions",
		apiKey:   providerAPIKey(prov.APIKey),
		meta:     meta,
		timeout:  timeout,
	}, nil
}

type chatImageExecutor struct {
	endpoint string
	apiKey   string
	meta     PromptMeta
	timeout  time.Duration
}

type chatCompletionRequest struct {
	Model       string       `json:"model"`
	Modalities  []string     `json:"modalities,omitempty"`
	Messages    []rawMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature *float64     `json:"temperature,omitempty"`
}

type rawMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (e *chatImageExecutor) Execute(ctx context.Context, prompt string, images [][]byte) (*pipeline.StageResult, error) {
	var parts []contentPart
	parts = append(parts, contentPart{Type: "text", Text: prompt})
	for _, img := range images {
		b64 := base64.StdEncoding.EncodeToString(img)
		parts = append(parts, contentPart{
			Type:     "image_url",
			ImageURL: &imageURL{URL: "data:image/jpeg;base64," + b64},
		})
	}

	reqBody := chatCompletionRequest{
		Model:      e.meta.Model,
		Modalities: []string{"text", "image"},
		Messages:   []rawMessage{{Role: "user", Content: parts}},
	}
	if e.meta.MaxTokens > 0 {
		reqBody.MaxTokens = e.meta.MaxTokens
	}
	if e.meta.Temperature > 0 {
		reqBody.Temperature = &e.meta.Temperature
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	slog.Debug("llm: chat image (raw HTTP)", "model", e.meta.Model, "endpoint", e.endpoint)

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, e.endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" && e.apiKey != "none" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat completion request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("chat completion returned %d: %s", resp.StatusCode, string(respBody))
	}

	return extractImageFromChatResponse(respBody)
}

func extractImageFromChatResponse(respBody []byte) (*pipeline.StageResult, error) {
	var resp chatCompletionResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := resp.Choices[0].Message.Content

	var parts []contentPart
	if err := json.Unmarshal(content, &parts); err == nil {
		var text string
		for _, part := range parts {
			switch part.Type {
			case "image_url":
				if part.ImageURL != nil {
					imgData, ct, err := resolveImageURL(part.ImageURL.URL)
					if err != nil {
						return nil, fmt.Errorf("resolve image from response: %w", err)
					}
					return &pipeline.StageResult{
						OutputType:  "image",
						Text:        text,
						ImageData:   imgData,
						ContentType: ct,
					}, nil
				}
			case "text":
				text = part.Text
			}
		}
		return nil, fmt.Errorf("no image_url part found in chat response content parts")
	}

	var textContent string
	if err := json.Unmarshal(content, &textContent); err == nil {
		return nil, fmt.Errorf("chat response contained only text, no image: %s", truncate(textContent, 100))
	}

	return nil, fmt.Errorf("unable to parse chat response content: %s", truncate(string(content), 200))
}

func resolveImageURL(url string) ([]byte, string, error) {
	if strings.HasPrefix(url, "data:") {
		parts := strings.SplitN(url, ",", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid data URL")
		}
		meta := parts[0]
		ct := "image/png"
		if idx := strings.Index(meta, ":"); idx >= 0 {
			rest := meta[idx+1:]
			if semi := strings.Index(rest, ";"); semi >= 0 {
				ct = rest[:semi]
			}
		}
		data, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, "", fmt.Errorf("decode base64 image: %w", err)
		}
		return data, ct, nil
	}

	return downloadImage(context.Background(), url)
}

func downloadImage(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	return data, ct, nil
}

// ---------------------------------------------------------------------------
// Image generation executor — openai-go (output: image, api_type: image_generation)
// ---------------------------------------------------------------------------

func newImageGenExecutor(providers map[string]config.ProviderConfig, meta PromptMeta, timeout time.Duration) (pipeline.StageExecutor, error) {
	prov, ok := providers[meta.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown AI provider %q", meta.Provider)
	}

	opts := []option.RequestOption{
		option.WithBaseURL(prov.Endpoint),
	}
	if prov.APIKey != "" {
		opts = append(opts, option.WithAPIKey(prov.APIKey))
	} else {
		opts = append(opts, option.WithAPIKey("unused"))
	}
	if timeout > 0 {
		opts = append(opts, option.WithHTTPClient(&http.Client{Timeout: timeout}))
	}
	client := openai.NewClient(opts...)
	return &imageGenExecutor{client: &client, meta: meta}, nil
}

type imageGenExecutor struct {
	client *openai.Client
	meta   PromptMeta
}

func (e *imageGenExecutor) Execute(ctx context.Context, prompt string, _ [][]byte) (*pipeline.StageResult, error) {
	slog.Debug("llm: image generation", "model", e.meta.Model, "provider", e.meta.Provider)

	params := openai.ImageGenerateParams{
		Prompt: prompt,
		Model:  e.meta.Model,
	}
	if e.meta.Size != "" {
		params.Size = openai.ImageGenerateParamsSize(e.meta.Size)
	}
	if e.meta.Quality != "" {
		params.Quality = openai.ImageGenerateParamsQuality(e.meta.Quality)
	}

	resp, err := e.client.Images.Generate(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("image generation failed: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("image generation returned no data")
	}

	item := resp.Data[0]
	if item.URL != "" {
		data, ct, err := downloadImage(ctx, item.URL)
		if err != nil {
			return nil, err
		}
		return &pipeline.StageResult{OutputType: "image", ImageData: data, ContentType: ct}, nil
	}
	if item.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return nil, fmt.Errorf("decode base64 image: %w", err)
		}
		return &pipeline.StageResult{OutputType: "image", ImageData: data, ContentType: "image/png"}, nil
	}

	return nil, fmt.Errorf("image generation returned neither URL nor base64 data")
}

// ---------------------------------------------------------------------------
// Image edit executor — raw HTTP multipart (output: image, api_type: image_edit)
// ---------------------------------------------------------------------------

func newImageEditExecutor(providers map[string]config.ProviderConfig, meta PromptMeta, timeout time.Duration) (pipeline.StageExecutor, error) {
	prov, ok := providers[meta.Provider]
	if !ok {
		return nil, fmt.Errorf("unknown AI provider %q", meta.Provider)
	}
	return &imageEditExecutor{
		endpoint: strings.TrimSuffix(prov.Endpoint, "/") + "/images/edits",
		apiKey:   providerAPIKey(prov.APIKey),
		meta:     meta,
		timeout:  timeout,
	}, nil
}

type imageEditExecutor struct {
	endpoint string
	apiKey   string
	meta     PromptMeta
	timeout  time.Duration
}

func (e *imageEditExecutor) Execute(ctx context.Context, prompt string, images [][]byte) (*pipeline.StageResult, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("image_edit requires at least one input image")
	}

	slog.Debug("llm: image edit", "model", e.meta.Model, "endpoint", e.endpoint, "images", len(images))

	callCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for i, img := range images {
		ct := detectImageContentType(img)
		ext := ".png"
		if ct == "image/jpeg" {
			ext = ".jpg"
		}
		partHeader := make(textproto.MIMEHeader)
		name := "image"
		if i > 0 {
			name = "image[]"
		}
		partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="image_%d%s"`, name, i, ext))
		partHeader.Set("Content-Type", ct)
		part, err := writer.CreatePart(partHeader)
		if err != nil {
			return nil, fmt.Errorf("create form part: %w", err)
		}
		if _, err := part.Write(img); err != nil {
			return nil, fmt.Errorf("write image data: %w", err)
		}
	}

	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("model", e.meta.Model)
	if e.meta.Size != "" {
		_ = writer.WriteField("size", e.meta.Size)
	}
	if e.meta.Quality != "" {
		_ = writer.WriteField("quality", e.meta.Quality)
	}
	_ = writer.WriteField("n", "1")

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, e.endpoint, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if e.apiKey != "" && e.apiKey != "none" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image edit request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("image edit returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
			URL     string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("image edit returned no data")
	}

	item := result.Data[0]
	if item.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			return nil, fmt.Errorf("decode base64 image: %w", err)
		}
		return &pipeline.StageResult{OutputType: "image", ImageData: data, ContentType: "image/png"}, nil
	}
	if item.URL != "" {
		data, ct, err := downloadImage(callCtx, item.URL)
		if err != nil {
			return nil, err
		}
		return &pipeline.StageResult{OutputType: "image", ImageData: data, ContentType: ct}, nil
	}

	return nil, fmt.Errorf("image edit returned neither b64_json nor url")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func providerAPIKey(key string) string {
	if key == "" {
		return "none"
	}
	return key
}

func detectImageContentType(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}
	if len(data) >= 4 && string(data[:4]) == "RIFF" {
		return "image/webp"
	}
	return "image/png"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
