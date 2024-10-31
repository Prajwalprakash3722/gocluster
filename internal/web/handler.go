package web

import (
	"agent/internal/cluster"
	"agent/internal/operator"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

//go:embed templates/*
var templateFS embed.FS

type Handler struct {
	manager         *cluster.Manager
	operatorManager *operator.OperatorManager
	clients         map[chan StatusUpdate]bool
	clientsMux      sync.RWMutex
	templates       *template.Template
}

type StatusUpdate struct {
	ClusterName string                   `json:"cluster_name"`
	LocalNode   *cluster.Node            `json:"local_node"`
	Leader      string                   `json:"leader"`
	Nodes       map[string]*cluster.Node `json:"nodes"`
	Timestamp   time.Time                `json:"timestamp"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func NewHandler(manager *cluster.Manager, operatorManager *operator.OperatorManager) (*Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	h := &Handler{
		manager:         manager,
		clients:         make(map[chan StatusUpdate]bool),
		templates:       tmpl,
		operatorManager: operatorManager,
	}
	go h.broadcastStatus()
	return h, nil
}

func (h *Handler) SetupRoutes(app *fiber.App) {
	// API routes
	api := app.Group("/api")

	api.Get("/status", h.handleAPIStatus)
	api.Get("/nodes", h.handleAPINodes)
	api.Get("/leader", h.handleAPILeader)
	api.Get("/node", h.handleAPINode)
	api.Get("/operator/list", h.handleAPIListOperators)
	api.Post("/operator/trigger/:name", h.handleAPIOperator)
	api.Get("/operator/schema/:name", h.handleAPIOperatorSchema) // Add this line

	// Web UI routes
	app.Get("/", h.handleIndex)
	app.Get("/events", h.handleEvents)
}

func (h *Handler) handleAPIStatus(c *fiber.Ctx) error {
	status := StatusUpdate{
		ClusterName: h.manager.GetClusterName(),
		LocalNode:   h.manager.GetLocalNode(),
		Leader:      h.manager.GetLeaderID(),
		Nodes:       h.manager.GetNodes(),
		Timestamp:   time.Now(),
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    status,
	})
}

func (h *Handler) handleAPINodes(c *fiber.Ctx) error {
	nodes := h.manager.GetNodes()

	nodesArray := make([]map[string]interface{}, 0, len(nodes))
	for id, node := range nodes {
		nodeMap := map[string]interface{}{
			"id":        id,
			"hostname":  node.Hostname,
			"address":   node.Address,
			"port":      node.Port,
			"state":     node.State,
			"last_seen": node.LastSeen,
		}
		nodesArray = append(nodesArray, nodeMap)
	}

	return c.JSON(APIResponse{
		Success: true,
		Data:    nodesArray,
	})
}

func (h *Handler) handleAPILeader(c *fiber.Ctx) error {
	leaderID := h.manager.GetLeaderID()
	nodes := h.manager.GetNodes()

	if leader, exists := nodes[leaderID]; exists {
		return c.JSON(APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"id":        leaderID,
				"hostname":  leader.Hostname,
				"address":   leader.Address,
				"port":      leader.Port,
				"last_seen": leader.LastSeen,
			},
		})
	}

	return c.JSON(APIResponse{
		Success: false,
		Error:   "no leader found",
	})
}

func (h *Handler) handleAPINode(c *fiber.Ctx) error {
	nodeID := c.Query("id")
	if nodeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error:   "node id required",
		})
	}

	nodes := h.manager.GetNodes()
	if node, exists := nodes[nodeID]; exists {
		return c.JSON(APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"id":        nodeID,
				"hostname":  node.Hostname,
				"address":   node.Address,
				"port":      node.Port,
				"state":     node.State,
				"last_seen": node.LastSeen,
				"is_leader": nodeID == h.manager.GetLeaderID(),
			},
		})
	}

	return c.JSON(APIResponse{
		Success: false,
		Error:   "node not found",
	})
}

func (h *Handler) handleAPIListOperators(c *fiber.Ctx) error {

	if h.operatorManager == nil {
		return c.JSON(APIResponse{
			Success: false,
			Error:   "operator manager not available, are you sure you have enabled operators?",
		})
	}

	operators := h.operatorManager.ListOperators()
	return c.JSON(APIResponse{
		Success: true,
		Data:    operators,
	})
}

func (h *Handler) handleIndex(c *fiber.Ctx) error {
	data := StatusUpdate{
		ClusterName: h.manager.GetClusterName(),
		LocalNode:   h.manager.GetLocalNode(),
		Leader:      h.manager.GetLeaderID(),
		Nodes:       h.manager.GetNodes(),
		Timestamp:   time.Now(),
	}

	err := h.templates.ExecuteTemplate(c.Response().BodyWriter(), "status", data)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}
	return nil
}

func (h *Handler) handleEvents(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	statusChan := make(chan StatusUpdate)
	h.clientsMux.Lock()
	h.clients[statusChan] = true
	h.clientsMux.Unlock()

	defer func() {
		h.clientsMux.Lock()
		delete(h.clients, statusChan)
		h.clientsMux.Unlock()
		close(statusChan)
	}()

	for {
		select {
		case status := <-statusChan:
			data, _ := json.Marshal(status)
			c.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
			if flusher, ok := c.Response().BodyWriter().(http.Flusher); ok {
				flusher.Flush()
			}
		case <-c.Context().Done():
			return nil
		}
	}
}

func (h *Handler) broadcastStatus() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		status := StatusUpdate{
			ClusterName: h.manager.GetClusterName(),
			LocalNode:   h.manager.GetLocalNode(),
			Leader:      h.manager.GetLeaderID(),
			Nodes:       h.manager.GetNodes(),
			Timestamp:   time.Now(),
		}

		h.clientsMux.RLock()
		for client := range h.clients {
			select {
			case client <- status:
			default:
			}
		}
		h.clientsMux.RUnlock()
	}
}

func (h *Handler) handleAPIOperator(c *fiber.Ctx) error {
	if c.Method() != fiber.MethodPost {
		return c.Status(fiber.StatusMethodNotAllowed).JSON(APIResponse{
			Success: false,
			Error:   "method not allowed",
		})
	}

	operatorName := c.Params("name")
	if operatorName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error:   "invalid operator path",
		})
	}

	var params map[string]interface{}
	if err := c.BodyParser(&params); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request body: %v", err),
		})
	}

	ctx := c.Context()
	if err := h.operatorManager.ExecuteOperator(ctx, operatorName, params); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return c.JSON(APIResponse{
		Success: true,
		Data: map[string]string{
			"message": fmt.Sprintf("operator %s executed successfully", operatorName),
		},
	})
}

func (h *Handler) handleAPIOperatorSchema(c *fiber.Ctx) error {
	operatorName := c.Params("name")
	if operatorName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Error:   "operator name is required",
		})
	}

	operator, err := h.operatorManager.GetOperator(operatorName)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(APIResponse{
			Success: false,
			Error:   fmt.Sprintf("operator %s not found", operatorName),
		})
	}

	info := operator.Info()

	return c.JSON(APIResponse{
		Success: true,
		Data:    info,
	})
}
