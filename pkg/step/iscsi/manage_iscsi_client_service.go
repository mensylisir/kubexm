package iscsi

import (
	"fmt"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// getServiceName retrieves the iSCSI service name.
// It first tries to load it from SharedData, then falls back to DetermineISCSIConfig.
func getServiceName(ctx *runtime.Context) (string, error) {
	if ctx.Host.Runner == nil || ctx.Host.Runner.Facts == nil || ctx.Host.Runner.Facts.OS == nil {
		return "", fmt.Errorf("OS facts not available for host %s", ctx.Host.Name)
	}

	if svcNameVal, found := ctx.SharedData.Load(ISCSIClientServiceNameKey); found {
		if svcName, ok := svcNameVal.(string); ok && svcName != "" {
			ctx.Logger.Debugf("Loaded iSCSI service name '%s' from SharedData for host %s", svcName, ctx.Host.Name)
			return svcName, nil
		}
		ctx.Logger.Warnf("Invalid iSCSI service name found in SharedData for host %s. Attempting to determine from OS facts.", ctx.Host.Name)
	}

	osID := ctx.Host.Runner.Facts.OS.ID
	ctx.Logger.Debugf("Attempting to determine iSCSI service name from OS facts (%s) for host %s", osID, ctx.Host.Name)
	_, svcName, err := DetermineISCSIConfig(osID)
	if err != nil {
		return "", fmt.Errorf("failed to determine iSCSI service name for OS %s on host %s: %w", osID, ctx.Host.Name, err)
	}
	if svcName == "" {
		return "", fmt.Errorf("determined iSCSI service name is empty for OS %s on host %s", osID, ctx.Host.Name)
	}
	// Store it back in SharedData for future use in this task execution
	ctx.SharedData.Store(ISCSIClientServiceNameKey, svcName)
	ctx.Logger.Debugf("Determined iSCSI service name '%s' for OS %s on host %s and stored in SharedData", svcName, osID, ctx.Host.Name)
	return svcName, nil
}

// --- EnableISCSIClientServiceStep ---

// EnableISCSIClientServiceStepSpec defines the specification for enabling the iSCSI client service.
type EnableISCSIClientServiceStepSpec struct{}

// GetName returns the name of the step.
func (s *EnableISCSIClientServiceStepSpec) GetName() string {
	return "Enable iSCSI Client Service"
}

// EnableISCSIClientServiceStepExecutor implements the logic for enabling the iSCSI client service.
type EnableISCSIClientServiceStepExecutor struct{}

// Check determines if the iSCSI client service is already enabled and active.
func (e *EnableISCSIClientServiceStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	svcName, err := getServiceName(ctx)
	if err != nil {
		return false, err // Error determining service name
	}

	// Check if service is enabled (autostart)
	// Note: Runner might not have IsServiceEnabled, so we rely on IsServiceActive as primary check for "done"
	// If IsServiceEnabled were available:
	// enabled, err := ctx.Host.Runner.IsServiceEnabled(ctx.GoContext, svcName)
	// if err != nil {
	//    return false, fmt.Errorf("failed to check if service %s is enabled on host %s: %w", svcName, ctx.Host.Name, err)
	// }
	// if !enabled {
	//    ctx.Logger.Infof("Service %s is not enabled on host %s.", svcName, ctx.Host.Name)
	//    return false, nil
	// }

	active, err := ctx.Host.Runner.IsServiceActive(ctx.GoContext, svcName)
	if err != nil {
		return false, fmt.Errorf("failed to check if service %s is active on host %s: %w", svcName, ctx.Host.Name, err)
	}

	if active {
		ctx.Logger.Infof("Service %s is already active (and assumed enabled) on host %s.", svcName, ctx.Host.Name)
		return true, nil
	}

	ctx.Logger.Infof("Service %s is not active on host %s.", svcName, ctx.Host.Name)
	return false, nil
}

// Execute enables and starts the iSCSI client service.
func (e *EnableISCSIClientServiceStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepName := s.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	svcName, err := getServiceName(ctx)
	if err != nil {
		res.Error = err
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	hostCtxLogger.Infof("Enabling service %s on host %s...", svcName, ctx.Host.Name)
	if err := ctx.Host.Runner.EnableService(ctx.GoContext, svcName); err != nil {
		res.Error = fmt.Errorf("failed to enable service %s on host %s: %w", svcName, ctx.Host.Name, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Infof("Service %s enabled successfully on host %s.", svcName, ctx.Host.Name)

	hostCtxLogger.Infof("Starting service %s on host %s...", svcName, ctx.Host.Name)
	if err := ctx.Host.Runner.StartService(ctx.GoContext, svcName); err != nil {
		res.Error = fmt.Errorf("failed to start service %s on host %s: %w", svcName, ctx.Host.Name, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Infof("Service %s started successfully on host %s.", svcName, ctx.Host.Name)

	res.SetSucceeded("iSCSI client service enabled and started successfully.")
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

// --- DisableISCSIClientServiceStep ---

// DisableISCSIClientServiceStepSpec defines the specification for disabling the iSCSI client service.
type DisableISCSIClientServiceStepSpec struct{}

// GetName returns the name of the step.
func (s *DisableISCSIClientServiceStepSpec) GetName() string {
	return "Disable iSCSI Client Service"
}

// DisableISCSIClientServiceStepExecutor implements the logic for disabling the iSCSI client service.
type DisableISCSIClientServiceStepExecutor struct{}

// Check determines if the iSCSI client service is already disabled (i.e., not active).
func (e *DisableISCSIClientServiceStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	svcName, err := getServiceName(ctx)
	if err != nil {
		// If service name cannot be determined (e.g. unsupported OS), consider it "done" for disable.
		ctx.Logger.Warnf("Cannot determine iSCSI service name for host %s (may be unsupported): %v. Assuming service is not managed or not active.", ctx.Host.Name, err)
		return true, nil
	}

	active, err := ctx.Host.Runner.IsServiceActive(ctx.GoContext, svcName)
	if err != nil {
		// If error checking status, assume it's not in a desired state.
		return false, fmt.Errorf("failed to check if service %s is active on host %s: %w", svcName, ctx.Host.Name, err)
	}

	if !active {
		ctx.Logger.Infof("Service %s is already inactive on host %s.", svcName, ctx.Host.Name)
		// Optionally, also check if service is disabled (if runner supports IsServiceEnabled)
		// For now, not active is sufficient for "done".
		return true, nil
	}

	ctx.Logger.Infof("Service %s is active on host %s. Needs to be stopped and disabled.", svcName, ctx.Host.Name)
	return false, nil
}

// Execute stops and disables the iSCSI client service.
func (e *DisableISCSIClientServiceStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepName := s.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	svcName, err := getServiceName(ctx)
	if err != nil {
		res.Error = err
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	hostCtxLogger.Infof("Stopping service %s on host %s...", svcName, ctx.Host.Name)
	if err := ctx.Host.Runner.StopService(ctx.GoContext, svcName); err != nil {
		// Log error but attempt to disable anyway, as service might already be stopped.
		hostCtxLogger.Warnf("Failed to stop service %s on host %s (may already be stopped): %v", svcName, ctx.Host.Name, err)
	} else {
		hostCtxLogger.Infof("Service %s stopped successfully on host %s.", svcName, ctx.Host.Name)
	}

	hostCtxLogger.Infof("Disabling service %s on host %s...", svcName, ctx.Host.Name)
	if err := ctx.Host.Runner.DisableService(ctx.GoContext, svcName); err != nil {
		res.Error = fmt.Errorf("failed to disable service %s on host %s: %w", svcName, ctx.Host.Name, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Infof("Service %s disabled successfully on host %s.", svcName, ctx.Host.Name)

	res.SetSucceeded("iSCSI client service stopped and disabled successfully.")
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&EnableISCSIClientServiceStepSpec{}), &EnableISCSIClientServiceStepExecutor{})
	step.Register(step.GetSpecTypeName(&DisableISCSIClientServiceStepSpec{}), &DisableISCSIClientServiceStepExecutor{})
}
