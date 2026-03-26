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

// PipelineConfig defines a sequence of stages.
type PipelineConfig struct {
	Stages []StageConfig `yaml:"stages"`
}

// StageConfig defines a single pipeline stage.
type StageConfig struct {
	Name       string `yaml:"name"`
	Output     string `yaml:"output"`      // "text" or "image"
	Prompt     string `yaml:"prompt"`       // path to prompt file
	ImageInput string `yaml:"image_input"` // "_source" or stage name
}
