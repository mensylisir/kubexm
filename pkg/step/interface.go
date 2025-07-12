package step

import (
	"context"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/cache"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner"
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
	GetStepCache() cache.StepCache
	GetTaskCache() cache.TaskCache
	GetModuleCache() cache.ModuleCache

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
    Precheck(ctx StepContext, host connector.Host) (isDone bool, err error)

    // Run executes the main logic of the step.
    Run(ctx StepContext, host connector.Host) error

    // Rollback attempts to revert the changes made by Run.
    // It's called only if Run fails.
    Rollback(ctx StepContext, host connector.Host) error
}