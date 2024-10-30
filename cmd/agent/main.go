package main

import (
	"agent/internal/cluster"
	aerospike_operator "agent/internal/operator/plugins/aerospike"
	"agent/internal/web"
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// OperatorConfig holds configuration for operators
type OperatorConfig struct {
	Enabled bool
}

func main() {
	var opts cluster.ManagerOptions
	var webAddr string
	var operatorCfg OperatorConfig

	// Cluster flags
	// this is not nice, need to refactor this using cobra or similar
	flag.StringVar(&opts.ConfigPath, "config", "cluster.conf", "Path to configuration file")
	flag.StringVar(&opts.BindAddress, "bind-address", "0.0.0.0", "Address to bind to")
	flag.IntVar(&opts.BindPort, "port", 7946, "Port to listen on")
	flag.StringVar(&webAddr, "web", "8080", "Web UI address (e.g., :8080)")

	// Operator flags
	flag.BoolVar(&operatorCfg.Enabled, "enable-operators", false, "Enable operator plugins")

	flag.Parse()

	// Create cluster manager
	manager, err := cluster.NewManager(opts)
	if err != nil {
		log.Fatalf("Failed to create cluster manager: %v", err)
	}

	// Initialize operators if enabled (enabling Aerospike operator for now, usually this would be a plugin system)
	if operatorCfg.Enabled {
		if err := initializeAerospikeOperator(manager); err != nil {
			log.Printf("Warning: Failed to initialize operators: %v", err)
		}
	}

	// Start cluster manager
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start cluster manager: %v", err)
	}

	// Start web server if enabled
	if webAddr != "" {
		handler, err := web.NewHandler(manager)
		if err != nil {
			log.Fatalf("Failed to create web handler: %v", err)
		}

		go func() {
			log.Printf("Starting web UI at http://%s", webAddr)
			if err := http.ListenAndServe(webAddr, handler); err != nil {
				log.Printf("Web server error: %v", err)
			}
		}()
	}

	// Wait for shutdown signal
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	log.Println("Shutting down...")
	manager.Stop()
}

// hardcoding the initialization of the Aerospike operator for now
// this would be done dynamically in a real system
func initializeAerospikeOperator(manager *cluster.Manager) error {
	aeroOp := aerospike_operator.New()
	err := aeroOp.Init(map[string]interface{}{
		"config_path": "/etc/aerospike/aerospike.conf",
	})
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	if err := manager.RegisterOperator(aeroOp); err != nil {
		return err
	}
	log.Printf("Registered Aerospike operator")

	params := map[string]interface{}{
		"operation": "add_namespace",
		"namespace": map[string]interface{}{
			"name": "checkout",
		},
	}

	ctx := context.Background()
	// manually calling execute for now, ideally this will be called externally to the cluster manager
	if err := aeroOp.Execute(ctx, params); err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}

	// After operations
	if err := aeroOp.Cleanup(); err != nil {
		log.Printf("Cleanup failed: %v", err)
	}

	return nil
}
