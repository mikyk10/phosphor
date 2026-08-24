package config

import "log/slog"

// GlobalConfig holds infrastructure settings loaded from config.yaml.
type GlobalConfig struct {
	Port     int            `yaml:"port"`
	LogLevel slog.Level     `yaml:"log_level"`
	Database DatabaseConfig `yaml:"database"`
	AI       AIConfig       `yaml:"ai"`
	Auth     AuthConfig     `yaml:"auth"`
	// MaxRequestBodyMB caps the size of a POST body (a source image).
	// Requests larger than this are rejected with 413.
	MaxRequestBodyMB int `yaml:"max_request_body_mb"`
	// MaxConcurrentPipelines bounds how many pipeline executions may run at
	// once. Each one fans out into LLM calls and headless Chrome tabs, so an
	// unbounded count runs away with both cost and memory.
	MaxConcurrentPipelines int `yaml:"max_concurrent_pipelines"`
}

// AuthConfig holds the optional shared-token protection for pipeline routes.
type AuthConfig struct {
	// Token, when non-empty, is required on every /pipeline request as
	// either an "Authorization: Bearer <token>" header or a ?key=<token>
	// query parameter. Empty means the endpoints are open.
	Token string `yaml:"token"`
}

// DatabaseConfig holds execution-history persistence settings.
type DatabaseConfig struct {
	DSN string `yaml:"dsn"`
	// MaxHistory is the number of most recent pipeline executions to keep.
	// Older rows (and their steps/outputs) are pruned periodically.
	// -1 disables pruning entirely.
	MaxHistory int `yaml:"max_history"`
	// StoreImageOutputs controls whether generated image blobs are persisted.
	// Text outputs are always stored; image blobs dominate DB growth.
	StoreImageOutputs bool `yaml:"store_image_outputs"`
}

// AIConfig holds LLM provider connection details and execution parameters.
type AIConfig struct {
	Providers         map[string]ProviderConfig `yaml:"providers"`
	RequestTimeoutSec int                       `yaml:"request_timeout_sec"`
	MaxRetries        int                       `yaml:"max_retries"`
}

// ProviderConfig holds connection details for a single LLM provider.
type ProviderConfig struct {
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"api_key"`
}

// ServiceConfig holds pipeline definitions loaded from service.yaml.
type ServiceConfig struct {
	Pipelines map[string]PipelineConfig `yaml:"pipelines"`
}

// PipelineConfig defines a sequence of stages with optional defaults.
type PipelineConfig struct {
	Defaults PipelineDefaults `yaml:"defaults"`
	Stages   []StageConfig    `yaml:"stages"`
}

// PipelineDefaults holds default values for pipeline execution parameters.
// Request parameters override these when provided.
type PipelineDefaults struct {
	Size    string `yaml:"size"`    // e.g. "1024x1024", "1536x1024"
	Quality string `yaml:"quality"` // e.g. "low", "high"
	MaxTags int    `yaml:"max_tags"`
}

// StageConfig defines a single pipeline stage.
type StageConfig struct {
	Name       string `yaml:"name"`
	Output     string `yaml:"output"`      // "text" or "image"
	Prompt     string `yaml:"prompt"`      // path to prompt file
	ImageInput string `yaml:"image_input"` // "_source" or stage name
	// HistoryLimit, when > 0, makes the text outputs of this stage's past
	// successful runs (most recent first) available to its prompt template
	// as {{.history}}. 0 (the default) looks up nothing.
	HistoryLimit int `yaml:"history_limit"`
}
