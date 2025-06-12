package operator

import (
	aerospike_operator "agent/internal/operator/plugins/aerospike"
	hello_operator "agent/internal/operator/plugins/hello"
	jobs_operator "agent/internal/operator/plugins/jobs"
	mysql_operator "agent/internal/operator/plugins/mysql"
	"agent/internal/types"
)

// GlobalRegistry is the global plugin registry
var GlobalRegistry *PluginRegistry

// init automatically registers all available operators
func init() {
	GlobalRegistry = NewPluginRegistry()

	// Register all operators with their factories and aliases
	registerAllOperators()
}

// registerAllOperators registers all available operators
func registerAllOperators() {
	// Hello operator
	GlobalRegistry.RegisterPlugin("hello",
		func() types.Operator { return hello_operator.New() },
		"hello-world", "test", "hello-operator")

	// MySQL operator
	GlobalRegistry.RegisterPlugin("mysql",
		func() types.Operator { return mysql_operator.New() },
		"mysql-operator", "mysql-db", "database")

	// Jobs operator
	GlobalRegistry.RegisterPlugin("jobs",
		func() types.Operator { return jobs_operator.New() },
		"job", "jobs-operator", "task", "tasks")

	// Aerospike operator
	GlobalRegistry.RegisterPlugin("aerospike",
		func() types.Operator { return aerospike_operator.New() },
		"aerospike-config", "aerospike-operator", "aero")
}

// LoadPlugins loads operators from a list using the global registry
func LoadPlugins(operatorManager *OperatorManager, plugins []string) error {
	return GlobalRegistry.AutoLoadOperators(operatorManager, plugins)
}

// ListAvailablePlugins returns all available plugins and their aliases
func ListAvailablePlugins() ([]string, map[string]string) {
	return GlobalRegistry.ListPlugins(), GlobalRegistry.ListAliases()
}

// CreateOperatorByName creates an operator by name using the global registry
func CreateOperatorByName(name string) (types.Operator, error) {
	return GlobalRegistry.CreateOperator(name)
}

// ValidatePluginName validates if a plugin name is available
func ValidatePluginName(name string) error {
	return GlobalRegistry.ValidatePlugin(name)
}
