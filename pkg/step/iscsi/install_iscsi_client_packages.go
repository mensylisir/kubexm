package iscsi

import (
	"fmt"
	"time"

	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

const (
	// ISCSIClientPackageNamesKey is the key for iSCSI client package names in SharedData.
	ISCSIClientPackageNamesKey = "ISCSIClientPackageNames"
	// ISCSIClientServiceNameKey is the key for iSCSI client service name in SharedData.
	ISCSIClientServiceNameKey = "ISCSIClientServiceName"
)

// InstallISCSIClientPackagesStepSpec defines the specification for installing iSCSI client packages.
type InstallISCSIClientPackagesStepSpec struct{}

// GetName returns the name of the step.
func (s *InstallISCSIClientPackagesStepSpec) GetName() string {
	return "Install iSCSI Client Packages"
}

// InstallISCSIClientPackagesStepExecutor implements the logic for installing iSCSI client packages.
type InstallISCSIClientPackagesStepExecutor struct{}

// Check determines if the iSCSI client packages are already installed.
func (e *InstallISCSIClientPackagesStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	// _, ok := s.(*InstallISCSIClientPackagesStepSpec)
	// if !ok {
	// 	return false, fmt.Errorf("unexpected spec type %T for InstallISCSIClientPackagesStepExecutor Check method", s)
	// }

	if ctx.Host.Runner == nil || ctx.Host.Runner.Facts == nil || ctx.Host.Runner.Facts.OS == nil {
		return false, fmt.Errorf("OS facts not available for host %s", ctx.Host.Name)
	}
	osID := ctx.Host.Runner.Facts.OS.ID

	pkgNames, _, err := DetermineISCSIConfig(osID)
	if err != nil {
		return false, fmt.Errorf("failed to determine iSCSI packages for OS %s on host %s: %w", osID, ctx.Host.Name, err)
	}

	if len(pkgNames) == 0 {
		return false, fmt.Errorf("no iSCSI packages defined for OS %s on host %s", osID, ctx.Host.Name)
	}

	for _, pkgName := range pkgNames {
		installed, err := ctx.Host.Runner.IsPackageInstalled(ctx.GoContext, pkgName)
		if err != nil {
			return false, fmt.Errorf("failed to check if package %s is installed on host %s: %w", pkgName, ctx.Host.Name, err)
		}
		if !installed {
			ctx.Logger.Infof("Package %s is not installed on host %s.", pkgName, ctx.Host.Name)
			return false, nil // Not done if any package is missing
		}
	}

	ctx.Logger.Infof("All required iSCSI client packages (%v) are already installed on host %s.", pkgNames, ctx.Host.Name)
	// Store in SharedData even in check if found, so subsequent steps can rely on it if install is skipped
	_, svcName, _ := DetermineISCSIConfig(osID) // error already checked
	ctx.SharedData.Store(ISCSIClientPackageNamesKey, pkgNames)
	ctx.SharedData.Store(ISCSIClientServiceNameKey, svcName)
	return true, nil
}

// Execute installs the iSCSI client packages.
func (e *InstallISCSIClientPackagesStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	// specAsserted, ok := s.(*InstallISCSIClientPackagesStepSpec)
	// if !ok {
	// 	myErr := fmt.Errorf("Execute: unexpected spec type %T for InstallISCSIClientPackagesStepExecutor", s)
	// 	stepName := "InstallISCSIClientPackages (type error)"; if s != nil { stepName = s.GetName() }
	// 	return step.NewResult(stepName, ctx.Host.Name, time.Now(), myErr)
	// }
	stepName := s.GetName() // Use GetName() from the spec interface
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

	pkgNames, svcName, err := DetermineISCSIConfig(osID)
	if err != nil {
		res.Error = fmt.Errorf("failed to determine iSCSI packages for OS %s: %w", osID, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	if len(pkgNames) == 0 {
		res.Error = fmt.Errorf("no iSCSI packages defined for OS %s", osID)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	hostCtxLogger.Infof("Identified iSCSI packages for %s: %v, service name: %s", osID, pkgNames, svcName)

	hostCtxLogger.Infof("Updating package cache on host %s...", ctx.Host.Name)
	if err := ctx.Host.Runner.UpdatePackageCache(ctx.GoContext); err != nil {
		res.Error = fmt.Errorf("failed to update package cache on host %s: %w", ctx.Host.Name, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Infof("Package cache updated successfully on host %s.", ctx.Host.Name)

	hostCtxLogger.Infof("Installing packages %v on host %s...", pkgNames, ctx.Host.Name)
	if err := ctx.Host.Runner.InstallPackages(ctx.GoContext, pkgNames...); err != nil {
		res.Error = fmt.Errorf("failed to install packages %v on host %s: %w", pkgNames, ctx.Host.Name, err)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	hostCtxLogger.Infof("Packages %v installed successfully on host %s.", pkgNames, ctx.Host.Name)

	ctx.SharedData.Store(ISCSIClientPackageNamesKey, pkgNames)
	ctx.SharedData.Store(ISCSIClientServiceNameKey, svcName)
	hostCtxLogger.Infof("Stored iSCSI package names and service name in SharedData.")

	res.SetSucceeded("iSCSI client packages installed successfully.")
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&InstallISCSIClientPackagesStepSpec{}), &InstallISCSIClientPackagesStepExecutor{})
}
