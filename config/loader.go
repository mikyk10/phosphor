package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"gopkg.in/yaml.v2"
)

// Load reads config.yaml and service.yaml from the given directory. Either
// file may be absent, in which case the shipped .example next to it is used,
// so a fresh checkout runs without copying anything first.
func Load(configDir string) (*GlobalConfig, *ServiceConfig, error) {
	global, err := loadYAMLOrExample[GlobalConfig](configDir + "/config.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("load config.yaml: %w", err)
	}

	svc, err := loadYAMLOrExample[ServiceConfig](configDir + "/service.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("load service.yaml: %w", err)
	}

	applyDefaults(global)

	if err := validate(global, svc); err != nil {
		return nil, nil, fmt.Errorf("config validation: %w", err)
	}

	return global, svc, nil
}

// loadYAMLOrExample reads path, falling back to "<path>.example" when path
// does not exist. Any other failure (bad permissions, malformed YAML) is
// reported as-is rather than being papered over with defaults.
func loadYAMLOrExample[T any](path string) (*T, error) {
	cfg, err := loadYAML[T](path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	example := path + ".example"
	cfg, exampleErr := loadYAML[T](example)
	if errors.Is(exampleErr, fs.ErrNotExist) {
		// Neither exists — report the file the caller actually asked for.
		return nil, err
	}
	if exampleErr != nil {
		return nil, fmt.Errorf("fallback %s: %w", example, exampleErr)
	}

	slog.Warn("config: file not found, falling back to the example defaults",
		"missing", path, "using", example)
	return cfg, nil
}

func loadYAML[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	expanded := os.ExpandEnv(string(data))

	var cfg T
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validate(global *GlobalConfig, svc *ServiceConfig) error {
	if len(global.AI.Providers) == 0 {
		return fmt.Errorf("no AI providers configured in config.yaml")
	}
	if len(svc.Pipelines) == 0 {
		return fmt.Errorf("no pipelines configured in service.yaml")
	}
	for name, p := range svc.Pipelines {
		if len(p.Stages) == 0 {
			return fmt.Errorf("pipeline %q has no stages", name)
		}
	}
	return nil
}

func applyDefaults(cfg *GlobalConfig) {
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if cfg.AI.RequestTimeoutSec == 0 {
		cfg.AI.RequestTimeoutSec = 120
	}
	if cfg.AI.MaxRetries == 0 {
		cfg.AI.MaxRetries = 3
	}
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = ":memory:"
	}
}
