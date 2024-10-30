package cluster

import (
	"agent/internal/config"
	"agent/internal/operator"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type ManagerOptions struct {
	ConfigPath  string
	BindAddress string
	BindPort    int
}

type Manager struct {
	cfg         *config.Config
	localNode   *Node
	nodes       map[string]*Node
	nodesMu     sync.RWMutex
	leaderID    string
	conn        *net.UDPConn
	ctx         context.Context
	cancel      context.CancelFunc
	operators   map[string]operator.Operator
	operatorsMu sync.RWMutex
}

func NewManager(opts ManagerOptions) (*Manager, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	shortHostname := hostname
	if idx := strings.Index(hostname, "."); idx != -1 {
		shortHostname = hostname[:idx]
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		cfg: cfg,
		localNode: &Node{
			ID:       shortHostname,
			Hostname: shortHostname,
			Address:  opts.BindAddress,
			Port:     opts.BindPort,
			State:    StateFollower,
			LastSeen: time.Now(),
		},
		nodes:  make(map[string]*Node),
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (m *Manager) Start() error {
	addr := &net.UDPAddr{
		IP:   net.ParseIP(m.localNode.Address),
		Port: m.localNode.Port,
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP listener: %w", err)
	}
	m.conn = conn

	// Initialize nodes from config
	for hostname, addrStr := range m.cfg.Nodes {
		shortHostname := hostname
		if idx := strings.Index(hostname, "."); idx != -1 {
			shortHostname = hostname[:idx]
		}

		if shortHostname != m.localNode.Hostname {
			// Parse the address from config
			_, port, err := config.ParseAddress(addrStr)
			if err != nil {
				log.Printf("Warning: invalid address for node %s: %v", hostname, err)
				continue
			}

			ip, err := resolveHostname(strings.Split(addrStr, ":")[0])
			if err != nil {
				log.Printf("Warning: could not resolve %s: %v", hostname, err)
				ip = strings.Split(addrStr, ":")[0]
			}

			m.nodes[shortHostname] = &Node{
				ID:       shortHostname,
				Hostname: strings.Split(addrStr, ":")[0],
				Address:  ip,
				Port:     port,
				State:    StateUnknown,
				LastSeen: time.Now(),
			}
			log.Printf("Added node %s with address %s:%d", shortHostname, ip, port)
		}
	}
	m.electNewLeader()
	go m.receiveMessages()
	go m.sendHeartbeats()
	go m.monitorNodes()
	go m.printStatus()

	log.Printf("Node started: %s listening on %s:%d",
		m.localNode.Hostname,
		m.localNode.Address,
		m.localNode.Port)

	return nil
}

func resolveHostname(hostname string) (string, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return "", fmt.Errorf("could not resolve host %s: %v", hostname, err)
	}

	// Prefer IPv4
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found for %s", hostname)
}

func (m *Manager) broadcast() {
	if m.leaderID == m.localNode.ID && m.localNode.State != StateLeader {
		m.localNode.State = StateLeader
	}
	data, err := m.localNode.Marshal()
	if err != nil {
		log.Printf("Error marshaling node data: %v", err)
		return
	}

	for hostname, node := range m.nodes {
		if hostname == m.localNode.Hostname {
			continue
		}

		// Resolve IP address
		ip, err := resolveHostname(node.Hostname)
		if err != nil {
			log.Printf("Error resolving hostname %s: %v", node.Hostname, err)
			continue
		}

		addr := &net.UDPAddr{
			IP:   net.ParseIP(ip),
			Port: node.Port,
		}

		_, err = m.conn.WriteToUDP(data, addr)
		if err != nil {
			log.Printf("Error sending heartbeat to %s (%s:%d): %v",
				hostname, ip, node.Port, err)
		} else {
			log.Printf("Sent heartbeat to %s (%s:%d)", hostname, ip, node.Port)
		}
	}
}

func (m *Manager) sendHeartbeats() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.broadcast()
		}
	}
}

func (m *Manager) receiveMessages() {
	buffer := make([]byte, 1024)
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
			n, remoteAddr, err := m.conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			var msg Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			// Skip messages from self
			if msg.ID == m.localNode.ID {
				continue
			}

			msg.Address = remoteAddr.IP.String()
			m.handleMessage(msg)
		}
	}
}

func (m *Manager) handleMessage(msg Message) {
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()

	// Update node information
	if msg.ID != m.localNode.ID {
		if existing, exists := m.nodes[msg.ID]; exists {
			existing.LastSeen = time.Now()
			existing.State = msg.State
			log.Printf("Updated node %s state to %s", msg.ID, msg.State)
		} else {
			m.nodes[msg.ID] = &Node{
				ID:       msg.ID,
				Hostname: msg.Hostname,
				Address:  msg.Address,
				Port:     msg.Port,
				State:    msg.State,
				LastSeen: time.Now(),
			}
			log.Printf("New node discovered: %s", msg.ID)
		}
	}

	// Handle leader election
	if msg.State == StateLeader {
		// If message is from current leader, update last seen
		if msg.ID == m.leaderID {
			return
		}

		// If message is from a node with lower ID than current leader
		if m.leaderID == "" || msg.ID < m.leaderID {
			m.leaderID = msg.ID
			m.localNode.State = StateFollower
			log.Printf("Following new leader: %s", msg.ID)
		} else if msg.ID > m.leaderID && m.localNode.ID == m.leaderID {
			// We're the leader and we have a lower ID, keep leadership
			log.Printf("Keeping leadership (lower ID than %s)", msg.ID)
		}
	}

	// If no leader, initiate election
	if m.leaderID == "" {
		m.electNewLeader()
	}
}

