package containerd

import (
	"context" // Required by runtime.Context
	"fmt"
	"time"

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
func (e *EnableAndStartContainerdStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	// Retrieve spec for its name, even if spec has no other fields.
	rawSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		// If spec is critical for logging or identification, handle error.
		// For now, proceeding with a default name if spec is not found/typed.
		ctx.Logger.Warnf("StepSpec not found in context for EnableAndStartContainerdStepExecutor Check method, using default logger name.")
	}
	specName := containerdServiceName + " service check" // Default name
	if s, ok := rawSpec.(*EnableAndStartContainerdStepSpec); ok {
		specName = s.GetName()
	}

	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", specName).Sugar()

	active, err := ctx.Host.Runner.IsServiceActive(ctx.GoContext, containerdServiceName)
	if err != nil {
		return false, fmt.Errorf("failed to check if %s service is active on host %s: %w", containerdServiceName, ctx.Host.Name, err)
	}
	if !active {
		hostCtxLogger.Debugf("Service %s is not active.", containerdServiceName)
		return false, nil
	}

	// TODO: Implement a robust IsServiceEnabled check in the runner and call it here if needed.
	// For now, if it's active, we consider the main goal of "running" as met for idempotency.
	// The Run method will still execute `EnableService` which is idempotent on its own.
	hostCtxLogger.Infof("Service %s is active.", containerdServiceName)
	return true, nil
}

// Execute enables and starts the containerd service.
func (e *EnableAndStartContainerdStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	rawSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for EnableAndStartContainerdStepExecutor Execute"))
	}
	spec, ok := rawSpec.(*EnableAndStartContainerdStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for EnableAndStartContainerdStepExecutor Execute: %T", rawSpec))
	}

	res := step.NewResult(ctx, startTime, nil) // Use new NewResult signature
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	hostCtxLogger.Infof("Performing daemon-reload (best effort) before managing %s.", containerdServiceName)
	if err := ctx.Host.Runner.DaemonReload(ctx.GoContext); err != nil {
	    hostCtxLogger.Warnf("daemon-reload reported error (may be non-critical): %v", err)
	}

	hostCtxLogger.Infof("Enabling %s service...", containerdServiceName)
	if err := ctx.Host.Runner.EnableService(ctx.GoContext, containerdServiceName); err != nil {
		res.Error = fmt.Errorf("failed to enable %s service: %w", containerdServiceName, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Successf("Service %s enabled.", containerdServiceName)

	hostCtxLogger.Infof("Starting %s service...", containerdServiceName)
	if err := ctx.Host.Runner.StartService(ctx.GoContext, containerdServiceName); err != nil {
		res.Error = fmt.Errorf("failed to start %s service: %w", containerdServiceName, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	hostCtxLogger.Successf("Service %s started.", containerdServiceName)

	active, verifyErr := ctx.Host.Runner.IsServiceActive(ctx.GoContext, containerdServiceName)
	if verifyErr != nil {
		// This is a failure of verification, not necessarily of the start command itself.
		res.Error = fmt.Errorf("failed to verify %s service status after start: %w", containerdServiceName, verifyErr)
		res.Status = step.StatusFailed // If verification fails, consider the step failed.
		res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step could not verify service status: %v", res.Error)
		return res
	}
	if !active {
		res.Error = fmt.Errorf("%s service started but reported as not active immediately after", containerdServiceName)
		res.Status = step.StatusFailed; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	res.EndTime = time.Now()
	res.Status = step.StatusSucceeded
	res.Message = fmt.Sprintf("%s service enabled and started successfully.", containerdServiceName)
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}
var _ step.StepExecutor = &EnableAndStartContainerdStepExecutor{}
