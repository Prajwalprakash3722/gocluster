// this is meant to be a example of a simple operator that can be used to test the gocluster
// has simple operation
// to checkout complex operator check the mysql_operator.go / aerospike_operator.go
package hello_operator

import (
	"agent/internal/types"
	"context"
	"fmt"
	"time"
)

type HelloOperator struct {
	defaultGreeting string
}

func New() types.Operator {
	return &HelloOperator{
		defaultGreeting: "Hello",
	}
}

// Info returns information about the operator
func (o *HelloOperator) Info() types.OperatorInfo {
	return types.OperatorInfo{
		Name:        "hello-world",
		Version:     "1.0.0",
		Description: "A simple hello world operator for testing",
		Author:      "prajwal.p",
		Operations: map[string]types.OperationSchema{
			"greet": {
				Description: "Send a greeting message",
				Parameters: map[string]types.ParamSchema{
					"name": {
						Type:        "string",
						Required:    true,
						Description: "Name of the person to greet",
					},
					"language": {
						Type:        "string",
						Required:    false,
						Default:     "english",
						Description: "Language for greeting (english, spanish, french)",
					},
				},
				Config: map[string]types.ParamSchema{
					"uppercase": {
						Type:        "bool",
						Required:    false,
						Default:     false,
						Description: "Convert greeting to uppercase",
					},
					"timestamp": {
						Type:        "bool",
						Required:    false,
						Default:     true,
						Description: "Include timestamp in greeting",
					},
				},
			},
		},
	}
}

// Init initializes the operator
func (o *HelloOperator) Init(config map[string]interface{}) error {
	if greeting, ok := config["default_greeting"].(string); ok {
		o.defaultGreeting = greeting
	}
	return nil
}

// Execute runs the operator
func (o *HelloOperator) Execute(ctx context.Context, params map[string]interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Extract operation parameters
		operation, ok := params["operation"].(string)
		if !ok {
			return fmt.Errorf("operation not specified")
		}

		if operation != "greet" {
			return fmt.Errorf("unknown operation: %s", operation)
		}

		operationParams, ok := params["params"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("operation parameters not provided")
		}

		// Get name parameter
		name, ok := operationParams["name"].(string)
		if !ok {
			return fmt.Errorf("name parameter is required")
		}

		// Get optional parameters
		language := getStringParam(operationParams, "language", "english")

		// Get config parameters
		configParams, _ := params["config"].(map[string]interface{})
		uppercase := getBoolParam(configParams, "uppercase", false)
		includeTimestamp := getBoolParam(configParams, "timestamp", true)

		// Generate greeting
		greeting := o.generateGreeting(language, name)

		if uppercase {
			greeting = fmt.Sprintf("%s!", greeting)
		}

		if includeTimestamp {
			greeting = fmt.Sprintf("[%s] %s", time.Now().Format(time.RFC3339), greeting)
		}

		fmt.Println(greeting)
		return nil
	}
}

// Helper function to generate greeting based on language
func (o *HelloOperator) generateGreeting(language, name string) string {
	switch language {
	case "spanish":
		return fmt.Sprintf("¡Hola, %s!", name)
	case "french":
		return fmt.Sprintf("Bonjour, %s!", name)
	default:
		return fmt.Sprintf("%s, %s!", o.defaultGreeting, name)
	}
}

// Rollback handles failure scenarios
func (o *HelloOperator) Rollback(ctx context.Context) error {
	// Nothing to rollback for this simple operator
	return nil
}

// Cleanup performs any necessary cleanup
func (o *HelloOperator) Cleanup() error {
	// Nothing to clean up for this simple operator
	return nil
}

// Helper functions
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok && val != "" {
		return val
	}
	return defaultValue
}

func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}
