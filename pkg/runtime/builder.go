package runtime

import (
	"context"
	"encoding/base64" // Added for PrivateKey decoding
	"fmt"
	"os" // Added for os.ReadFile
	"strings"
	"sync"
	"time" // Added for time.Duration and default timeout

	"golang.org/x/sync/errgroup" // For concurrent host initialization

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/config" // For config.ParseFromFile
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"path/filepath"
	// For EventRecorder initialization
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	// "k8s.io/client-go/kubernetes" // Would be needed for a real clientset
)

var osReadFileFS = os.ReadFile // Allow mocking for tests related to private key reading

// RuntimeBuilder constructs and initializes a runtime.Context.
type RuntimeBuilder struct {
	configFilepath   string
	clusterConfig    *v1alpha1.Cluster
	runnerSvc        runner.Runner
	connectionPool   *connector.ConnectionPool // Allow building from an already parsed config
	connectorFactory connector.Factory
}

// NewRuntimeBuilder creates a new builder for a given configuration file path.
func NewRuntimeBuilder(configFilepath string, runnerSvc runner.Runner, pool *connector.ConnectionPool, factory connector.Factory) *RuntimeBuilder {
	return &RuntimeBuilder{
		configFilepath:   configFilepath,
		runnerSvc:        runnerSvc,
		connectionPool:   pool,
		connectorFactory: factory,
	}
}

// NewRuntimeBuilderFromConfig creates a new builder from an already parsed Cluster configuration.
func NewRuntimeBuilderFromConfig(cfg *v1alpha1.Cluster, runnerSvc runner.Runner, pool *connector.ConnectionPool, factory connector.Factory) *RuntimeBuilder {
	return &RuntimeBuilder{
		clusterConfig:    cfg,
		runnerSvc:        runnerSvc,
		connectionPool:   pool,
		connectorFactory: factory,
	}
}

