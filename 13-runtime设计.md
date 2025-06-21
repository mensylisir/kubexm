pkg/runtime - 上下文与状态管理
runtime.Context 结构体: 系统的“上帝对象”，持有所有状态和依赖。
```aiignore
package runtime

import (
    "context"
    "github.com/mensylisir/kubexm/pkg/apis/v1alpha1"
    "github.com/mensylisir/kubexm/pkg/cache"
    "github.com/mensylisir/kubexm/pkg/connector"
    "github.com/mensylisir/kubexm/pkg/engine"
    "github.com/mensylisir/kubexm/pkg/logger" // Assuming a logger package
    "github.com/mensylisir/kubexm/pkg/runner"
)

type Context struct {
    GoContext      context.Context
    Logger         *logger.Logger
    Engine         engine.Engine
    Runner         runner.Runner
    Recorder       *event.Recorder
    ClusterConfig  *v1alpha1.Cluster
    HostRuntimes   map[string]*HostRuntime // Key: host.GetName()
    ConnectionPool *connector.ConnectionPool
    
    GlobalWorkDir           string
	GlobalVerbose           bool
	GlobalIgnoreErr         bool
	GlobalConnectionTimeout time.Duration
    
    // Scoped caches
    pipelineCache cache.PipelineCache
    moduleCache   cache.ModuleCache
    taskCache     cache.TaskCache
    stepCache     cache.StepCache
}

type HostRuntime struct {
    Host  connector.Host
    Conn  connector.Connector
    Facts *runner.Facts
}

// NewContextWithGoContext is a helper to create a new context with a different Go context,
// for passing down cancellation signals from errgroup.
func NewContextWithGoContext(gCtx context.Context, parent *Context) *Context {
	newCtx := *parent
	newCtx.GoCtx = gCtx
	return &newCtx
}
```
runtime Facade 接口: 定义了暴露给各层的安全视图。
```aiignore
package runtime

// ... imports ...

type PipelineContext interface {
    GetLogger() *logger.Logger
    GetClusterConfig() *v1alpha1.Cluster
    GoContext() context.Context
}

type ModuleContext interface {
    PipelineContext
    // Module-specific getters can be added here
}

type TaskContext interface {
    ModuleContext
    GetHostsByRole(role string) ([]connector.Host, error)
    GetHostFacts(host connector.Host) (*runner.Facts, error)
    GetClusterConfig() *v1alpha1.Cluster
    GetGlobalWorkDir() string // Expose the main work directory for planning local paths
}

type StepContext interface {
    GetLogger() *logger.Logger
    GetRecorder() *event.Recorder
    GetRunner() runner.Runner
    GetConnectorForHost(host connector.Host) (connector.Connector, error)
    GetHostFacts(host connector.Host) (*runner.Facts, error)
    GoContext() context.Context
}


```

