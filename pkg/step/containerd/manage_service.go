package containerd

import (
	"context"
	"fmt"
	"time"

	"github.com/kubexms/kubexms/pkg/runner" // For DetectInitSystem if it were on runner
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

const containerdServiceName = "containerd"

// EnableAndStartContainerdStep enables and starts the containerd service.
type EnableAndStartContainerdStep struct{}

// Name returns a human-readable name for the step.
func (s *EnableAndStartContainerdStep) Name() string {
	return fmt.Sprintf("Enable and Start %s service", containerdServiceName)
}

// Check determines if the containerd service is already active.
// A more thorough check might also verify if it's enabled, but this
// often depends on the init system and can be complex to generify.
// For now, active is the primary concern for "isDone".
func (s *EnableAndStartContainerdStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	active, err := ctx.Host.Runner.IsServiceActive(ctx.GoContext, containerdServiceName)
	if err != nil {
		// If checking status itself fails, we can't determine if it's done.
		return false, fmt.Errorf("failed to check if %s service is active on host %s: %w", containerdServiceName, ctx.Host.Name, err)
	}
	if !active {
		hostCtxLogger.Debugf("%s service is not active.", containerdServiceName)
		return false, nil
	}

	// TODO: Add a check for `IsServiceEnabled` if available and deemed necessary for idempotency.
	// For example:
	// enabled, err := ctx.Host.Runner.IsServiceEnabled(ctx.GoContext, containerdServiceName)
	// if err != nil {
	//     return false, fmt.Errorf("failed to check if %s service is enabled: %w", containerdServiceName, err)
	// }
	// if !enabled {
	//     hostCtxLogger.Debugf("%s service is active but not enabled.", containerdServiceName)
	//     return false, nil
	// }

	hostCtxLogger.Infof("%s service is active.", containerdServiceName)
	return true, nil // Active (and assumed enabled for simplicity of this check)
}

// Run enables and starts the containerd service.
func (s *EnableAndStartContainerdStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	// Perform daemon-reload before managing the service. This is often necessary
	// after changing service unit files or configurations (like containerd's config.toml).
	hostCtxLogger.Infof("Performing daemon-reload before managing %s service (best effort).", containerdServiceName)
	if err := ctx.Host.Runner.DaemonReload(ctx.GoContext); err != nil {
	    // Log as warning, as not all init systems support it or might not be strictly needed every time.
	    hostCtxLogger.Warnf("daemon-reload reported an error on host %s (may be non-critical): %v", ctx.Host.Name, err)
	}


	hostCtxLogger.Infof("Enabling %s service...", containerdServiceName)
	if err := ctx.Host.Runner.EnableService(ctx.GoContext, containerdServiceName); err != nil {
		res.Error = fmt.Errorf("failed to enable %s service on host %s: %w", containerdServiceName, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Successf("%s service enabled.", containerdServiceName)

	hostCtxLogger.Infof("Starting %s service...", containerdServiceName)
	if err := ctx.Host.Runner.StartService(ctx.GoContext, containerdServiceName); err != nil {
		res.Error = fmt.Errorf("failed to start %s service on host %s: %w", containerdServiceName, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Successf("%s service started.", containerdServiceName)

	// Verify it's active after start attempt
	active, verifyErr := ctx.Host.Runner.IsServiceActive(ctx.GoContext, containerdServiceName)
	if verifyErr != nil {
		// This is a soft failure for the step if start didn't error but verification did.
		// Or, make it a hard failure. For now, log as warning and proceed to success if start was ok.
		res.Message = fmt.Sprintf("%s service enabled and start command issued. Verification of active status failed: %v", containerdServiceName, verifyErr)
		hostCtxLogger.Warnf("Could not verify %s service status after start on host %s: %v. Assuming start was successful if no error from start command.", containerdServiceName, ctx.Host.Name, verifyErr)
		// Do not mark as failed just for this, if StartService itself didn't error.
	} else if !active {
		res.Error = fmt.Errorf("%s service started but reported as not active immediately after on host %s", containerdServiceName, ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}


	res.EndTime = time.Now()
	if res.Status == "" { // If not set to Failed by any error path above
		res.Status = "Succeeded"
	}
	if res.Message == "" { // If no warning/error message populated
		res.Message = fmt.Sprintf("%s service enabled and started successfully on host %s.", containerdServiceName, ctx.Host.Name)
	}
	hostCtxLogger.Successf("Step finished with status %s: %s", res.Status, res.Message)
	return res
}

var _ step.Step = &EnableAndStartContainerdStep{}