// Build constructs the full runtime Context.
// It initializes services, connects to hosts, gathers facts, and sets up working directories.
// The engineInst is the pre-initialized DAG execution engine.
func (b *RuntimeBuilder) Build(ctx context.Context, engineInst engine.Engine) (*Context, func(), error) {
	log := logger.Get() // Use a base logger for initial setup messages

	var currentClusterConfig *v1alpha1.Cluster
	var err error

	if b.clusterConfig != nil {
		log.Info("Building runtime from pre-loaded configuration object...")
		currentClusterConfig = b.clusterConfig
		// Apply defaults and validate again, in case the pre-loaded config wasn't processed.
		// This is important if Build is called with a raw config object.
		v1alpha1.SetDefaults_Cluster(currentClusterConfig)
		if err = v1alpha1.Validate_Cluster(currentClusterConfig); err != nil {
			log.Error(err, "Validation failed for pre-loaded cluster configuration")
			return nil, nil, fmt.Errorf("validation failed for pre-loaded cluster configuration: %w", err)
		}
	} else if b.configFilepath != "" {
		log.Info("Building runtime from configuration file...", "configFile", b.configFilepath)
		currentClusterConfig, err = config.ParseFromFile(b.configFilepath) // Uses new pkg/config
		if err != nil {
			log.Error(err, "Failed to parse cluster configuration file", "configFile", b.configFilepath)
			return nil, nil, fmt.Errorf("failed to parse cluster config '%s': %w", b.configFilepath, err)
		}
	} else {
		return nil, nil, fmt.Errorf("RuntimeBuilder: either configFilepath or clusterConfig must be provided")
	}

	if engineInst == nil {
		return nil, nil, fmt.Errorf("engine instance cannot be nil when building runtime context")
	}

	// Initialize core services
	//runnerSvc := runner.New() // Stateless runner service
	//connectionPool := connector.NewConnectionPool(connector.DefaultPoolConfig())
	cleanupFunc := func() {
		log.Info("Shutting down connection pool...")
		b.connectionPool.Shutdown()
		// Add any other global cleanup tasks here
	}

	// Prepare the main Context structure
	runtimeCtx := &Context{
		GoCtx:  ctx,
		Logger: log,
		Engine: engineInst,
		Runner: b.runnerSvc,
		// Recorder will be initialized below
		ClusterConfig: currentClusterConfig,
		hostInfoMap:   make(map[string]*HostRuntimeInfo),
		// Global settings from clusterConfig.Spec.Global
		GlobalVerbose:           currentClusterConfig.Spec.Global.Verbose,
		GlobalIgnoreErr:         currentClusterConfig.Spec.Global.IgnoreErr,
		GlobalConnectionTimeout: currentClusterConfig.Spec.Global.ConnectionTimeout,
		// Initialize caches
		PipelineCache:  cache.NewPipelineCache(),
		ModuleCache:    cache.NewModuleCache(),
		TaskCache:      cache.NewTaskCache(),
		StepCache:      cache.NewStepCache(),
		ConnectionPool: b.connectionPool, // Assign created pool to context
	}
	if runtimeCtx.GlobalConnectionTimeout <= 0 { // Ensure a sensible default
		runtimeCtx.GlobalConnectionTimeout = 30 * time.Second
	}

	// Initialize EventRecorder
	// For a real implementation, a Kubernetes clientset would be needed.
	// For now, creating a simple broadcaster and recorder.
	// This might need to be adjusted if a real clientset becomes available in the builder.
	eventBroadcaster := record.NewBroadcaster()
	// TODO: If running inside a K8s cluster, use a proper event sink.
	// For CLI tool, logging events might be sufficient initially.
	// eventBroadcaster.StartLogging(log.ZapLogger().Sugar().Infof) // Example: log events
	// If you have a corev1.EventsGetter (from a clientset), you'd use:
	// eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: corev1Client.Events("")})
	// As a placeholder, we'll create a recorder that doesn't sink to API server.
	// This recorder will effectively be a no-op for API server events but allows the field to be set.
	runtimeCtx.Recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "kubexm-cli"})

	// Setup working directories on the control machine
	if err := b.setupGlobalWorkDirs(runtimeCtx); err != nil {
		cleanupFunc() // Call cleanup on error
		return nil, nil, err
	}

	// Initialize host runtimes (connect, gather facts) concurrently
	if err := b.initializeAllHosts(runtimeCtx, b.connectionPool, b.runnerSvc); err != nil {
		cleanupFunc()
		return nil, nil, err
	}

	log.Info("Runtime environment built successfully.")
	return runtimeCtx, cleanupFunc, nil
}

// setupGlobalWorkDirs sets up the main working directory and cluster-specific artifact directories.
func (b *RuntimeBuilder) setupGlobalWorkDirs(rc *Context) error {
	log := rc.Logger
	clusterName := rc.ClusterConfig.Name

	// 1. Determine and create GlobalWorkDir: $(pwd)/.kubexm (base for Kubexm operations)
	//    Then, the cluster-specific work dir is ${GlobalWorkDir}/${cluster_name}
	//    This cluster-specific dir will be stored as rc.GlobalWorkDir for the context.
	baseWorkDir := rc.ClusterConfig.Spec.Global.WorkDir
	if baseWorkDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Error(err, "Failed to get current working directory")
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		baseWorkDir = cwd // $(pwd)
		log.Info("Global.WorkDir in config not set, using current working directory as base.", "path", baseWorkDir)
	}

	kubexmOperationalRoot := filepath.Join(baseWorkDir, common.KUBEXM) // $(pwd)/.kubexm
	if err := util.CreateDir(kubexmOperationalRoot); err != nil {
		log.Error(err, "Failed to create Kubexm operational root directory", "path", kubexmOperationalRoot)
		return fmt.Errorf("failed to create Kubexm operational root dir '%s': %w", kubexmOperationalRoot, err)
	}

	// This becomes the main work directory for the context, specific to this cluster.
	rc.GlobalWorkDir = filepath.Join(kubexmOperationalRoot, clusterName) // $(pwd)/.kubexm/${cluster_name}
	log.Info("Ensuring cluster-specific global work directory exists.", "path", rc.GlobalWorkDir)
	if err := util.CreateDir(rc.GlobalWorkDir); err != nil {
		log.Error(err, "Failed to create cluster-specific global work directory", "path", rc.GlobalWorkDir)
		return fmt.Errorf("failed to create cluster-specific global work dir '%s': %w", rc.GlobalWorkDir, err)
	}

	// Create standard subdirectories within the cluster's GlobalWorkDir
	dirsToCreate := map[string]string{
		"logs":              filepath.Join(rc.GlobalWorkDir, common.DefaultLogsDir),
		"certs":             rc.GetCertsDir(), // Uses accessor which builds path from GlobalWorkDir
		"etcd_certs":        rc.GetEtcdCertsDir(),
		"etcd_artifacts":    rc.GetEtcdArtifactsDir(),
		"k8s_artifacts":     rc.GetKubernetesArtifactsDir(),
		"runtime_artifacts": rc.GetContainerRuntimeArtifactsDir(),
	}

	for name, dirPath := range dirsToCreate {
		log.Info(fmt.Sprintf("Ensuring %s directory exists.", name), "path", dirPath)
		if err := util.CreateDir(dirPath); err != nil {
			log.Error(err, fmt.Sprintf("Failed to create %s directory", name), "path", dirPath)
			return fmt.Errorf("failed to create %s directory '%s': %w", name, dirPath, err)
		}
	}
	return nil
}

