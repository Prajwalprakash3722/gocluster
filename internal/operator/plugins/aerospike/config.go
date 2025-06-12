package aerospike

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AerospikeConfig represents the Aerospike configuration
type AerospikeConfig struct {
	Service    Service     `json:"service"`
	Logging    Logging     `json:"logging"`
	Network    Network     `json:"network"`
	Namespaces []Namespace `json:"namespaces"`
}

// Service represents the service configuration section
type Service struct {
	User                       string   `json:"user"`
	Group                      string   `json:"group"`
	PidsFile                   string   `json:"pids_file"`
	ProtocolsEnabled           []string `json:"protocols_enabled"`
	ServiceThreads             int      `json:"service_threads"`
	TransactionQueues          int      `json:"transaction_queues"`
	TransactionThreadsPerQueue int      `json:"transaction_threads_per_queue"`
}

// Logging represents the logging configuration section
type Logging struct {
	File    string  `json:"file"`
	Console Console `json:"console"`
}

// Console represents the console logging configuration
type Console struct {
	Context string `json:"context"`
}

// Network represents the network configuration section
type Network struct {
	Service   NetworkService   `json:"service"`
	Heartbeat NetworkHeartbeat `json:"heartbeat"`
	Fabric    NetworkFabric    `json:"fabric"`
	Info      NetworkInfo      `json:"info"`
}

