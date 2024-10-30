// FYI some parts are generated from chatGpt, IDK anything about aerospike configuration,
// just a simple example of how operators can be developed and used
package aerospike_operator

import (
	"agent/internal/operator"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AerospikeOperator handles Aerospike configuration operations
type AerospikeOperator struct {
	confPath   string
	config     *ConfigNode
	backupPath string
	lastBackup string
}

// OperatorConfig defines the configuration for the Aerospike operator
type OperatorConfig struct {
	ConfigPath string `json:"config_path"`
	Validate   bool   `json:"validate"`
}

// New creates a new Aerospike configuration operator
func New() *AerospikeOperator {
	return &AerospikeOperator{}
}

// Name returns the operator name
func (o *AerospikeOperator) Name() string {
	return "aerospike-config"
}

// Info returns information about the operator
func (o *AerospikeOperator) Info() operator.OperatorInfo {
	return operator.OperatorInfo{
		Name:        "aerospike-config",
		Version:     "1.0.0",
		Description: "Reads and manages Aerospike configuration files",
		Parameters: []operator.ParameterInfo{
			{
				Name:        "config_path",
				Type:        "string",
				Required:    true,
				Description: "Path to Aerospike configuration file",
			},
			{
				Name:        "validate",
				Type:        "bool",
				Required:    false,
				Description: "Validate configuration after reading",
				Default:     "true",
			},
		},
	}
}

// Init initializes the operator
func (o *AerospikeOperator) Init(config map[string]interface{}) error {
	path, ok := config["config_path"].(string)
	if !ok {
		return fmt.Errorf("config_path not provided")
	}

	o.confPath = path
	o.backupPath = filepath.Join(filepath.Dir(path), "backups")

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(o.backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	return nil
}

// Execute runs the operator
func (o *AerospikeOperator) Execute(ctx context.Context, params map[string]interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Create backup before making any changes
		if err := o.createBackup(); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}

		// Parse current configuration
		config, err := ParseConfig(o.confPath)
		if err != nil {
			return fmt.Errorf("failed to parse config: %v", err)
		}
		o.config = config
		params["config"] = config

		// Check operation type
		if opType, ok := params["operation"].(string); ok {
			switch opType {
			case "add_namespace":
				// Extract namespace configuration from params
				nsParams, ok := params["namespace"].(map[string]interface{})
				if !ok {
					return fmt.Errorf("namespace parameters not provided")
				}

				// Create namespace configuration
				nsConfig := NamespaceConfig{
					Name:               nsParams["name"].(string),
					MemorySize:         getStringParam(nsParams, "memory_size", "1G"),
					ReplicationFactor:  getIntParam(nsParams, "replication_factor", 2),
					DefaultTTL:         getIntParam(nsParams, "default_ttl", 0),
					HighWaterDiskPct:   getIntParam(nsParams, "high_water_disk_pct", 70),
					HighWaterMemoryPct: getIntParam(nsParams, "high_water_memory_pct", 70),
					StopWritesPct:      getIntParam(nsParams, "stop_writes_pct", 90),
					NsupPeriod:         getIntParam(nsParams, "nsup_period", 120),
					StorageEngine:      getStringParam(nsParams, "storage_engine", "device"),
					DataFile:           getStringParam(nsParams, "data_file", fmt.Sprintf("/var/lib/aerospike/%s.dat", nsParams["name"].(string))),
					FileSize:           getStringParam(nsParams, "file_size", "2G"),
					DataInMemory:       getBoolParam(nsParams, "data_in_memory", true),
					WriteBlockSize:     getStringParam(nsParams, "write_block_size", "128K"),
					DefragLwmPct:       getIntParam(nsParams, "defrag_lwm_pct", 50),
					DefragStartupMin:   getIntParam(nsParams, "defrag_startup_min", 10),
				}

				return o.AddNamespace(ctx, nsConfig)

			default:
				return fmt.Errorf("unknown operation type: %s", opType)
			}
		}

		// If validate is requested
		if validate, ok := params["validate"].(bool); ok && validate {
			if err := o.validateConfig(); err != nil {
				// Trigger rollback if validation fails
				if rbErr := o.Rollback(ctx); rbErr != nil {
					return fmt.Errorf("validation failed: %v, rollback failed: %v", err, rbErr)
				}
				return fmt.Errorf("validation failed and rolled back: %v", err)
			}
		}

		return nil
	}
}

// Helper functions for parameter extraction with default values
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

// Rollback handles failure scenarios by restoring from backup
func (o *AerospikeOperator) Rollback(ctx context.Context) error {
	if o.lastBackup == "" {
		return fmt.Errorf("no backup available for rollback")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Restore from last backup
		err := os.Rename(o.lastBackup, o.confPath)
		if err != nil {
			return fmt.Errorf("failed to restore from backup: %v", err)
		}

		// Clear the backup reference
		o.lastBackup = ""
		return nil
	}
}

// Cleanup performs any necessary cleanup
func (o *AerospikeOperator) Cleanup() error {
	// Clean up old backups
	if o.backupPath != "" {
		entries, err := os.ReadDir(o.backupPath)
		if err != nil {
			return fmt.Errorf("failed to read backup directory: %v", err)
		}

		// Keep only the last 5 backups
		maxBackups := 5
		if len(entries) > maxBackups {
			// Sort entries by modification time
			for i, entry := range entries[:len(entries)-maxBackups] {
				backupPath := filepath.Join(o.backupPath, entry.Name())
				if err := os.Remove(backupPath); err != nil {
					return fmt.Errorf("failed to cleanup backup %d: %v", i, err)
				}
			}
		}
	}

	return nil
}

// Helper methods

// createBackup creates a backup of the current configuration
func (o *AerospikeOperator) createBackup() error {
	timestamp := time.Now().Format("20060102-150405")
	backupFile := filepath.Join(o.backupPath, fmt.Sprintf("aerospike.conf.%s", timestamp))

	// Read the current config
	content, err := os.ReadFile(o.confPath)
	if err != nil {
		return fmt.Errorf("failed to read current config: %v", err)
	}

	// Write to backup file
	if err := os.WriteFile(backupFile, content, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %v", err)
	}

	o.lastBackup = backupFile
	return nil
}

// validateConfig performs basic validation of the configuration
func (o *AerospikeOperator) validateConfig() error {
	if o.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	// Check for required sections
	requiredSections := []string{"service", "network", "namespace"}
	for _, section := range requiredSections {
		if sections := o.config.GetSection(section); len(sections) == 0 {
			return fmt.Errorf("required section '%s' not found", section)
		}
	}
	return nil
}
