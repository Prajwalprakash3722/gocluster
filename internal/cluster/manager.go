package cluster

import (
	"agent/internal/config"
	"agent/internal/operator"
	"agent/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Manager handles cluster operations and node management
type Manager struct {
	config          *config.Config
	nodeID          string
	nodes           map[string]*types.Node
	isLeader        bool
	operatorManager *operator.OperatorManager

	// Backend clients
	etcdClient *clientv3.Client
	zkConn     *zk.Conn

	// Synchronization
	mutex  sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	// Network
	udpConn  *net.UDPConn
	stopChan chan struct{}
}

// ManagerOptions contains options for creating a new manager
type ManagerOptions struct {
	ConfigPath      string
	BindAddress     string
	BindPort        int
	OperatorManager *operator.OperatorManager
}

// NewManager creates a new cluster manager
func NewManager(opts ManagerOptions) (*Manager, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Use hostname as node ID for better user experience
	// Add port for uniqueness in case of multiple instances per host
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	nodeID := fmt.Sprintf("%s:%d", hostname, opts.BindPort)

	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		config:          cfg,
		nodeID:          nodeID,
		nodes:           make(map[string]*types.Node),
		operatorManager: opts.OperatorManager,
		ctx:             ctx,
		cancel:          cancel,
		stopChan:        make(chan struct{}),
	}

	// Initialize backend
	if err := manager.initBackend(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize backend: %v", err)
	}

	// Setup UDP listener for node discovery
	if err := manager.setupUDPListener(opts.BindAddress, opts.BindPort); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup UDP listener: %v", err)
	}

	return manager, nil
}

// initBackend initializes the appropriate backend
func (m *Manager) initBackend() error {
	switch m.config.Backend.Type {
	case types.BackendEtcd:
		return m.initEtcdBackend()
	case types.BackendZooKeeper:
		return m.initZooKeeperBackend()
	case types.BackendMemory:
		// Memory backend doesn't need initialization
		return nil
	default:
		return fmt.Errorf("unsupported backend type: %s", m.config.Backend.Type)
	}
}

// initEtcdBackend initializes etcd backend
func (m *Manager) initEtcdBackend() error {
	cfg := clientv3.Config{
		Endpoints:   m.config.Backend.EtcdEndpoints,
		DialTimeout: 5 * time.Second,
	}

	if m.config.Backend.EtcdUsername != "" {
		cfg.Username = m.config.Backend.EtcdUsername
		cfg.Password = m.config.Backend.EtcdPassword
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %v", err)
	}

	m.etcdClient = client
	return nil
}

// initZooKeeperBackend initializes ZooKeeper backend
func (m *Manager) initZooKeeperBackend() error {
	timeout, err := time.ParseDuration(m.config.Backend.ZKTimeout)
	if err != nil {
		timeout = 30 * time.Second
	}

	conn, _, err := zk.Connect(m.config.Backend.ZKHosts, timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to ZooKeeper: %v", err)
	}

	m.zkConn = conn
	return nil
}

// setupUDPListener sets up UDP listener for node discovery
func (m *Manager) setupUDPListener(address string, port int) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", address, port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %v", err)
	}

	m.udpConn = conn
	return nil
}

// Start starts the cluster manager
func (m *Manager) Start() error {
	log.Printf("Starting cluster manager with node ID: %s", m.nodeID)

	// Add self to nodes
	hostname, _ := os.Hostname()
	m.addNode(&types.Node{
		ID:       m.nodeID,
		Address:  m.udpConn.LocalAddr().String(),
		Status:   "running",
		LastSeen: time.Now(),
		IsLeader: false,
		Metadata: map[string]interface{}{
			"hostname": hostname,
		},
	})

	// Start leader election
	go m.runLeaderElection()

	// Start UDP listener
	go m.runUDPListener()

	// Start node discovery
	go m.runNodeDiscovery()

	// Start health monitoring
	go m.runHealthMonitor()

	log.Printf("Cluster manager started successfully")
	return nil
}

// Stop stops the cluster manager
func (m *Manager) Stop() {
	log.Printf("Stopping cluster manager...")

	close(m.stopChan)
	m.cancel()

	if m.udpConn != nil {
		m.udpConn.Close()
	}

	if m.etcdClient != nil {
		m.etcdClient.Close()
	}

	if m.zkConn != nil {
		m.zkConn.Close()
	}

	log.Printf("Cluster manager stopped")
}

// runLeaderElection runs the leader election process
func (m *Manager) runLeaderElection() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performLeaderElection()
		}
	}
}

// performLeaderElection performs leader election logic
func (m *Manager) performLeaderElection() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Simple leader election: node with lowest ID becomes leader
	var leaderID string
	for id, node := range m.nodes {
		if node.Status == "running" {
			if leaderID == "" || id < leaderID {
				leaderID = id
			}
		}
	}

	// Update leader status
	wasLeader := m.isLeader
	m.isLeader = (leaderID == m.nodeID)

	if m.isLeader != wasLeader {
		if m.isLeader {
			log.Printf("Became cluster leader")
		} else {
			log.Printf("Lost cluster leadership")
		}
	}

	// Update all nodes' leader status
	for id, node := range m.nodes {
		node.IsLeader = (id == leaderID)
	}
}

