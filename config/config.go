package config

import "log/slog"

// GlobalConfig holds infrastructure settings loaded from config.yaml.
type GlobalConfig struct {
	Port     int        `yaml:"port"`
	LogLevel slog.Level `yaml:"log_level"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	AI AIConfig `yaml:"ai"`
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
	Width       int    `yaml:"width"`
	Height      int    `yaml:"height"`
	Orientation string `yaml:"orientation"`
	Quality     string `yaml:"quality"`
	MaxTags     int    `yaml:"max_tags"`
}

// StageConfig defines a single pipeline stage.
type StageConfig struct {
	Name       string `yaml:"name"`
	Output     string `yaml:"output"`      // "text" or "image"
	Prompt     string `yaml:"prompt"`       // path to prompt file
	ImageInput string `yaml:"image_input"` // "_source" or stage name
}
