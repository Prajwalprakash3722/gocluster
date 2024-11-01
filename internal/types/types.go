// internal/types/types.go
package types

import (
	"context"
	"encoding/json"
	"time"
)

type NodeState string

const (
	StateUnknown  NodeState = "unknown"
	StateFollower NodeState = "follower"
	StateLeader   NodeState = "leader"
	StateOffline  NodeState = "offline"
)

type Node struct {
	ID       string    `json:"id"`
	Hostname string    `json:"hostname"`
	Address  string    `json:"address"`
	Port     int       `json:"port"`
	State    NodeState `json:"state"`
	LastSeen time.Time `json:"last_seen"`
}

type ClusterOperations interface {
	GetLocalNodeID() string
	GetLocalNodeState() NodeState
	IsLeader() bool
	GetNodeCount() int
	BroadcastOperatorExecution(msg OperatorExecMessage) error
	SendResultToLeader(result OperatorResult) error
	SendExecRequestToLeader(msg OperatorExecMessage) error
}

type OperatorExecMessage struct {
	ExecutionID  string                 `json:"execution_id"`
	OperatorName string                 `json:"operator_name"`
	Operation    string                 `json:"operation"`
	Params       map[string]interface{} `json:"params"`
	Config       map[string]interface{} `json:"config"`
	Parallel     bool                   `json:"parallel"`
}

type OperatorResult struct {
	ExecutionID  string                 `json:"execution_id"`
	NodeID       string                 `json:"node_id"`
	NodeHostname string                 `json:"node_hostname"`
	NodeAddress  string                 `json:"node_address"`
	NodeState    NodeState              `json:"node_state"`
	OperatorName string                 `json:"operator_name"`
	Operation    string                 `json:"operation"`
	Params       map[string]interface{} `json:"params"`
	Config       map[string]interface{} `json:"config"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Duration     string                 `json:"duration"`
	Success      bool                   `json:"success"`
	Result       interface{}            `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
}

type DistributedExecution struct {
	ExecutionID string
	StartTime   time.Time
	Results     map[string]*OperatorResult
	ResultsChan chan *OperatorResult
	Done        chan struct{}
}

func NewDistributedExecution(executionID string) *DistributedExecution {
	return &DistributedExecution{
		ExecutionID: executionID,
		StartTime:   time.Now(),
		Results:     make(map[string]*OperatorResult),
		ResultsChan: make(chan *OperatorResult),
		Done:        make(chan struct{}),
	}
}

// Operator defines the interface that all operators must implement
// Operator represents a plugin that can perform operations on the cluster
type Operator interface {
	// Name returns the unique identifier of the operator
	Info() OperatorInfo

	// Init initializes the operator with cluster configuration
	Init(config map[string]interface{}) error

	// Execute runs the operator's logic
	Execute(ctx context.Context, params map[string]interface{}) error

	// Rollback handles failure scenarios
	Rollback(ctx context.Context) error

	// Cleanup performs any necessary cleanup
	Cleanup() error
}

// ParameterInfo describes a parameter that the operator accepts
type ParameterInfo struct {
	Name        string // Name of the parameter
	Type        string // Type of the parameter (string, int, bool, etc)
	Required    bool   // Whether the parameter is required
	Description string // Description of what the parameter does
	Default     string // Default value, if any
}

// OperatorInfo contains operator metadata and schema
type OperatorInfo struct {
	Name        string                     `json:"name"`
	Version     string                     `json:"version"`
	Description string                     `json:"description"`
	Author      string                     `json:"author"`
	Operations  map[string]OperationSchema `json:"operations"`
}

// OperationSchema describes an individual operation
type OperationSchema struct {
	Description string                 `json:"description"`
	Parameters  map[string]ParamSchema `json:"parameters"`
	Namespace   map[string]ParamSchema `json:"namespace"`
	Config      map[string]ParamSchema `json:"config"`
}

// ParamSchema describes parameter requirements
type ParamSchema struct {
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
}

// Status represents the current state of an operator execution
type Status struct {
	State    string                 // Current state (running, completed, failed)
	Progress int                    // Progress percentage (0-100)
	Message  string                 // Current status message
	Data     map[string]interface{} // Additional status data
}

type MessageType string

const (
	MessageTypeHeartbeat      MessageType = "heartbeat"
	MessageTypeOperatorExec   MessageType = "operator_exec"
	MessageTypeOperatorResult MessageType = "operator_result"
)

const (
	HeartbeatInterval = 2 * time.Second
	NodeTimeout       = 6 * time.Second
)

type Message struct {
	ID        string          `json:"id"`
	Hostname  string          `json:"hostname"`
	Address   string          `json:"address"`
	Port      int             `json:"port"`
	State     NodeState       `json:"state"`
	Timestamp time.Time       `json:"timestamp"`
	Type      MessageType     `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
}

func (n *Node) Marshal() ([]byte, error) {
	msg := Message{
		Type:      "heartbeat",
		ID:        n.ID,
		Hostname:  n.Hostname,
		Address:   n.Address,
		Port:      n.Port,
		State:     n.State,
		Timestamp: time.Now(),
	}
	return json.Marshal(msg)
}
