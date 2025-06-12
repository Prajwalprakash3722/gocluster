package web

import (
	"agent/internal/cluster"
	"agent/internal/operator"
	"agent/internal/types"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/css/* static/js/*
var staticFS embed.FS

// Handler handles web requests
type Handler struct {
	clusterManager  *cluster.Manager
	operatorManager *operator.OperatorManager
	templates       *template.Template
}

// NewHandler creates a new web handler
func NewHandler(clusterManager *cluster.Manager, operatorManager *operator.OperatorManager) (*Handler, error) {
	// Parse embedded templates
	templates, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded templates: %v", err)
	}

	return &Handler{
		clusterManager:  clusterManager,
		operatorManager: operatorManager,
		templates:       templates,
	}, nil
}

// SetupRoutes sets up all web routes
func (h *Handler) SetupRoutes(app *fiber.App) {
	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Static files served from embedded FS
	staticFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return
	}
	app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(staticFS),
	}))

	// Main page
	app.Get("/", h.handleIndex)

	// Git-style navigation routes
	app.Get("/dashboard", h.handleIndex)
	app.Get("/nodes", h.handleNodesPage)
	app.Get("/operators", h.handleOperatorsPage)
	app.Get("/operator-forms", h.handleOperatorFormsPage)
	app.Get("/operations", h.handleOperationsPage)
	app.Get("/logs", h.handleLogsPage)
	app.Get("/settings", h.handleSettingsPage)

	// API routes
	api := app.Group("/api")

	// Cluster endpoints
	api.Get("/status", h.handleStatus)
	api.Get("/nodes", h.handleNodes)
	api.Get("/leader", h.handleLeader)

	// Operator endpoints
	api.Get("/operators", h.handleListOperators)
	api.Get("/operators/:name", h.handleGetOperator)
	api.Get("/operator/schema/:name", h.handleGetOperatorSchema)
	api.Post("/operator/trigger/:name", h.handleTriggerOperator)

	// Enhanced operator execution endpoints
	api.Post("/operator/execute", h.handleExecuteOperation)
	api.Post("/operator/broadcast/:name", h.handleBroadcastOperation)
	api.Post("/operator/rolling/:name", h.handleRollingOperation)
	api.Get("/executions", h.handleListExecutions)
	api.Get("/executions/:id", h.handleGetExecution)
	api.Get("/executions/recent/:limit", h.handleRecentExecutions)

	// New API endpoints for enhanced UI
	api.Get("/operations/history", h.handleOperationHistory)
	api.Get("/logs/recent", h.handleRecentLogs)
	api.Post("/operator/execute", h.handleExecuteOperation)
	api.Get("/cluster/metrics", h.handleClusterMetrics)

	// Health check
	api.Get("/health", h.handleHealth)

	// Server-Sent Events for real-time updates
	app.Get("/events", h.handleEvents)
}

// handleIndex serves the main dashboard page
func (h *Handler) handleIndex(c *fiber.Ctx) error {
	clusterState := h.clusterManager.GetClusterState()
	operators := h.operatorManager.ListOperators()
	nodes := h.clusterManager.GetNodes()

	// Create a template data structure matching the original design
	data := struct {
		ClusterName string
		LocalNode   struct {
			ID      string
			State   string
			Address string
		}
		Leader       string
		Nodes        map[string]*types.Node
		Operators    map[string]types.OperatorInfo
		RunningNodes int
		LastUpdated  string
	}{
		ClusterName: "GoCluster", // You might want to get this from config
		LocalNode: struct {
			ID      string
			State   string
			Address string
		}{
			ID:      h.clusterManager.GetNodeID(),
			State:   getNodeState(h.clusterManager.IsLeader()),
			Address: "localhost:8080", // Complete address with port
		},
		Leader:       clusterState.Leader,
		Nodes:        nodes,
		Operators:    operators,
		RunningNodes: countRunningNodes(nodes),
		LastUpdated:  time.Now().Format("2006-01-02 15:04:05"),
	}

	// Render template
	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "status", data)
}

// handleStatus returns cluster status
func (h *Handler) handleStatus(c *fiber.Ctx) error {
	state := h.clusterManager.GetClusterState()

	return c.JSON(fiber.Map{
		"success": true,
		"data": map[string]interface{}{
			"cluster_state": state,
			"node_id":       h.clusterManager.GetNodeID(),
			"is_leader":     h.clusterManager.IsLeader(),
			"timestamp":     time.Now(),
		},
	})
}

// handleNodes returns all cluster nodes
func (h *Handler) handleNodes(c *fiber.Ctx) error {
	nodes := h.clusterManager.GetNodes()

	return c.JSON(fiber.Map{
		"success": true,
		"data": map[string]interface{}{
			"nodes": nodes,
			"count": len(nodes),
		},
	})
}

// handleLeader returns current leader information
func (h *Handler) handleLeader(c *fiber.Ctx) error {
	leaderID := h.clusterManager.GetLeader()
	nodes := h.clusterManager.GetNodes()

	var leaderNode *types.Node
	if leaderID != "" {
		leaderNode = nodes[leaderID]
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": map[string]interface{}{
			"leader_id":   leaderID,
			"leader_node": leaderNode,
			"is_leader":   h.clusterManager.IsLeader(),
		},
	})
}

// handleListOperators returns all registered operators
func (h *Handler) handleListOperators(c *fiber.Ctx) error {
	operators := h.operatorManager.ListOperators()

	return c.JSON(fiber.Map{
		"success": true,
		"data": map[string]interface{}{
			"operators": operators,
			"count":     len(operators),
		},
	})
}

// handleGetOperator returns information about a specific operator
func (h *Handler) handleGetOperator(c *fiber.Ctx) error {
	name := c.Params("name")

	operator, err := h.operatorManager.GetOperator(name)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Operator %s not found", name),
		})
	}

	info := operator.Info()
	operations := operator.GetOperations()

	return c.JSON(fiber.Map{
		"success": true,
		"data": map[string]interface{}{
			"info":       info,
			"operations": operations,
		},
	})
}

// handleTriggerOperator triggers an operator operation with enhanced tracking
func (h *Handler) handleTriggerOperator(c *fiber.Ctx) error {
	name := c.Params("name")

	var request types.OperatorRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	// Validate required fields
	if request.Operation == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Operation is required",
		})
	}

	if request.Params == nil {
		request.Params = make(map[string]interface{})
	}

	// Create context with timeout
	timeout := 30 * time.Second
	if timeoutStr := c.Query("timeout"); timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = t
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Determine execution type from request
	executionType := request.ExecutionType
	if executionType == "" {
		// Backward compatibility - infer from legacy fields
		if request.Parallel {
			executionType = "broadcast"
		} else if request.NodeID != "" {
			executionType = "node"
		} else if len(request.TargetNodes) > 1 {
			executionType = "rolling"
		} else {
			executionType = "local"
		}
	}

	switch executionType {
	case "broadcast":
		execution, err := h.operatorManager.BroadcastOperationWithTracking(ctx, name, request.Operation, request.Params)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Failed to broadcast operation: %v", err),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf("Operation %s broadcasted to all nodes", request.Operation),
			"data": fiber.Map{
				"execution_id":   execution.ID,
				"operation":      request.Operation,
				"execution_type": "broadcast",
				"status":         execution.Status,
				"target_nodes":   execution.TargetNodes,
				"timestamp":      execution.StartTime,
			},
		})

	case "rolling":
		nodeOrder := request.TargetNodes
		if len(nodeOrder) == 0 {
			// Default to all nodes
			nodes := h.clusterManager.GetNodes()
			for nodeID := range nodes {
				nodeOrder = append(nodeOrder, nodeID)
			}
		}

		// Add rolling delay to params if specified
		if request.RollingDelay != "" {
			if request.Params == nil {
				request.Params = make(map[string]interface{})
			}
			request.Params["rolling_delay"] = request.RollingDelay
		}

		execution, err := h.operatorManager.RollingExecuteOperation(ctx, name, request.Operation, request.Params, nodeOrder)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Failed to execute rolling operation: %v", err),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf("Rolling operation %s started across %d nodes", request.Operation, len(nodeOrder)),
			"data": fiber.Map{
				"execution_id":   execution.ID,
				"operation":      request.Operation,
				"execution_type": "rolling",
				"status":         execution.Status,
				"target_nodes":   nodeOrder,
				"rolling_delay":  request.RollingDelay,
				"timestamp":      execution.StartTime,
			},
		})

	case "node":
		targetNodeID := request.NodeID
		if targetNodeID == "" && len(request.TargetNodes) > 0 {
			targetNodeID = request.TargetNodes[0]
		}
		if targetNodeID == "" {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "node_id or target_nodes required for node execution",
			})
		}

		execution, err := h.operatorManager.NodeExecuteOperation(ctx, targetNodeID, name, request.Operation, request.Params)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   fmt.Sprintf("Failed to send operation to node: %v", err),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf("Operation %s sent to node %s", request.Operation, targetNodeID),
			"data": fiber.Map{
				"execution_id":   execution.ID,
				"operation":      request.Operation,
				"execution_type": "node",
				"node_id":        targetNodeID,
				"status":         execution.Status,
				"timestamp":      execution.StartTime,
			},
		})

	case "local":
		// Execute operation locally with tracking
		execution, err := h.operatorManager.ExecuteOperationWithTracking(ctx, name, request.Operation, request.Params, "local", []string{h.clusterManager.GetNodeID()})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf("Operation %s executed locally", request.Operation),
			"data": fiber.Map{
				"execution_id":   execution.ID,
				"operation":      request.Operation,
				"execution_type": "local",
				"status":         execution.Status,
				"result":         execution.Result,
				"duration":       execution.Duration,
				"timestamp":      execution.StartTime,
			},
		})

	default:
		// Default to local execution for unknown types
		execution, err := h.operatorManager.ExecuteOperationWithTracking(ctx, name, request.Operation, request.Params, "local", []string{h.clusterManager.GetNodeID()})
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": fmt.Sprintf("Operation %s executed locally", request.Operation),
			"data": fiber.Map{
				"execution_id":   execution.ID,
				"operation":      request.Operation,
				"execution_type": "local",
				"status":         execution.Status,
				"result":         execution.Result,
				"duration":       execution.Duration,
				"timestamp":      execution.StartTime,
			},
		})
	}
}

// handleGetOperatorSchema returns the schema for an operator's operations
func (h *Handler) handleGetOperatorSchema(c *fiber.Ctx) error {
	name := c.Params("name")

	operator, err := h.operatorManager.GetOperator(name)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Operator %s not found", name),
		})
	}

	// Get the dynamic schema from the operator
	schema := operator.GetOperationSchema()

	return c.JSON(fiber.Map{
		"success": true,
		"data":    schema,
	})
}

// handleHealth returns health status
func (h *Handler) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success": true,
		"data": map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now(),
			"node_id":   h.clusterManager.GetNodeID(),
		},
	})
}

// Git-inspired page handlers

// handleNodesPage serves the nodes management page
func (h *Handler) handleNodesPage(c *fiber.Ctx) error {
	nodes := h.clusterManager.GetNodes()

	data := map[string]interface{}{
		"Title":     "Cluster Nodes",
		"Nodes":     nodes,
		"NodeCount": len(nodes),
		"Running":   countRunningNodes(nodes),
		"Leader":    h.clusterManager.GetLeader(),
		"NodeID":    h.clusterManager.GetNodeID(),
		"IsLeader":  h.clusterManager.IsLeader(),
		"Timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}

	html := generateGitStylePage("nodes", data)
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// handleOperatorsPage serves the operators management page
func (h *Handler) handleOperatorsPage(c *fiber.Ctx) error {
	operators := h.operatorManager.ListOperators()

	data := map[string]interface{}{
		"Title":     "Cluster Operators",
		"Operators": operators,
		"Count":     len(operators),
		"NodeID":    h.clusterManager.GetNodeID(),
		"IsLeader":  h.clusterManager.IsLeader(),
		"Timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}

	html := generateGitStylePage("operators", data)
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// handleOperatorFormsPage serves the operator forms page
func (h *Handler) handleOperatorFormsPage(c *fiber.Ctx) error {
	clusterState := h.clusterManager.GetClusterState()
	operators := h.operatorManager.ListOperators()
	nodes := h.clusterManager.GetNodes()

	// Create template data structure matching the main dashboard
	data := struct {
		ClusterName string
		LocalNode   struct {
			ID      string
			State   string
			Address string
		}
		Leader       string
		Nodes        map[string]*types.Node
		Operators    map[string]types.OperatorInfo
		RunningNodes int
		LastUpdated  string
	}{
		ClusterName: "GoCluster",
		LocalNode: struct {
			ID      string
			State   string
			Address string
		}{
			ID:      h.clusterManager.GetNodeID(),
			State:   getNodeState(h.clusterManager.IsLeader()),
			Address: "localhost:8080",
		},
		Leader:       clusterState.Leader,
		Nodes:        nodes,
		Operators:    operators,
		RunningNodes: countRunningNodes(nodes),
		LastUpdated:  time.Now().Format("2006-01-02 15:04:05"),
	}

	c.Set("Content-Type", "text/html")
	return h.templates.ExecuteTemplate(c.Response().BodyWriter(), "operator-forms", data)
}

// handleOperationsPage serves the operations history page
func (h *Handler) handleOperationsPage(c *fiber.Ctx) error {
	data := map[string]interface{}{
		"Title":     "Operations History",
		"NodeID":    h.clusterManager.GetNodeID(),
		"IsLeader":  h.clusterManager.IsLeader(),
		"Timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}

	html := generateGitStylePage("operations", data)
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// handleLogsPage serves the logs viewer page
func (h *Handler) handleLogsPage(c *fiber.Ctx) error {
	data := map[string]interface{}{
		"Title":     "System Logs",
		"NodeID":    h.clusterManager.GetNodeID(),
		"IsLeader":  h.clusterManager.IsLeader(),
		"Timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}

	html := generateGitStylePage("logs", data)
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// handleSettingsPage serves the settings page
func (h *Handler) handleSettingsPage(c *fiber.Ctx) error {
	data := map[string]interface{}{
		"Title":     "Cluster Settings",
		"NodeID":    h.clusterManager.GetNodeID(),
		"IsLeader":  h.clusterManager.IsLeader(),
		"Timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}

	html := generateGitStylePage("settings", data)
	c.Set("Content-Type", "text/html")
	return c.SendString(html)
}

// Enhanced API handlers

// handleOperationHistory returns operation history
func (h *Handler) handleOperationHistory(c *fiber.Ctx) error {
	// Mock data for now - in real implementation, this would come from a database
	history := []map[string]interface{}{
		{
			"id":        "op-001",
			"operator":  "hello",
			"operation": "greet",
			"status":    "success",
			"node":      h.clusterManager.GetNodeID(),
			"timestamp": time.Now().Add(-5 * time.Minute),
			"duration":  "250ms",
		},
		{
			"id":        "op-002",
			"operator":  "mysql",
			"operation": "backup",
			"status":    "success",
			"node":      h.clusterManager.GetNodeID(),
			"timestamp": time.Now().Add(-15 * time.Minute),
			"duration":  "1.2s",
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    history,
	})
}

// handleRecentLogs returns recent system logs
func (h *Handler) handleRecentLogs(c *fiber.Ctx) error {
	// Mock data for now
	logs := []map[string]interface{}{
		{
			"level":     "INFO",
			"message":   "Cluster node started successfully",
			"timestamp": time.Now().Add(-2 * time.Minute),
			"source":    "cluster-manager",
		},
		{
			"level":     "INFO",
			"message":   "Leader election completed",
			"timestamp": time.Now().Add(-1 * time.Minute),
			"source":    "cluster-manager",
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
	})
}

// handleExecuteOperation executes an operator operation with enhanced parameters
func (h *Handler) handleExecuteOperation(c *fiber.Ctx) error {
	var req struct {
		Operator  string                 `json:"operator"`
		Operation string                 `json:"operation"`
		Params    map[string]interface{} `json:"params"`
		NodeID    string                 `json:"node_id,omitempty"`
		Broadcast bool                   `json:"broadcast,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.operatorManager.ExecuteOperation(ctx, req.Operator, req.Operation, req.Params)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// handleClusterMetrics returns cluster performance metrics
func (h *Handler) handleClusterMetrics(c *fiber.Ctx) error {
	nodes := h.clusterManager.GetNodes()
	operators := h.operatorManager.ListOperators()

	metrics := map[string]interface{}{
		"cluster": map[string]interface{}{
			"total_nodes":   len(nodes),
			"running_nodes": countRunningNodes(nodes),
			"leader_node":   h.clusterManager.GetLeader(),
			"uptime":        "24h 15m", // Mock data
		},
		"operators": map[string]interface{}{
			"total_count": len(operators),
			"active":      len(operators), // All registered operators are considered active
		},
		"performance": map[string]interface{}{
			"avg_response_time": "125ms",
			"success_rate":      99.5,
			"last_hour_ops":     42,
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    metrics,
	})
}

// handleBroadcastOperation broadcasts an operation to all nodes with tracking
func (h *Handler) handleBroadcastOperation(c *fiber.Ctx) error {
	name := c.Params("name")

	var request types.OperatorRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if request.Operation == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Operation is required",
		})
	}

	if request.Params == nil {
		request.Params = make(map[string]interface{})
	}

	// Create context with timeout
	timeout := 60 * time.Second
	if timeoutStr := c.Query("timeout"); timeoutStr != "" {
		if t, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = t
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Broadcast operation with tracking
	execution, err := h.operatorManager.BroadcastOperationWithTracking(ctx, name, request.Operation, request.Params)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to broadcast operation: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Operation %s broadcasted to all nodes", request.Operation),
		"data": fiber.Map{
			"execution_id": execution.ID,
			"operation":    request.Operation,
			"operator":     name,
			"status":       execution.Status,
			"target_nodes": execution.TargetNodes,
			"timestamp":    execution.StartTime,
		},
	})
}

// handleRollingOperation executes an operation across nodes in specified order
func (h *Handler) handleRollingOperation(c *fiber.Ctx) error {
	name := c.Params("name")

	var request struct {
		Operation string                 `json:"operation"`
		Params    map[string]interface{} `json:"params"`
		NodeOrder []string               `json:"node_order"` // Specify exact order
		Delay     string                 `json:"delay"`      // Delay between nodes (e.g., "30s")
	}

	if err := c.BodyParser(&request); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
		})
	}

	if request.Operation == "" {
		return c.Status(400).JSON(fiber.Map{
			"success": false,
			"error":   "Operation is required",
		})
	}

	if len(request.NodeOrder) == 0 {
		// Default to all nodes in order
		nodes := h.clusterManager.GetNodes()
		for nodeID := range nodes {
			request.NodeOrder = append(request.NodeOrder, nodeID)
		}
	}

	if request.Params == nil {
		request.Params = make(map[string]interface{})
	}

	// Add rolling delay to params if specified
	if request.Delay != "" {
		request.Params["rolling_delay"] = request.Delay
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute) // Longer timeout for rolling
	defer cancel()

	// Execute rolling operation
	execution, err := h.operatorManager.RollingExecuteOperation(ctx, name, request.Operation, request.Params, request.NodeOrder)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Failed to execute rolling operation: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": fmt.Sprintf("Rolling operation %s started across %d nodes", request.Operation, len(request.NodeOrder)),
		"data": fiber.Map{
			"execution_id": execution.ID,
			"operation":    request.Operation,
			"operator":     name,
			"status":       execution.Status,
			"node_order":   request.NodeOrder,
			"delay":        request.Delay,
			"timestamp":    execution.StartTime,
		},
	})
}

// handleListExecutions returns all execution results
func (h *Handler) handleListExecutions(c *fiber.Ctx) error {
	executions := h.operatorManager.ListExecutions()

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"executions": executions,
			"count":      len(executions),
		},
	})
}

// handleGetExecution returns a specific execution result by ID (UUID)
func (h *Handler) handleGetExecution(c *fiber.Ctx) error {
	executionID := c.Params("id")

	execution, err := h.operatorManager.GetExecution(executionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error":   fmt.Sprintf("Execution %s not found", executionID),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    execution,
	})
}

// handleRecentExecutions returns recent execution results
func (h *Handler) handleRecentExecutions(c *fiber.Ctx) error {
	limitStr := c.Params("limit")
	limit := 100 // default

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	executions := h.operatorManager.GetRecentExecutions(limit)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"executions": executions,
			"count":      len(executions),
			"limit":      limit,
		},
	})
}

// handleEvents provides Server-Sent Events for real-time updates
func (h *Handler) handleEvents(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Headers", "Cache-Control")

	// Create a channel for this client
	clientChan := make(chan map[string]interface{}, 10)

	// Start a goroutine to send periodic updates
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		defer close(clientChan)

		for {
			select {
			case <-ticker.C:
				// Get current cluster state
				nodes := h.clusterManager.GetNodes()
				data := map[string]interface{}{
					"timestamp": time.Now(),
					"nodes":     len(nodes),
					"running":   countRunningNodes(nodes),
					"leader":    h.clusterManager.GetLeader(),
				}

				select {
				case clientChan <- data:
				default:
					// Client disconnected
					return
				}
			case <-c.Context().Done():
				// Client disconnected
				return
			}
		}
	}()

	// Send data to client
	for data := range clientChan {
		jsonData, _ := json.Marshal(data)
		if _, err := fmt.Fprintf(c, "data: %s\n\n", jsonData); err != nil {
			break
		}
		if err := c.Context().Err(); err != nil {
			break
		}
	}

	return nil
}

// getNodeState converts boolean leader status to string state
func getNodeState(isLeader bool) string {
	if isLeader {
		return "leader"
	}
	return "follower"
}

// generateDashboardHTML generates a simple HTML dashboard
func generateDashboardHTML(data map[string]interface{}) string {
	clusterState := data["ClusterState"].(*types.ClusterState)
	operators := data["Operators"].(map[string]types.OperatorInfo)
	nodeID := data["NodeID"].(string)
	isLeader := data["IsLeader"].(bool)
	timestamp := data["Timestamp"].(string)

	leaderStatus := "Follower"
	if isLeader {
		leaderStatus = "Leader"
	}

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>GoCluster Manager Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: #2c3e50; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .card { background: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .status { padding: 5px 10px; border-radius: 4px; color: white; font-weight: bold; }
        .status.running { background-color: #27ae60; }
        .status.unreachable { background-color: #e74c3c; }
        .status.leader { background-color: #f39c12; }
        .node { padding: 10px; border: 1px solid #ddd; border-radius: 4px; margin-bottom: 10px; }
        .operator { padding: 10px; border: 1px solid #ddd; border-radius: 4px; margin-bottom: 10px; background-color: #f8f9fa; }
        h1, h2, h3 { margin-top: 0; }
        .timestamp { color: #666; font-size: 0.9em; }
        .leader-badge { background-color: #f39c12; color: white; padding: 2px 8px; border-radius: 12px; font-size: 0.8em; }
    </style>
    <script>
        function refreshPage() {
            location.reload();
        }
        
        function triggerOperator(operatorName, operation) {
            const name = prompt('Enter name parameter:') || 'World';
            
            fetch('/api/operator/trigger/' + operatorName, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    operation: operation,
                    params: { name: name }
                })
            })
            .then(response => response.json())
            .then(data => {
                alert('Operation result: ' + JSON.stringify(data, null, 2));
            })
            .catch(error => {
                alert('Error: ' + error);
            });
        }
        
        // Auto-refresh every 30 seconds
        setInterval(refreshPage, 30000);
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 GoCluster Manager Dashboard</h1>
            <p>Node ID: <strong>%s</strong> | Status: <span class="status %s">%s</span> | Last Updated: %s</p>
            <button onclick="refreshPage()" style="background: #3498db; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;">🔄 Refresh</button>
        </div>
        
        <div class="grid">
            <div class="card">
                <h2>📊 Cluster Overview</h2>
                <p><strong>Cluster Leader:</strong> %s</p>
                <p><strong>Total Nodes:</strong> %d</p>
                <p><strong>Running Nodes:</strong> %d</p>
                <p><strong>Last Updated:</strong> %s</p>
            </div>
            
            <div class="card">
                <h2>🖥️ Cluster Nodes</h2>
                %s
            </div>
            
            <div class="card">
                <h2>⚙️ Available Operators</h2>
                %s
            </div>
        </div>
        
        <div class="card">
            <h2>🔧 Quick Actions</h2>
            <div style="display: flex; gap: 10px; flex-wrap: wrap;">
                <button onclick="triggerOperator('hello', 'greet')" style="background: #27ae60; color: white; border: none; padding: 10px 15px; border-radius: 4px; cursor: pointer;">👋 Say Hello</button>
                <button onclick="window.open('/api/status', '_blank')" style="background: #3498db; color: white; border: none; padding: 10px 15px; border-radius: 4px; cursor: pointer;">📈 View API Status</button>
                <button onclick="window.open('/api/nodes', '_blank')" style="background: #9b59b6; color: white; border: none; padding: 10px 15px; border-radius: 4px; cursor: pointer;">🌐 View Nodes API</button>
            </div>
        </div>
    </div>
</body>
</html>`,
		nodeID,
		strings.ToLower(leaderStatus),
		leaderStatus,
		timestamp,
		clusterState.Leader,
		len(clusterState.Nodes),
		countRunningNodes(clusterState.Nodes),
		clusterState.LastUpdated.Format("2006-01-02 15:04:05"),
		generateNodesHTML(clusterState.Nodes),
		generateOperatorsHTML(operators))

	return html
}

// countRunningNodes counts the number of running nodes
func countRunningNodes(nodes map[string]*types.Node) int {
	count := 0
	for _, node := range nodes {
		if node.Status == "running" {
			count++
		}
	}
	return count
}

// generateNodesHTML generates HTML for the nodes section
func generateNodesHTML(nodes map[string]*types.Node) string {
	if len(nodes) == 0 {
		return "<p>No nodes found</p>"
	}

	html := ""
	for _, node := range nodes {
		leaderBadge := ""
		if node.IsLeader {
			leaderBadge = `<span class="leader-badge">LEADER</span>`
		}

		lastSeen := node.LastSeen.Format("15:04:05")

		html += fmt.Sprintf(`
			<div class="node">
				<strong>%s</strong> %s<br>
				<small>Address: %s | Status: <span class="status %s">%s</span> | Last Seen: %s</small>
			</div>`,
			node.ID, leaderBadge, node.Address, node.Status, strings.ToUpper(node.Status), lastSeen)
	}

	return html
}

// generateOperatorsHTML generates HTML for the operators section
func generateOperatorsHTML(operators map[string]types.OperatorInfo) string {
	if len(operators) == 0 {
		return "<p>No operators registered</p>"
	}

	html := ""
	for _, op := range operators {
		html += fmt.Sprintf(`
			<div class="operator">
				<strong>%s</strong> v%s<br>
				<small>%s</small><br>
				<small style="color: #666;">Author: %s</small>
			</div>`,
			op.Name, op.Version, op.Description, op.Author)
	}

	return html
}

// generateGitStylePage generates a Git-inspired page layout
func generateGitStylePage(page string, data map[string]interface{}) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - GoCluster</title>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans', Helvetica, Arial, sans-serif;
            background-color: #0d1117;
            color: #c9d1d9;
            line-height: 1.5;
        }
        
        .github-header {
            background-color: #21262d;
            border-bottom: 1px solid #30363d;
            padding: 16px 32px;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
        
        .github-logo {
            display: flex;
            align-items: center;
            gap: 12px;
            font-size: 20px;
            font-weight: 600;
            color: #f0f6fc;
        }
        
        .github-nav {
            display: flex;
            gap: 24px;
            align-items: center;
        }
        
        .github-nav a {
            color: #c9d1d9;
            text-decoration: none;
            padding: 8px 16px;
            border-radius: 6px;
            transition: background-color 0.2s;
        }
        
        .github-nav a:hover, .github-nav a.active {
            background-color: #30363d;
            color: #f0f6fc;
        }
        
        .user-info {
            display: flex;
            align-items: center;
            gap: 8px;
            color: #7d8590;
            font-size: 14px;
        }
        
        .container {
            max-width: 1280px;
            margin: 0 auto;
            padding: 32px;
        }
        
        .repo-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 24px;
            padding-bottom: 16px;
            border-bottom: 1px solid #30363d;
        }
        
        .repo-title {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        
        .repo-title h1 {
            font-size: 20px;
            font-weight: 600;
            color: #58a6ff;
        }
        
        .repo-stats {
            display: flex;
            gap: 16px;
            align-items: center;
        }
        
        .stat-item {
            display: flex;
            align-items: center;
            gap: 4px;
            font-size: 14px;
            color: #7d8590;
        }
        
        .content-tabs {
            display: flex;
            border-bottom: 1px solid #30363d;
            margin-bottom: 24px;
        }
        
        .content-tabs a {
            color: #7d8590;
            text-decoration: none;
            padding: 8px 16px;
            border-bottom: 2px solid transparent;
            transition: all 0.2s;
        }
        
        .content-tabs a:hover {
            color: #c9d1d9;
        }
        
        .content-tabs a.active {
            color: #f0f6fc;
            border-bottom-color: #fd7e14;
        }
        
        .main-content {
            background-color: #0d1117;
            border: 1px solid #30363d;
            border-radius: 6px;
            overflow: hidden;
        }
        
        .file-header {
            background-color: #161b22;
            padding: 8px 16px;
            border-bottom: 1px solid #30363d;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
        
        .file-content {
            padding: 16px;
        }
        
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 16px;
            margin-bottom: 24px;
        }
        
        .card {
            background-color: #161b22;
            border: 1px solid #30363d;
            border-radius: 6px;
            padding: 16px;
        }
        
        .card h3 {
            margin-bottom: 12px;
            color: #f0f6fc;
            font-size: 16px;
        }
        
        .node-item, .operator-item {
            background-color: #0d1117;
            border: 1px solid #30363d;
            border-radius: 6px;
            padding: 12px;
            margin-bottom: 8px;
        }
        
        .node-header {
            display: flex;
            align-items: center;
            justify-content: between;
            margin-bottom: 8px;
        }
        
        .node-id {
            font-weight: 600;
            color: #58a6ff;
        }
        
        .leader-badge {
            background-color: #238636;
            color: white;
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 12px;
            margin-left: 8px;
        }
        
        .status-badge {
            padding: 2px 8px;
            border-radius: 12px;
            font-size: 12px;
            font-weight: 500;
        }
        
        .status-running {
            background-color: #238636;
            color: white;
        }
        
        .status-unreachable {
            background-color: #da3633;
            color: white;
        }
        
        .node-details {
            font-size: 14px;
            color: #7d8590;
        }
        
        .btn {
            background-color: #238636;
            color: white;
            border: none;
            padding: 6px 16px;
            border-radius: 6px;
            cursor: pointer;
            font-size: 14px;
            text-decoration: none;
            display: inline-block;
            transition: background-color 0.2s;
        }
        
        .btn:hover {
            background-color: #2ea043;
        }
        
        .btn-secondary {
            background-color: #21262d;
            border: 1px solid #30363d;
            color: #c9d1d9;
        }
        
        .btn-secondary:hover {
            background-color: #30363d;
        }
        
        .command-palette {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background-color: rgba(13, 17, 23, 0.8);
            display: none;
            align-items: flex-start;
            justify-content: center;
            padding-top: 10vh;
            z-index: 1000;
        }
        
        .palette-content {
            background-color: #21262d;
            border: 1px solid #30363d;
            border-radius: 12px;
            width: 100%%;
            max-width: 600px;
            box-shadow: 0 16px 32px rgba(0, 0, 0, 0.4);
        }
        
        .palette-input {
            width: 100%%;
            padding: 16px;
            background-color: transparent;
            border: none;
            color: #c9d1d9;
            font-size: 16px;
            outline: none;
        }
        
        .palette-results {
            max-height: 300px;
            overflow-y: auto;
        }
        
        .palette-item {
            padding: 12px 16px;
            cursor: pointer;
            border-top: 1px solid #30363d;
            display: flex;
            align-items: center;
            gap: 12px;
        }
        
        .palette-item:hover {
            background-color: #30363d;
        }
        
        .flash-message {
            background-color: #0969da;
            color: white;
            padding: 12px 16px;
            border-radius: 6px;
            margin-bottom: 16px;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        
        .actions-bar {
            display: flex;
            gap: 8px;
            margin-bottom: 16px;
            flex-wrap: wrap;
        }
        
        @media (max-width: 768px) {
            .github-header {
                padding: 16px;
                flex-direction: column;
                gap: 16px;
            }
            
            .github-nav {
                order: 2;
                flex-wrap: wrap;
                justify-content: center;
            }
            
            .container {
                padding: 16px;
            }
            
            .repo-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 16px;
            }
        }
    </style>
</head>
<body>
    <header class="github-header">
        <div class="github-logo">
            <i class="fas fa-cube"></i>
            <span>GoCluster</span>
        </div>
        
        <nav class="github-nav">
            <a href="/" class="%s">Dashboard</a>
            <a href="/nodes" class="%s">Nodes</a>
            <a href="/operators" class="%s">Operators</a>
            <a href="/operator-forms" class="%s">Operator Forms</a>
            <a href="/operations" class="%s">Operations</a>
            <a href="/logs" class="%s">Logs</a>
            <a href="/settings" class="%s">Settings</a>
        </nav>
        
        <div class="user-info">
            <i class="fas fa-server"></i>
            <span>%s</span>
            %s
        </div>
    </header>

    <div class="container">
        <div class="repo-header">
            <div class="repo-title">
                <i class="fas fa-project-diagram"></i>
                <h1>%s</h1>
            </div>
            <div class="repo-stats">
                <div class="stat-item">
                    <i class="fas fa-clock"></i>
                    <span>%s</span>
                </div>
            </div>
        </div>

        %s
    </div>

    <!-- Command Palette -->
    <div class="command-palette" id="commandPalette">
        <div class="palette-content">
            <input type="text" class="palette-input" placeholder="Type a command..." id="paletteInput">
            <div class="palette-results" id="paletteResults"></div>
        </div>
    </div>

    <script>
        // Command Palette
        document.addEventListener('keydown', function(e) {
            if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
                e.preventDefault();
                document.getElementById('commandPalette').style.display = 'flex';
                document.getElementById('paletteInput').focus();
            }
            if (e.key === 'Escape') {
                document.getElementById('commandPalette').style.display = 'none';
            }
        });
        
        document.getElementById('commandPalette').addEventListener('click', function(e) {
            if (e.target === this) {
                this.style.display = 'none';
            }
        });
        
        // Auto-refresh
        setInterval(function() {
            window.location.reload();
        }, 30000);
        
        // Operator execution
        function executeOperation(operator, operation) {
            const params = prompt('Enter parameters (JSON format):') || '{}';
            let parsedParams;
            try {
                parsedParams = JSON.parse(params);
            } catch (e) {
                alert('Invalid JSON parameters');
                return;
            }
            
            fetch('/api/operator/execute', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    operator: operator,
                    operation: operation,
                    params: parsedParams
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    alert('Operation executed successfully!');
                    location.reload();
                } else {
                    alert('Error: ' + data.error);
                }
            });
        }
    </script>
</body>
</html>`,
		data["Title"],
		getActiveClass(page, "dashboard"),
		getActiveClass(page, "nodes"),
		getActiveClass(page, "operators"),
		getActiveClass(page, "operator-forms"),
		getActiveClass(page, "operations"),
		getActiveClass(page, "logs"),
		getActiveClass(page, "settings"),
		data["NodeID"],
		getLeaderBadge(data),
		data["Title"],
		data["Timestamp"],
		generatePageContent(page, data),
	)
}

// getActiveClass returns "active" if the current page matches
func getActiveClass(current, page string) string {
	if current == page {
		return "active"
	}
	return ""
}

// getLeaderBadge returns a leader badge if the node is a leader
func getLeaderBadge(data map[string]interface{}) string {
	if isLeader, ok := data["IsLeader"].(bool); ok && isLeader {
		return `<span class="leader-badge">LEADER</span>`
	}
	return ""
}

// generatePageContent generates content for different pages
func generatePageContent(page string, data map[string]interface{}) string {
	switch page {
	case "nodes":
		return generateNodesContent(data)
	case "operators":
		return generateOperatorsContent(data)
	case "operations":
		return generateOperationsContent(data)
	case "logs":
		return generateLogsContent(data)
	case "settings":
		return generateSettingsContent(data)
	default:
		return generateDashboardContent(data)
	}
}

// generateNodesContent generates the nodes page content
func generateNodesContent(data map[string]interface{}) string {
	nodes, ok := data["Nodes"].(map[string]*types.Node)
	if !ok || len(nodes) == 0 {
		return `<div class="main-content">
			<div class="file-header">
				<span><i class="fas fa-server"></i> Cluster Nodes</span>
			</div>
			<div class="file-content">
				<p>No nodes found in the cluster.</p>
			</div>
		</div>`
	}

	content := `<div class="main-content">
		<div class="file-header">
			<span><i class="fas fa-server"></i> Cluster Nodes</span>
			<div class="actions-bar">
				<button class="btn btn-secondary" onclick="location.reload()">
					<i class="fas fa-sync"></i> Refresh
				</button>
			</div>
		</div>
		<div class="file-content">`

	for _, node := range nodes {
		leaderBadge := ""
		if node.IsLeader {
			leaderBadge = `<span class="leader-badge">LEADER</span>`
		}

		statusClass := "status-running"
		if node.Status != "running" {
			statusClass = "status-unreachable"
		}

		content += fmt.Sprintf(`
			<div class="node-item">
				<div class="node-header">
					<div class="node-id">%s %s</div>
					<span class="status-badge %s">%s</span>
				</div>
				<div class="node-details">
					<div><i class="fas fa-network-wired"></i> %s</div>
					<div><i class="fas fa-clock"></i> Last seen: %s</div>
				</div>
			</div>`,
			node.ID, leaderBadge, statusClass, strings.ToUpper(node.Status),
			node.Address, node.LastSeen.Format("2006-01-02 15:04:05"))
	}

	content += `</div></div>`
	return content
}

// generateOperatorsContent generates the operators page content
func generateOperatorsContent(data map[string]interface{}) string {
	operators, ok := data["Operators"].(map[string]types.OperatorInfo)
	if !ok || len(operators) == 0 {
		return `<div class="main-content">
			<div class="file-header">
				<span><i class="fas fa-cogs"></i> Cluster Operators</span>
			</div>
			<div class="file-content">
				<p>No operators registered.</p>
			</div>
		</div>`
	}

	content := `<div class="main-content">
		<div class="file-header">
			<span><i class="fas fa-cogs"></i> Cluster Operators</span>
		</div>
		<div class="file-content">`

	for _, op := range operators {
		content += fmt.Sprintf(`
			<div class="operator-item">
				<div style="display: flex; justify-content: space-between; align-items: center;">
					<div>
						<div class="node-id">%s <span style="color: #7d8590;">v%s</span></div>
						<div class="node-details">%s</div>
						<div class="node-details"><i class="fas fa-user"></i> %s</div>
					</div>
					<div style="display: flex; gap: 8px;">
						<button class="btn btn-secondary" onclick="executeOperation('%s', 'status')">
							<i class="fas fa-info"></i> Status
						</button>
						<button class="btn" onclick="executeOperation('%s', 'info')">
							<i class="fas fa-play"></i> Execute
						</button>
					</div>
				</div>
			</div>`,
			op.Name, op.Version, op.Description, op.Author, op.Name, op.Name)
	}

	content += `</div></div>`
	return content
}

// generateOperationsContent generates the operations page content
func generateOperationsContent(data map[string]interface{}) string {
	return `<div class="main-content">
		<div class="file-header">
			<span><i class="fas fa-history"></i> Operations History</span>
			<div class="actions-bar">
				<button class="btn btn-secondary" onclick="loadOperationHistory()">
					<i class="fas fa-sync"></i> Refresh
				</button>
			</div>
		</div>
		<div class="file-content" id="operationsContent">
			<p>Loading operations history...</p>
		</div>
	</div>
	<script>
		function loadOperationHistory() {
			fetch('/api/operations/history')
				.then(response => response.json())
				.then(data => {
					const content = document.getElementById('operationsContent');
					if (data.success && data.data.length > 0) {
						let html = '';
						data.data.forEach(op => {
							const statusClass = op.status === 'success' ? 'status-running' : 'status-unreachable';
							html += '<div class="operator-item">' +
								'<div style="display: flex; justify-content: space-between; align-items: center;">' +
								'<div>' +
								'<div class="node-id">' + op.operator + ' → ' + op.operation + '</div>' +
								'<div class="node-details">' +
								'<i class="fas fa-server"></i> ' + op.node + ' | ' +
								'<i class="fas fa-clock"></i> ' + new Date(op.timestamp).toLocaleString() + ' | ' +
								'<i class="fas fa-stopwatch"></i> ' + op.duration +
								'</div>' +
								'</div>' +
								'<span class="status-badge ' + statusClass + '">' + op.status.toUpperCase() + '</span>' +
								'</div>' +
								'</div>';
						});
						content.innerHTML = html;
					} else {
						content.innerHTML = '<p>No operations found.</p>';
					}
				});
		}
		loadOperationHistory();
	</script>`
}

// generateLogsContent generates the logs page content
func generateLogsContent(data map[string]interface{}) string {
	return `<div class="main-content">
		<div class="file-header">
			<span><i class="fas fa-file-alt"></i> System Logs</span>
			<div class="actions-bar">
				<button class="btn btn-secondary" onclick="loadRecentLogs()">
					<i class="fas fa-sync"></i> Refresh
				</button>
			</div>
		</div>
		<div class="file-content" id="logsContent">
			<p>Loading recent logs...</p>
		</div>
	</div>
	<script>
		function loadRecentLogs() {
			fetch('/api/logs/recent')
				.then(response => response.json())
				.then(data => {
					const content = document.getElementById('logsContent');
					if (data.success && data.data.length > 0) {
						let html = '<div style="font-family: monospace; background: #0d1117; padding: 16px; border-radius: 6px;">';
						data.data.forEach(log => {
							const levelColor = log.level === 'ERROR' ? '#da3633' : log.level === 'WARN' ? '#fd7e14' : '#7d8590';
							html += '<div style="margin-bottom: 8px;">' +
								'<span style="color: ' + levelColor + ';">[' + log.level + ']</span> ' +
								'<span style="color: #7d8590;">' + new Date(log.timestamp).toLocaleString() + '</span> ' +
								'<span style="color: #c9d1d9;">' + log.message + '</span> ' +
								'<span style="color: #58a6ff;">(' + log.source + ')</span>' +
								'</div>';
						});
						html += '</div>';
						content.innerHTML = html;
					} else {
						content.innerHTML = '<p>No logs found.</p>';
					}
				});
		}
		loadRecentLogs();
	</script>`
}

// generateSettingsContent generates the settings page content
func generateSettingsContent(data map[string]interface{}) string {
	return `<div class="main-content">
		<div class="file-header">
			<span><i class="fas fa-cog"></i> Cluster Settings</span>
		</div>
		<div class="file-content">
			<div class="grid">
				<div class="card">
					<h3>Node Configuration</h3>
					<p><strong>Node ID:</strong> ` + fmt.Sprintf("%v", data["NodeID"]) + `</p>
					<p><strong>Leader Status:</strong> ` + fmt.Sprintf("%v", data["IsLeader"]) + `</p>
				</div>
				<div class="card">
					<h3>Cluster Health</h3>
					<button class="btn" onclick="location.href='/api/health'">
						<i class="fas fa-heartbeat"></i> Check Health
					</button>
				</div>
			</div>
		</div>
	</div>`
}

// generateDashboardContent generates the dashboard page content
func generateDashboardContent(data map[string]interface{}) string {
	return generateDashboardHTML(data)
}
