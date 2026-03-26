package llm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePrompt_WithFrontmatter(t *testing.T) {
	content := `---
provider: openai
model: gpt-4o
api_type: chat
temperature: 1.2
max_tokens: 300
size: 1024x1024
quality: high
---
Hello, world!`

	p, err := ParsePrompt(content)
	require.NoError(t, err)
	assert.Equal(t, "openai", p.Meta.Provider)
	assert.Equal(t, "gpt-4o", p.Meta.Model)
	assert.Equal(t, "chat", p.Meta.ApiType)
	assert.Equal(t, 1.2, p.Meta.Temperature)
	assert.Equal(t, 300, p.Meta.MaxTokens)
	assert.Equal(t, "1024x1024", p.Meta.Size)
	assert.Equal(t, "high", p.Meta.Quality)
	assert.Equal(t, "Hello, world!", p.Body)
	assert.Len(t, p.Hash, 12)
}

func TestParsePrompt_WithoutFrontmatter(t *testing.T) {
	p, err := ParsePrompt("Just a plain prompt")
	require.NoError(t, err)
	assert.Equal(t, PromptMeta{}, p.Meta)
	assert.Equal(t, "Just a plain prompt", p.Body)
	assert.Len(t, p.Hash, 12)
}

func TestParsePrompt_MissingClosingDelimiter(t *testing.T) {
	_, err := ParsePrompt("---\nprovider: openai\n")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing closing ---")
}

func TestParsePrompt_HashIsDeterministic(t *testing.T) {
	p1, _ := ParsePrompt("same body")
	p2, _ := ParsePrompt("same body")
	assert.Equal(t, p1.Hash, p2.Hash)

	p3, _ := ParsePrompt("different body")
	assert.NotEqual(t, p1.Hash, p3.Hash)
}

func TestLoadPrompt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	err := os.WriteFile(path, []byte("---\nprovider: ollama\nmodel: qwen\n---\nDescribe this."), 0644)
	require.NoError(t, err)

	p, err := LoadPrompt(path)
	require.NoError(t, err)
	assert.Equal(t, "ollama", p.Meta.Provider)
	assert.Equal(t, "qwen", p.Meta.Model)
	assert.Equal(t, "Describe this.", p.Body)
}

func TestRenderPrompt_PrevOutput(t *testing.T) {
	body := "Previous: {{.prev.output}}"
	data := TemplateData{
		Prev: StageOutput{Text: "hello from previous stage"},
	}
	result, err := RenderPrompt(body, data)
	require.NoError(t, err)
	assert.Equal(t, "Previous: hello from previous stage", result)
}

func TestRenderPrompt_StageReference(t *testing.T) {
	body := "Description: {{.stages.descriptor.output}}"
	data := TemplateData{
		Stages: map[string]StageOutput{
			"descriptor": {Text: "a photo of a cat"},
		},
	}
	result, err := RenderPrompt(body, data)
	require.NoError(t, err)
	assert.Equal(t, "Description: a photo of a cat", result)
}

func TestRenderPrompt_ConfigVariable(t *testing.T) {
	body := "Extract up to {{.config.MaxTags}} tags."
	data := TemplateData{
		Config: map[string]any{"MaxTags": 10},
	}
	result, err := RenderPrompt(body, data)
	require.NoError(t, err)
	assert.Equal(t, "Extract up to 10 tags.", result)
}
