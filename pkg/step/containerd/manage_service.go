package containerd

import (
	"context" // Required by runtime.Context
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Added if not already (should be from previous)
	// "github.com/kubexms/kubexms/pkg/runner" // Not directly needed here
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const containerdServiceName = "containerd" // Keep this package-level const

// EnableAndStartContainerdStepSpec defines parameters for managing the containerd service.
// No specific parameters for this version, but StepName can be used for custom naming.
type EnableAndStartContainerdStepSpec struct {
	StepName string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *EnableAndStartContainerdStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	return fmt.Sprintf("Enable and Start %s service", containerdServiceName)
}
var _ spec.StepSpec = &EnableAndStartContainerdStepSpec{}

// EnableAndStartContainerdStepExecutor implements the logic for EnableAndStartContainerdStepSpec.
type EnableAndStartContainerdStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&EnableAndStartContainerdStepSpec{}), &EnableAndStartContainerdStepExecutor{})
}

// Check determines if the containerd service is active.
// A more thorough check might also verify if it's enabled.
func (e *EnableAndStartContainerdStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host not found in context for EnableAndStartContainerdStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Warn("StepSpec not found in context, using default logger name for check.")
	}
	specName := containerdServiceName + " service check" // Default name
	if s, okSpec := rawSpec.(*EnableAndStartContainerdStepSpec); okSpec {
		specName = s.GetName()
	}
	logger = logger.With("step", specName)


	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	active, err := conn.IsServiceActive(goCtx, containerdServiceName) // Use connector
	if err != nil {
		logger.Error("Failed to check service active status", "service", containerdServiceName, "error", err)
		return false, fmt.Errorf("failed to check if %s service is active on host %s: %w", containerdServiceName, currentHost.GetName(), err)
	}
	if !active {
		logger.Debug("Service is not active.", "service", containerdServiceName)
		return false, nil
	}

	logger.Info("Service is active.", "service", containerdServiceName)
	return true, nil
}

// Execute enables and starts the containerd service.
func (e *EnableAndStartContainerdStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host not found in context for EnableAndStartContainerdStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for EnableAndStartContainerdStepExecutor Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*EnableAndStartContainerdStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected StepSpec type for EnableAndStartContainerdStepExecutor Execute: %T", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("step", spec.GetName())


	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	logger.Info("Performing daemon-reload (best effort).", "service", containerdServiceName)
	if err := conn.DaemonReload(goCtx); err != nil { // Use connector
	    logger.Warn("daemon-reload reported error (may be non-critical).", "error", err)
	}

	logger.Info("Enabling service.", "service", containerdServiceName)
	if err := conn.EnableService(goCtx, containerdServiceName); err != nil { // Use connector
		logger.Error("Failed to enable service.", "service", containerdServiceName, "error", err)
		res.Error = fmt.Errorf("failed to enable %s service: %w", containerdServiceName, err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Service enabled.", "service", containerdServiceName)

	logger.Info("Starting service.", "service", containerdServiceName)
	if err := conn.StartService(goCtx, containerdServiceName); err != nil { // Use connector
		logger.Error("Failed to start service.", "service", containerdServiceName, "error", err)
		res.Error = fmt.Errorf("failed to start %s service: %w", containerdServiceName, err)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger.Info("Service started.", "service", containerdServiceName)

	active, verifyErr := conn.IsServiceActive(goCtx, containerdServiceName) // Use connector
	if verifyErr != nil {
		logger.Error("Failed to verify service status after start.", "service", containerdServiceName, "error", verifyErr)
		res.Error = fmt.Errorf("failed to verify %s service status after start: %w", containerdServiceName, verifyErr)
		res.Status = step.StatusFailed
		res.EndTime = time.Now()
		return res
	}
	if !active {
		logger.Error("Service started but reported as not active immediately after.", "service", containerdServiceName)
		res.Error = fmt.Errorf("%s service started but reported as not active immediately after", containerdServiceName)
		res.Status = step.StatusFailed; res.EndTime = time.Now()
		return res
	}

	res.EndTime = time.Now()
	res.Status = step.StatusSucceeded
	res.Message = fmt.Sprintf("%s service enabled and started successfully.", containerdServiceName)
	logger.Info("Step succeeded.", "message", res.Message)
	return res
}
var _ step.StepExecutor = &EnableAndStartContainerdStepExecutor{}
