// internal/cluster/manager.go
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
	"strings"
	"sync"
	"time"
)

type ManagerOptions struct {
	ConfigPath      string
	BindAddress     string
	BindPort        int
	OperatorManager *operator.OperatorManager
}

type Manager struct {
	cfg             *config.Config
	localNode       *types.Node
	nodes           map[string]*types.Node
	nodesMu         sync.RWMutex
	leaderID        string
	conn            *net.UDPConn
	ctx             context.Context
	cancel          context.CancelFunc
	operatorManager *operator.OperatorManager
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
		localNode: &types.Node{
			ID:       shortHostname,
			Hostname: shortHostname,
			Address:  opts.BindAddress,
			Port:     opts.BindPort,
			State:    types.StateFollower,
			LastSeen: time.Now(),
		},
		nodes:           make(map[string]*types.Node),
		ctx:             ctx,
		cancel:          cancel,
		operatorManager: opts.OperatorManager,
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

			m.nodes[shortHostname] = &types.Node{
				ID:       shortHostname,
				Hostname: strings.Split(addrStr, ":")[0],
				Address:  ip,
				Port:     port,
				State:    types.StateUnknown,
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

func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		m.conn.Close()
	}
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

func (m *Manager) ExecuteOperator(ctx context.Context, execMsg types.OperatorExecMessage) error {
	log.Printf("[ClusterManager] Starting operator execution on node %s", m.localNode.ID)
	log.Printf("[ClusterManager] Execution details - ID: %s, Operator: %s, Operation: %s",
		execMsg.ExecutionID, execMsg.OperatorName, execMsg.Operation)
	if m.operatorManager == nil {
		log.Printf("[ClusterManager] Operator manager not initialized")
		return fmt.Errorf("operator manager not initialized")
	}
	op, err := m.operatorManager.GetOperator(execMsg.OperatorName)
	if err != nil {
		log.Printf("[ClusterManager] Error getting operator %s: %v", execMsg.OperatorName, err)
		return err
	}
	log.Printf("[ClusterManager] Successfully retrieved operator %s", execMsg.OperatorName)

	startTime := time.Now()
	log.Printf("[ClusterManager] Starting execution at: %v", startTime)

	params := map[string]interface{}{
		"operation": execMsg.Operation,
		"params":    execMsg.Params,
		"config":    execMsg.Config,
	}
	log.Printf("[ClusterManager] Executing with params: %+v", params)

	err = op.Execute(ctx, params)
	endTime := time.Now()
	duration := time.Since(startTime)

	log.Printf("[ClusterManager] Execution completed in %v, success: %v", duration, err == nil)
	if err != nil {
		op.Rollback(ctx)
		log.Printf("[ClusterManager] Execution error: %v", err)
	}

	result := types.OperatorResult{
		ExecutionID:  execMsg.ExecutionID,
		NodeID:       m.localNode.ID,
		NodeHostname: m.localNode.Hostname,
		NodeAddress:  m.localNode.Address,
		NodeState:    m.localNode.State,
		OperatorName: execMsg.OperatorName,
		Operation:    execMsg.Operation,
		Params:       execMsg.Params,
		Config:       execMsg.Config,
		StartTime:    startTime,
		EndTime:      endTime,
		Duration:     duration.String(),
		Success:      err == nil,
	}
	if err != nil {
		result.Error = err.Error()
	}

	log.Printf("[ClusterManager] Created result for execution ID %s: %+v", execMsg.ExecutionID, result)

	isLeader := m.localNode.State == types.StateLeader
	log.Printf("[ClusterManager] Current node is leader: %v", isLeader)

	if !isLeader {
		log.Printf("[ClusterManager] Sending result to leader node")
		err = m.SendResultToLeader(result)
		if err != nil {
			log.Printf("[ClusterManager] Error sending result to leader: %v", err)
		} else {
			log.Printf("[ClusterManager] Successfully sent result to leader")
		}
		return err
	}

	log.Printf("[ClusterManager] We are leader, handling result directly")
	m.operatorManager.HandleOperatorResult(&result)
	log.Printf("[ClusterManager] Successfully handled result for execution ID: %s", execMsg.ExecutionID)

	return nil
}

func (m *Manager) GetLocalNodeID() string {
	return m.localNode.ID
}

func (m *Manager) GetLocalNodeState() types.NodeState {
	return m.localNode.State
}

func (m *Manager) IsLeader() bool {
	return m.localNode.State == types.StateLeader
}

func (m *Manager) GetNodeCount() int {
	m.nodesMu.RLock()
	defer m.nodesMu.RUnlock()
	return len(m.nodes) + 1
}

func (m *Manager) BroadcastOperatorExecution(msg types.OperatorExecMessage) error {
	log.Printf("[ClusterManager] Starting broadcast for operator execution: %s", msg.OperatorName)

	// Execute locally
	log.Printf("[ClusterManager] Executing operator locally first")
	go func() {
		if err := m.ExecuteOperator(context.Background(), msg); err != nil {
			log.Printf("[ClusterManager] Error in local execution: %v", err)
		} else {
			log.Printf("[ClusterManager] Local execution started successfully")
		}
	}()

	// Create wrapped message
	execData, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ClusterManager] Error marshaling exec message: %v", err)
		return err
	}

	message := types.Message{
		ID:       m.localNode.ID,
		Hostname: m.localNode.Hostname,
		Address:  m.localNode.Address,
		Port:     m.localNode.Port,
		State:    m.localNode.State,
		Type:     types.MessageTypeOperatorExec,
		Data:     execData,
	}

	// Marshal the complete message
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("[ClusterManager] Error marshaling wrapper message: %v", err)
		return err
	}

	log.Printf("[ClusterManager] Broadcasting to %d nodes", len(m.nodes))
	for _, node := range m.nodes {
		addr := &net.UDPAddr{
			IP:   net.ParseIP(node.Address),
			Port: node.Port,
		}
		log.Printf("[ClusterManager] Sending to node %s at %s:%d",
			node.Hostname, node.Address, node.Port)

		n, err := m.conn.WriteToUDP(data, addr)
		if err != nil {
			log.Printf("[ClusterManager] Error sending to %s: %v", node.Hostname, err)
		} else {
			log.Printf("[ClusterManager] Successfully sent %d bytes to %s", n, node.Hostname)
		}
	}

	return nil
}