pkg/runtime/builder.go - 运行时构建器 (您的实现 + DAG适配)
```aiignore
package runtime

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/engine"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/parser" // Assuming parser package exists
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/util"   // Assuming util package with CreateDir exists
)

var osReadFile = os.ReadFile // For mocking in tests

// RuntimeBuilder builds a fully initialized runtime Context.
type RuntimeBuilder struct {
	configFile string
}

// NewRuntimeBuilder creates a new RuntimeBuilder.
func NewRuntimeBuilder(configFile string) *RuntimeBuilder {
	return &RuntimeBuilder{configFile: configFile}
}

// Build constructs the full runtime Context. It's the main entry point.
func (b *RuntimeBuilder) Build(ctx context.Context) (*Context, func(), error) {
	// 1. Parse Configuration
	log := logger.Get() // Use a base logger for pre-init phase
	log.Info("Building runtime environment...", "configFile", b.configFile)
	clusterConfig, err := parser.ParseFromFile(b.configFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse cluster config '%s': %w", b.configFile, err)
	}

	// 2. Initialize Core Services & Resources
	runnerSvc := runner.New()
	engineSvc := engine.NewExecutor() // This would be the new DAG-aware engine
	pool := connector.NewConnectionPool(connector.DefaultPoolConfig())
	cleanupFunc := func() {
		log.Info("Shutting down connection pool...")
		pool.Shutdown()
	}

	// 3. Initialize Host Runtimes Concurrently
	hostRuntimes := make(map[string]*HostRuntime)
	var mu sync.Mutex
	g, gCtx := errgroup.WithContext(ctx)

	for i := range clusterConfig.Spec.Hosts {
		// Capture loop variable correctly for goroutines
		currentHostCfg := clusterConfig.Spec.Hosts[i]
		g.Go(func() error {
			return b.initializeHost(gCtx, ¤tHostCfg, clusterConfig, pool, runnerSvc, &mu, hostRuntimes)
		})
	}

	// Also initialize the special "control-node" for local operations.
	// This is a key part of the DAG model to handle local preparations.
	g.Go(func() error {
		controlNodeHostSpec := v1alpha1.Host{
			Name:    common.ControlNodeHostName,
			Type:    "local", // Explicitly use type for dispatching
			Address: "127.0.0.1",
			Roles:   []string{common.ControlNodeRole},
		}
		return b.initializeHost(gCtx, &controlNodeHostSpec, clusterConfig, pool, runnerSvc, &mu, hostRuntimes)
	})


	if err := g.Wait(); err != nil {
		log.Error(err, "Failed during concurrent host initialization")
		cleanupFunc()
		return nil, nil, fmt.Errorf("failed during host initialization: %w", err)
	}
	log.Info("All hosts initialized successfully.")
	
	// 4. Assemble the Final Context
	runtimeCtx := &Context{
		GoCtx:         ctx,
		Logger:        log,
		Engine:        engineSvc,
		Runner:        runnerSvc,
		ClusterConfig: clusterConfig,
		HostRuntimes:  hostRuntimes,
		ConnectionPool: pool,
	}

	// 5. Setup Work Directories (logic from your implementation)
	b.setupWorkDirs(runtimeCtx)
	
	// 6. Initialize Caches
	runtimeCtx.PipelineCache = cache.NewPipelineCache()
	runtimeCtx.ModuleCache = cache.NewModuleCache()
	runtimeCtx.TaskCache = cache.NewTaskCache()
	runtimeCtx.StepCache = cache.NewStepCache()

	log.Info("Runtime environment built successfully.")
	return runtimeCtx, cleanupFunc, nil
}


// initializeHost contains the logic to connect and gather facts for a single host.
func (b *RuntimeBuilder) initializeHost(ctx context.Context, hostCfg *v1alpha1.Host, clusterCfg *v1alpha1.Cluster, pool *connector.ConnectionPool, runnerSvc runner.Runner, mu *sync.Mutex, runtimes map[string]*HostRuntime) error {
	hostLogger := logger.Get().With("host", hostCfg.Name)
	hostLogger.Info("Initializing runtime for host...")

	var conn connector.Connector
	if strings.ToLower(hostCfg.Type) == "local" {
		conn = &connector.LocalConnector{}
	} else {
		conn = connector.NewSSHConnector(pool)
	}

	// Prepare connection config (logic from your implementation)
	connectionCfg := connector.ConnectionCfg{
		Host:    hostCfg.Address,
		Port:    hostCfg.Port,
		User:    hostCfg.User,
		Password: hostCfg.Password,
		Timeout: connector.DefaultConnectTimeout, // Default
	}
	// ... (your logic for PrivateKey, PrivateKeyPath, and Timeout overrides) ...
	
	if err := conn.Connect(ctx, connectionCfg); err != nil {
		return fmt.Errorf("failed to connect to host %s: %w", hostCfg.Name, err)
	}

	facts, err := runnerSvc.GatherFacts(ctx, conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to gather facts for host %s: %w", hostCfg.Name, err)
	}
	hostLogger.Info("Successfully gathered facts.", "OS", facts.OS.PrettyName)

	host := connector.NewHostFromSpec(*hostCfg) // Create the abstract host object
	hr := &HostRuntime{
		Host:  host,
		Conn:  conn,
		Facts: facts,
	}
	
	mu.Lock()
	runtimes[host.GetName()] = hr
	mu.Unlock()

	return nil
}

// setupWorkDirs handles the creation of all necessary local working directories.
func (b *RuntimeBuilder) setupWorkDirs(ctx *Context) error {
	log := ctx.Logger
	
	// Determine and create the main work directory
	workDir := ctx.ClusterConfig.Spec.Global.WorkDir
	if workDir == "" {
		cwd, _ := os.Getwd()
		workDir = filepath.Join(cwd, common.DefaultWorkDirName, ctx.ClusterConfig.Name)
		log.Info("Global.WorkDir not set, using default.", "path", workDir)
	}
	ctx.GlobalWorkDir = workDir
	if err := util.CreateDir(workDir); err != nil {
		return fmt.Errorf("failed to create global work directory '%s': %w", workDir, err)
	}

	// Create host-specific subdirectories
	for _, hostRuntime := range ctx.HostRuntimes {
		// Don't create a dir for the special control-node, or handle as needed
		if hostRuntime.Host.GetName() == common.ControlNodeHostName {
			continue
		}
		hostWorkDir := filepath.Join(workDir, hostRuntime.Host.GetName())
		if err := util.CreateDir(hostWorkDir); err != nil {
			return fmt.Errorf("failed to create host work dir for '%s': %w", hostRuntime.Host.GetName(), err)
		}
	}
	return nil
}
```
设计优化点:
逻辑重构: 将主 Build 函数的逻辑拆分到辅助函数 initializeHost 和 setupWorkDirs 中，使得 Build 函数本身更清晰，只负责编排。
拥抱DAG模型: 在 Build 函数中，明确地、程序化地创建了一个特殊的 control-node 主机。这是我们之前讨论的将本地操作统一到执行模型中的关键一步。现在 Builder 负责创建这个虚拟节点。
日志上下文: initializeHost 为每个goroutine创建了带有主机名的日志记录器，这在并发执行时对于调试至关重要。
健壮性: 使用了 for i := range ... 的方式来正确捕获循环变量，避免了常见的goroutine陷阱。
清晰的职责: setupWorkDirs 专门负责所有文件系统目录的创建，职责单一。