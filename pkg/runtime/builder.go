package runtime

import (
	"context"
	"fmt"
	"strings" // Added for strings.ToLower
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger" // Assuming your logger package
	"github.com/mensylisir/kubexm/pkg/parser"  // Assuming your parser package for config
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // Corrected path
)

// RuntimeBuilder builds a fully initialized runtime Context.
type RuntimeBuilder struct {
	configFile string
	// Potentially add other builder options here, like overriding default logger, runner, engine, etc.
}

// NewRuntimeBuilder creates a new RuntimeBuilder.
func NewRuntimeBuilder(configFile string) *RuntimeBuilder {
	return &RuntimeBuilder{configFile: configFile}
}

// Build constructs and initializes the full runtime Context.
// It parses configuration, sets up connections to hosts, gathers initial facts,
// and prepares all necessary services.
func (b *RuntimeBuilder) Build(ctx context.Context) (*Context, func(), error) {
	// Initialize logger (assuming a simple New function for now)
	log := logger.New() // Replace with your actual logger initialization
	log.Info("Building runtime environment...", "configFile", b.configFile)

	// Parse cluster configuration file
	// The issue mentions `parser.ParseFromFile`. This needs to exist.
	clusterConfig, err := parser.ParseFromFile(b.configFile)
	if err != nil {
		log.Error(err, "Failed to parse cluster configuration")
		return nil, nil, fmt.Errorf("failed to parse cluster config '%s': %w", b.configFile, err)
	}
	// Apply defaults and validation to the cluster config if not done by parser
	// v1alpha1.SetDefaults_Cluster(clusterConfig) // Assuming this is how defaults are set
	// if err := v1alpha1.Validate_Cluster(clusterConfig); err != nil {
	//    log.Error(err, "Cluster configuration validation failed")
	//	  return nil, nil, fmt.Errorf("cluster configuration validation failed: %w", err)
	// }


	// Initialize services
	runnerSvc := runner.New()
	engineSvc := engine.NewExecutor() // Using NewExecutor from engine package

	// Initialize connection pool
	// Using DefaultPoolConfig, replace if you have custom config
	poolConfig := connector.DefaultPoolConfig()
	pool := connector.NewConnectionPool(poolConfig)

	// Cleanup function to be called by the consumer
	cleanupFunc := func() {
		log.Info("Shutting down connection pool...")
		pool.Shutdown()
		// Add any other cleanup tasks here (e.g., closing log files if any)
	}

	hostRuntimes := make(map[string]*HostRuntime)
	var mu sync.Mutex // To protect concurrent writes to hostRuntimes map

	// Use an errgroup for concurrent initialization of hosts
	g, gCtx := errgroup.WithContext(ctx)
	// Limit concurrency if needed: g.SetLimit(10)

	if clusterConfig.Spec.Hosts == nil || len(clusterConfig.Spec.Hosts) == 0 {
		log.Warn("No hosts defined in the cluster configuration.")
		// Decide if this is an error or if an empty HostRuntimes is acceptable
	}

	for _, hostCfg := range clusterConfig.Spec.Hosts {
		currentHostCfg := hostCfg // Capture range variable for goroutine
		g.Go(func() error {
			log.Info("Initializing runtime for host...", "host", currentHostCfg.Name)

			var conn connector.Connector
			// Determine connector type (e.g., local or SSH)
			// Assuming HostSpec has a field like 'Type' or connection details imply it.
			// The issue's HostSpec example has `Type string`. Let's use it.
			// Also, `Address` field can indicate local.
			if strings.ToLower(currentHostCfg.Type) == "local" || currentHostCfg.Address == "localhost" || currentHostCfg.Address == "127.0.0.1" {
				conn = &connector.LocalConnector{} // No pool needed for local
				log.Info("Using LocalConnector for host", "host", currentHostCfg.Name)
			} else {
				// Pass the pool to NewSSHConnector if it uses it, or pass config directly.
				// The original snippet had NewSSHConnector(pool).
				conn = connector.NewSSHConnector(pool)
				log.Info("Using SSHConnector for host", "host", currentHostCfg.Name)
			}

			// Connect to the host
			// The connector.Connect method needs the HostSpec.
			// We need to ensure currentHostCfg (v1alpha1.HostSpec) can be used by conn.Connect.
			// This might mean connector.Connect takes an interface that HostSpec implements,
			// or a specific type. Let's assume it takes the HostSpec directly or compatible.
			// For now, we need to convert v1alpha1.HostSpec to connector.ConnectionCfg
			connectionCfg := connector.ConnectionCfg{
				Host:           currentHostCfg.Address,
				Port:           currentHostCfg.Port,
				User:           currentHostCfg.User,
				Password:       currentHostCfg.Password,
				PrivateKeyPath: currentHostCfg.PrivateKeyPath,
				// PrivateKey: currentHostCfg.PrivateKey, // This needs decoding if it's base64
				Timeout: clusterConfig.Spec.Global.ConnectionTimeout, // Assuming global timeout applies
			}
			// Handle PrivateKey (base64 string in v1alpha1.HostSpec)
			// This logic should ideally be part of connector.Connect or a helper
			// For now, let's assume connector.Connect handles a ConnectionCfg that might have PrivateKey bytes
			// or the SSHConnector itself reads from PrivateKeyPath if PrivateKey bytes are not set.
			// The NewRuntime in the old model did this decoding and path reading.
			// For this builder, we'll assume the connector.Connect will handle it based on ConnectionCfg.

			if err := conn.Connect(gCtx, connectionCfg); err != nil { // Pass connectionCfg
				log.Error(err, "Failed to connect to host", "host", currentHostCfg.Name)
				return fmt.Errorf("failed to connect to host %s: %w", currentHostCfg.Name, err)
			}
			log.Info("Successfully connected to host.", "host", currentHostCfg.Name)

			// Gather facts for the host
			log.Info("Gathering facts for host...", "host", currentHostCfg.Name)
			facts, err := runnerSvc.GatherFacts(gCtx, conn)
			if err != nil {
				conn.Close() // Attempt to close connection if fact gathering fails
				log.Error(err, "Failed to gather facts for host", "host", currentHostCfg.Name)
				return fmt.Errorf("failed to gather facts for host %s: %w", currentHostCfg.Name, err)
			}
			log.Info("Successfully gathered facts for host.", "host", currentHostCfg.Name, "OS", facts.OS.PrettyName)

			// Create Host object for connector.Host interface
			// This assumes HostFromSpec can take v1alpha1.HostSpec
			host := connector.NewHostFromSpec(currentHostCfg) // NewHostFromSpec needs to be defined in connector pkg


			hr := &HostRuntime{
				Host:  host, // This is now a connector.Host
				Conn:  conn,
				Facts: facts,
			}

			mu.Lock()
			hostRuntimes[hr.Host.GetName()] = hr // Assuming Host.GetName() provides the unique key
			mu.Unlock()

			log.Info("Runtime initialized for host.", "host", currentHostCfg.Name)
			return nil
		})
	}

	// Wait for all host initializations to complete
	if err := g.Wait(); err != nil {
		log.Error(err, "Failed during concurrent host initialization")
		cleanupFunc() // Call cleanup if build fails
		return nil, nil, fmt.Errorf("failed during concurrent host initialization: %w", err)
	}

	log.Info("All hosts initialized successfully.")

	// Construct the main runtime Context
	runtimeCtx := &Context{
		GoCtx:         ctx,
		Logger:        log,
		Engine:        engineSvc,
		Runner:        runnerSvc,
		ClusterConfig: clusterConfig,
		HostRuntimes:  hostRuntimes,
		// Initialize caches if they are part of the Context struct and used
		// pipelineCache: cache.New(), // Example
	}

	log.Info("Runtime environment built successfully.")
	return runtimeCtx, cleanupFunc, nil
}
