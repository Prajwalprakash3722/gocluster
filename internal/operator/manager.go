package operator

import (
	"context"
	"fmt"
	"sync"
)

// OperatorManager handles registration and execution of operators
type OperatorManager struct {
	operators map[string]Operator
	mu        sync.RWMutex
}

// NewOperatorManager creates a new operator manager
func NewOperatorManager() *OperatorManager {
	return &OperatorManager{
		operators: make(map[string]Operator),
	}
}

// RegisterOperator registers a new operator
func (m *OperatorManager) RegisterOperator(op Operator) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := op.Info().Name
	if _, exists := m.operators[name]; exists {
		return fmt.Errorf("operator %s already registered", name)
	}

	m.operators[name] = op
	return nil
}

// ExecuteOperator executes a specific operator
func (m *OperatorManager) ExecuteOperator(ctx context.Context, name string, params map[string]interface{}) error {
	m.mu.RLock()
	op, exists := m.operators[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("operator %s not found", name)
	}

	if err := op.Execute(ctx, params); err != nil {
		if rbErr := op.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("execution failed: %v, rollback failed: %v", err, rbErr)
		}
		return fmt.Errorf("execution failed and rolled back: %v", err)
	}

	return nil
}
