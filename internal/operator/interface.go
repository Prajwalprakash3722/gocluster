package operator

import (
	"context"
)

// Operator defines the interface that all operators must implement
// Operator represents a plugin that can perform operations on the cluster
type Operator interface {
	// Name returns the unique identifier of the operator
	Info() OperatorInfo

	// Init initializes the operator with cluster configuration
	Init(config map[string]interface{}) error

	// Execute runs the operator's logic
	Execute(ctx context.Context, params map[string]interface{}) error

	// Rollback handles failure scenarios
	Rollback(ctx context.Context) error

	// Cleanup performs any necessary cleanup
	Cleanup() error
}

// ParameterInfo describes a parameter that the operator accepts
type ParameterInfo struct {
	Name        string // Name of the parameter
	Type        string // Type of the parameter (string, int, bool, etc)
	Required    bool   // Whether the parameter is required
	Description string // Description of what the parameter does
	Default     string // Default value, if any
}

// OperatorInfo contains operator metadata and schema
type OperatorInfo struct {
	Name        string                     `json:"name"`
	Version     string                     `json:"version"`
	Description string                     `json:"description"`
	Author      string                     `json:"author"`
	Operations  map[string]OperationSchema `json:"operations"`
}

// OperationSchema describes an individual operation
type OperationSchema struct {
	Description string                 `json:"description"`
	Parameters  map[string]ParamSchema `json:"parameters"`
	Namespace   map[string]ParamSchema `json:"namespace"`
	Config      map[string]ParamSchema `json:"config"`
}

// ParamSchema describes parameter requirements
type ParamSchema struct {
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
}

// Status represents the current state of an operator execution
type Status struct {
	State    string                 // Current state (running, completed, failed)
	Progress int                    // Progress percentage (0-100)
	Message  string                 // Current status message
	Data     map[string]interface{} // Additional status data
}
