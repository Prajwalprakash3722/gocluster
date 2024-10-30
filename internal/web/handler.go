package web

import (
	"agent/internal/cluster"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

type Handler struct {
	manager    *cluster.Manager
	clients    map[chan StatusUpdate]bool
	clientsMux sync.RWMutex
	templates  *template.Template
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

func NewHandler(manager *cluster.Manager) (*Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	h := &Handler{
		manager:   manager,
		clients:   make(map[chan StatusUpdate]bool),
		templates: tmpl,
	}
	go h.broadcastStatus()
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// API routes
	if strings.HasPrefix(r.URL.Path, "/api/") {
		h.handleAPI(w, r)
		return
	}

	// Web UI routes
	switch r.URL.Path {
	case "/":
		h.handleIndex(w, r)
	case "/events":
		h.handleEvents(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/api/status":
		h.handleAPIStatus(w, r)
	case "/api/nodes":
		h.handleAPINodes(w, r)
	case "/api/leader":
		h.handleAPILeader(w, r)
	case "/api/node":
		h.handleAPINode(w, r)
	default:
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "endpoint not found",
		})
	}
}

func (h *Handler) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	status := StatusUpdate{
		ClusterName: h.manager.GetClusterName(),
		LocalNode:   h.manager.GetLocalNode(),
		Leader:      h.manager.GetLeaderID(),
		Nodes:       h.manager.GetNodes(),
		Timestamp:   time.Now(),
	}

	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    status,
	})
}

func (h *Handler) handleAPINodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.manager.GetNodes()

	// Convert to a more API-friendly format
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

	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    nodesArray,
	})
}

func (h *Handler) handleAPILeader(w http.ResponseWriter, r *http.Request) {
	leaderID := h.manager.GetLeaderID()
	nodes := h.manager.GetNodes()

	if leader, exists := nodes[leaderID]; exists {
		json.NewEncoder(w).Encode(APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"id":        leaderID,
				"hostname":  leader.Hostname,
				"address":   leader.Address,
				"port":      leader.Port,
				"last_seen": leader.LastSeen,
			},
		})
		return
	}

	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   "no leader found",
	})
}

func (h *Handler) handleAPINode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("id")
	if nodeID == "" {
		json.NewEncoder(w).Encode(APIResponse{
			Success: false,
			Error:   "node id required",
		})
		return
	}

	nodes := h.manager.GetNodes()
	if node, exists := nodes[nodeID]; exists {
		json.NewEncoder(w).Encode(APIResponse{
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
		return
	}

	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   "node not found",
	})
}

func (h *Handler) handleAPIOperator(w http.ResponseWriter, r *http.Request) {

}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := StatusUpdate{
		ClusterName: h.manager.GetClusterName(),
		LocalNode:   h.manager.GetLocalNode(),
		Leader:      h.manager.GetLeaderID(),
		Nodes:       h.manager.GetNodes(),
		Timestamp:   time.Now(),
	}

	err := h.templates.ExecuteTemplate(w, "status", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

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
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
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
