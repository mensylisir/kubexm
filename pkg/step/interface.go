package step

import (
	"context"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec" // For spec.StepMeta
)

// StepContext defines the context passed to individual steps.
// It is implemented by the runtime and provided by the engine.
type StepContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetHost() connector.Host  // The current host this context is for
	GetRunner() runner.Runner // To execute commands, etc.
	GetClusterConfig() *v1alpha1.Cluster
	GetStepCache() cache.StepCache
	GetTaskCache() cache.TaskCache
	GetModuleCache() cache.ModuleCache
	GetPipelineCache() cache.PipelineCache

	// Host and cluster information access
	GetHostsByRole(role string) ([]connector.Host, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error)              // Get facts for any host
	GetCurrentHostFacts() (*runner.Facts, error)                          // Convenience for current host
	GetConnectorForHost(host connector.Host) (connector.Connector, error) // Get connector for any host
	GetCurrentHostConnector() (connector.Connector, error)                // Convenience for current host
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

// Step defines the interface that all concrete steps must implement.
// Steps are designed to be idempotent.
type Step interface {
	// Meta returns the step's metadata, including its name and description.
	Meta() *spec.StepMeta

	// Precheck determines if the step's conditions are already met or if execution is required.
	// It is called by the Engine before Run.
	// - ctx: The StepContext providing access to runtime services and host-specific info.
	// - host: The target host for this precheck.
	// Returns:
	//   - bool: true if the step is considered done/skipped, false if Run needs to be called.
	//   - error: Any error encountered during the precheck. If an error occurs,
	//            the step is generally considered failed and Run will not be called.
	Precheck(ctx StepContext, host connector.Host) (bool, error)

	// Run executes the primary logic of the step.
	// It is called by the Engine if Precheck returns false and no error.
	// - ctx: The StepContext providing access to runtime services and host-specific info.
	// - host: The target host for this execution.
	// Returns:
	//   - error: Any error encountered during execution. If an error occurs,
	//            the Engine may attempt to call Rollback.
	Run(ctx StepContext, host connector.Host) error

	// Rollback attempts to revert any changes made by the Run method.
	// It is called by the Engine if Run returns an error.
	// Implementations should be idempotent.
	// - ctx: The StepContext providing access to runtime services and host-specific info.
	// - host: The target host for this rollback.
	// Returns:
	//   - error: Any error encountered during rollback.
	Rollback(ctx StepContext, host connector.Host) error
}
