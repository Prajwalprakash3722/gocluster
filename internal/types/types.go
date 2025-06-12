package types

import (
	"context"
	"time"
)

// Node represents a cluster node
type Node struct {
	ID       string                 `json:"id"`
	Address  string                 `json:"address"`
	Status   string                 `json:"status"`
	LastSeen time.Time              `json:"last_seen"`
	IsLeader bool                   `json:"is_leader"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ClusterState represents the current state of the cluster
type ClusterState struct {
	Nodes       map[string]*Node `json:"nodes"`
	Leader      string           `json:"leader"`
	LastUpdated time.Time        `json:"last_updated"`
	Version     int64            `json:"version"`
}

// OperatorInfo provides metadata about an operator
type OperatorInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// OperationResult represents the result of an operation
type OperationResult struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	NodeID    string                 `json:"node_id"`
}

// Operator interface defines the contract for cluster operators
type Operator interface {
	// Info returns operator metadata
	Info() OperatorInfo

	// Init initializes the operator with configuration
	Init(config map[string]interface{}) error

	// Execute performs an operation with given parameters
	Execute(ctx context.Context, operation string, params map[string]interface{}) (*OperationResult, error)

	// GetOperations returns list of supported operations
	GetOperations() []string

	// GetOperationSchema returns the schema for all operations
	GetOperationSchema() OperatorSchema

	// Cleanup performs cleanup when operator is being stopped
	Cleanup() error
}

// ClusterOperations interface defines operations that can be performed on the cluster
type ClusterOperations interface {
	// GetNodes returns all nodes in the cluster
	GetNodes() map[string]*Node

	// GetLeader returns the current leader node ID
	GetLeader() string

	// IsLeader checks if current node is the leader
	IsLeader() bool

	// GetNodeID returns the current node's ID
	GetNodeID() string

	// BroadcastToNodes sends a message to all nodes
	BroadcastToNodes(operation string, data interface{}) error

	// SendToNode sends a message to a specific node
	SendToNode(nodeID string, operation string, data interface{}) error
}

// OperatorRequest represents a request to execute an operator
type OperatorRequest struct {
	Operation     string                 `json:"operation"`
	Params        map[string]interface{} `json:"params"`
	Config        map[string]interface{} `json:"config,omitempty"`
	NodeID        string                 `json:"node_id,omitempty"`
	Parallel      bool                   `json:"parallel,omitempty"`
	ExecutionType string                 `json:"execution_type,omitempty"` // "local", "node", "broadcast", "rolling"
	TargetNodes   []string               `json:"target_nodes,omitempty"`   // For rolling or specific node list
	RollingDelay  string                 `json:"rolling_delay,omitempty"`  // Delay between nodes (e.g., "30s")
}

// OperatorResponse represents the response from an operator execution
type OperatorResponse struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	NodeID    string                 `json:"node_id"`
	Operation string                 `json:"operation"`
}

// BackendType defines the type of backend storage
type BackendType string

const (
	BackendMemory    BackendType = "memory"
	BackendEtcd      BackendType = "etcd"
	BackendZooKeeper BackendType = "zookeeper"
)

// BackendConfig represents backend configuration
type BackendConfig struct {
	Type      BackendType `yaml:"type"`
	Namespace string      `yaml:"namespace"`

	// etcd specific
	EtcdEndpoints []string `yaml:"etcd_endpoints"`
	EtcdUsername  string   `yaml:"etcd_username"`
	EtcdPassword  string   `yaml:"etcd_password"`
	EtcdCertFile  string   `yaml:"etcd_cert_file"`
	EtcdKeyFile   string   `yaml:"etcd_key_file"`
	EtcdCAFile    string   `yaml:"etcd_ca_file"`

	// ZooKeeper specific
	ZKHosts       []string `yaml:"zk_hosts"`
	ZKTimeout     string   `yaml:"zk_timeout"`
	ZKSessionPath string   `yaml:"zk_session_path"`

	// Leader election
	LeaderTTL     string `yaml:"leader_ttl"`
	RenewInterval string `yaml:"renew_interval"`
}

// JobStep represents a single step in a job
type JobStep struct {
	Name            string            `yaml:"name" json:"name"`
	Script          string            `yaml:"script,omitempty" json:"script,omitempty"`
	Command         string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args            []string          `yaml:"args,omitempty" json:"args,omitempty"`
	WorkingDir      string            `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
	Environment     map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Timeout         string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ContinueOnError bool              `yaml:"continue_on_error,omitempty" json:"continue_on_error,omitempty"`
}

// Job represents a job that can be executed
type Job struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	WorkingDir  string            `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty" json:"environment,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	OnFailure   string            `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`
	Steps       []JobStep         `yaml:"steps" json:"steps"`
}

// JobExecution represents a job execution instance
type JobExecution struct {
	ID          string                 `json:"id"`
	JobName     string                 `json:"job_name"`
	Status      string                 `json:"status"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Result      *OperationResult       `json:"result,omitempty"`
	Logs        []string               `json:"logs,omitempty"`
	StepResults map[string]interface{} `json:"step_results,omitempty"`
}

// ParameterSchema defines the schema for an operation parameter
type ParameterSchema struct {
	Type        string      `json:"type"`                // "string", "int", "bool", "float", "array", "object"
	Required    bool        `json:"required"`            // Whether the parameter is required
	Default     interface{} `json:"default"`             // Default value if not provided
	Description string      `json:"description"`         // Human readable description
	Options     []string    `json:"options,omitempty"`   // Valid options for enum-like parameters
	MinValue    *float64    `json:"min_value,omitempty"` // Minimum value for numeric types
	MaxValue    *float64    `json:"max_value,omitempty"` // Maximum value for numeric types
	Pattern     string      `json:"pattern,omitempty"`   // Regex pattern for string validation
	Example     interface{} `json:"example,omitempty"`   // Example value
}

// OperationSchema defines the schema for an operation
type OperationSchema struct {
	Description string                     `json:"description"`
	Parameters  map[string]ParameterSchema `json:"parameters"`
	Config      map[string]ParameterSchema `json:"config,omitempty"`
	Examples    []map[string]interface{}   `json:"examples,omitempty"`
}

// OperatorSchema defines the complete schema for an operator
type OperatorSchema struct {
	Name        string                     `json:"name"`
	Version     string                     `json:"version"`
	Description string                     `json:"description"`
	Author      string                     `json:"author"`
	Operations  map[string]OperationSchema `json:"operations"`
}
