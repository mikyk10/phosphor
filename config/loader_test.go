package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	globalYAML = `
port: 9999
ai:
  providers:
    openai:
      endpoint: https://api.openai.com/v1
`
	serviceYAML = `
pipelines:
  demo:
    stages:
      - name: only
        output: text
        prompt: prompts/demo.md
`
)

// writeConfigDir creates a config directory. Entries with a nil value are
// skipped, so a test can leave a file out.
func writeConfigDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}
	return dir
}

func TestLoad_UsesRealFilesWhenPresent(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"config.yaml":          globalYAML,
		"config.yaml.example":  "port: 1\nai:\n  providers:\n    x:\n      endpoint: e\n",
		"service.yaml":         serviceYAML,
		"service.yaml.example": "pipelines:\n  other:\n    stages:\n      - name: s\n        output: text\n        prompt: p\n",
	})

	global, svc, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, 9999, global.Port, "the real file wins over the example")
	require.Contains(t, svc.Pipelines, "demo")
	require.NotContains(t, svc.Pipelines, "other")
}

// A fresh checkout has only the .example files; it must still boot.
func TestLoad_FallsBackToExamples(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"config.yaml.example":  globalYAML,
		"service.yaml.example": serviceYAML,
	})

	global, svc, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, 9999, global.Port)
	require.Contains(t, svc.Pipelines, "demo")
}

func TestLoad_FallsBackPerFile(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"config.yaml":          globalYAML,
		"service.yaml.example": serviceYAML,
	})

	global, svc, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, 9999, global.Port)
	require.Contains(t, svc.Pipelines, "demo")
}

func TestLoad_ReportsTheRequestedPathWhenNothingExists(t *testing.T) {
	_, _, err := Load(t.TempDir())
	require.Error(t, err)
	require.ErrorContains(t, err, "config.yaml")
	require.NotContains(t, err.Error(), ".example", "the example is a fallback, not what was asked for")
}

// A malformed example must surface as a parse error rather than being
// silently swallowed into zero values.
func TestLoad_MalformedExampleIsReported(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"config.yaml.example":  "port: [not, an, int\n",
		"service.yaml.example": serviceYAML,
	})

	_, _, err := Load(dir)
	require.Error(t, err)
	require.ErrorContains(t, err, "config.yaml.example")
}

// A broken real file must not fall through to the example.
func TestLoad_MalformedRealFileDoesNotFallBack(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"config.yaml":          "port: [not, an, int\n",
		"config.yaml.example":  globalYAML,
		"service.yaml.example": serviceYAML,
	})

	_, _, err := Load(dir)
	require.Error(t, err)
	require.NotContains(t, err.Error(), ".example")
}

func TestLoad_AppliesDefaultsToTheExample(t *testing.T) {
	dir := writeConfigDir(t, map[string]string{
		"config.yaml.example":  "ai:\n  providers:\n    openai:\n      endpoint: e\n",
		"service.yaml.example": serviceYAML,
	})

	global, _, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, 8080, global.Port)
	require.Equal(t, 120, global.AI.RequestTimeoutSec)
	require.Equal(t, 3, global.AI.MaxRetries)
	require.Equal(t, ":memory:", global.Database.DSN)
}

func TestLoad_ExpandsEnvInExamples(t *testing.T) {
	t.Setenv("PHOSPHOR_TEST_ENDPOINT", "http://example.test/v1")
	dir := writeConfigDir(t, map[string]string{
		"config.yaml.example":  "ai:\n  providers:\n    openai:\n      endpoint: ${PHOSPHOR_TEST_ENDPOINT}\n",
		"service.yaml.example": serviceYAML,
	})

	global, _, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, "http://example.test/v1", global.AI.Providers["openai"].Endpoint)
}

// The repository's own shipped examples must be a working configuration,
// since that is exactly what a fresh checkout runs on.
func TestLoad_ShippedExamplesAreValid(t *testing.T) {
	global, svc, err := Load("../config")
	require.NoError(t, err)
	require.NotZero(t, global.Port)
	require.NotEmpty(t, global.AI.Providers)
	require.NotEmpty(t, svc.Pipelines)
	require.Contains(t, svc.Pipelines, "dashboard")
}
