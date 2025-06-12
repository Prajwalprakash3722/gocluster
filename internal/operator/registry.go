package operator

import (
	"agent/internal/types"
	"fmt"
	"log"
)

// PluginFactory is a function that creates a new operator instance
type PluginFactory func() types.Operator

// PluginRegistry manages operator plugin registration and discovery
type PluginRegistry struct {
	factories map[string]PluginFactory
	aliases   map[string]string // alias -> canonical name
}

// NewPluginRegistry creates a new plugin registry
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		factories: make(map[string]PluginFactory),
		aliases:   make(map[string]string),
	}
}

// RegisterPlugin registers an operator factory with the registry
func (pr *PluginRegistry) RegisterPlugin(name string, factory PluginFactory, aliases ...string) {
	pr.factories[name] = factory

	// Register aliases
	for _, alias := range aliases {
		pr.aliases[alias] = name
	}

	log.Printf("Registered plugin: %s with %d aliases", name, len(aliases))
}

// CreateOperator creates an operator instance by name
func (pr *PluginRegistry) CreateOperator(name string) (types.Operator, error) {
	// Check if it's an alias
	if canonical, exists := pr.aliases[name]; exists {
		name = canonical
	}

	factory, exists := pr.factories[name]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	return factory(), nil
}

// ListPlugins returns all available plugin names
func (pr *PluginRegistry) ListPlugins() []string {
	var plugins []string
	for name := range pr.factories {
		plugins = append(plugins, name)
	}
	return plugins
}

// ListAliases returns all available aliases
func (pr *PluginRegistry) ListAliases() map[string]string {
	aliases := make(map[string]string)
	for alias, canonical := range pr.aliases {
		aliases[alias] = canonical
	}
	return aliases
}

// GetOperatorInfo returns operator info without creating an instance
func (pr *PluginRegistry) GetOperatorInfo(name string) (*types.OperatorInfo, error) {
	operator, err := pr.CreateOperator(name)
	if err != nil {
		return nil, err
	}

	info := operator.Info()
	return &info, nil
}

// AutoLoadOperators loads operators from a list with automatic configuration
func (pr *PluginRegistry) AutoLoadOperators(operatorManager *OperatorManager, plugins []string) error {
	for _, pluginName := range plugins {
		if err := pr.LoadOperator(operatorManager, pluginName); err != nil {
			log.Printf("Warning: Failed to load plugin %s: %v", pluginName, err)
			continue
		}
	}
	return nil
}

// LoadOperator loads a single operator with smart default configuration
func (pr *PluginRegistry) LoadOperator(operatorManager *OperatorManager, pluginName string) error {
	operator, err := pr.CreateOperator(pluginName)
	if err != nil {
		return fmt.Errorf("failed to create operator %s: %v", pluginName, err)
	}

	// Get smart default configuration
	config := pr.getDefaultConfig(pluginName, operator)

	// Initialize operator
	if err := operator.Init(config); err != nil {
		return fmt.Errorf("failed to initialize operator %s: %v", pluginName, err)
	}

	// Register operator
	if err := operatorManager.RegisterOperator(operator); err != nil {
		return fmt.Errorf("failed to register operator %s: %v", pluginName, err)
	}

	log.Printf("Successfully loaded operator: %s", operator.Info().Name)
	return nil
}

// getDefaultConfig returns smart default configuration for operators
func (pr *PluginRegistry) getDefaultConfig(pluginName string, _operator types.Operator) map[string]interface{} {
	config := make(map[string]interface{})

	switch pluginName {
	case "hello", "hello-world":
		config["default_greeting"] = "Hello"

	case "mysql":
		config["dsn"] = "user:password@tcp(localhost:3306)/"
		config["backup_dir"] = "/var/lib/mysql/backups"

	case "jobs":
		config["working_dir"] = "/tmp"
		config["timeout"] = "30m"
		config["log_dir"] = "/tmp/gocluster-jobs"

	case "aerospike", "aerospike-config":
		config["config_path"] = "/etc/aerospike/aerospike.conf"

	default:
	}

	return config
}

// ValidatePlugin checks if a plugin is valid and can be loaded
func (pr *PluginRegistry) ValidatePlugin(name string) error {
	operator, err := pr.CreateOperator(name)
	if err != nil {
		return err
	}

	info := operator.Info()
	if info.Name == "" {
		return fmt.Errorf("operator has empty name")
	}

	if info.Version == "" {
		return fmt.Errorf("operator has empty version")
	}

	if len(operator.GetOperations()) == 0 {
		return fmt.Errorf("operator has no operations")
	}

	return nil
}
