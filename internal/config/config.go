package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ClusterName   string
	DiscoveryPort int
	Nodes         map[string]string
}

func Load(filepath string) (*Config, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	cfg := &Config{
		Nodes: make(map[string]string),
	}

	var section string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		switch section {
		case "cluster":
			switch key {
			case "name":
				cfg.ClusterName = value
			case "discovery_port":
				port, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid discovery port: %w", err)
				}
				cfg.DiscoveryPort = port
			}
		case "nodes":
			cfg.Nodes[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	return cfg, validateConfig(cfg)
}

func validateConfig(cfg *Config) error {
	if cfg.ClusterName == "" {
		return fmt.Errorf("cluster name is required")
	}
	if cfg.DiscoveryPort == 0 {
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
