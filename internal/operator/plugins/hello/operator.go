package hello

import (
	"agent/internal/types"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HelloOperator implements a simple hello world operator
type HelloOperator struct {
	defaultGreeting string
}

// New creates a new hello operator instance
func New() *HelloOperator {
	return &HelloOperator{
		defaultGreeting: "Hello",
	}
}

// Info returns operator information
func (h *HelloOperator) Info() types.OperatorInfo {
	return types.OperatorInfo{
		Name:        "hello",
		Version:     "1.0.0",
		Description: "A simple hello world operator for testing",
		Author:      "prajwal.p",
	}
}

// Init initializes the operator with configuration
func (h *HelloOperator) Init(config map[string]interface{}) error {
	if greeting, ok := config["default_greeting"].(string); ok {
		h.defaultGreeting = greeting
	}
	return nil
}

// GetOperations returns the list of supported operations
func (h *HelloOperator) GetOperations() []string {
	return []string{"greet", "info", "status", "touch_file"}
}

// Execute performs the specified operation
func (h *HelloOperator) Execute(ctx context.Context, operation string, params map[string]interface{}) (*types.OperationResult, error) {
	result := &types.OperationResult{
		Timestamp: time.Now(),
		NodeID:    "local", // This will be set by the operator manager
	}

	switch operation {
	case "greet":
		return h.greet(params, result)
	case "info":
		return h.info(result)
	case "status":
		return h.status(result)
	case "touch_file":
		return h.touchFile(params, result)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported operation: %s", operation)
		return result, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// greet performs a greeting operation
func (h *HelloOperator) greet(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	name := "World"
	if n, ok := params["name"].(string); ok && n != "" {
		name = n
	}

	greeting := h.defaultGreeting
	if g, ok := params["greeting"].(string); ok && g != "" {
		greeting = g
	}

	customMessage := ""
	if msg, ok := params["custom_message"].(string); ok && msg != "" {
		customMessage = msg
	}

	var message string
	if customMessage != "" {
		message = fmt.Sprintf("%s, %s! %s", greeting, name, customMessage)
	} else {
		message = fmt.Sprintf("%s, %s!", greeting, name)
	}

	result.Success = true
	result.Message = message
	result.Data = map[string]interface{}{
		"greeting":       greeting,
		"name":           name,
		"custom_message": customMessage,
		"full_message":   message,
	}

	return result, nil
}

// info returns operator information
func (h *HelloOperator) info(result *types.OperationResult) (*types.OperationResult, error) {
	info := h.Info()
	result.Success = true
	result.Message = fmt.Sprintf("Hello Operator v%s", info.Version)
	result.Data = map[string]interface{}{
		"name":             info.Name,
		"version":          info.Version,
		"description":      info.Description,
		"author":           info.Author,
		"operations":       h.GetOperations(),
		"default_greeting": h.defaultGreeting,
	}

	return result, nil
}

// status returns operator status
func (h *HelloOperator) status(result *types.OperationResult) (*types.OperationResult, error) {
	result.Success = true
	result.Message = "Hello operator is running normally"
	result.Data = map[string]interface{}{
		"status":           "running",
		"uptime":           "N/A", // Could track this if needed
		"default_greeting": h.defaultGreeting,
	}

	return result, nil
}

// touchFile creates a file with hostname and timestamp information
func (h *HelloOperator) touchFile(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	// Get target directory, default to /tmp
	targetDir := "/tmp"
	if dir, ok := params["directory"].(string); ok && dir != "" {
		targetDir = dir
	}

	// Get custom filename or use default
	filename := "gocluster-execution.txt"
	if name, ok := params["filename"].(string); ok && name != "" {
		filename = name
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create full file path
	filePath := filepath.Join(targetDir, filename)

	// Create content with execution details
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	content := fmt.Sprintf("GoCluster Execution Evidence\n"+
		"============================\n"+
		"Hostname: %s\n"+
		"Timestamp: %s\n"+
		"Operation: touch_file\n"+
		"Operator: hello\n"+
		"File created on: %s\n"+
		"Working directory: %s\n",
		hostname, timestamp, hostname, targetDir)

	// Write file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to create file: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("File created successfully on %s", hostname)
	result.Data = map[string]interface{}{
		"hostname":  hostname,
		"file_path": filePath,
		"timestamp": timestamp,
		"content":   content,
		"directory": targetDir,
		"filename":  filename,
	}

	return result, nil
}

// Cleanup performs cleanup when the operator is being stopped
func (h *HelloOperator) Cleanup() error {
	// Nothing to cleanup for this simple operator
	return nil
}

// GetOperationSchema returns the schema for all operations
func (h *HelloOperator) GetOperationSchema() types.OperatorSchema {
	info := h.Info()

	return types.OperatorSchema{
		Name:        info.Name,
		Version:     info.Version,
		Description: info.Description,
		Author:      info.Author,
		Operations: map[string]types.OperationSchema{
			"greet": {
				Description: "Send a greeting message",
				Parameters: map[string]types.ParameterSchema{
					"name": {
						Type:        "string",
						Required:    false,
						Default:     "World",
						Description: "Name to greet",
						Example:     "Prajwal",
					},
					"greeting": {
						Type:        "string",
						Required:    false,
						Default:     h.defaultGreeting,
						Description: "Greeting to use",
						Options:     []string{"Hello", "Hi", "Hey", "Good morning", "Good evening"},
						Example:     "Hello",
					},
					"custom_message": {
						Type:        "string",
						Required:    false,
						Description: "Additional custom message to append",
						Example:     "How are you today?",
					},
				},
				Examples: []map[string]interface{}{
					{
						"name":           "Prajwal",
						"greeting":       "Hello",
						"custom_message": "Hope you're having a great day!",
					},
					{
						"name": "Team",
					},
				},
			},
			"info": {
				Description: "Get operator information",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"status": {
				Description: "Get operator status",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"touch_file": {
				Description: "Create a file with execution evidence (hostname, timestamp)",
				Parameters: map[string]types.ParameterSchema{
					"directory": {
						Type:        "string",
						Required:    false,
						Default:     "/tmp",
						Description: "Directory where to create the file",
						Example:     "/tmp",
					},
					"filename": {
						Type:        "string",
						Required:    false,
						Default:     "gocluster-execution.txt",
						Description: "Name of the file to create",
						Example:     "execution-evidence.txt",
					},
				},
				Examples: []map[string]interface{}{
					{
						"directory": "/tmp",
						"filename":  "node-execution-proof.txt",
					},
					{
						"directory": "/var/log",
						"filename":  "gocluster-test.log",
					},
				},
			},
		},
	}
}
