package runtime

import (
	"context"

	"github.com/mensylisir/kubexm/pkg/cache"     // Added for StepCache
	"github.com/mensylisir/kubexm/pkg/connector" // Updated import path
	"github.com/mensylisir/kubexm/pkg/logger"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runner"    // Updated import path
	// "github.com/mensylisir/kubexm/api/v1alpha1" // Assuming this path from issue, might need adjustment
	// For now, let's use a more generic config placeholder if api/v1alpha1 is not yet defined or used by this facade directly.
	// Instead of a direct v1alpha1.Cluster, let's use an interface or a more abstract config type if possible,
	// or defer its full usage until ClusterConfig type is confirmed.
	// For now, sticking to the issue's v1alpha1.Cluster for GetClusterConfig.
	// If "github.com/mensylisir/kubexm/api/v1alpha1" is not found, this will cause an error.
	// This path "github.com/mensylisir/kubexm/api/v1alpha1" seems to be a custom API definition.
	// It should be "github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" based on earlier file listing.
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// PipelineContext is the view for the Pipeline layer.
type PipelineContext interface {
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster // Uses the corrected path
	GoContext() context.Context          // Added GoContext from StepContext for consistency and use in Plan
}

// ModuleContext is the view for the Module layer.
// It needs everything PipelineContext has.
type ModuleContext interface {
	PipelineContext
	// Potentially add methods specific to module planning if any, e.g. GetModuleConfig(name string)
}

// TaskContext is the view for the Task layer.
// It needs everything ModuleContext has, plus host and fact access.
type TaskContext interface {
	ModuleContext
	GetHostsByRole(role string) ([]connector.Host, error) // Error was missing in issue spec
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	// GetConnectorForHost might also be useful here if tasks need direct conn access outside steps
}

// StepContext is the view for the Step layer.
type StepContext interface {
	GetLogger() *logger.Logger
	GetRunner() runner.Runner
	GetConnectorForHost(host connector.Host) (connector.Connector, error)
	GetHostFacts(host connector.Host) (*runner.Facts, error)
	GoContext() context.Context
	StepCache() cache.StepCache // Added method
	GetHost() connector.Host    // Added method for current host
	TaskCache() cache.TaskCache // Added method for Task level cache access
	ModuleCache() cache.ModuleCache // Added method for Module level cache access
}
