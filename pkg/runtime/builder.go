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
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/parser"
	"github.com/mensylisir/kubexm/pkg/runner"
	// engine "github.com/mensylisir/kubexm/pkg/engine" // Keep for engine.Engine type if Context needs it, but not for NewExecutor
	// Actually, Context will need engine.Engine type, so runtime package scope will import engine.
	// The builder itself just won't call NewExecutor().
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
// It now requires an engine.Engine instance to be passed in.
func (b *RuntimeBuilder) BuildFromFile(ctx context.Context, eng engine.Engine) (*Context, func(), error) {
	initialLog := logger.Get() // Use a general logger for file parsing
	initialLog.Info("Building runtime environment from file...", "configFile", b.configFile)

	clusterConfig, err := parser.ParseFromFile(b.configFile)
	if err != nil {
		initialLog.Error(err, "Failed to parse cluster configuration")
		return nil, nil, fmt.Errorf("failed to parse cluster config '%s': %w", b.configFile, err)
	}
	return b.BuildFromConfig(ctx, clusterConfig, eng, initialLog)
}

// BuildFromConfig constructs and initializes the full runtime Context from a parsed Cluster object.
// It requires an engine.Engine instance. If baseLogger is nil, a new logger will be initialized.
func (b *RuntimeBuilder) BuildFromConfig(ctx context.Context, clusterConfig *v1alpha1.Cluster, eng engine.Engine, baseLogger *logger.Logger) (*Context, func(), error) {
	var log *logger.Logger
	if baseLogger != nil {
		log = baseLogger
	} else {
		log = logger.Get()
	}
	log.Info("Building runtime environment from configuration object...")

	if eng == nil {
		return nil, nil, fmt.Errorf("engine instance cannot be nil when building runtime context")
	}

	runnerSvc := runner.New()
	// engineSvc := engine.NewExecutor() // No longer instantiated here
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
		Engine:        eng, // Use the passed-in engine instance
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

	// Determine and create the GlobalWorkDir (e.g., $(pwd)/.kubexm)
	// This is the root for all local operations and artifacts before distribution.
	if runtimeCtx.GlobalWorkDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Error(err, "Failed to get current working directory for GlobalWorkDir generation")
			cleanupFunc()
			return nil, nil, fmt.Errorf("failed to get current working directory: %w", err)
		}
		runtimeCtx.GlobalWorkDir = filepath.Join(cwd, common.KUBEXM) // Default: $(pwd)/.kubexm
		log.Info("Global.WorkDir not specified, using default.", "path", runtimeCtx.GlobalWorkDir)
	}
	log.Info("Ensuring GlobalWorkDir exists.", "path", runtimeCtx.GlobalWorkDir)
	if err := util.CreateDir(runtimeCtx.GlobalWorkDir); err != nil {
		log.Error(err, "Failed to create GlobalWorkDir", "path", runtimeCtx.GlobalWorkDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create global work directory '%s': %w", runtimeCtx.GlobalWorkDir, err)
	}

	// Create HostWorkDir for each host: $(pwd)/.kubexm/${hostname}
	// This is for program's local, host-specific (but not cluster-artifact-specific) files.
	for _, hr := range hostRuntimes {
		// This includes the control-node, which will have a directory like .kubexm/kubexm-control-node
		hostSpecificWorkDir := filepath.Join(runtimeCtx.GlobalWorkDir, hr.Host.GetName())
		log.Info("Ensuring host-specific work directory exists (local to program execution).", "host", hr.Host.GetName(), "path", hostSpecificWorkDir)
		if err := util.CreateDir(hostSpecificWorkDir); err != nil {
			log.Error(err, "Failed to create host-specific work directory", "host", hr.Host.GetName(), "path", hostSpecificWorkDir)
			cleanupFunc()
			return nil, nil, fmt.Errorf("failed to create host-specific work directory '%s' for host '%s': %w", hostSpecificWorkDir, hr.Host.GetName(), err)
		}
	}

	// Cluster specific base directory for artifacts: ${GlobalWorkDir}/${cluster_name}
	// e.g., $(pwd)/.kubexm/mycluster
	clusterArtifactsBaseDir := filepath.Join(runtimeCtx.GlobalWorkDir, clusterConfig.Name)
	runtimeCtx.ClusterArtifactsDir = clusterArtifactsBaseDir // Store this in context
	log.Info("Ensuring cluster-specific artifacts base directory exists.", "path", clusterArtifactsBaseDir)
	if err := util.CreateDir(clusterArtifactsBaseDir); err != nil {
		log.Error(err, "Failed to create cluster-specific artifacts base directory", "path", clusterArtifactsBaseDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create cluster-specific artifacts base directory '%s': %w", clusterArtifactsBaseDir, err)
	}

	// Logs directory: ${GlobalWorkDir}/${cluster_name}/logs
	logDir := filepath.Join(clusterArtifactsBaseDir, common.DefaultLogsDir)
	log.Info("Ensuring log directory exists.", "path", logDir)
	if err := util.CreateDir(logDir); err != nil {
		log.Error(err, "Failed to create log directory", "path", logDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create log directory '%s': %w", logDir, err)
	}

	// Base directories for different types of artifacts within the cluster's artifact directory
	// Certs base: ${GlobalWorkDir}/${cluster_name}/certs
	certsBaseDir := filepath.Join(clusterArtifactsBaseDir, common.DefaultCertsDir)
	log.Info("Ensuring certificates base directory exists.", "path", certsBaseDir)
	if err := util.CreateDir(certsBaseDir); err != nil {
		log.Error(err, "Failed to create certificates base directory", "path", certsBaseDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create certificates base directory '%s': %w", certsBaseDir, err)
	}
	// ETCD certs directory: ${GlobalWorkDir}/${cluster_name}/certs/etcd
	etcdCertsDir := filepath.Join(certsBaseDir, "etcd")
	log.Info("Ensuring ETCD certificates directory exists.", "path", etcdCertsDir)
	if err := util.CreateDir(etcdCertsDir); err != nil {
		log.Error(err, "Failed to create ETCD certificates directory", "path", etcdCertsDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create ETCD certificates directory '%s': %w", etcdCertsDir, err)
	}

	// ETCD binaries base directory: ${GlobalWorkDir}/${cluster_name}/etcd
	// Specific versions/arch will be subdirectories: etcd/${etcd_version}/${arch}/
	etcdArtifactsCompDir := filepath.Join(clusterArtifactsBaseDir, "etcd")
	log.Info("Ensuring ETCD component artifacts base directory exists.", "path", etcdArtifactsCompDir)
	if err := util.CreateDir(etcdArtifactsCompDir); err != nil {
		log.Error(err, "Failed to create ETCD component artifacts base directory", "path", etcdArtifactsCompDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create ETCD component artifacts base directory '%s': %w", etcdArtifactsCompDir, err)
	}

	// Container Runtime binaries base directory: ${GlobalWorkDir}/${cluster_name}/container_runtime
	// Specific name/versions/arch will be subdirectories
	containerRuntimeArtifactsCompDir := filepath.Join(clusterArtifactsBaseDir, common.DefaultContainerRuntimeDir)
	log.Info("Ensuring Container Runtime component artifacts base directory exists.", "path", containerRuntimeArtifactsCompDir)
	if err := util.CreateDir(containerRuntimeArtifactsCompDir); err != nil {
		log.Error(err, "Failed to create Container Runtime component artifacts base directory", "path", containerRuntimeArtifactsCompDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create Container Runtime component artifacts base directory '%s': %w", containerRuntimeArtifactsCompDir, err)
	}

	// Kubernetes binaries base directory: ${GlobalWorkDir}/${cluster_name}/kubernetes
	// Specific versions/arch will be subdirectories
	kubernetesArtifactsCompDir := filepath.Join(clusterArtifactsBaseDir, common.DefaultKubernetesDir)
	log.Info("Ensuring Kubernetes component artifacts base directory exists.", "path", kubernetesArtifactsCompDir)
	if err := util.CreateDir(kubernetesArtifactsCompDir); err != nil {
		log.Error(err, "Failed to create Kubernetes component artifacts base directory", "path", kubernetesArtifactsCompDir)
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed to create Kubernetes component artifacts base directory '%s': %w", kubernetesArtifactsCompDir, err)
	}

	if runtimeCtx.GlobalConnectionTimeout <= 0 {
		runtimeCtx.GlobalConnectionTimeout = 30 * time.Second
	}

	runtimeCtx.PipelineCache = cache.NewPipelineCache()
	runtimeCtx.ModuleCache = cache.NewModuleCache()
	runtimeCtx.TaskCache = cache.NewTaskCache()
	runtimeCtx.StepCache = cache.NewStepCache()

	// Set the ControlNode in the context
	if cnHR, ok := hostRuntimes[common.ControlNodeHostName]; ok {
		runtimeCtx.ControlNode = cnHR.Host
	} else {
		// This should ideally not happen if the control node goroutine succeeded
		err := fmt.Errorf("control node HostRuntime not found after initialization")
		log.Error(err, "Failed to set ControlNode in runtime context")
		cleanupFunc()
		return nil, nil, err
	}

	log.Info("Runtime environment built successfully.")
	return runtimeCtx, cleanupFunc, nil
}
