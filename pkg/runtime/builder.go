package runtime

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"net/http"
	"sync"
	"time"
)

type Builder struct {
	configFilepath         string
	clusterConfig          *v1alpha1.Cluster
	httpClientOverride     *http.Client
	loggerOverride         *logger.Logger
	connectionPoolOverride *connector.ConnectionPool
	poolConfigOverride     *connector.PoolConfig
	runIDOverride          string
}

func (b *Builder) WithRunID(runID string) *Builder {
	if runID != "" {
		b.runIDOverride = runID
	}
	return b
}

func (b *Builder) WithHttpClient(client *http.Client) *Builder {
	b.httpClientOverride = client
	return b
}

func (b *Builder) WithLogger(log *logger.Logger) *Builder {
	b.loggerOverride = log
	return b
}

func (b *Builder) WithConnectionPool(connectionPool *connector.ConnectionPool) *Builder {
	b.connectionPoolOverride = connectionPool
	return b
}

func (b *Builder) WithPoolConfig(poolConfig *connector.PoolConfig) *Builder {
	b.poolConfigOverride = poolConfig
	return b
}

func NewBuilder(configFilepath string) *Builder {
	return &Builder{configFilepath: configFilepath}
}

func NewBuilderFromConfig(cfg *v1alpha1.Cluster) *Builder {
	return &Builder{clusterConfig: cfg}
}

func (b *Builder) Build(ctx context.Context) (*Context, func(), error) {
	var log *logger.Logger
	if b.loggerOverride != nil {
		log = b.loggerOverride
	} else {
		log = logger.Get()
	}

	var runID string
	if b.runIDOverride != "" {
		runID = b.runIDOverride
	} else {
		runID = uuid.New().String()
	}

	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "kubexm-cli"})

	var httpClient *http.Client
	if b.httpClientOverride != nil {
		log.Info("Using user-provided http.Client.")
		httpClient = b.httpClientOverride
	} else {
		log.Info("Creating default http.Client.")
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
			},
		}
	}

	currentClusterConfig, err := b.getOrParseConfig()
	if err != nil {
		return nil, nil, err
	}

	var poolConfig *connector.PoolConfig
	if b.poolConfigOverride != nil {
		poolConfig = b.poolConfigOverride
	} else {
		poolConfig = connector.DefaultPoolConfig()
		if currentClusterConfig.Spec.Global.ConnectionTimeout > 0 {
			poolConfig.ConnectTimeout = currentClusterConfig.Spec.Global.ConnectionTimeout
		}
	}

	var connectionPool *connector.ConnectionPool
	if b.connectionPoolOverride != nil {
		connectionPool = b.connectionPoolOverride
	} else {
		log.Info("Initializing internal services...")

		connectionPool = connector.NewConnectionPool(poolConfig)
	}

	connectorFactory := connector.NewFactory()
	runnerSvc := runner.NewRunner()
	//execEngine := engine.NewExecutor()
	cleanupFunc := func() {
		log.Info("Shutting down connection pool...")
		connectionPool.Shutdown()
	}
	pipelineCache := cache.NewPipelineCache()
	moduleCache := cache.NewModuleCache(pipelineCache)
	taskCache := cache.NewTaskCache(moduleCache)
	stepCache := cache.NewStepCache(taskCache)

	runtimeCtx := &Context{
		GoCtx:    ctx,
		Logger:   log,
		Runner:   runnerSvc,
		Recorder: recorder,
		//Engine:                  execEngine,
		ClusterConfig: currentClusterConfig,

		GlobalWorkDir:           currentClusterConfig.Spec.Global.WorkDir,
		GlobalVerbose:           currentClusterConfig.Spec.Global.Verbose,
		GlobalIgnoreErr:         currentClusterConfig.Spec.Global.IgnoreErr,
		GlobalConnectionTimeout: currentClusterConfig.Spec.Global.ConnectionTimeout,
		GlobalOfflineMode:       currentClusterConfig.Spec.Global.OfflineMode,

		PipelineCache: pipelineCache,
		ModuleCache:   moduleCache,
		TaskCache:     taskCache,
		StepCache:     stepCache,

		hostInfoMap:    make(map[string]*HostRuntimeInfo),
		ConnectionPool: connectionPool,

		stepExecutionID:    "",
		executionStartTime: time.Now(),

		httpClient:               httpClient,
		currentStepRuntimeConfig: map[string]interface{}{},
		RunID:                    runID,
	}

	if err := b.initializeAllHosts(runtimeCtx, connectorFactory, runnerSvc); err != nil {
		cleanupFunc()
		return nil, nil, err
	}
	runtimeCtx.controlNode = runtimeCtx.GetHostsByRole(common.ControlNodeRole)[0]
	log.Info("Runtime environment built successfully.")
	return runtimeCtx, cleanupFunc, nil
}

