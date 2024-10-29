package web

import (
	"agent/internal/cluster"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
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
	switch r.URL.Path {
	case "/":
		h.handleIndex(w, r)
	case "/events":
		h.handleEvents(w, r)
	default:
		http.NotFound(w, r)
	}
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