// runUDPListener listens for UDP discovery messages and operator messages
func (m *Manager) runUDPListener() {
	buffer := make([]byte, 4096) // Increased buffer size for operator messages

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
			m.udpConn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := m.udpConn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("UDP read error: %v", err)
				continue
			}

			m.handleUDPMessage(buffer[:n], addr)
		}
	}
}

// handleUDPMessage handles incoming UDP messages (both discovery and operator messages)
func (m *Manager) handleUDPMessage(data []byte, addr *net.UDPAddr) {
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return
	}

	// Check if this is a discovery message (has node_id field)
	if _, ok := message["node_id"].(string); ok {
		m.handleDiscoveryMessage(data, addr)
		return
	}

	// Check if this is an operator message (has operation field)
	if operation, ok := message["operation"].(string); ok {
		if operation == "operator_response" {
			m.handleOperatorResponse(data, addr, message)
		} else {
			m.handleOperatorMessage(data, addr, operation, message)
		}
		return
	}

	// Unknown message type, ignore
	log.Printf("Received unknown message type from %s", addr.String())
}

// handleOperatorMessage handles incoming operator messages
func (m *Manager) handleOperatorMessage(data []byte, addr *net.UDPAddr, operation string, message map[string]interface{}) {
	// Extract operator name from operation string (format: "operator:operatorName")
	if !strings.HasPrefix(operation, "operator:") {
		log.Printf("Invalid operator operation format: %s", operation)
		return
	}

	operatorName := strings.TrimPrefix(operation, "operator:")
	fromNodeID, _ := message["from"].(string)

	// Extract the operator request from the data field
	dataField, ok := message["data"]
	if !ok {
		log.Printf("Missing data field in operator message from %s", fromNodeID)
		return
	}

	// Convert data field to OperatorRequest
	var request types.OperatorRequest

	// Try to convert the data field to JSON and then unmarshal it
	dataBytes, err := json.Marshal(dataField)
	if err != nil {
		log.Printf("Failed to marshal operator data from %s: %v", fromNodeID, err)
		return
	}

	if err := json.Unmarshal(dataBytes, &request); err != nil {
		log.Printf("Failed to unmarshal operator request from %s: %v", fromNodeID, err)
		return
	}

	// Log the incoming operator request
	log.Printf("[Node %s] Received operator request '%s' from node %s", m.nodeID, request.Operation, fromNodeID)

	// Handle the operator request using the operator manager
	if m.operatorManager != nil {
		ctx := context.Background() // You might want to add a timeout here
		response := m.operatorManager.HandleOperatorRequest(ctx, operatorName, &request)

		// Log the execution result
		if response.Success {
			log.Printf("[Node %s] Successfully executed operation '%s' on operator '%s'", m.nodeID, request.Operation, operatorName)
		} else {
			log.Printf("[Node %s] Failed to execute operation '%s' on operator '%s': %s", m.nodeID, request.Operation, operatorName, response.Error)
		}

		// Send response back to the originating node
		if fromNodeID != "" && fromNodeID != m.nodeID {
			responseMessage := map[string]interface{}{
				"operation":    "operator_response",
				"response_to":  request.Operation,
				"execution_id": request.NodeID, // Use NodeID as execution tracking
				"operator":     operatorName,
				"success":      response.Success,
				"message":      response.Message,
				"data":         response.Data,
				"error":        response.Error,
				"timestamp":    response.Timestamp,
				"executed_on":  m.nodeID,
				"from":         m.nodeID,
			}

			responseData, _ := json.Marshal(responseMessage)
			if node, exists := m.GetNodes()[fromNodeID]; exists {
				if err := m.sendToNodeAddress(node.Address, responseData); err != nil {
					log.Printf("[Node %s] Failed to send response to node %s: %v", m.nodeID, fromNodeID, err)
				} else {
					log.Printf("[Node %s] Sent execution response back to node %s", m.nodeID, fromNodeID)
				}
			}
		}
	} else {
		log.Printf("[Node %s] No operator manager available to handle request", m.nodeID)
	}
}

// handleOperatorResponse handles incoming operator response messages
func (m *Manager) handleOperatorResponse(data []byte, addr *net.UDPAddr, message map[string]interface{}) {
	fromNodeID, _ := message["from"].(string)
	executionID, _ := message["execution_id"].(string)
	operator, _ := message["operator"].(string)
	success, _ := message["success"].(bool)
	responseMsg, _ := message["message"].(string)
	executedOn, _ := message["executed_on"].(string)

	if success {
		log.Printf("[Node %s] Received SUCCESS response from %s for execution %s: %s",
			m.nodeID, fromNodeID, executionID, responseMsg)
	} else {
		errorMsg, _ := message["error"].(string)
		log.Printf("[Node %s] Received FAILURE response from %s for execution %s: %s",
			m.nodeID, fromNodeID, executionID, errorMsg)
	}

	log.Printf("[Node %s] Operation executed on remote node %s (operator: %s)",
		m.nodeID, executedOn, operator)
}

