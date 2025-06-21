###pkg/step - 原子执行单元
#### step.Step 接口: 定义所有 Step 必须实现的行为。
##### step的interface.go
```aiignore
package step

import (
    "github.com/mensylisir/kubexm/pkg/connector"
    "github.com/mensylisir/kubexm/pkg/runtime"
    "github.com/mensylisir/kubexm/pkg/spec"
)
// StepContext defines the context passed to individual steps.
// It is implemented by the runtime and provided by the engine.
type StepContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetHost() connector.Host // The current host this context is for
	GetRunner() runner.Runner // To execute commands, etc.
	GetClusterConfig() *v1alpha1.Cluster
	StepCache() cache.StepCache
	TaskCache() cache.TaskCache
	ModuleCache() cache.ModuleCache

	// Host and cluster information access
	GetHostsByRole(role string) ([]connector.Host, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error) // Get facts for any host
	GetCurrentHostFacts() (*runner.Facts, error)             // Convenience for current host
	GetConnectorForHost(host connector.Host) (connector.Connector, error) // Get connector for any host
	GetCurrentHostConnector() (connector.Connector, error) // Convenience for current host
	GetControlNode() (connector.Host, error)

	// Global configurations
	GetGlobalWorkDir() string
	IsVerbose() bool
	ShouldIgnoreErr() bool
	GetGlobalConnectionTimeout() time.Duration

	// Artifact path helpers
	GetClusterArtifactsDir() string
	GetCertsDir() string
	GetEtcdCertsDir() string
	GetComponentArtifactsDir(componentName string) string
	GetEtcdArtifactsDir() string
	GetContainerRuntimeArtifactsDir() string
	GetKubernetesArtifactsDir() string
	GetFileDownloadPath(componentName, version, arch, filename string) string
	GetHostDir(hostname string) string // Local workdir for a given host

	// WithGoContext is needed by the engine to propagate errgroup's context.
	WithGoContext(goCtx context.Context) StepContext
}

// Step defines an atomic, idempotent execution unit.
type Step interface {
    // Meta returns the step's metadata.
    Meta() *spec.StepMeta

    // Precheck determines if the step's desired state is already met.
    // If it returns true, Run will be skipped.
    Precheck(ctx runtime.StepContext, host connector.Host) (isDone bool, err error)

    // Run executes the main logic of the step.
    Run(ctx runtime.StepContext, host connector.Host) error

    // Rollback attempts to revert the changes made by Run.
    // It's called only if Run fails.
    Rollback(ctx runtime.StepContext, host connector.Host) error
}
```
示例 Step 规格 - command.CommandStepSpec: 这是您提供的完美示例，它展示了一个 Step 规格应该如何设计：包含 StepMeta、所有配置字段，并实现 step.Step 接口。其他 Step 如 UploadFileStepSpec, InstallPackageStepSpec 等都将遵循此模式。