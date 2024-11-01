// internal/operator/manager.go
package operator

import (
	"agent/internal/types"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type OperatorManager struct {
	operators    map[string]types.Operator
	operatorsMu  sync.RWMutex
	clusterOps   types.ClusterOperations
	executions   map[string]*types.DistributedExecution
	executionsMu sync.RWMutex
}

func NewOperatorManager() *OperatorManager {
	return &OperatorManager{
		operators:  make(map[string]types.Operator),
		executions: make(map[string]*types.DistributedExecution),
	}
}

func (m *OperatorManager) SetClusterOperations(ops types.ClusterOperations) {
	m.clusterOps = ops
}

func (m *OperatorManager) RegisterOperator(op types.Operator) error {
	m.operatorsMu.Lock()
	defer m.operatorsMu.Unlock()

	name := op.Info().Name
	if _, exists := m.operators[name]; exists {
		return fmt.Errorf("operator %s already registered", name)
	}

	m.operators[name] = op
	return nil
}

func (m *OperatorManager) GetOperator(name string) (types.Operator, error) {
	m.operatorsMu.RLock()
	defer m.operatorsMu.RUnlock()

	op, exists := m.operators[name]
	if !exists {
		return nil, fmt.Errorf("operator %s not found", name)
	}

	return op, nil
}

func (m *OperatorManager) ExecuteOperator(ctx context.Context, name string, params map[string]interface{}) error {
	log.Printf("[OperatorManager] Starting execution for operator: %s", name)
	log.Printf("[OperatorManager] Params received: %+v", params)

	// Generate execution ID
	executionID := uuid.New().String()
	log.Printf("[OperatorManager] Generated execution ID: %s", executionID)

	// Create execution message
	execMsg := types.OperatorExecMessage{
		ExecutionID:  executionID,
		OperatorName: name,
		Operation:    params["operation"].(string),
		Params:       params["params"].(map[string]interface{}),
		Config:       params["config"].(map[string]interface{}),
		Parallel:     true,
	}
	log.Printf("[OperatorManager] Created execution message: %+v", execMsg)

	// Check leadership status
	isLeader := m.clusterOps.IsLeader()
	log.Printf("[OperatorManager] Current node is leader: %v", isLeader)

	// If we're not the leader, forward to leader
	if !isLeader {
		log.Printf("[OperatorManager] Not leader, forwarding request to leader")
		err := m.clusterOps.SendExecRequestToLeader(execMsg)
		if err != nil {
			log.Printf("[OperatorManager] Error forwarding to leader: %v", err)
			return err
		}
		log.Printf("[OperatorManager] Successfully forwarded request to leader")
		return nil
	}

	log.Printf("[OperatorManager] We are leader, proceeding with execution")
	execution := types.NewDistributedExecution(executionID)

	// Store execution
	m.executionsMu.Lock()
	m.executions[executionID] = execution
	m.executionsMu.Unlock()
	log.Printf("[OperatorManager] Stored execution tracking for ID: %s", executionID)

	// Start gathering results in background
	log.Printf("[OperatorManager] Starting result gatherer for execution: %s", executionID)
	go m.gatherResults(execution)

	// Broadcast to all nodes including self
	log.Printf("[OperatorManager] Broadcasting execution to all nodes")
	if err := m.clusterOps.BroadcastOperatorExecution(execMsg); err != nil {
		log.Printf("[OperatorManager] Error broadcasting execution: %v", err)
		return fmt.Errorf("failed to broadcast execution: %w", err)
	}

	log.Printf("[OperatorManager] Successfully initiated distributed execution: %s", executionID)
	return nil
}

func (m *OperatorManager) gatherResults(execution *types.DistributedExecution) {
	nodeCount := m.clusterOps.GetNodeCount()
	resultCount := 0

	for {
		select {
		case result := <-execution.ResultsChan:
			m.executionsMu.Lock()
			execution.Results[result.NodeID] = result
			resultCount++

			if resultCount >= nodeCount {
				close(execution.Done)
				m.executionsMu.Unlock()
				return
			}
			m.executionsMu.Unlock()

		case <-time.After(30 * time.Second):
			m.executionsMu.Lock()
			close(execution.Done)
			m.executionsMu.Unlock()
			return
		}
	}
}

func (m *OperatorManager) HandleOperatorResult(result *types.OperatorResult) {
	m.executionsMu.Lock()
	defer m.executionsMu.Unlock()

	if execution, exists := m.executions[result.ExecutionID]; exists {
		execution.ResultsChan <- result
	}
}

// ListOperators returns a list of all registered operators
func (m *OperatorManager) ListOperators() []types.OperatorInfo {
	m.operatorsMu.RLock()
	defer m.operatorsMu.RUnlock()

	var operators []types.OperatorInfo
	for name := range m.operators {

		op, err := m.GetOperator(name)
		if err != nil {
			return nil
		}
		operators = append(operators, op.Info())
	}
	return operators
}
