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

// ExecutionResult stores the result of an operation execution
type ExecutionResult struct {
	ID            string                 `json:"id"`
	OperatorName  string                 `json:"operator_name"`
	Operation     string                 `json:"operation"`
	NodeID        string                 `json:"node_id"`
	Status        string                 `json:"status"` // "running", "success", "failed"
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Duration      string                 `json:"duration,omitempty"`
	Result        *types.OperationResult `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Params        map[string]interface{} `json:"params"`
	TargetNodes   []string               `json:"target_nodes,omitempty"`
	ExecutionType string                 `json:"execution_type"` // "local", "broadcast", "node", "rolling"
}

// OperatorManager manages all registered operators
type OperatorManager struct {
	operators       map[string]types.Operator
	clusterOps      types.ClusterOperations
	executions      map[string]*ExecutionResult
	executionsMutex sync.RWMutex
	mutex           sync.RWMutex
}

// NewOperatorManager creates a new operator manager
func NewOperatorManager() *OperatorManager {
	return &OperatorManager{
		operators:  make(map[string]types.Operator),
		executions: make(map[string]*ExecutionResult),
	}
}

// RegisterOperator registers a new operator
func (om *OperatorManager) RegisterOperator(operator types.Operator) error {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	info := operator.Info()
	if info.Name == "" {
		return fmt.Errorf("operator name cannot be empty")
	}

	if _, exists := om.operators[info.Name]; exists {
		return fmt.Errorf("operator %s is already registered", info.Name)
	}

	om.operators[info.Name] = operator
	log.Printf("Registered operator: %s (version: %s)", info.Name, info.Version)

	return nil
}

// UnregisterOperator unregisters an operator
func (om *OperatorManager) UnregisterOperator(name string) error {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	operator, exists := om.operators[name]
	if !exists {
		return fmt.Errorf("operator %s is not registered", name)
	}

	// Cleanup the operator
	if err := operator.Cleanup(); err != nil {
		log.Printf("Warning: cleanup failed for operator %s: %v", name, err)
	}

	delete(om.operators, name)
	log.Printf("Unregistered operator: %s", name)

	return nil
}

// GetOperator returns an operator by name
func (om *OperatorManager) GetOperator(name string) (types.Operator, error) {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	operator, exists := om.operators[name]
	if !exists {
		return nil, fmt.Errorf("operator %s is not registered", name)
	}

	return operator, nil
}

// ListOperators returns all registered operators
func (om *OperatorManager) ListOperators() map[string]types.OperatorInfo {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	result := make(map[string]types.OperatorInfo)
	for name, operator := range om.operators {
		result[name] = operator.Info()
	}

	return result
}

// ExecuteOperation executes an operation on the specified operator
func (om *OperatorManager) ExecuteOperation(ctx context.Context, operatorName, operation string, params map[string]interface{}) (*types.OperationResult, error) {
	operator, err := om.GetOperator(operatorName)
	if err != nil {
		return nil, err
	}

	return operator.Execute(ctx, operation, params)
}

// SetClusterOperations sets the cluster operations interface
func (om *OperatorManager) SetClusterOperations(clusterOps types.ClusterOperations) {
	om.mutex.Lock()
	defer om.mutex.Unlock()
	om.clusterOps = clusterOps
}

// GetClusterOperations returns the cluster operations interface
func (om *OperatorManager) GetClusterOperations() types.ClusterOperations {
	om.mutex.RLock()
	defer om.mutex.RUnlock()
	return om.clusterOps
}

// BroadcastOperation broadcasts an operation to all nodes in the cluster
func (om *OperatorManager) BroadcastOperation(ctx context.Context, operatorName, operation string, params map[string]interface{}) error {
	if om.clusterOps == nil {
		return fmt.Errorf("cluster operations not available")
	}

	request := types.OperatorRequest{
		Operation: operation,
		Params:    params,
		Parallel:  true,
	}

	return om.clusterOps.BroadcastToNodes(fmt.Sprintf("operator:%s", operatorName), request)
}

// SendOperationToNode sends an operation to a specific node
func (om *OperatorManager) SendOperationToNode(ctx context.Context, nodeID, operatorName, operation string, params map[string]interface{}) error {
	if om.clusterOps == nil {
		return fmt.Errorf("cluster operations not available")
	}

	request := types.OperatorRequest{
		Operation: operation,
		Params:    params,
		NodeID:    nodeID,
	}

	return om.clusterOps.SendToNode(nodeID, fmt.Sprintf("operator:%s", operatorName), request)
}

// HandleOperatorRequest handles an incoming operator request
func (om *OperatorManager) HandleOperatorRequest(ctx context.Context, operatorName string, request *types.OperatorRequest) *types.OperatorResponse {
	result, err := om.ExecuteOperation(ctx, operatorName, request.Operation, request.Params)

	response := &types.OperatorResponse{
		Operation: request.Operation,
		NodeID:    om.getNodeID(),
	}

	if err != nil {
		response.Success = false
		response.Error = err.Error()
		response.Message = fmt.Sprintf("Operation %s failed on operator %s", request.Operation, operatorName)
	} else {
		response.Success = result.Success
		response.Message = result.Message
		response.Data = result.Data
		response.Error = result.Error
		response.Timestamp = result.Timestamp
	}

	return response
}

// getNodeID returns the current node ID
func (om *OperatorManager) getNodeID() string {
	if om.clusterOps != nil {
		return om.clusterOps.GetNodeID()
	}
	return "unknown"
}

// Shutdown shuts down all operators
func (om *OperatorManager) Shutdown() {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	for name, operator := range om.operators {
		if err := operator.Cleanup(); err != nil {
			log.Printf("Warning: cleanup failed for operator %s: %v", name, err)
		}
	}

	om.operators = make(map[string]types.Operator)
	log.Println("All operators shut down")
}

// ExecuteOperationWithTracking executes an operation with full tracking and logging
func (om *OperatorManager) ExecuteOperationWithTracking(ctx context.Context, operatorName, operation string, params map[string]interface{}, executionType string, targetNodes []string) (*ExecutionResult, error) {
	// Create execution record
	execution := &ExecutionResult{
		ID:            generateExecutionID(),
		OperatorName:  operatorName,
		Operation:     operation,
		NodeID:        om.getNodeID(),
		Status:        "running",
		StartTime:     time.Now(),
		Params:        params,
		TargetNodes:   targetNodes,
		ExecutionType: executionType,
	}

	// Store execution
	om.storeExecution(execution)

	// Log execution start
	log.Printf("[Execution %s] Starting %s operation '%s' on operator '%s'",
		execution.ID, executionType, operation, operatorName)

	var result *types.OperationResult
	var err error

	// Handle different execution types
	switch executionType {
	case "node":
		// Send to specific node
		if len(targetNodes) > 0 && targetNodes[0] != om.getNodeID() {
			// Send to remote node
			if om.clusterOps != nil {
				err = om.SendOperationToNode(ctx, targetNodes[0], operatorName, operation, params)
				if err == nil {
					// Create a success result for remote execution
					result = &types.OperationResult{
						Success:   true,
						Message:   fmt.Sprintf("Operation sent to node %s", targetNodes[0]),
						Timestamp: time.Now(),
						NodeID:    targetNodes[0],
					}
				}
			} else {
				err = fmt.Errorf("cluster operations not available")
			}
		} else {
			// Execute locally
			result, err = om.ExecuteOperation(ctx, operatorName, operation, params)
		}
	case "broadcast":
		// Broadcast to all nodes
		if om.clusterOps != nil {
			err = om.BroadcastOperation(ctx, operatorName, operation, params)
			if err == nil {
				result = &types.OperationResult{
					Success:   true,
					Message:   "Operation broadcasted to all nodes",
					Timestamp: time.Now(),
					NodeID:    om.getNodeID(),
				}
			}
		} else {
			// Fallback to local execution
			result, err = om.ExecuteOperation(ctx, operatorName, operation, params)
		}
	default:
		// Local execution
		result, err = om.ExecuteOperation(ctx, operatorName, operation, params)
	}

	// Update execution record
	endTime := time.Now()
	execution.EndTime = &endTime
	execution.Duration = endTime.Sub(execution.StartTime).String()

	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
		log.Printf("[Execution %s] FAILED after %s: %v", execution.ID, execution.Duration, err)
	} else {
		execution.Status = "success"
		execution.Result = result
		log.Printf("[Execution %s] SUCCESS after %s: %s", execution.ID, execution.Duration, result.Message)
	}

	// Update stored execution
	om.storeExecution(execution)

	return execution, err
}

// BroadcastOperationWithTracking broadcasts an operation to all nodes with tracking
func (om *OperatorManager) BroadcastOperationWithTracking(ctx context.Context, operatorName, operation string, params map[string]interface{}) (*ExecutionResult, error) {
	nodes := []string{}
	if om.clusterOps != nil {
		clusterNodes := om.clusterOps.GetNodes()
		for nodeID := range clusterNodes {
			nodes = append(nodes, nodeID)
		}
	}

	execution, err := om.ExecuteOperationWithTracking(ctx, operatorName, operation, params, "broadcast", nodes)

	if err == nil && om.clusterOps != nil {
		// Actually broadcast to cluster
		broadcastErr := om.BroadcastOperation(ctx, operatorName, operation, params)
		if broadcastErr != nil {
			execution.Status = "failed"
			execution.Error = fmt.Sprintf("Broadcast failed: %v", broadcastErr)
			now := time.Now()
			execution.EndTime = &now
			execution.Duration = now.Sub(execution.StartTime).String()
			om.storeExecution(execution)
			return execution, broadcastErr
		}
	}

	return execution, err
}

// RollingExecuteOperation executes an operation across nodes in specified order
func (om *OperatorManager) RollingExecuteOperation(ctx context.Context, operatorName, operation string, params map[string]interface{}, nodeOrder []string) (*ExecutionResult, error) {
	execution := &ExecutionResult{
		ID:            generateExecutionID(),
		OperatorName:  operatorName,
		Operation:     operation,
		NodeID:        om.getNodeID(),
		Status:        "running",
		StartTime:     time.Now(),
		Params:        params,
		TargetNodes:   nodeOrder,
		ExecutionType: "rolling",
	}

	om.storeExecution(execution)

	log.Printf("[Execution %s] Starting rolling execution across %d nodes: %v",
		execution.ID, len(nodeOrder), nodeOrder)

	var lastErr error
	for i, nodeID := range nodeOrder {
		log.Printf("[Execution %s] Step %d/%d: Executing on node %s",
			execution.ID, i+1, len(nodeOrder), nodeID)

		if nodeID == om.getNodeID() {
			// Execute locally
			_, err := om.ExecuteOperation(ctx, operatorName, operation, params)
			if err != nil {
				lastErr = err
				log.Printf("[Execution %s] Failed on node %s: %v", execution.ID, nodeID, err)
				break
			}
		} else if om.clusterOps != nil {
			// Send to remote node
			err := om.SendOperationToNode(ctx, nodeID, operatorName, operation, params)
			if err != nil {
				lastErr = err
				log.Printf("[Execution %s] Failed to send to node %s: %v", execution.ID, nodeID, err)
				break
			}
		}

		log.Printf("[Execution %s] Completed step %d/%d on node %s",
			execution.ID, i+1, len(nodeOrder), nodeID)

		// Add delay between nodes if specified
		if delay, ok := params["rolling_delay"].(string); ok && i < len(nodeOrder)-1 {
			if duration, err := time.ParseDuration(delay); err == nil {
				log.Printf("[Execution %s] Waiting %s before next node", execution.ID, delay)
				time.Sleep(duration)
			}
		}
	}

	// Update execution record
	endTime := time.Now()
	execution.EndTime = &endTime
	execution.Duration = endTime.Sub(execution.StartTime).String()

	if lastErr != nil {
		execution.Status = "failed"
		execution.Error = lastErr.Error()
		log.Printf("[Execution %s] Rolling execution FAILED after %s", execution.ID, execution.Duration)
	} else {
		execution.Status = "success"
		log.Printf("[Execution %s] Rolling execution SUCCESS after %s", execution.ID, execution.Duration)
	}

	om.storeExecution(execution)
	return execution, lastErr
}

// GetExecution returns an execution result by ID
func (om *OperatorManager) GetExecution(executionID string) (*ExecutionResult, error) {
	om.executionsMutex.RLock()
	defer om.executionsMutex.RUnlock()

	execution, exists := om.executions[executionID]
	if !exists {
		return nil, fmt.Errorf("execution %s not found", executionID)
	}

	return execution, nil
}

// ListExecutions returns all execution results
func (om *OperatorManager) ListExecutions() []*ExecutionResult {
	om.executionsMutex.RLock()
	defer om.executionsMutex.RUnlock()

	executions := make([]*ExecutionResult, 0, len(om.executions))
	for _, execution := range om.executions {
		executions = append(executions, execution)
	}

	return executions
}

// GetRecentExecutions returns recent execution results (last 100)
func (om *OperatorManager) GetRecentExecutions(limit int) []*ExecutionResult {
	if limit <= 0 {
		limit = 100
	}

	allExecutions := om.ListExecutions()

	// Sort by start time (most recent first)
	for i := 0; i < len(allExecutions)-1; i++ {
		for j := i + 1; j < len(allExecutions); j++ {
			if allExecutions[i].StartTime.Before(allExecutions[j].StartTime) {
				allExecutions[i], allExecutions[j] = allExecutions[j], allExecutions[i]
			}
		}
	}

	if len(allExecutions) > limit {
		allExecutions = allExecutions[:limit]
	}

	return allExecutions
}

// storeExecution stores an execution result
func (om *OperatorManager) storeExecution(execution *ExecutionResult) {
	om.executionsMutex.Lock()
	defer om.executionsMutex.Unlock()

	om.executions[execution.ID] = execution

	// Keep only last 1000 executions to prevent memory leak
	if len(om.executions) > 1000 {
		// Find oldest execution
		var oldestID string
		var oldestTime time.Time
		first := true

		for id, exec := range om.executions {
			if first || exec.StartTime.Before(oldestTime) {
				oldestID = id
				oldestTime = exec.StartTime
				first = false
			}
		}

		if oldestID != "" {
			delete(om.executions, oldestID)
		}
	}
}

// generateExecutionID generates a unique execution ID using UUID
func generateExecutionID() string {
	return uuid.New().String()
}

// NodeExecuteOperation executes an operation on a specific node with tracking
func (om *OperatorManager) NodeExecuteOperation(ctx context.Context, nodeID, operatorName, operation string, params map[string]interface{}) (*ExecutionResult, error) {
	execution := &ExecutionResult{
		ID:            generateExecutionID(),
		OperatorName:  operatorName,
		Operation:     operation,
		NodeID:        om.getNodeID(),
		Status:        "running",
		StartTime:     time.Now(),
		Params:        params,
		TargetNodes:   []string{nodeID},
		ExecutionType: "node",
	}

	// Store execution
	om.storeExecution(execution)

	// Log execution start
	log.Printf("[Execution %s] Starting node operation '%s' on operator '%s' (target: %s)",
		execution.ID, operation, operatorName, nodeID)

	var err error
	if nodeID == om.getNodeID() {
		// Execute locally if target is current node
		_, err = om.ExecuteOperation(ctx, operatorName, operation, params)
	} else if om.clusterOps != nil {
		// Send to remote node
		err = om.SendOperationToNode(ctx, nodeID, operatorName, operation, params)
	} else {
		err = fmt.Errorf("cluster operations not available for remote execution")
	}

	// Update execution record
	endTime := time.Now()
	execution.EndTime = &endTime
	execution.Duration = endTime.Sub(execution.StartTime).String()

	if err != nil {
		execution.Status = "failed"
		execution.Error = err.Error()
		log.Printf("[Execution %s] FAILED after %s: %v", execution.ID, execution.Duration, err)
	} else {
		execution.Status = "success"
		log.Printf("[Execution %s] SUCCESS after %s: Operation sent to node %s", execution.ID, execution.Duration, nodeID)
	}

	// Update stored execution
	om.storeExecution(execution)

	return execution, err
}