// NetworkService represents the service network configuration
type NetworkService struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// NetworkHeartbeat represents the heartbeat network configuration
type NetworkHeartbeat struct {
	Mode     string `json:"mode"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Interval int    `json:"interval"`
	Timeout  int    `json:"timeout"`
}

// NetworkFabric represents the fabric network configuration
type NetworkFabric struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// NetworkInfo represents the info network configuration
type NetworkInfo struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// Namespace represents a namespace configuration
type Namespace struct {
	Name              string  `json:"name"`
	ReplicationFactor int     `json:"replication_factor"`
	MemorySize        string  `json:"memory_size"`
	DefaultTTL        string  `json:"default_ttl"`
	HighWaterPercent  int     `json:"high_water_percent"`
	StopWritesPercent int     `json:"stop_writes_percent"`
	Storage           Storage `json:"storage"`
}

// Storage represents storage configuration for a namespace
type Storage struct {
	Engine       string `json:"engine"`
	Filesize     string `json:"filesize,omitempty"`
	DataInMemory bool   `json:"data_in_memory,omitempty"`
}

// LoadConfig loads Aerospike configuration from a file
func LoadConfig(configPath string) (*AerospikeConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	config := &AerospikeConfig{
		Namespaces: []Namespace{},
	}

	scanner := bufio.NewScanner(file)
	var currentSection string
	var currentNamespace *Namespace
	var inStorage bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle section starts
		if strings.HasPrefix(line, "service {") {
			currentSection = "service"
			continue
		} else if strings.HasPrefix(line, "logging {") {
			currentSection = "logging"
			continue
		} else if strings.HasPrefix(line, "network {") {
			currentSection = "network"
			continue
		} else if strings.HasPrefix(line, "namespace ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				namespaceName := strings.Trim(parts[1], "{")
				currentNamespace = &Namespace{Name: namespaceName}
				config.Namespaces = append(config.Namespaces, *currentNamespace)
				currentNamespace = &config.Namespaces[len(config.Namespaces)-1]
				currentSection = "namespace"
			}
			continue
		} else if strings.HasPrefix(line, "storage {") && currentSection == "namespace" {
			inStorage = true
			continue
		}

		// Handle section ends
		if line == "}" {
			if inStorage {
				inStorage = false
			} else {
				currentSection = ""
				currentNamespace = nil
			}
			continue
		}

		// Parse configuration directives
		if err := parseConfigLine(line, currentSection, config, currentNamespace, inStorage); err != nil {
			// Log warning but continue parsing
			fmt.Printf("Warning: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	return config, nil
}

// parseConfigLine parses a single configuration line
func parseConfigLine(line, section string, config *AerospikeConfig, namespace *Namespace, inStorage bool) error {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("invalid config line: %s", line)
	}

	key := parts[0]
	value := strings.Join(parts[1:], " ")

	switch section {
	case "service":
		return parseServiceConfig(key, value, &config.Service)
	case "logging":
		return parseLoggingConfig(key, value, &config.Logging)
	case "network":
		return parseNetworkConfig(key, value, &config.Network)
	case "namespace":
		if inStorage {
			return parseStorageConfig(key, value, &namespace.Storage)
		}
		return parseNamespaceConfig(key, value, namespace)
	}

	return nil
}

// parseServiceConfig parses service configuration
func parseServiceConfig(key, value string, service *Service) error {
	switch key {
	case "user":
		service.User = value
	case "group":
		service.Group = value
	case "pids-file":
		service.PidsFile = value
	case "protocols-enabled":
		service.ProtocolsEnabled = strings.Split(value, ",")
	case "service-threads":
		if v, err := strconv.Atoi(value); err == nil {
			service.ServiceThreads = v
		}
	case "transaction-queues":
		if v, err := strconv.Atoi(value); err == nil {
			service.TransactionQueues = v
		}
	case "transaction-threads-per-queue":
		if v, err := strconv.Atoi(value); err == nil {
			service.TransactionThreadsPerQueue = v
		}
	}
	return nil
}

// parseLoggingConfig parses logging configuration
func parseLoggingConfig(key, value string, logging *Logging) error {
	switch key {
	case "file":
		logging.File = value
	case "console":
		// Handle console context
		logging.Console.Context = value
	}
	return nil
}

// parseNetworkConfig parses network configuration
func parseNetworkConfig(key, value string, network *Network) error {
	switch key {
	case "service":
		return parseNetworkService(value, &network.Service)
	case "heartbeat":
		return parseNetworkHeartbeat(value, &network.Heartbeat)
	case "fabric":
		return parseNetworkFabric(value, &network.Fabric)
	case "info":
		return parseNetworkInfo(value, &network.Info)
	}
	return nil
}

// parseNetworkService parses network service configuration
func parseNetworkService(value string, service *NetworkService) error {
	parts := strings.Fields(value)
	for _, part := range parts {
		if strings.HasPrefix(part, "address") {
			service.Address = strings.TrimPrefix(part, "address ")
		} else if strings.HasPrefix(part, "port") {
			if port, err := strconv.Atoi(strings.TrimPrefix(part, "port ")); err == nil {
				service.Port = port
			}
		}
	}
	return nil
}

// parseNetworkHeartbeat parses network heartbeat configuration
func parseNetworkHeartbeat(value string, heartbeat *NetworkHeartbeat) error {
	parts := strings.Fields(value)
	for _, part := range parts {
		if strings.HasPrefix(part, "mode") {
			heartbeat.Mode = strings.TrimPrefix(part, "mode ")
		} else if strings.HasPrefix(part, "address") {
			heartbeat.Address = strings.TrimPrefix(part, "address ")
		} else if strings.HasPrefix(part, "port") {
			if port, err := strconv.Atoi(strings.TrimPrefix(part, "port ")); err == nil {
				heartbeat.Port = port
			}
		} else if strings.HasPrefix(part, "interval") {
			if interval, err := strconv.Atoi(strings.TrimPrefix(part, "interval ")); err == nil {
				heartbeat.Interval = interval
			}
		} else if strings.HasPrefix(part, "timeout") {
			if timeout, err := strconv.Atoi(strings.TrimPrefix(part, "timeout ")); err == nil {
				heartbeat.Timeout = timeout
			}
		}
	}
	return nil
}

// parseNetworkFabric parses network fabric configuration
func parseNetworkFabric(value string, fabric *NetworkFabric) error {
	parts := strings.Fields(value)
	for _, part := range parts {
		if strings.HasPrefix(part, "address") {
			fabric.Address = strings.TrimPrefix(part, "address ")
		} else if strings.HasPrefix(part, "port") {
			if port, err := strconv.Atoi(strings.TrimPrefix(part, "port ")); err == nil {
				fabric.Port = port
			}
		}
	}
	return nil
}

// parseNetworkInfo parses network info configuration
func parseNetworkInfo(value string, info *NetworkInfo) error {
	parts := strings.Fields(value)
	for _, part := range parts {
		if strings.HasPrefix(part, "address") {
			info.Address = strings.TrimPrefix(part, "address ")
		} else if strings.HasPrefix(part, "port") {
			if port, err := strconv.Atoi(strings.TrimPrefix(part, "port ")); err == nil {
				info.Port = port
			}
		}
	}
	return nil
}

// parseNamespaceConfig parses namespace configuration
func parseNamespaceConfig(key, value string, namespace *Namespace) error {
	switch key {
	case "replication-factor":
		if v, err := strconv.Atoi(value); err == nil {
			namespace.ReplicationFactor = v
		}
	case "memory-size":
		namespace.MemorySize = value
	case "default-ttl":
		namespace.DefaultTTL = value
	case "high-water-percent":
		if v, err := strconv.Atoi(value); err == nil {
			namespace.HighWaterPercent = v
		}
	case "stop-writes-pct":
		if v, err := strconv.Atoi(value); err == nil {
			namespace.StopWritesPercent = v
		}
	}
	return nil
}

// parseStorageConfig parses storage configuration
func parseStorageConfig(key, value string, storage *Storage) error {
	switch key {
	case "engine":
		storage.Engine = value
	case "filesize":
		storage.Filesize = value
	case "data-in-memory":
		storage.DataInMemory = value == "true"
	}
	return nil
}

// Save saves the configuration to a file
func (c *AerospikeConfig) Save(configPath string) error {
	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer file.Close()

	// Write service section
	fmt.Fprintf(file, "service {\n")
	if c.Service.User != "" {
		fmt.Fprintf(file, "    user %s\n", c.Service.User)
	}
	if c.Service.Group != "" {
		fmt.Fprintf(file, "    group %s\n", c.Service.Group)
	}
	if c.Service.PidsFile != "" {
		fmt.Fprintf(file, "    pids-file %s\n", c.Service.PidsFile)
	}
	if len(c.Service.ProtocolsEnabled) > 0 {
		fmt.Fprintf(file, "    protocols-enabled %s\n", strings.Join(c.Service.ProtocolsEnabled, ","))
	}
	if c.Service.ServiceThreads > 0 {
		fmt.Fprintf(file, "    service-threads %d\n", c.Service.ServiceThreads)
	}
	if c.Service.TransactionQueues > 0 {
		fmt.Fprintf(file, "    transaction-queues %d\n", c.Service.TransactionQueues)
	}
	if c.Service.TransactionThreadsPerQueue > 0 {
		fmt.Fprintf(file, "    transaction-threads-per-queue %d\n", c.Service.TransactionThreadsPerQueue)
	}
	fmt.Fprintf(file, "}\n\n")

	// Write logging section
	fmt.Fprintf(file, "logging {\n")
	if c.Logging.File != "" {
		fmt.Fprintf(file, "    file %s\n", c.Logging.File)
	}
	if c.Logging.Console.Context != "" {
		fmt.Fprintf(file, "    console {\n")
		fmt.Fprintf(file, "        context %s\n", c.Logging.Console.Context)
		fmt.Fprintf(file, "    }\n")
	}
	fmt.Fprintf(file, "}\n\n")

	// Write network section
	fmt.Fprintf(file, "network {\n")
	fmt.Fprintf(file, "    service {\n")
	if c.Network.Service.Address != "" {
		fmt.Fprintf(file, "        address %s\n", c.Network.Service.Address)
	}
	if c.Network.Service.Port > 0 {
		fmt.Fprintf(file, "        port %d\n", c.Network.Service.Port)
	}
	fmt.Fprintf(file, "    }\n")

	fmt.Fprintf(file, "    heartbeat {\n")
	if c.Network.Heartbeat.Mode != "" {
		fmt.Fprintf(file, "        mode %s\n", c.Network.Heartbeat.Mode)
	}
	if c.Network.Heartbeat.Address != "" {
		fmt.Fprintf(file, "        address %s\n", c.Network.Heartbeat.Address)
	}
	if c.Network.Heartbeat.Port > 0 {
		fmt.Fprintf(file, "        port %d\n", c.Network.Heartbeat.Port)
	}
	if c.Network.Heartbeat.Interval > 0 {
		fmt.Fprintf(file, "        interval %d\n", c.Network.Heartbeat.Interval)
	}
	if c.Network.Heartbeat.Timeout > 0 {
		fmt.Fprintf(file, "        timeout %d\n", c.Network.Heartbeat.Timeout)
	}
	fmt.Fprintf(file, "    }\n")

	if c.Network.Fabric.Address != "" || c.Network.Fabric.Port > 0 {
		fmt.Fprintf(file, "    fabric {\n")
		if c.Network.Fabric.Address != "" {
			fmt.Fprintf(file, "        address %s\n", c.Network.Fabric.Address)
		}
		if c.Network.Fabric.Port > 0 {
			fmt.Fprintf(file, "        port %d\n", c.Network.Fabric.Port)
		}
		fmt.Fprintf(file, "    }\n")
	}

	if c.Network.Info.Address != "" || c.Network.Info.Port > 0 {
		fmt.Fprintf(file, "    info {\n")
		if c.Network.Info.Address != "" {
			fmt.Fprintf(file, "        address %s\n", c.Network.Info.Address)
		}
		if c.Network.Info.Port > 0 {
			fmt.Fprintf(file, "        port %d\n", c.Network.Info.Port)
		}
		fmt.Fprintf(file, "    }\n")
	}

	fmt.Fprintf(file, "}\n\n")

	// Write namespaces
	for _, ns := range c.Namespaces {
		fmt.Fprintf(file, "namespace %s {\n", ns.Name)
		if ns.ReplicationFactor > 0 {
			fmt.Fprintf(file, "    replication-factor %d\n", ns.ReplicationFactor)
		}
		if ns.MemorySize != "" {
			fmt.Fprintf(file, "    memory-size %s\n", ns.MemorySize)
		}
		if ns.DefaultTTL != "" {
			fmt.Fprintf(file, "    default-ttl %s\n", ns.DefaultTTL)
		}
		if ns.HighWaterPercent > 0 {
			fmt.Fprintf(file, "    high-water-percent %d\n", ns.HighWaterPercent)
		}
		if ns.StopWritesPercent > 0 {
			fmt.Fprintf(file, "    stop-writes-pct %d\n", ns.StopWritesPercent)
		}

		fmt.Fprintf(file, "    storage {\n")
		if ns.Storage.Engine != "" {
			fmt.Fprintf(file, "        engine %s\n", ns.Storage.Engine)
		}
		if ns.Storage.Filesize != "" {
			fmt.Fprintf(file, "        filesize %s\n", ns.Storage.Filesize)
		}
		if ns.Storage.DataInMemory {
			fmt.Fprintf(file, "        data-in-memory true\n")
		}
		fmt.Fprintf(file, "    }\n")

		fmt.Fprintf(file, "}\n\n")
	}

	return nil
}

// getDefaultConfig returns a default Aerospike configuration
func getDefaultConfig() *AerospikeConfig {
	return &AerospikeConfig{
		Service: Service{
			User:                       "aerospike",
			Group:                      "aerospike",
			PidsFile:                   "/var/run/aerospike/asd.pid",
			ProtocolsEnabled:           []string{"tcp"},
			ServiceThreads:             4,
			TransactionQueues:          4,
			TransactionThreadsPerQueue: 4,
		},
		Logging: Logging{
			File: "/var/log/aerospike/aerospike.log",
			Console: Console{
				Context: "any info",
			},
		},
		Network: Network{
			Service: NetworkService{
				Address: "any",
				Port:    3000,
			},
			Heartbeat: NetworkHeartbeat{
				Mode:     "multicast",
				Address:  "239.1.99.222",
				Port:     9918,
				Interval: 150,
				Timeout:  10,
			},
			Fabric: NetworkFabric{
				Address: "any",
				Port:    3001,
			},
			Info: NetworkInfo{
				Address: "any",
				Port:    3003,
			},
		},
		Namespaces: []Namespace{
			{
				Name:              "test",
				ReplicationFactor: 2,
				MemorySize:        "1G",
				DefaultTTL:        "30d",
				HighWaterPercent:  60,
				StopWritesPercent: 90,
				Storage: Storage{
					Engine: "memory",
				},
			},
		},
	}
}