// initializeAllHosts connects to all defined hosts (including control node) and gathers facts.
func (b *RuntimeBuilder) initializeAllHosts(rc *Context, pool *connector.ConnectionPool, runnerSvc runner.Runner) error {
	log := rc.Logger
	g, gCtx := errgroup.WithContext(rc.GoCtx)
	var mu sync.Mutex // To protect access to rc.hostInfoMap and rc.controlNode

	// Define all hosts to be initialized, including the special control node
	allHostSpecs := make([]v1alpha1.HostSpec, 0, len(rc.ClusterConfig.Spec.Hosts)+1)
	controlNodeSpec := v1alpha1.HostSpec{
		Name:    common.ControlNodeHostName,
		Type:    "local",
		Address: "127.0.0.1",
		Roles:   []string{common.ControlNodeRole},
	}
	allHostSpecs = append(allHostSpecs, controlNodeSpec)
	allHostSpecs = append(allHostSpecs, rc.ClusterConfig.Spec.Hosts...)

	for _, hostCfg := range allHostSpecs {
		currentHostCfg := hostCfg // Capture range variable
		g.Go(func() error {
			hri, err := b.initializeSingleHost(gCtx, currentHostCfg, rc.ClusterConfig.Spec.Global, pool, runnerSvc, log)
			if err != nil {
				return err // Error already contains host name for context
			}
			mu.Lock()
			rc.hostInfoMap[hri.Host.GetName()] = hri
			if hri.Host.GetName() == common.ControlNodeHostName {
				rc.controlNode = hri.Host
			}
			// Create host-specific directory within the cluster's global work dir
			// e.g., $(pwd)/.kubexm/${cluster_name}/${hostname}
			hostClusterDir := rc.GetHostDir(hri.Host.GetName())
			if err := util.CreateDir(hostClusterDir); err != nil {
				mu.Unlock()
				log.Error(err, "Failed to create host-specific cluster directory", "host", hri.Host.GetName(), "path", hostClusterDir)
				return fmt.Errorf("failed to create host-specific cluster directory '%s' for host '%s': %w", hostClusterDir, hri.Host.GetName(), err)
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Error(err, "One or more hosts failed to initialize")
		return fmt.Errorf("host initialization failed: %w", err)
	}

	if rc.controlNode == nil {
		// This should not happen if the control node initialization was successful
		return fmt.Errorf("critical error: control node was not set in runtime context after initialization")
	}

	log.Info("All hosts initialized successfully.")
	return nil
}

// initializeSingleHost handles connection and fact gathering for one host.
func (b *RuntimeBuilder) initializeSingleHost(ctx context.Context, hostCfg v1alpha1.HostSpec, globalCfg *v1alpha1.GlobalSpec, pool *connector.ConnectionPool, runnerSvc runner.Runner, parentLogger *logger.Logger) (*HostRuntimeInfo, error) {
	log := parentLogger.With("host", hostCfg.Name)
	log.Info("Initializing...")

	var conn connector.Connector
	isLocal := strings.ToLower(hostCfg.Type) == "local" || hostCfg.Address == "localhost" || hostCfg.Address == "127.0.0.1"
	if isLocal && hostCfg.Name != common.ControlNodeHostName {
		log.Warn("Host configured as local but name is not the control node name. Ensure this is intended.", "type", hostCfg.Type, "address", hostCfg.Address)
		// Proceeding as local if type/address indicates so.
	}

	if isLocal {
		conn = b.connectorFactory.NewLocalConnector()
		log.Info("Using LocalConnector.")
	} else {
		conn = b.connectorFactory.NewSSHConnector(pool) // SSH connector uses the pool
		log.Info("Using SSHConnector.")
	}

	// Prepare ConnectionCfg
	connCfg := connector.ConnectionCfg{
		Host:           hostCfg.Address,
		Port:           hostCfg.Port, // Defaults should be applied by v1alpha1.SetDefaults_HostSpec
		User:           hostCfg.User,
		Password:       hostCfg.Password,
		PrivateKeyPath: hostCfg.PrivateKeyPath,
		Timeout:        30 * time.Second, // Default connection timeout
	}
	if globalCfg != nil && globalCfg.ConnectionTimeout > 0 {
		connCfg.Timeout = globalCfg.ConnectionTimeout
	}

	// Handle inline private key
	if hostCfg.PrivateKey != "" {
		decodedKey, err := base64.StdEncoding.DecodeString(hostCfg.PrivateKey)
		if err != nil {
			log.Error(err, "Failed to decode base64 private key")
			return nil, fmt.Errorf("host %s: failed to decode private key: %w", hostCfg.Name, err)
		}
		connCfg.PrivateKey = decodedKey
		connCfg.PrivateKeyPath = "" // Clear path if key content is provided
		log.Debug("Using provided base64 private key content.")
	} else if hostCfg.PrivateKeyPath != "" {
		// Ensure PrivateKeyPath is absolute or resolved correctly if relative
		// For now, assume it's either absolute or resolvable by os.ReadFile
		keyBytes, err := osReadFileFS(hostCfg.PrivateKeyPath)
		if err != nil {
			log.Error(err, "Failed to read private key from path", "path", hostCfg.PrivateKeyPath)
			return nil, fmt.Errorf("host %s: failed to read private key file '%s': %w", hostCfg.Name, hostCfg.PrivateKeyPath, err)
		}
		connCfg.PrivateKey = keyBytes
		log.Debug("Loaded private key from path.", "path", hostCfg.PrivateKeyPath)
	}

	if err := conn.Connect(ctx, connCfg); err != nil {
		log.Error(err, "Connection failed")
		return nil, fmt.Errorf("host %s: connection failed: %w", hostCfg.Name, err)
	}
	log.Info("Successfully connected.")

	log.Info("Gathering facts...")
	facts, err := runnerSvc.GatherFacts(ctx, conn)
	if err != nil {
		//conn.Close() // Attempt to close connection on error
		log.Error(err, "Fact gathering failed")
		return nil, fmt.Errorf("host %s: fact gathering failed: %w", hostCfg.Name, err)
	}
	log.Info("Successfully gathered facts.", "OS", facts.OS.ID, "Version", facts.OS.VersionID, "Arch", facts.OS.Arch)

	abstractHost := connector.NewHostFromSpec(hostCfg)
	hri := &HostRuntimeInfo{
		Host:  abstractHost,
		Conn:  conn,
		Facts: facts,
	}
	log.Info("Initialization complete.")
	return hri, nil
}
