package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Load reads config.yaml and service.yaml from the given directory.
func Load(configDir string) (*GlobalConfig, *ServiceConfig, error) {
	global, err := loadYAML[GlobalConfig](configDir + "/config.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("load config.yaml: %w", err)
	}

	svc, err := loadYAML[ServiceConfig](configDir + "/service.yaml")
	if err != nil {
		return nil, nil, fmt.Errorf("load service.yaml: %w", err)
	}

	applyDefaults(global)

	if err := validate(global, svc); err != nil {
		return nil, nil, fmt.Errorf("config validation: %w", err)
	}

	return global, svc, nil
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
