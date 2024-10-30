package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Cluster struct {
		Name            string `yaml:"name"`
		DiscoveryPort   int    `yaml:"discovery_port"`
		BindAddress     string `yaml:"bind_address"`
		WebAddress      string `yaml:"web_address"`
		EnableOperators bool   `yaml:"enable_operators"`
	} `yaml:"cluster"`
	Nodes   map[string]string `yaml:"nodes"`
	Plugins []string          `yaml:"plugins"`
}

func Load(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, validateConfig(&cfg)
}

func validateConfig(cfg *Config) error {
	if cfg.Cluster.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	if cfg.Cluster.DiscoveryPort == 0 {
		return fmt.Errorf("discovery port is required")
	}
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("at least one node must be configured")
	}
	return nil
}

func ParseAddress(addr string) (string, int, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid address format: %s", addr)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, fmt.Errorf("invalid port number: %w", err)
	}
	return parts[0], port, nil
}
