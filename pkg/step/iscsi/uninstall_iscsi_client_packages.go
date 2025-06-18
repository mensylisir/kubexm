package iscsi

import (
	"fmt"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// UninstallISCSIClientPackagesStepSpec defines the specification for uninstalling iSCSI client packages.
type UninstallISCSIClientPackagesStepSpec struct{}

// GetName returns the name of the step.
func (s *UninstallISCSIClientPackagesStepSpec) GetName() string {
	return "Uninstall iSCSI Client Packages"
}

// UninstallISCSIClientPackagesStepExecutor implements the logic for uninstalling iSCSI client packages.
type UninstallISCSIClientPackagesStepExecutor struct{}

// Check determines if the iSCSI client packages are already uninstalled.
func (e *UninstallISCSIClientPackagesStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	// _, ok := s.(*UninstallISCSIClientPackagesStepSpec)
	// if !ok {
	// 	return false, fmt.Errorf("unexpected spec type %T for UninstallISCSIClientPackagesStepExecutor Check method", s)
	// }

	if ctx.Host.Runner == nil || ctx.Host.Runner.Facts == nil || ctx.Host.Runner.Facts.OS == nil {
		return false, fmt.Errorf("OS facts not available for host %s", ctx.Host.Name)
	}
	osID := ctx.Host.Runner.Facts.OS.ID

	pkgNames, _, err := DetermineISCSIConfig(osID)
	if err != nil {
		// If OS is not supported, we can't determine packages, so it's effectively "done" for this host from this step's perspective.
		// Or, one might choose to error out if strict adherence to supported OS is required for uninstallation too.
		// For now, let's assume if we can't determine packages, we can't check if they are installed, so log and return true.
		ctx.Logger.Warnf("Cannot determine iSCSI packages for OS %s on host %s (may be unsupported): %v. Assuming packages are not installed or managed by this step.", osID, ctx.Host.Name, err)
		return true, nil
	}

	if len(pkgNames) == 0 {
		ctx.Logger.Infof("No iSCSI packages defined for OS %s on host %s. Assuming uninstalled.", osID, ctx.Host.Name)
		return true, nil
	}

	for _, pkgName := range pkgNames {
		installed, err := ctx.Host.Runner.IsPackageInstalled(ctx.GoContext, pkgName)
		if err != nil {
			return false, fmt.Errorf("failed to check if package %s is installed on host %s: %w", pkgName, ctx.Host.Name, err)
		}
		if installed {
			ctx.Logger.Infof("Package %s is still installed on host %s. Uninstallation needed.", pkgName, ctx.Host.Name)
			return false, nil // Not done if any package is still installed
		}
	}

	ctx.Logger.Infof("All determined iSCSI client packages (%v) are not installed on host %s.", pkgNames, ctx.Host.Name)
	return true, nil
}

// Execute uninstalls the iSCSI client packages.
func (e *UninstallISCSIClientPackagesStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	// specAsserted, ok := s.(*UninstallISCSIClientPackagesStepSpec)
	// if !ok {
	// 	myErr := fmt.Errorf("Execute: unexpected spec type %T for UninstallISCSIClientPackagesStepExecutor", s)
	// 	stepName := "UninstallISCSIClientPackages (type error)"; if s != nil { stepName = s.GetName() }
	// 	return step.NewResult(stepName, ctx.Host.Name, time.Now(), myErr)
	// }
	stepName := s.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	if ctx.Host.Runner == nil || ctx.Host.Runner.Facts == nil || ctx.Host.Runner.Facts.OS == nil {
		res.Error = fmt.Errorf("OS facts not available for host %s", ctx.Host.Name)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	osID := ctx.Host.Runner.Facts.OS.ID

	pkgNames, _, err := DetermineISCSIConfig(osID)
	if err != nil {
		res.Error = fmt.Errorf("failed to determine iSCSI packages for OS %s: %w. Cannot proceed with uninstallation.", osID, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	if len(pkgNames) == 0 {
		hostCtxLogger.Infof("No iSCSI packages defined for OS %s. Nothing to uninstall.", osID)
		res.SetSucceeded("No iSCSI packages to uninstall for this OS.")
		return res
	}

	hostCtxLogger.Infof("Attempting to uninstall packages %v on host %s...", pkgNames, ctx.Host.Name)
	// Some package managers require packages to be explicitly installed before attempting removal,
	// but RemovePackages should ideally handle "not installed" gracefully.
	// If not, a pre-check with IsPackageInstalled might be needed for each before adding to a list for removal.
	// For now, assume RemovePackages handles this.
	if err := ctx.Host.Runner.RemovePackages(ctx.GoContext, pkgNames...); err != nil {
		res.Error = fmt.Errorf("failed to remove packages %v on host %s: %w", pkgNames, ctx.Host.Name, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Infof("Packages %v removed successfully (or were already removed) on host %s.", pkgNames, ctx.Host.Name)

	// Optional: Clean up SharedData. This is good practice.
	// ctx.SharedData.Delete(ISCSIClientPackageNamesKey)
	// ctx.SharedData.Delete(ISCSIClientServiceNameKey)
	// hostCtxLogger.Infof("Removed iSCSI keys from SharedData.")

	res.SetSucceeded("iSCSI client packages uninstalled successfully.")
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&UninstallISCSIClientPackagesStepSpec{}), &UninstallISCSIClientPackagesStepExecutor{})
}
