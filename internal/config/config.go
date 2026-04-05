package config

import (
	"os"
	"time"

	"go.yaml.in/yaml/v2"
)

type Target struct {
	Name       string `yaml:"name"`
	Namespace  string `yaml:"namespace"`
	Deployment string `yaml:"deployment"`
}

type Config struct {
	Interval time.Duration `yaml:"interval"`
	Targets  []Target      `yaml:"targets"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}

	if cfg.Interval == 0 {
		cfg.Interval = 15 * time.Second
	}

	return &cfg, nil
}