func (b *Builder) getOrParseConfig() (*v1alpha1.Cluster, error) {
	if b.clusterConfig != nil {
		v1alpha1.SetDefaults_Cluster(b.clusterConfig)
		verrs := validation.ValidationErrors{}
		v1alpha1.Validate_Cluster(b.clusterConfig, &verrs)
		if verrs.HasErrors() {
			return nil, fmt.Errorf("validation failed for pre-loaded cluster configuration: %w", verrs.Error())
		}
		return b.clusterConfig, nil
	}
	if b.configFilepath != "" {
		return config.ParseYAML(b.configFilepath)
	}
	return nil, fmt.Errorf("RuntimeBuilder requires either a config file path or a pre-loaded config object")
}

func (b *Builder) initializeAllHosts(rc *Context, factory connector.Factory, runnerSvc runner.Runner) error {
	log := rc.Logger
	g, gCtx := errgroup.WithContext(rc.GoCtx)
	var mu sync.Mutex

	allHostSpecs := make([]v1alpha1.HostSpec, 0, len(rc.ClusterConfig.Spec.Hosts)+1)
	controlNodeSpec := v1alpha1.HostSpec{
		Name:    common.ControlNodeHostName,
		Address: "127.0.0.1",
		Roles:   []string{common.ControlNodeRole},
	}
	allHostSpecs = append(allHostSpecs, controlNodeSpec)
	allHostSpecs = append(allHostSpecs, rc.ClusterConfig.Spec.Hosts...)

	log.Info("Initializing all hosts in parallel...", "count", len(allHostSpecs))
	for _, hostCfg := range allHostSpecs {
		currentHostCfg := hostCfg
		g.Go(func() error {
			hri, err := b.initializeSingleHost(gCtx, currentHostCfg, factory, runnerSvc, rc.ConnectionPool, log)
			if err != nil {
				return err
			}
			mu.Lock()
			rc.hostInfoMap[hri.Host.GetName()] = hri
			if hri.Host.IsRole(common.ControlNodeRole) {
				rc.controlNode = hri.Host
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("one or more hosts failed to initialize: %w", err)
	}
	if rc.controlNode == nil {
		return fmt.Errorf("critical error: control node was not set after initialization")
	}
	log.Info("All hosts initialized successfully.")
	return nil
}

func (b *Builder) initializeSingleHost(ctx context.Context, hostCfg v1alpha1.HostSpec, factory connector.Factory, runnerSvc runner.Runner, pool *connector.ConnectionPool, parentLogger *logger.Logger) (*HostRuntimeInfo, error) {
	log := parentLogger.With("host", hostCfg.Name)
	log.Info("Initializing...")

	host := connector.NewHostFromSpec(hostCfg)
	conn, err := factory.NewConnectorForHost(host, pool)
	if err != nil {
		return nil, fmt.Errorf("host %s: failed to create connector: %w", host.GetName(), err)
	}
	log.Info(fmt.Sprintf("Using %T for connection.", conn))

	connCfg, err := factory.NewConnectionCfg(host, b.clusterConfig.Spec.Global.ConnectionTimeout)
	if err != nil {
		return nil, err
	}
	if err := conn.Connect(ctx, connCfg); err != nil {
		return nil, fmt.Errorf("host %s: connection failed: %w", hostCfg.Name, err)
	}
	log.Info("Successfully connected.")
	log.Info("Gathering facts...")
	facts, err := runnerSvc.GatherFacts(ctx, conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("host %s: fact gathering failed: %w", hostCfg.Name, err)
	}
	log.Info("Successfully gathered facts.", "OS", facts.OS.ID, "Arch", facts.OS.Arch)

	hri := &HostRuntimeInfo{
		Host:  host,
		Conn:  conn,
		Facts: facts,
	}
	log.Info("Initialization complete.")
	return hri, nil
}
