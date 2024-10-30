package aerospike_operator

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// ConfigNode represents a node in the Aerospike configuration tree
type ConfigNode struct {
	Name       string                  // Name of the section/key
	Value      string                  // Value if it's a property
	Properties map[string]string       // Key-value properties
	Children   map[string][]ConfigNode // Named sections can have multiple instances (e.g., multiple namespaces)
}

// NewConfigNode creates a new configuration node
func NewConfigNode(name string) *ConfigNode {
	return &ConfigNode{
		Name:       name,
		Properties: make(map[string]string),
		Children:   make(map[string][]ConfigNode),
	}
}

// ParseConfig reads and parses an Aerospike configuration file
func ParseConfig(filePath string) (*ConfigNode, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	root := NewConfigNode("root")
	stack := []*ConfigNode{root}
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle section end
		if line == "}" {
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			continue
		}

		current := stack[len(stack)-1]

		// Handle section start
		if strings.HasSuffix(line, "{") {
			sectionDeclaration := strings.TrimSuffix(line, "{")
			parts := strings.Fields(sectionDeclaration)

			var newNode *ConfigNode
			if len(parts) == 1 {
				// Simple section like "network {"
				newNode = NewConfigNode(parts[0])
			} else {
				// Named section like "namespace test {"
				newNode = NewConfigNode(parts[1])
				// Add to the appropriate section list
				current.Children[parts[0]] = append(current.Children[parts[0]], *newNode)
			}
			stack = append(stack, newNode)
			continue
		}

		// Handle properties
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			key := parts[0]
			value := strings.Join(parts[1:], " ")
			current.Properties[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file at line %d: %v", lineNum, err)
	}

	return root, nil
}

// Helper methods for ConfigNode

func (n *ConfigNode) GetNamespace(name string) *ConfigNode {
	namespaces, exists := n.Children["namespace"]
	if !exists {
		return nil
	}

	for _, ns := range namespaces {
		if ns.Name == name {
			return &ns
		}
	}
	return nil
}

func (n *ConfigNode) GetProperty(key string) (string, bool) {
	value, exists := n.Properties[key]
	return value, exists
}

func (n *ConfigNode) GetSection(name string) []ConfigNode {
	return n.Children[name]
}

// ToString returns a string representation of the config node
func (n *ConfigNode) ToString(indent string) string {
	var sb strings.Builder

	// Write properties
	for k, v := range n.Properties {
		sb.WriteString(fmt.Sprintf("%s%s %s\n", indent, k, v))
	}

	// Write children
	for section, nodes := range n.Children {
		for _, node := range nodes {
			if node.Name != "" {
				sb.WriteString(fmt.Sprintf("%s%s %s {\n", indent, section, node.Name))
			} else {
				sb.WriteString(fmt.Sprintf("%s%s {\n", indent, section))
			}
			sb.WriteString(node.ToString(indent + "  "))
			sb.WriteString(fmt.Sprintf("%s}\n", indent))
		}
	}

	return sb.String()
}

// NamespaceConfig represents the configuration for a new namespace
type NamespaceConfig struct {
	Name               string
	MemorySize         string
	ReplicationFactor  int
	DefaultTTL         int
	HighWaterDiskPct   int
	HighWaterMemoryPct int
	StopWritesPct      int
	NsupPeriod         int
	StorageEngine      string
	DataFile           string
	FileSize           string
	DataInMemory       bool
	WriteBlockSize     string
	DefragLwmPct       int
	DefragStartupMin   int
}

// DefaultNamespaceConfig returns a default configuration for a new namespace
func DefaultNamespaceConfig(name string) NamespaceConfig {
	return NamespaceConfig{
		Name:               name,
		MemorySize:         "1G",
		ReplicationFactor:  2,
		DefaultTTL:         0,
		HighWaterDiskPct:   70,
		HighWaterMemoryPct: 70,
		StopWritesPct:      90,
		NsupPeriod:         120,
		StorageEngine:      "device",
		DataFile:           fmt.Sprintf("/var/lib/aerospike/%s.dat", name),
		FileSize:           "2G",
		DataInMemory:       true,
		WriteBlockSize:     "128K",
		DefragLwmPct:       50,
		DefragStartupMin:   10,
	}
}

// AddNamespace adds a new namespace to the Aerospike configuration
func (o *AerospikeOperator) AddNamespace(ctx context.Context, config NamespaceConfig) error {
	// Read and parse current configuration
	currentConfig, err := ParseConfig(o.confPath)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	// Check if namespace already exists
	if ns := currentConfig.GetNamespace(config.Name); ns != nil {
		return fmt.Errorf("namespace %s already exists", config.Name)
	}

	// Create backup before modification
	if err := o.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}

	// Read the current config file content
	content, err := os.ReadFile(o.confPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// Generate namespace configuration
	nsConfig := generateNamespaceConfig(config)

	// Append the new namespace to the file
	newContent := append(content, []byte("\n"+nsConfig)...)

	// Write the updated configuration
	err = os.WriteFile(o.confPath, newContent, 0644)
	if err != nil {
		// Attempt rollback if write fails
		if rbErr := o.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("failed to write config: %v, rollback failed: %v", err, rbErr)
		}
		return fmt.Errorf("failed to write config and rolled back: %v", err)
	}

	fmt.Printf("Successfully added namespace '%s' to configuration\n", config.Name)
	return nil
}

// generateNamespaceConfig creates the namespace configuration string
func generateNamespaceConfig(config NamespaceConfig) string {
	return fmt.Sprintf(`namespace %s {
        enable-xdr false
        memory-size %s
        replication-factor %d
        default-ttl %d
        high-water-disk-pct %d
        high-water-memory-pct %d
        stop-writes-pct %d
        nsup-period %d
        storage-engine %s {
                file %s
                filesize %s
                data-in-memory %t
                write-block-size %s
                defrag-lwm-pct %d
                defrag-startup-minimum %d
        }
}`,
		config.Name,
		config.MemorySize,
		config.ReplicationFactor,
		config.DefaultTTL,
		config.HighWaterDiskPct,
		config.HighWaterMemoryPct,
		config.StopWritesPct,
		config.NsupPeriod,
		config.StorageEngine,
		config.DataFile,
		config.FileSize,
		config.DataInMemory,
		config.WriteBlockSize,
		config.DefragLwmPct,
		config.DefragStartupMin,
	)
}