func (m *Manager) monitorNodes() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkNodesHealth()
		}
	}
}

func (m *Manager) checkNodesHealth() {
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()

	now := time.Now()
	leaderLost := false

	// Check for dead nodes
	for id, node := range m.nodes {
		if now.Sub(node.LastSeen) > nodeTimeout {
			log.Printf("Node timeout: %s", id)
			delete(m.nodes, id)

			if id == m.leaderID {
				log.Printf("Leader %s timed out, initiating new election", id)
				m.leaderID = ""
				leaderLost = true
			}
		}
	}

	// Immediate leader election if leader was lost
	if leaderLost {
		m.electNewLeader()
	}
}

func (m *Manager) printStatus() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			fmt.Println("\nCluster Status:")
			fmt.Printf("Local node: %s (State: %s)\n",
				m.localNode.Hostname,
				m.localNode.State)
			fmt.Printf("Leader: %s\n", m.leaderID)

			m.nodesMu.RLock()
			fmt.Println("Cluster nodes:")
			for _, node := range m.nodes {
				fmt.Printf("- %s (State: %s, Last seen: %s)\n",
					node.Hostname,
					node.State,
					time.Since(node.LastSeen).Round(time.Second))
			}
			m.nodesMu.RUnlock()
		}
	}
}

func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		m.conn.Close()
	}
}

func (m *Manager) electNewLeader() {
	// Already have a leader
	if m.leaderID != "" {
		return
	}

	// Find the node with lowest ID (including ourselves) (we can also choose this random or by hashing but kiss for now)
	lowestID := m.localNode.ID
	isLowest := true

	for id := range m.nodes {
		if id < lowestID {
			isLowest = false
			lowestID = id
		}
	}

	if isLowest {
		m.leaderID = m.localNode.ID
		m.localNode.State = StateLeader
		log.Printf("Became new leader: %s", m.localNode.ID)
	} else {
		m.localNode.State = StateFollower
		m.leaderID = lowestID
		log.Printf("Following new leader: %s", lowestID)
	}
}

func (m *Manager) GetNodes() map[string]*Node {
	m.nodesMu.RLock()
	defer m.nodesMu.RUnlock()

	nodes := make(map[string]*Node)
	for k, v := range m.nodes {
		nodeCopy := *v
		nodes[k] = &nodeCopy
	}
	return nodes
}

func (m *Manager) GetLocalNode() *Node {
	nodeCopy := *m.localNode
	return &nodeCopy
}

func (m *Manager) GetLeaderID() string {
	return m.leaderID
}

func (m *Manager) GetClusterName() string {
	return m.cfg.Cluster.Name
}

func (m *Manager) RegisterOperator(op operator.Operator) error {
	m.operatorsMu.Lock()
	defer m.operatorsMu.Unlock()

	if m.operators == nil {
		m.operators = make(map[string]operator.Operator)
	}

	name := op.Info().Name
	if _, exists := m.operators[name]; exists {
		return fmt.Errorf("operator %s already registered", name)
	}

	m.operators[name] = op
	return nil
}

func (m *Manager) GetOperator(name string) (operator.Operator, error) {
	m.operatorsMu.RLock()
	defer m.operatorsMu.RUnlock()

	op, exists := m.operators[name]
	if !exists {
		return nil, fmt.Errorf("operator %s not found", name)
	}

	return op, nil
}

func (m *Manager) ExecuteOperator(ctx context.Context, name string, params map[string]interface{}) error {
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

// ListOperators returns a list of all registered operators
func (m *Manager) ListOperators() []operator.OperatorInfo {
	m.operatorsMu.RLock()
	defer m.operatorsMu.RUnlock()

	var operators []operator.OperatorInfo
	for name := range m.operators {

		op, err := m.GetOperator(name)
		if err != nil {
			return nil
		}
		operators = append(operators, op.Info())
	}
	return operators
}

// BroadcastOperatorResult broadcasts operator results to all nodes if needed
func (m *Manager) BroadcastOperatorResult(operatorName string, result map[string]interface{}) {
	// Only leader can broadcast operator results
	if m.localNode.State != StateLeader {
		return
	}

	// Create operator result message
	msg := Message{
		ID:       m.localNode.ID,
		Hostname: m.localNode.Hostname,
		State:    m.localNode.State,
		Type:     "operator_result",
		Data: map[string]interface{}{
			"operator": operatorName,
			"result":   result,
		},
	}

	// Broadcast to all nodes
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling operator result: %v", err)
		return
	}

	for _, node := range m.nodes {
		addr := &net.UDPAddr{
			IP:   net.ParseIP(node.Address),
			Port: node.Port,
		}

		_, err = m.conn.WriteToUDP(data, addr)
		if err != nil {
			log.Printf("Error sending operator result to %s: %v", node.Hostname, err)
		}
	}
}

func validateParams(params map[string]interface{}, schema map[string]operator.ParamSchema) error {
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

// TODO external triggering of operator execution
// TODO operator result handling
