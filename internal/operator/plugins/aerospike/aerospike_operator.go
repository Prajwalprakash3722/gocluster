package aerospike

import (
	"agent/internal/types"
	"context"
	"fmt"
	"os"
	"time"
)

// AerospikeOperator implements Aerospike database operations
type AerospikeOperator struct {
	configPath string
	config     *AerospikeConfig
}

// New creates a new Aerospike operator instance
func New() *AerospikeOperator {
	return &AerospikeOperator{}
}

// Info returns operator information
func (a *AerospikeOperator) Info() types.OperatorInfo {
	return types.OperatorInfo{
		Name:        "aerospike",
		Version:     "1.0.0",
		Description: "Aerospike database configuration and management operations",
		Author:      "prajwal.p",
	}
}

// Init initializes the operator with configuration
func (a *AerospikeOperator) Init(config map[string]interface{}) error {
	if configPath, ok := config["config_path"].(string); ok {
		a.configPath = configPath
	} else {
		a.configPath = "/etc/aerospike/aerospike.conf"
	}

	// Load Aerospike configuration
	if err := a.loadConfig(); err != nil {
		return fmt.Errorf("failed to load Aerospike config: %v", err)
	}

	return nil
}

// GetOperations returns the list of supported operations
func (a *AerospikeOperator) GetOperations() []string {
	return []string{
		"get_config", "update_config", "add_namespace", "remove_namespace",
		"update_namespace", "get_namespaces", "validate_config", "backup_config",
	}
}

