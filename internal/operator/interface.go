package operator

import (
	"context"
)

// Operator defines the interface that all operators must implement
// Operator represents a plugin that can perform operations on the cluster
type Operator interface {
	// Name returns the unique identifier of the operator
	Name() string

	// Init initializes the operator with cluster configuration
	Init(config map[string]interface{}) error

	// Execute runs the operator's logic
	Execute(ctx context.Context, params map[string]interface{}) error

	// Rollback handles failure scenarios
	Rollback(ctx context.Context) error

	// Cleanup performs any necessary cleanup
	Cleanup() error
}

// OperatorInfo contains metadata about an operator
type OperatorInfo struct {
	Name        string          // Unique name of the operator
	Version     string          // Version of the operator
	Description string          // Description of what the operator does
	Author      string          // Author of the operator
	Parameters  []ParameterInfo // List of parameters the operator accepts
}

// ParameterInfo describes a parameter that the operator accepts
type ParameterInfo struct {
	Name        string // Name of the parameter
	Type        string // Type of the parameter (string, int, bool, etc)
	Required    bool   // Whether the parameter is required
	Description string // Description of what the parameter does
	Default     string // Default value, if any
}

// Status represents the current state of an operator execution
type Status struct {
	State    string                 // Current state (running, completed, failed)
	Progress int                    // Progress percentage (0-100)
	Message  string                 // Current status message
	Data     map[string]interface{} // Additional status data
}
