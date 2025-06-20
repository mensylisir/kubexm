package runtime

import (
	"context"
	"encoding/base64" // Added for PrivateKey decoding
	"fmt"
	"os" // Added for os.ReadFile
	"strings"
	"sync"
	// "time" // Not directly used by builder logic itself, but by ConnectionTimeout

	"golang.org/x/sync/errgroup"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/parser"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// osReadFile is a variable that defaults to os.ReadFile, allowing it to be mocked for tests.
// This was present in the old runtime.go and is useful here too.
var osReadFile = os.ReadFile


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
func (b *RuntimeBuilder) Build(ctx context.Context) (*Context, func(), error) {
	log := logger.Get()
	log.Info("Building runtime environment...", "configFile", b.configFile)

	clusterConfig, err := parser.ParseFromFile(b.configFile)
	if err != nil {
		log.Error(err, "Failed to parse cluster configuration")
		return nil, nil, fmt.Errorf("failed to parse cluster config '%s': %w", b.configFile, err)
	}

	// Assuming defaults and validation are handled by the parser or called here if needed.
	// v1alpha1.SetDefaults_Cluster(clusterConfig)
	// if err := v1alpha1.Validate_Cluster(clusterConfig); err != nil { ... }


	runnerSvc := runner.New()
	engineSvc := engine.NewExecutor()
	poolConfig := connector.DefaultPoolConfig()
	pool := connector.NewConnectionPool(poolConfig)

	cleanupFunc := func() {
		log.Info("Shutting down connection pool...")
		pool.Shutdown()
	}

	hostRuntimes := make(map[string]*HostRuntime)
	var mu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)

	if clusterConfig.Spec.Hosts == nil || len(clusterConfig.Spec.Hosts) == 0 {
		log.Warn("No hosts defined in the cluster configuration.")
	}

	for _, hostCfg := range clusterConfig.Spec.Hosts {
		currentHostCfg := hostCfg
		g.Go(func() error {
			hostLogger := log.With("host", currentHostCfg.Name) // Create a logger specific to this host for initialization
			hostLogger.Info("Initializing runtime for host...")

			var conn connector.Connector
			if strings.ToLower(currentHostCfg.Type) == "local" || currentHostCfg.Address == "localhost" || currentHostCfg.Address == "127.0.0.1" {
				conn = &connector.LocalConnector{}
				hostLogger.Info("Using LocalConnector")
			} else {
				conn = connector.NewSSHConnector(pool)
				hostLogger.Info("Using SSHConnector")
			}

			// Construct connector.ConnectionCfg
			connectionCfg := connector.ConnectionCfg{
				Host:           currentHostCfg.Address,
				Port:           currentHostCfg.Port,
				User:           currentHostCfg.User,
				Password:       currentHostCfg.Password,
				PrivateKeyPath: currentHostCfg.PrivateKeyPath,
				// Timeout will be set from global or host-specific config below
			}

			// Handle PrivateKey: decode if present, else read from path
			var hostPrivateKeyBytes []byte
			if currentHostCfg.PrivateKey != "" { // PrivateKey is base64 string in v1alpha1.HostSpec
				decodedKey, decodeErr := base64.StdEncoding.DecodeString(currentHostCfg.PrivateKey)
				if decodeErr != nil {
					err := fmt.Errorf("failed to decode base64 private key for host %s: %w", currentHostCfg.Name, decodeErr)
					hostLogger.Error(err, "PrivateKey decoding failed")
					return err
				}
				hostPrivateKeyBytes = decodedKey
				connectionCfg.PrivateKeyPath = "" // Clear path if key content is used directly
				hostLogger.Debug("Using provided base64 private key content.")
			} else if currentHostCfg.PrivateKeyPath != "" {
				keyFileBytes, readErr := osReadFile(currentHostCfg.PrivateKeyPath) // Use mockable osReadFile
				if readErr != nil {
					err := fmt.Errorf("failed to read private key file '%s' for host %s: %w", currentHostCfg.PrivateKeyPath, currentHostCfg.Name, readErr)
					hostLogger.Error(err, "PrivateKey file reading failed")
					return err
				}
				hostPrivateKeyBytes = keyFileBytes
				hostLogger.Debug("Loaded private key from path.", "path", currentHostCfg.PrivateKeyPath)
			}
			connectionCfg.PrivateKey = hostPrivateKeyBytes // Assign the []byte to ConnectionCfg

			// Set Timeout
			if currentHostCfg.ConnectionTimeout > 0 {
				connectionCfg.Timeout = currentHostCfg.ConnectionTimeout
			} else if clusterConfig.Spec.Global != nil && clusterConfig.Spec.Global.ConnectionTimeout > 0 {
				connectionCfg.Timeout = clusterConfig.Spec.Global.ConnectionTimeout
			} else {
				connectionCfg.Timeout = connector.DefaultConnectTimeout // Fallback to a default from connector package
			}


			if err := conn.Connect(gCtx, connectionCfg); err != nil {
				hostLogger.Error(err, "Failed to connect to host")
				return fmt.Errorf("failed to connect to host %s (%s): %w", currentHostCfg.Name, currentHostCfg.Address, err)
			}
			hostLogger.Info("Successfully connected to host.")

			hostLogger.Info("Gathering facts for host...")
			facts, err := runnerSvc.GatherFacts(gCtx, conn)
			if err != nil {
				conn.Close()
				hostLogger.Error(err, "Failed to gather facts for host")
				return fmt.Errorf("failed to gather facts for host %s: %w", currentHostCfg.Name, err)
			}
			hostLogger.Info("Successfully gathered facts for host.", "OS", facts.OS.PrettyName)

			host := connector.NewHostFromSpec(currentHostCfg)

			hr := &HostRuntime{
				Host:  host,
				Conn:  conn,
				Facts: facts,
			}

			mu.Lock()
			hostRuntimes[hr.Host.GetName()] = hr
			mu.Unlock()

			hostLogger.Info("Runtime initialized for host.")
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Error(err, "Failed during concurrent host initialization")
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed during concurrent host initialization: %w", err)
	}

	log.Info("All hosts initialized successfully.")

	runtimeCtx := &Context{
		GoCtx:         ctx,
		Logger:        log,
		Engine:        engineSvc,
		Runner:        runnerSvc,
		ClusterConfig: clusterConfig,
		HostRuntimes:  hostRuntimes,
	}

	log.Info("Runtime environment built successfully.")
	return runtimeCtx, cleanupFunc, nil
}
