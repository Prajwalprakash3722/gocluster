package main

import (
	"agent/internal/cluster"
	"agent/internal/config"
	"agent/internal/operator"
	"agent/internal/types"
	"agent/internal/web"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/spf13/cobra"
)

// initializePlugins loads operators using the new plugin registry system
func initializePlugins(operatorManager *operator.OperatorManager, plugins []string) error {
	if len(plugins) == 0 {
		log.Println("No plugins specified")
		return nil
	}

	log.Printf("Loading %d plugins...", len(plugins))

	// Load all plugins using the global registry
	return operator.LoadPlugins(operatorManager, plugins)
}

func runServer(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	isDaemon, _ := cmd.Flags().GetBool("daemon")

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	// Override backend configuration with CLI flags if provided
	if backendType, _ := cmd.Flags().GetString("backend-type"); backendType != "" {
		cfg.Backend.Type = types.BackendType(backendType)
	}

	if backendNamespace, _ := cmd.Flags().GetString("backend-namespace"); backendNamespace != "" {
		cfg.Backend.Namespace = backendNamespace
	}

	if etcdEndpoints, _ := cmd.Flags().GetStringSlice("etcd-endpoints"); len(etcdEndpoints) > 0 {
		cfg.Backend.EtcdEndpoints = etcdEndpoints
	}

	if zkHosts, _ := cmd.Flags().GetStringSlice("zk-hosts"); len(zkHosts) > 0 {
		cfg.Backend.ZKHosts = zkHosts
	}

	fmt.Printf("Starting gocluster-manager with backend: %s\n", cfg.Backend.Type)
	if cfg.Backend.Namespace != "" {
		fmt.Printf("Backend namespace: %s\n", cfg.Backend.Namespace)
	}

	operatorManager := operator.NewOperatorManager()
	opts := cluster.ManagerOptions{
		ConfigPath:      configPath,
		BindAddress:     cfg.Cluster.BindAddress,
		BindPort:        cfg.Cluster.DiscoveryPort,
		OperatorManager: operatorManager,
	}

	manager, err := cluster.NewManager(opts)
	if err != nil {
		return fmt.Errorf("failed to create cluster manager: %v", err)
	}

	operatorManager.SetClusterOperations(manager)

	if cfg.Cluster.EnableOperators {
		if err := initializePlugins(operatorManager, cfg.Plugins); err != nil {
			return fmt.Errorf("failed to initialize plugins: %v", err)
		}
	}

	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start cluster manager: %v", err)
	}

	if cfg.Cluster.WebAddress != "" {
		handler, err := web.NewHandler(manager, operatorManager)
		if err != nil {
			return fmt.Errorf("failed to create web handler: %v", err)
		}

		app := fiber.New(fiber.Config{DisableStartupMessage: true})
		handler.SetupRoutes(app)

		go func() {
			log.Printf("Starting web UI at http://%s", cfg.Cluster.WebAddress)
			if err := app.Listen(cfg.Cluster.WebAddress); err != nil {
				log.Printf("Web server error: %v", err)
			}
		}()
	}

	if isDaemon {
		if err := daemon(); err != nil {
			return fmt.Errorf("failed to daemonize: %v", err)
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

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
	rootCmd := &cobra.Command{
		Use:   "gocluster-manager",
		Short: "Cluster manager with operator support",
		RunE:  runServer,
	}

	rootCmd.PersistentFlags().StringP("config", "c", "cluster.conf", "Path to configuration file")
	rootCmd.PersistentFlags().BoolP("daemon", "d", false, "Run in daemon mode")

	// Backend configuration flags
	rootCmd.PersistentFlags().String("backend-type", "", "Backend type (memory, etcd, zookeeper) - overrides config file")
	rootCmd.PersistentFlags().String("backend-namespace", "", "Backend namespace - overrides config file")
	rootCmd.PersistentFlags().StringSlice("etcd-endpoints", nil, "Etcd endpoints (e.g., localhost:2379) - overrides config file")
	rootCmd.PersistentFlags().StringSlice("zk-hosts", nil, "ZooKeeper hosts (e.g., localhost:2181) - overrides config file")

	subCmds := []*cobra.Command{
		{
			Use:   "server",
			Short: "Start the cluster server",
			RunE:  runServer,
		},
	}

	for _, subCmd := range subCmds {
		subCmd.PersistentFlags().StringP("config", "c", "cluster.conf", "Path to configuration file")
		subCmd.PersistentFlags().BoolP("daemon", "d", false, "Run in daemon mode")
		rootCmd.AddCommand(subCmd)
	}

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		log.Fatalf("Failed to execute: %v", err)
	}
}
