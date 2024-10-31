package operator

import (
	"context"
	"fmt"
	"sync"
)

// OperatorManager handles registration and execution of operators
type OperatorManager struct {
	operators   map[string]Operator
	operatorsMu sync.RWMutex
}

// NewOperatorManager creates a new operator manager
func NewOperatorManager() *OperatorManager {
	return &OperatorManager{
		operators: make(map[string]Operator),
	}
}

// RegisterOperator registers a new operator
func (m *OperatorManager) RegisterOperator(op Operator) error {
	m.operatorsMu.Lock()
	defer m.operatorsMu.Unlock()

	if m.operators == nil {
		m.operators = make(map[string]Operator)
	}

	name := op.Info().Name
	if _, exists := m.operators[name]; exists {
		return fmt.Errorf("operator %s already registered", name)
	}

	m.operators[name] = op
	return nil
}

func (m *OperatorManager) GetOperator(name string) (Operator, error) {
	m.operatorsMu.RLock()
	defer m.operatorsMu.RUnlock()

	op, exists := m.operators[name]
	if !exists {
		return nil, fmt.Errorf("operator %s not found", name)
	}

	return op, nil
}

// ListOperators returns a list of all registered operators
func (m *OperatorManager) ListOperators() []OperatorInfo {
	m.operatorsMu.RLock()
	defer m.operatorsMu.RUnlock()

	var operators []OperatorInfo
	for name := range m.operators {

		op, err := m.GetOperator(name)
		if err != nil {
			return nil
		}
		operators = append(operators, op.Info())
	}
	return operators
}

func (m *OperatorManager) ExecuteOperator(ctx context.Context, name string, params map[string]interface{}) error {
	op, err := m.GetOperator(name)
	if err != nil {
		return fmt.Errorf("operator %s not found", name)
	}

	// Check if operation parameter exists
	operation, exists := params["operation"].(string)
	if !exists {
		return fmt.Errorf("operation parameter is missing")
	}

	// Get operator info for validation
	info := op.Info()

	// Check if operation exists in schema
	opSchema, exists := info.Operations[operation]
	if !exists {
		return fmt.Errorf("operation %s not found for operator %s", operation, name)
	}

	// Extract the nested parameters
	var operationParams map[string]interface{}
	if p, ok := params["params"].(map[string]interface{}); ok {
		operationParams = p
	} else {
		operationParams = make(map[string]interface{})
	}

	// Extract config parameters
	var configParams map[string]interface{}
	if c, ok := params["config"].(map[string]interface{}); ok {
		configParams = c
	} else {
		configParams = make(map[string]interface{})
	}

	// Validate operation parameters
	if err := validateParams(operationParams, opSchema.Parameters); err != nil {
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	// Validate config parameters if schema has config requirements
	if opSchema.Config != nil {
		if err := validateParams(configParams, opSchema.Config); err != nil {
			return fmt.Errorf("config validation failed: %w", err)
		}
	}

	// Execute the operator with the original params to maintain structure
	if err := op.Execute(ctx, params); err != nil {
		return fmt.Errorf("operator execution failed: %w", err)
	}

	return op.Cleanup()
}

func validateParams(params map[string]interface{}, schema map[string]ParamSchema) error {
	for name, paramSchema := range schema {
		if paramSchema.Required {
			value, exists := params[name]
			if !exists {
				return fmt.Errorf("required parameter %s is missing", name)
			}
			if value == nil {
				return fmt.Errorf("required parameter %s cannot be nil", name)
			}
		}
	}
	return nil
}
