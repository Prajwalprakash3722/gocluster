package main

import (
	"agent/internal/cluster"
	"agent/internal/config"
	"agent/internal/operator"
	aerospike_operator "agent/internal/operator/plugins/aerospike"
	"agent/internal/web"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/spf13/cobra"
)

func initializePlugins(operatorManager *operator.OperatorManager, plugins []string) error {
	log.Printf("Initializing plugins: %v", plugins)

	if len(plugins) == 0 {
		log.Printf("No plugins configured")
		return nil
	}

	for _, plugin := range plugins {
		log.Printf("Attempting to initialize plugin: %s", plugin)

		switch strings.TrimSpace(strings.ToLower(plugin)) {
		case "aerospike-config":
			log.Printf("Initializing Aerospike operator plugin")
			if err := initializeAerospikeOperator(operatorManager); err != nil {
				log.Printf("Failed to initialize aerospike operator: %v", err)
				return fmt.Errorf("failed to initialize aerospike operator: %v", err)
			}
			log.Printf("Successfully initialized Aerospike operator plugin")
		default:
			log.Printf("Warning: Unknown plugin %s", plugin)
		}
	}
	return nil
}

func initializeAerospikeOperator(operatorManager *operator.OperatorManager) error {
	aeroOp := aerospike_operator.New()
	err := aeroOp.Init(map[string]interface{}{
		"config_path": "/etc/aerospike/aerospike.conf",
	})
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	if err := operatorManager.RegisterOperator(aeroOp); err != nil {
		return err
	}
	log.Printf("Registered Aerospike operator")

	return nil
}

func runServer(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	isDaemon, _ := cmd.Flags().GetBool("daemon")

	log.Printf("Loading configuration from: %s", configPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	log.Printf("Loaded configuration: cluster=%s, operators=%v", cfg.Cluster.Name, cfg.Cluster.EnableOperators)

	opts := cluster.ManagerOptions{
		ConfigPath:  configPath,
		BindAddress: cfg.Cluster.BindAddress,
		BindPort:    cfg.Cluster.DiscoveryPort,
	}

	log.Printf("Creating cluster manager with options: %+v", opts)
	manager, err := cluster.NewManager(opts)
	log.Printf("Creating operator manager")
	operatorManager := operator.NewOperatorManager()
	if err != nil {
		return fmt.Errorf("failed to create cluster manager: %v", err)
	}

	if cfg.Cluster.EnableOperators {
		log.Printf("Operators enabled, initializing plugins")
		if err := initializePlugins(operatorManager, cfg.Plugins); err != nil {
			log.Printf("Warning: Failed to initialize plugins: %v", err)
			return fmt.Errorf("failed to initialize plugins: %v", err)
		}
	} else {
		log.Printf("Operators disabled, skipping plugin initialization")
	}

	log.Printf("Starting cluster manager")
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start cluster manager: %v", err)
	}

	if cfg.Cluster.WebAddress != "" {
		log.Printf("Initializing web handler")
		handler, err := web.NewHandler(manager, operatorManager)
		if err != nil {
			return fmt.Errorf("failed to create web handler: %v", err)
		}

		app := fiber.New(fiber.Config{
			DisableStartupMessage: true,
		})
		handler.SetupRoutes(app)

		go func() {
			log.Printf("Starting web UI at http://%s", cfg.Cluster.WebAddress)
			if err := app.Listen(cfg.Cluster.WebAddress); err != nil {
				log.Printf("Web server error: %v", err)
			}
		}()
	}

	if isDaemon {
		log.Printf("Daemonizing process")
		if err := daemon(); err != nil {
			return fmt.Errorf("failed to daemonize: %v", err)
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("Waiting for shutdown signal")
	<-signals

	log.Println("Shutting down...")
	manager.Stop()
	return nil
}

func daemon() error {
	if os.Getppid() == 1 {
		return nil
	}

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--daemon" || args[i] == "-d" {
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.Start()
	os.Exit(0)
	return nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "agent",
		Short: "Cluster agent with plugin support",
		RunE:  runServer,
	}

	rootCmd.PersistentFlags().StringP("config", "c", "cluster.conf", "Path to configuration file")
	rootCmd.PersistentFlags().BoolP("daemon", "d", false, "Run in daemon mode")

	ctx := context.Background()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}
}
