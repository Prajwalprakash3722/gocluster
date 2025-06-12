package config

import (
	"agent/internal/types"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Cluster ClusterConfig       `yaml:"cluster"`
	Backend types.BackendConfig `yaml:"backend"`
	Nodes   map[string]string   `yaml:"nodes"`
	Plugins []string            `yaml:"plugins"`
}

// ClusterConfig represents cluster-specific configuration
type ClusterConfig struct {
	Name            string `yaml:"name"`
	DiscoveryPort   int    `yaml:"discovery_port"`
	BindAddress     string `yaml:"bind_address"`
	WebAddress      string `yaml:"web_address"`
	EnableOperators bool   `yaml:"enable_operators"`
}

// Load loads configuration from a file
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		return getDefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Config file %s not found, using defaults\n", configPath)
			return getDefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Apply defaults for missing values
	applyDefaults(&config)

	return &config, nil
}

// getDefaultConfig returns a default configuration
func getDefaultConfig() *Config {
	config := &Config{
		Cluster: ClusterConfig{
			Name:            "default-cluster",
			DiscoveryPort:   7946,
			BindAddress:     "0.0.0.0",
			WebAddress:      "0.0.0.0:8080",
			EnableOperators: true,
		},
		Backend: types.BackendConfig{
			Type:          types.BackendMemory,
			Namespace:     "gocluster-default",
			LeaderTTL:     "30s",
			RenewInterval: "10s",
		},
		Nodes:   make(map[string]string),
		Plugins: []string{"hello"},
	}

	applyDefaults(config)
	return config
}

// applyDefaults applies default values to missing configuration fields
func applyDefaults(config *Config) {
	// Cluster defaults
	if config.Cluster.Name == "" {
		config.Cluster.Name = "default-cluster"
	}
	if config.Cluster.DiscoveryPort == 0 {
		config.Cluster.DiscoveryPort = 7946
	}
	if config.Cluster.BindAddress == "" {
		config.Cluster.BindAddress = "0.0.0.0"
	}
	if config.Cluster.WebAddress == "" {
		config.Cluster.WebAddress = "0.0.0.0:8080"
	}

	// Backend defaults
	if config.Backend.Type == "" {
		config.Backend.Type = types.BackendMemory
	}
	if config.Backend.Namespace == "" {
		config.Backend.Namespace = "gocluster-default"
	}
	if config.Backend.LeaderTTL == "" {
		config.Backend.LeaderTTL = "30s"
	}
	if config.Backend.RenewInterval == "" {
		config.Backend.RenewInterval = "10s"
	}

	// ZooKeeper defaults
	if config.Backend.Type == types.BackendZooKeeper {
		if config.Backend.ZKTimeout == "" {
			config.Backend.ZKTimeout = "30s"
		}
		if config.Backend.ZKSessionPath == "" {
			config.Backend.ZKSessionPath = "/gocluster/sessions"
		}
		if len(config.Backend.ZKHosts) == 0 {
			config.Backend.ZKHosts = []string{"localhost:2181"}
		}
	}

	// etcd defaults
	if config.Backend.Type == types.BackendEtcd {
		if len(config.Backend.EtcdEndpoints) == 0 {
			config.Backend.EtcdEndpoints = []string{"localhost:2379"}
		}
	}

	// Nodes default
	if config.Nodes == nil {
		config.Nodes = make(map[string]string)
	}

	// Plugins default
	if config.Plugins == nil {
		config.Plugins = []string{"hello"}
	}
}

// Save saves the configuration to a file
func (c *Config) Save(configPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Cluster.Name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	if c.Cluster.DiscoveryPort <= 0 || c.Cluster.DiscoveryPort > 65535 {
		return fmt.Errorf("discovery port must be between 1 and 65535")
	}

	if c.Backend.Type == "" {
		return fmt.Errorf("backend type cannot be empty")
	}

	switch c.Backend.Type {
	case types.BackendMemory:
		// Memory backend doesn't need additional validation
	case types.BackendEtcd:
		if len(c.Backend.EtcdEndpoints) == 0 {
			return fmt.Errorf("etcd endpoints cannot be empty when using etcd backend")
		}
	case types.BackendZooKeeper:
		if len(c.Backend.ZKHosts) == 0 {
			return fmt.Errorf("zookeeper hosts cannot be empty when using zookeeper backend")
		}
	default:
		return fmt.Errorf("unsupported backend type: %s", c.Backend.Type)
	}

	return nil
}