func (m *Manager) SendResultToLeader(result types.OperatorResult) error {
	if m.leaderID == "" {
		return fmt.Errorf("no leader available")
	}

	var leaderNode *types.Node
	m.nodesMu.RLock()
	for _, node := range m.nodes {
		if node.ID == m.leaderID {
			leaderNode = node
			break
		}
	}
	m.nodesMu.RUnlock()

	if leaderNode == nil {
		return fmt.Errorf("leader node not found")
	}

	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	addr := &net.UDPAddr{
		IP:   net.ParseIP(leaderNode.Address),
		Port: leaderNode.Port,
	}

	_, err = m.conn.WriteToUDP(data, addr)
	return err
}

func (m *Manager) SendExecRequestToLeader(msg types.OperatorExecMessage) error {
	if m.leaderID == "" {
		return fmt.Errorf("no leader available")
	}

	var leaderNode *types.Node
	m.nodesMu.RLock()
	for _, node := range m.nodes {
		if node.ID == m.leaderID {
			leaderNode = node
			break
		}
	}
	m.nodesMu.RUnlock()

	if leaderNode == nil {
		return fmt.Errorf("leader node not found")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	addr := &net.UDPAddr{
		IP:   net.ParseIP(leaderNode.Address),
		Port: leaderNode.Port,
	}

	_, err = m.conn.WriteToUDP(data, addr)
	return err
}

func (m *Manager) GetNodes() map[string]*types.Node {
	m.nodesMu.RLock()
	defer m.nodesMu.RUnlock()

	// Create a copy of the nodes map to avoid data races
	nodes := make(map[string]*types.Node)
	for k, v := range m.nodes {
		nodeCopy := *v
		nodes[k] = &nodeCopy
	}
	return nodes
}

func (m *Manager) GetLocalNode() *types.Node {
	// Return a copy to avoid data races
	nodeCopy := *m.localNode
	return &nodeCopy
}

func (m *Manager) GetLeaderID() string {
	return m.leaderID
}

func (m *Manager) GetClusterName() string {
	return m.cfg.Cluster.Name
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
		m.localNode.State = types.StateLeader
		log.Printf("Became new leader: %s", m.localNode.ID)
	} else {
		m.localNode.State = types.StateFollower
		m.leaderID = lowestID
		log.Printf("Following new leader: %s", lowestID)
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

func (m *Manager) checkNodesHealth() {
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()

	now := time.Now()
	leaderLost := false

	// Check for dead nodes
	for id, node := range m.nodes {
		if now.Sub(node.LastSeen) > types.NodeTimeout {
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

func (m *Manager) monitorNodes() {
	ticker := time.NewTicker(types.HeartbeatInterval)
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

func (m *Manager) handleMessage(msg types.Message) {
	m.nodesMu.Lock()
	defer m.nodesMu.Unlock()

	// Update node information
	if msg.ID != m.localNode.ID {
		if existing, exists := m.nodes[msg.ID]; exists {
			existing.LastSeen = time.Now()
			existing.State = msg.State
			log.Printf("Updated node %s state to %s", msg.ID, msg.State)
		} else {
			m.nodes[msg.ID] = &types.Node{
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
	if msg.State == types.StateLeader {
		// If message is from current leader, update last seen
		if msg.ID == m.leaderID {
			return
		}

		// If message is from a node with lower ID than current leader
		if m.leaderID == "" || msg.ID < m.leaderID {
			m.leaderID = msg.ID
			m.localNode.State = types.StateFollower
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

func (m *Manager) sendHeartbeats() {
	ticker := time.NewTicker(types.HeartbeatInterval)
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
	buffer := make([]byte, 4096) // Increased buffer size
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
			n, remoteAddr, err := m.conn.ReadFromUDP(buffer)
			if err != nil {
				log.Printf("[ClusterManager] Error reading UDP: %v", err)
				continue
			}
			log.Printf("[ClusterManager] Received %d bytes from %s", n, remoteAddr.String())

			var msg types.Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				log.Printf("[ClusterManager] Error unmarshaling message: %v", err)
				log.Printf("[ClusterManager] Raw message: %s", string(buffer[:n]))
				continue
			}

			log.Printf("[ClusterManager] Received message type: %s from node: %s", msg.Type, msg.ID)

			// Skip messages from self
			if msg.ID == m.localNode.ID {
				log.Printf("[ClusterManager] Skipping message from self")
				continue
			}

			msg.Address = remoteAddr.IP.String()

			switch msg.Type {
			case types.MessageTypeHeartbeat:
				m.handleMessage(msg)

			case types.MessageTypeOperatorExec:
				log.Printf("[ClusterManager] Received operator execution message")
				var execMsg types.OperatorExecMessage
				if err := json.Unmarshal(msg.Data, &execMsg); err != nil {
					log.Printf("[ClusterManager] Error unmarshaling operator exec message: %v", err)
					continue
				}
				log.Printf("[ClusterManager] Executing operator from message: %+v", execMsg)
				go m.ExecuteOperator(context.Background(), execMsg)

			case types.MessageTypeOperatorResult:
				log.Printf("[ClusterManager] Received operator result message")
				var result types.OperatorResult
				if err := json.Unmarshal(msg.Data, &result); err != nil {
					log.Printf("[ClusterManager] Error unmarshaling operator result: %v", err)
					continue
				}
				if m.operatorManager != nil {
					log.Printf("[ClusterManager] Handling operator result: %+v", result)
					m.operatorManager.HandleOperatorResult(&result)
				}

			default:
				log.Printf("[ClusterManager] Unknown message type: %s", msg.Type)
			}
		}
	}
}

func (m *Manager) broadcast() {
	if m.leaderID == m.localNode.ID && m.localNode.State != types.StateLeader {
		m.localNode.State = types.StateLeader
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
