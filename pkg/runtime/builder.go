package runtime

import (
	"context"
	"encoding/base64" // Added for PrivateKey decoding
	"fmt"
	"os" // Added for os.ReadFile
	"strings"
	"sync"
	"time" // Added for time.Duration and default timeout

	"golang.org/x/sync/errgroup"

	"github.com/mensylisir/kubexm/pkg/cache" // Added for cache initialization
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/parser"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common" // Added for constants like KUBEXM
	"github.com/mensylisir/kubexm/pkg/util"   // Added for CreateDir
	"path/filepath"                           // Added for filepath operations
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

// BuildFromFile constructs and initializes the full runtime Context from a configuration file.
func (b *RuntimeBuilder) BuildFromFile(ctx context.Context) (*Context, func(), error) {
	initialLog := logger.Get() // Use a general logger for file parsing
	initialLog.Info("Building runtime environment from file...", "configFile", b.configFile)

	clusterConfig, err := parser.ParseFromFile(b.configFile)
	if err != nil {
		initialLog.Error(err, "Failed to parse cluster configuration")
		return nil, nil, fmt.Errorf("failed to parse cluster config '%s': %w", b.configFile, err)
	}

	// Pass nil for baseLogger, so BuildFromConfig uses logger.Get() internally or a logger derived from it.
	// Or, we could pass initialLog here if we want the file parsing log to be the base.
	// For now, let BuildFromConfig decide its logger if nil is passed.
	return b.BuildFromConfig(ctx, clusterConfig, initialLog)
}

// BuildFromConfig constructs and initializes the full runtime Context from a parsed Cluster object.
// If baseLogger is nil, a new logger will be initialized.
func (b *RuntimeBuilder) BuildFromConfig(ctx context.Context, clusterConfig *v1alpha1.Cluster, baseLogger *logger.Logger) (*Context, func(), error) {
	var log *logger.Logger
	if baseLogger != nil {
		log = baseLogger
	} else {
		log = logger.Get()
	}
	log.Info("Building runtime environment from configuration object...")

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

	// --- Initialize Control Node (Local Host) ---
	g.Go(func() error {
		controlNodeHostSpec := v1alpha1.Host{
			Name:    common.ControlNodeHostName, // e.g., "kubexm-control-node"
			Type:    "local",
			Address: "127.0.0.1", // Or other loopback
			Roles:   []string{common.ControlNodeRole}, // e.g., "control-node"
			// Other fields like User, Port, PrivateKeyPath are not relevant for local.
		}
		hostLogger := log.With("host", controlNodeHostSpec.Name)
		hostLogger.Info("Initializing runtime for local control-node...")

		conn := &connector.LocalConnector{}
		hostLogger.Info("Using LocalConnector for control-node.")

		// Connect (no-op for local but good for consistency)
		if err := conn.Connect(gCtx, connector.ConnectionCfg{Host: controlNodeHostSpec.Address}); err != nil {
			// Should not happen for local connector, but handle defensively
			hostLogger.Error(err, "Failed to 'connect' local control-node connector")
			return fmt.Errorf("failed to connect local control-node connector: %w", err)
		}
		hostLogger.Info("Successfully 'connected' to local control-node.")

		hostLogger.Info("Gathering facts for local control-node...")
		facts, err := runnerSvc.GatherFacts(gCtx, conn)
		if err != nil {
			conn.Close() // Should be no-op for local
			hostLogger.Error(err, "Failed to gather facts for local control-node")
			return fmt.Errorf("failed to gather facts for local control-node %s: %w", controlNodeHostSpec.Name, err)
		}
		hostLogger.Info("Successfully gathered facts for local control-node.", "OS", facts.OS.PrettyName)

		host := connector.NewHostFromSpec(controlNodeHostSpec)
		hr := &HostRuntime{
			Host:  host,
			Conn:  conn,
			Facts: facts,
		}
		mu.Lock()
		hostRuntimes[hr.Host.GetName()] = hr
		mu.Unlock()
		hostLogger.Info("Runtime initialized for local control-node.")
		return nil
	})

	// --- Initialize Defined Remote Hosts ---
	if clusterConfig.Spec.Hosts == nil || len(clusterConfig.Spec.Hosts) == 0 {
		log.Info("No remote hosts defined in the cluster configuration.") // Changed from Warn
	}

	for _, hostCfg := range clusterConfig.Spec.Hosts {
		currentHostCfg := hostCfg // Capture range variable for goroutine
		g.Go(func() error {
			hostLogger := log.With("host", currentHostCfg.Name)
			hostLogger.Info("Initializing runtime for host...")

			var conn connector.Connector
			if strings.ToLower(currentHostCfg.Type) == "local" || currentHostCfg.Address == "localhost" || currentHostCfg.Address == "127.0.0.1" {
				conn = &connector.LocalConnector{}
				hostLogger.Info("Using LocalConnector")
			} else {
				conn = connector.NewSSHConnector(pool)
				hostLogger.Info("Using SSHConnector")
			}

			connectionCfg := connector.ConnectionCfg{
				Host:           currentHostCfg.Address,
				Port:           currentHostCfg.Port,
				User:           currentHostCfg.User,
				Password:       currentHostCfg.Password,
				PrivateKeyPath: currentHostCfg.PrivateKeyPath,
			}

			var hostPrivateKeyBytes []byte
			if currentHostCfg.PrivateKey != "" {
				decodedKey, decodeErr := base64.StdEncoding.DecodeString(currentHostCfg.PrivateKey)
				if decodeErr != nil {
					err := fmt.Errorf("failed to decode base64 private key for host %s: %w", currentHostCfg.Name, decodeErr)
					hostLogger.Error(err, "PrivateKey decoding failed")
					return err
				}
				hostPrivateKeyBytes = decodedKey
				connectionCfg.PrivateKeyPath = ""
				hostLogger.Debug("Using provided base64 private key content.")
			} else if currentHostCfg.PrivateKeyPath != "" {
				keyFileBytes, readErr := osReadFile(currentHostCfg.PrivateKeyPath)
				if readErr != nil {
					err := fmt.Errorf("failed to read private key file '%s' for host %s: %w", currentHostCfg.PrivateKeyPath, currentHostCfg.Name, readErr)
					hostLogger.Error(err, "PrivateKey file reading failed")
					return err
				}
				hostPrivateKeyBytes = keyFileBytes
				hostLogger.Debug("Loaded private key from path.", "path", currentHostCfg.PrivateKeyPath)
			}
			connectionCfg.PrivateKey = hostPrivateKeyBytes

			if currentHostCfg.ConnectionTimeout > 0 {
				connectionCfg.Timeout = currentHostCfg.ConnectionTimeout
			} else if clusterConfig.Spec.Global != nil && clusterConfig.Spec.Global.ConnectionTimeout > 0 {
				connectionCfg.Timeout = clusterConfig.Spec.Global.ConnectionTimeout
			} else {
				connectionCfg.Timeout = connector.DefaultConnectTimeout
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
		Logger:        log, // Use the logger determined at the start of this function
		Engine:        engineSvc,
		Runner:        runnerSvc,
		ClusterConfig: clusterConfig,
		HostRuntimes:  hostRuntimes,
		ConnectionPool: pool,
	}

	if clusterConfig.Spec.Global != nil {
		runtimeCtx.GlobalWorkDir = clusterConfig.Spec.Global.WorkDir
		runtimeCtx.GlobalVerbose = clusterConfig.Spec.Global.Verbose
		runtimeCtx.GlobalIgnoreErr = clusterConfig.Spec.Global.IgnoreErr
		runtimeCtx.GlobalConnectionTimeout = clusterConfig.Spec.Global.ConnectionTimeout
	}

	if runtimeCtx.GlobalWorkDir == "" {
		// Generate default GlobalWorkDir based on current working directory
		cwd, err := os.Getwd()
		if err != nil {
			log.Error(err, "Failed to get current working directory for GlobalWorkDir generation")
			cleanupFunc()
			return nil, nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		defaultWorkDir := filepath.Join(cwd, common.KUBEXM) // Base is .kubexm
		log.Info("Global.WorkDir not set, using default base.", "path", defaultWorkDir)
		runtimeCtx.GlobalWorkDir = defaultWorkDir
	}

	// Ensure GlobalWorkDir (e.g., $(pwd)/.kubexm) exists
	log.Info("Ensuring GlobalWorkDir (base for all operations) exists.", "path", runtimeCtx.GlobalWorkDir)
	if err := util.CreateDir(runtimeCtx.GlobalWorkDir); err != nil {
		log.Error(err, "Failed to create GlobalWorkDir", "path", runtimeCtx.GlobalWorkDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create global work directory '%s': %w", runtimeCtx.GlobalWorkDir, err)
	}

	// Cluster specific base directory: $(pwd)/.kubexm/${cluster_name}
	clusterBaseDir := filepath.Join(runtimeCtx.GlobalWorkDir, clusterConfig.Name)
	log.Info("Ensuring cluster-specific base directory exists.", "path", clusterBaseDir)
	if err := util.CreateDir(clusterBaseDir); err != nil {
		log.Error(err, "Failed to create cluster-specific base directory", "path", clusterBaseDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create cluster-specific base directory '%s': %w", clusterBaseDir, err)
	}

	// Logs directory: $(pwd)/.kubexm/${cluster_name}/logs
	logDir := filepath.Join(clusterBaseDir, common.DefaultLogsDir)
	log.Info("Ensuring log directory exists.", "path", logDir)
	if err := util.CreateDir(logDir); err != nil {
		log.Error(err, "Failed to create log directory", "path", logDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create log directory '%s': %w", logDir, err)
	}

	// ETCD certs directory: $(pwd)/.kubexm/${cluster_name}/certs/etcd/
	etcdCertsDir := filepath.Join(clusterBaseDir, "certs", "etcd")
	log.Info("Ensuring ETCD certificates directory exists.", "path", etcdCertsDir)
	if err := util.CreateDir(etcdCertsDir); err != nil {
		log.Error(err, "Failed to create ETCD certificates directory", "path", etcdCertsDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create ETCD certificates directory '%s': %w", etcdCertsDir, err)
	}

	// Host-specific work directories: $(pwd)/.kubexm/${hostname}
	// These are general per-host directories, not necessarily tied to a cluster's artifacts.
	// This seems to be a separate requirement from the artifact paths.
	for _, hr := range hostRuntimes {
		// It's possible the control-node itself might not need a dedicated directory under .kubexm/${hostname}
		// if all its "work" is within the cluster-specific paths.
		// For now, creating for all, including control-node.
		hostSpecificBaseDir := filepath.Join(runtimeCtx.GlobalWorkDir, hr.Host.GetName())
		log.Info("Ensuring host-specific work directory exists.", "host", hr.Host.GetName(), "path", hostSpecificBaseDir)
		if err := util.CreateDir(hostSpecificBaseDir); err != nil {
			log.Error(err, "Failed to create host-specific work directory", "host", hr.Host.GetName(), "path", hostSpecificBaseDir)
			cleanupFunc()
			return nil, nil, fmt.Errorf("failed to create host-specific work directory '%s' for host '%s': %w", hostSpecificBaseDir, hr.Host.GetName(), err)
		}
	}

	// Artifact directories (per cluster, per component, per version, per arch)
	// These will be created on demand by resource handles or steps, but the base paths can be prepared or logged.
	// Example: ETCD binaries path structure
	// $(pwd)/.kubexm/${cluster_name}/etcd/${etcd_version}/${arch}/
	// This level of granularity (version, arch) is usually handled when the actual artifact is processed.
	// The builder can ensure the component base exists: $(pwd)/.kubexm/${cluster_name}/etcd/
	etcdArtifactsBaseDir := filepath.Join(clusterBaseDir, "etcd")
	log.Info("Ensuring ETCD artifacts base directory exists.", "path", etcdArtifactsBaseDir)
	if err := util.CreateDir(etcdArtifactsBaseDir); err != nil {
		// Error handling
	}

	containerRuntimeArtifactsBaseDir := filepath.Join(clusterBaseDir, "container_runtime")
	log.Info("Ensuring Container Runtime artifacts base directory exists.", "path", containerRuntimeArtifactsBaseDir)
	if err := util.CreateDir(containerRuntimeArtifactsBaseDir); err != nil {
		// Error handling
	}

	kubernetesArtifactsBaseDir := filepath.Join(clusterBaseDir, "kubernetes")
	log.Info("Ensuring Kubernetes artifacts base directory exists.", "path", kubernetesArtifactsBaseDir)
	if err := util.CreateDir(kubernetesArtifactsBaseDir); err != nil {
		// Error handling
	}


	if runtimeCtx.GlobalConnectionTimeout <= 0 {
		runtimeCtx.GlobalConnectionTimeout = 30 * time.Second
	}

	runtimeCtx.PipelineCache = cache.NewPipelineCache()
	runtimeCtx.ModuleCache = cache.NewModuleCache()
	runtimeCtx.TaskCache = cache.NewTaskCache()
	runtimeCtx.StepCache = cache.NewStepCache()

	log.Info("Runtime environment built successfully.")
	return runtimeCtx, cleanupFunc, nil
}
