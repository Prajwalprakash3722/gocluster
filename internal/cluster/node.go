package cluster

import (
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

type MessageType string

const (
	MessageTypeHeartbeat       MessageType = "heartbeat"
	MessageTypeOperator        MessageType = "operator"
	MessageTypeOperatorResult  MessageType = "operator_result"
	MessageTypeOperatorError   MessageType = "operator_error"
	MessageTypeOperatorTrigger MessageType = "operator_trigger"
	MessageTypeOperatorStatus  MessageType = "operator_status"
)

const (
	heartbeatInterval = 2 * time.Second
	nodeTimeout       = 6 * time.Second
)

type Node struct {
	ID       string    `json:"id"`
	Hostname string    `json:"hostname"`
	Address  string    `json:"address"`
	Port     int       `json:"port"`
	State    NodeState `json:"state"`
	LastSeen time.Time `json:"last_seen"`
}

type Message struct {
	ID        string                 `json:"id"`
	Hostname  string                 `json:"hostname"`
	Address   string                 `json:"address"`
	Port      int                    `json:"port"`
	State     NodeState              `json:"state"`
	Timestamp time.Time              `json:"timestamp"`
	Type      MessageType            `json:"type"`
	Data      map[string]interface{} `json:"data,omitempty"`
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