// handleDiscoveryMessage handles incoming discovery messages
func (m *Manager) handleDiscoveryMessage(data []byte, addr *net.UDPAddr) {
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return
	}

	nodeID, ok := message["node_id"].(string)
	if !ok || nodeID == m.nodeID {
		return
	}

	status, _ := message["status"].(string)
	if status == "" {
		status = "running"
	}

	hostname, _ := message["hostname"].(string)
	if hostname == "" {
		hostname = nodeID // fallback to nodeID if hostname not provided
	}

	node := &types.Node{
		ID:       nodeID,
		Address:  addr.String(),
		Status:   status,
		LastSeen: time.Now(),
		Metadata: map[string]interface{}{
			"hostname": hostname,
		},
	}

	m.addNode(node)
}

// runNodeDiscovery sends discovery messages to known nodes
func (m *Manager) runNodeDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.sendDiscoveryMessages()
		}
	}
}

// sendDiscoveryMessages sends discovery messages to configured nodes
func (m *Manager) sendDiscoveryMessages() {
	hostname, _ := os.Hostname()
	message := map[string]interface{}{
		"node_id":   m.nodeID,
		"status":    "running",
		"hostname":  hostname,
		"timestamp": time.Now().Unix(),
	}

	data, _ := json.Marshal(message)

	for _, nodeAddr := range m.config.Nodes {
		addr, err := net.ResolveUDPAddr("udp", nodeAddr)
		if err != nil {
			continue
		}

		m.udpConn.WriteToUDP(data, addr)
	}
}

// runHealthMonitor monitors node health
func (m *Manager) runHealthMonitor() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkNodeHealth()
		}
	}
}

// checkNodeHealth checks the health of all nodes
func (m *Manager) checkNodeHealth() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for id, node := range m.nodes {
		if id == m.nodeID {
			continue // Skip self
		}

		if now.Sub(node.LastSeen) > 2*time.Minute {
			node.Status = "unreachable"
		}
	}
}

// addNode adds or updates a node in the cluster
func (m *Manager) addNode(node *types.Node) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	existing, exists := m.nodes[node.ID]
	if exists {
		existing.LastSeen = node.LastSeen
		existing.Status = node.Status
		if node.Address != "" {
			existing.Address = node.Address
		}
		// Update metadata if provided
		if node.Metadata != nil {
			existing.Metadata = node.Metadata
		}
	} else {
		m.nodes[node.ID] = node
		log.Printf("Added node: %s (%s)", node.ID, node.Address)
	}
}

// Implement ClusterOperations interface

// GetNodes returns all nodes in the cluster
func (m *Manager) GetNodes() map[string]*types.Node {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*types.Node)
	for id, node := range m.nodes {
		result[id] = &types.Node{
			ID:       node.ID,
			Address:  node.Address,
			Status:   node.Status,
			LastSeen: node.LastSeen,
			IsLeader: node.IsLeader,
			Metadata: node.Metadata,
		}
	}
	return result
}

// GetLeader returns the current leader node ID
func (m *Manager) GetLeader() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for id, node := range m.nodes {
		if node.IsLeader {
			return id
		}
	}
	return ""
}

// IsLeader checks if current node is the leader
func (m *Manager) IsLeader() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isLeader
}

// GetNodeID returns the current node's ID
func (m *Manager) GetNodeID() string {
	return m.nodeID
}

// BroadcastToNodes sends a message to all nodes
func (m *Manager) BroadcastToNodes(operation string, data interface{}) error {
	message := map[string]interface{}{
		"operation": operation,
		"data":      data,
		"from":      m.nodeID,
		"timestamp": time.Now().Unix(),
	}

	messageData, _ := json.Marshal(message)

	for id, node := range m.GetNodes() {
		if id == m.nodeID {
			continue // Skip self
		}

		if err := m.sendToNodeAddress(node.Address, messageData); err != nil {
			log.Printf("Failed to send message to node %s: %v", id, err)
		}
	}

	return nil
}

// SendToNode sends a message to a specific node
func (m *Manager) SendToNode(nodeID string, operation string, data interface{}) error {
	node, exists := m.GetNodes()[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	message := map[string]interface{}{
		"operation": operation,
		"data":      data,
		"from":      m.nodeID,
		"timestamp": time.Now().Unix(),
	}

	messageData, _ := json.Marshal(message)
	return m.sendToNodeAddress(node.Address, messageData)
}

// sendToNodeAddress sends data to a specific node address
func (m *Manager) sendToNodeAddress(address string, data []byte) error {
	// Extract host and port from address
	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid address format: %s", address)
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid port in address: %s", address)
	}

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", parts[0], port))
	if err != nil {
		return err
	}

	_, err = m.udpConn.WriteToUDP(data, addr)
	return err
}

// GetClusterState returns the current cluster state
func (m *Manager) GetClusterState() *types.ClusterState {
	nodes := m.GetNodes()
	leader := m.GetLeader()

	return &types.ClusterState{
		Nodes:       nodes,
		Leader:      leader,
		LastUpdated: time.Now(),
		Version:     1, // Simple version for now
	}
}