// Execute performs the specified operation
func (a *AerospikeOperator) Execute(ctx context.Context, operation string, params map[string]interface{}) (*types.OperationResult, error) {
	result := &types.OperationResult{
		Timestamp: time.Now(),
		NodeID:    "local",
	}

	switch operation {
	case "get_config":
		return a.getConfig(result)
	case "update_config":
		return a.updateConfig(params, result)
	case "add_namespace":
		return a.addNamespace(params, result)
	case "remove_namespace":
		return a.removeNamespace(params, result)
	case "update_namespace":
		return a.updateNamespace(params, result)
	case "get_namespaces":
		return a.getNamespaces(result)
	case "validate_config":
		return a.validateConfig(result)
	case "backup_config":
		return a.backupConfig(params, result)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported operation: %s", operation)
		return result, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// loadConfig loads the Aerospike configuration from file
func (a *AerospikeOperator) loadConfig() error {
	if _, err := os.Stat(a.configPath); os.IsNotExist(err) {
		// Create default config if it doesn't exist
		a.config = getDefaultConfig()
		return a.saveConfig()
	}

	config, err := LoadConfig(a.configPath)
	if err != nil {
		return err
	}

	a.config = config
	return nil
}

// saveConfig saves the current configuration to file
func (a *AerospikeOperator) saveConfig() error {
	return a.config.Save(a.configPath)
}

// getConfig returns the current Aerospike configuration
func (a *AerospikeOperator) getConfig(result *types.OperationResult) (*types.OperationResult, error) {
	result.Success = true
	result.Message = "Aerospike configuration retrieved successfully"
	result.Data = map[string]interface{}{
		"config":      a.config,
		"config_path": a.configPath,
	}

	return result, nil
}

// updateConfig updates the Aerospike configuration
func (a *AerospikeOperator) updateConfig(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	// Service configuration updates
	if service, ok := params["service"].(map[string]interface{}); ok {
		if user, ok := service["user"].(string); ok {
			a.config.Service.User = user
		}
		if group, ok := service["group"].(string); ok {
			a.config.Service.Group = group
		}
		if pidFile, ok := service["pids_file"].(string); ok {
			a.config.Service.PidsFile = pidFile
		}
		if protocolsEnabled, ok := service["protocols_enabled"].([]interface{}); ok {
			protocols := make([]string, len(protocolsEnabled))
			for i, p := range protocolsEnabled {
				if protocol, ok := p.(string); ok {
					protocols[i] = protocol
				}
			}
			a.config.Service.ProtocolsEnabled = protocols
		}
	}

	// Logging configuration updates
	if logging, ok := params["logging"].(map[string]interface{}); ok {
		if file, ok := logging["file"].(string); ok {
			a.config.Logging.File = file
		}
		if console, ok := logging["console"].(map[string]interface{}); ok {
			if context, ok := console["context"].(string); ok {
				a.config.Logging.Console.Context = context
			}
		}
	}

	// Network configuration updates
	if network, ok := params["network"].(map[string]interface{}); ok {
		if service, ok := network["service"].(map[string]interface{}); ok {
			if address, ok := service["address"].(string); ok {
				a.config.Network.Service.Address = address
			}
			if port, ok := service["port"].(int); ok {
				a.config.Network.Service.Port = port
			}
		}
		if heartbeat, ok := network["heartbeat"].(map[string]interface{}); ok {
			if mode, ok := heartbeat["mode"].(string); ok {
				a.config.Network.Heartbeat.Mode = mode
			}
			if address, ok := heartbeat["address"].(string); ok {
				a.config.Network.Heartbeat.Address = address
			}
			if port, ok := heartbeat["port"].(int); ok {
				a.config.Network.Heartbeat.Port = port
			}
			if interval, ok := heartbeat["interval"].(int); ok {
				a.config.Network.Heartbeat.Interval = interval
			}
			if timeout, ok := heartbeat["timeout"].(int); ok {
				a.config.Network.Heartbeat.Timeout = timeout
			}
		}
	}

	if err := a.saveConfig(); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to save config: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = "Aerospike configuration updated successfully"
	result.Data = map[string]interface{}{
		"config_path": a.configPath,
		"updated":     true,
	}

	return result, nil
}

// addNamespace adds a new namespace to the configuration
func (a *AerospikeOperator) addNamespace(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		result.Success = false
		result.Error = "namespace name is required"
		return result, fmt.Errorf("namespace name is required")
	}

	// Check if namespace already exists
	for _, ns := range a.config.Namespaces {
		if ns.Name == name {
			result.Success = false
			result.Error = fmt.Sprintf("namespace %s already exists", name)
			return result, fmt.Errorf("namespace %s already exists", name)
		}
	}

	// Create new namespace with defaults
	namespace := Namespace{
		Name:              name,
		ReplicationFactor: 2,
		MemorySize:        "1G",
		DefaultTTL:        "30d",
		HighWaterPercent:  60,
		StopWritesPercent: 90,
		Storage: Storage{
			Engine: "memory",
		},
	}

	// Override defaults with provided parameters
	if replicationFactor, ok := params["replication_factor"].(int); ok {
		namespace.ReplicationFactor = replicationFactor
	}
	if memorySize, ok := params["memory_size"].(string); ok {
		namespace.MemorySize = memorySize
	}
	if defaultTTL, ok := params["default_ttl"].(string); ok {
		namespace.DefaultTTL = defaultTTL
	}
	if highWaterPercent, ok := params["high_water_percent"].(int); ok {
		namespace.HighWaterPercent = highWaterPercent
	}
	if stopWritesPercent, ok := params["stop_writes_percent"].(int); ok {
		namespace.StopWritesPercent = stopWritesPercent
	}
	if storage, ok := params["storage"].(map[string]interface{}); ok {
		if engine, ok := storage["engine"].(string); ok {
			namespace.Storage.Engine = engine
		}
		if filesize, ok := storage["filesize"].(string); ok {
			namespace.Storage.Filesize = filesize
		}
		if dataInMemory, ok := storage["data_in_memory"].(bool); ok {
			namespace.Storage.DataInMemory = dataInMemory
		}
	}

	a.config.Namespaces = append(a.config.Namespaces, namespace)

	if err := a.saveConfig(); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to save config: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("Namespace %s added successfully", name)
	result.Data = map[string]interface{}{
		"namespace":   namespace,
		"config_path": a.configPath,
	}

	return result, nil
}

// removeNamespace removes a namespace from the configuration
func (a *AerospikeOperator) removeNamespace(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		result.Success = false
		result.Error = "namespace name is required"
		return result, fmt.Errorf("namespace name is required")
	}

	// Find and remove namespace
	found := false
	for i, ns := range a.config.Namespaces {
		if ns.Name == name {
			a.config.Namespaces = append(a.config.Namespaces[:i], a.config.Namespaces[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		result.Success = false
		result.Error = fmt.Sprintf("namespace %s not found", name)
		return result, fmt.Errorf("namespace %s not found", name)
	}

	if err := a.saveConfig(); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to save config: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("Namespace %s removed successfully", name)
	result.Data = map[string]interface{}{
		"namespace_name": name,
		"config_path":    a.configPath,
	}

	return result, nil
}

// updateNamespace updates an existing namespace
func (a *AerospikeOperator) updateNamespace(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		result.Success = false
		result.Error = "namespace name is required"
		return result, fmt.Errorf("namespace name is required")
	}

	// Find namespace
	var namespace *Namespace
	for i := range a.config.Namespaces {
		if a.config.Namespaces[i].Name == name {
			namespace = &a.config.Namespaces[i]
			break
		}
	}

	if namespace == nil {
		result.Success = false
		result.Error = fmt.Sprintf("namespace %s not found", name)
		return result, fmt.Errorf("namespace %s not found", name)
	}

	// Update namespace properties
	if replicationFactor, ok := params["replication_factor"].(int); ok {
		namespace.ReplicationFactor = replicationFactor
	}
	if memorySize, ok := params["memory_size"].(string); ok {
		namespace.MemorySize = memorySize
	}
	if defaultTTL, ok := params["default_ttl"].(string); ok {
		namespace.DefaultTTL = defaultTTL
	}
	if highWaterPercent, ok := params["high_water_percent"].(int); ok {
		namespace.HighWaterPercent = highWaterPercent
	}
	if stopWritesPercent, ok := params["stop_writes_percent"].(int); ok {
		namespace.StopWritesPercent = stopWritesPercent
	}
	if storage, ok := params["storage"].(map[string]interface{}); ok {
		if engine, ok := storage["engine"].(string); ok {
			namespace.Storage.Engine = engine
		}
		if filesize, ok := storage["filesize"].(string); ok {
			namespace.Storage.Filesize = filesize
		}
		if dataInMemory, ok := storage["data_in_memory"].(bool); ok {
			namespace.Storage.DataInMemory = dataInMemory
		}
	}

	if err := a.saveConfig(); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to save config: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("Namespace %s updated successfully", name)
	result.Data = map[string]interface{}{
		"namespace":   *namespace,
		"config_path": a.configPath,
	}

	return result, nil
}

// getNamespaces returns all configured namespaces
func (a *AerospikeOperator) getNamespaces(result *types.OperationResult) (*types.OperationResult, error) {
	result.Success = true
	result.Message = "Namespaces retrieved successfully"
	result.Data = map[string]interface{}{
		"namespaces":      a.config.Namespaces,
		"namespace_count": len(a.config.Namespaces),
		"config_path":     a.configPath,
	}

	return result, nil
}

// validateConfig validates the current configuration
func (a *AerospikeOperator) validateConfig(result *types.OperationResult) (*types.OperationResult, error) {
	errors := []string{}

	// Validate service configuration
	if a.config.Service.User == "" {
		errors = append(errors, "service user is required")
	}
	if a.config.Service.Group == "" {
		errors = append(errors, "service group is required")
	}

	// Validate network configuration
	if a.config.Network.Service.Address == "" {
		errors = append(errors, "service address is required")
	}
	if a.config.Network.Service.Port <= 0 || a.config.Network.Service.Port > 65535 {
		errors = append(errors, "service port must be between 1 and 65535")
	}

	// Validate namespaces
	if len(a.config.Namespaces) == 0 {
		errors = append(errors, "at least one namespace is required")
	}

	for _, ns := range a.config.Namespaces {
		if ns.Name == "" {
			errors = append(errors, "namespace name is required")
		}
		if ns.ReplicationFactor <= 0 {
			errors = append(errors, fmt.Sprintf("namespace %s: replication factor must be greater than 0", ns.Name))
		}
		if ns.MemorySize == "" {
			errors = append(errors, fmt.Sprintf("namespace %s: memory size is required", ns.Name))
		}
	}

	if len(errors) > 0 {
		result.Success = false
		result.Error = "Configuration validation failed"
		result.Data = map[string]interface{}{
			"errors": errors,
			"valid":  false,
		}
		return result, fmt.Errorf("configuration validation failed: %v", errors)
	}

	result.Success = true
	result.Message = "Configuration is valid"
	result.Data = map[string]interface{}{
		"valid":           true,
		"config_path":     a.configPath,
		"namespaces":      len(a.config.Namespaces),
		"validation_time": time.Now(),
	}

	return result, nil
}

// backupConfig creates a backup of the current configuration
func (a *AerospikeOperator) backupConfig(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	backupPath, ok := params["backup_path"].(string)
	if !ok || backupPath == "" {
		// Generate default backup path
		timestamp := time.Now().Format("20060102_150405")
		backupPath = fmt.Sprintf("/tmp/aerospike_config_backup_%s.conf", timestamp)
	}

	// Copy current config file to backup location
	if err := a.config.Save(backupPath); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create backup: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = "Configuration backup created successfully"
	result.Data = map[string]interface{}{
		"backup_path":   backupPath,
		"original_path": a.configPath,
		"backup_time":   time.Now(),
		"namespaces":    len(a.config.Namespaces),
	}

	return result, nil
}

// Cleanup performs cleanup when the operator is being stopped
func (a *AerospikeOperator) Cleanup() error {
	// Perform any necessary cleanup
	return nil
}

// GetOperationSchema returns the schema for all operations
func (a *AerospikeOperator) GetOperationSchema() types.OperatorSchema {
	info := a.Info()

	return types.OperatorSchema{
		Name:        info.Name,
		Version:     info.Version,
		Description: info.Description,
		Author:      info.Author,
		Operations: map[string]types.OperationSchema{
			"get_config": {
				Description: "Get current Aerospike configuration",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"update_config": {
				Description: "Update Aerospike configuration",
				Parameters: map[string]types.ParameterSchema{
					"config": {
						Type:        "object",
						Required:    true,
						Description: "Configuration object with Aerospike settings",
						Example: map[string]interface{}{
							"service": map[string]interface{}{
								"paxos-single-replica-limit": 1,
								"proto-fd-max":               15000,
							},
						},
					},
				},
				Examples: []map[string]interface{}{
					{
						"config": map[string]interface{}{
							"service": map[string]interface{}{
								"paxos-single-replica-limit": 1,
								"proto-fd-max":               15000,
							},
							"network": map[string]interface{}{
								"service": map[string]interface{}{
									"address": "0.0.0.0",
									"port":    3000,
								},
							},
						},
					},
				},
			},
			"add_namespace": {
				Description: "Add a new namespace to Aerospike configuration",
				Parameters: map[string]types.ParameterSchema{
					"namespace": {
						Type:        "string",
						Required:    true,
						Description: "Name of the namespace to add",
						Example:     "test",
					},
					"config": {
						Type:        "object",
						Required:    true,
						Description: "Namespace configuration",
						Example: map[string]interface{}{
							"replication-factor": 2,
							"memory-size":        "1G",
							"default-ttl":        "30d",
						},
					},
				},
				Examples: []map[string]interface{}{
					{
						"namespace": "test",
						"config": map[string]interface{}{
							"replication-factor": 2,
							"memory-size":        "1G",
							"default-ttl":        "30d",
							"storage-engine": map[string]interface{}{
								"type":      "memory",
								"data-size": "1G",
							},
						},
					},
				},
			},
			"remove_namespace": {
				Description: "Remove a namespace from Aerospike configuration",
				Parameters: map[string]types.ParameterSchema{
					"namespace": {
						Type:        "string",
						Required:    true,
						Description: "Name of the namespace to remove",
						Example:     "test",
					},
				},
				Examples: []map[string]interface{}{
					{"namespace": "test"},
				},
			},
			"update_namespace": {
				Description: "Update an existing namespace configuration",
				Parameters: map[string]types.ParameterSchema{
					"namespace": {
						Type:        "string",
						Required:    true,
						Description: "Name of the namespace to update",
						Example:     "test",
					},
					"config": {
						Type:        "object",
						Required:    true,
						Description: "Updated namespace configuration",
						Example: map[string]interface{}{
							"memory-size": "2G",
							"default-ttl": "60d",
						},
					},
				},
				Examples: []map[string]interface{}{
					{
						"namespace": "test",
						"config": map[string]interface{}{
							"memory-size": "2G",
							"default-ttl": "60d",
						},
					},
				},
			},
			"validate_config": {
				Description: "Validate the current Aerospike configuration",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"backup_config": {
				Description: "Create a backup of the current configuration",
				Parameters: map[string]types.ParameterSchema{
					"backup_path": {
						Type:        "string",
						Required:    false,
						Description: "Path to save the backup (auto-generated if not provided)",
						Example:     "/backup/aerospike_config_20240612.conf",
					},
				},
				Examples: []map[string]interface{}{
					{},
					{"backup_path": "/backup/aerospike_config_custom.conf"},
				},
			},
			"restore_config": {
				Description: "Restore configuration from a backup",
				Parameters: map[string]types.ParameterSchema{
					"backup_path": {
						Type:        "string",
						Required:    true,
						Description: "Path to the backup file to restore",
						Example:     "/backup/aerospike_config_20240612.conf",
					},
				},
				Examples: []map[string]interface{}{
					{"backup_path": "/backup/aerospike_config_20240612.conf"},
				},
			},
			"reload_config": {
				Description: "Reload Aerospike configuration (restart service)",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"status": {
				Description: "Get Aerospike service status",
				Parameters:  map[string]types.ParameterSchema{},
			},
		},
	}
}
