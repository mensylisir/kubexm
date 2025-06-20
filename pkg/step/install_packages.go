package step

import (
	// Ensure GoContext is part of StepContext or accessible through it for runner calls.
	// Assuming runtime.StepContext provides GoContext() for the runner methods.
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

type InstallPackagesStep struct {
	Packages []string
}

func NewInstallPackagesStep(packages ...string) Step {
	return &InstallPackagesStep{Packages: packages}
}

func (s *InstallPackagesStep) Name() string { return "InstallPackages" }
func (s *InstallPackagesStep) Description() string { return fmt.Sprintf("Install packages: %v", s.Packages) }
func (s *InstallPackagesStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// For this example, rollback might try to remove packages.
	// However, a simple no-op is also common if uninstalling is complex or risky.
	// ctx.GetLogger().Infof("Rollback for InstallPackages on host %s: removing packages %v (if implemented)", host.GetName(), s.Packages)
	// runnerSvc := ctx.GetRunner()
	// conn, err := ctx.GetConnectorForHost(host)
	// if err != nil { return err }
	// facts, err := ctx.GetHostFacts(host)
	// if err != nil { return err }
	// return runnerSvc.RemovePackages(ctx.GoContext(), conn, facts, s.Packages...)
	return nil // Example: No-op rollback
}

func (s *InstallPackagesStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
    runnerSvc := ctx.GetRunner()
    conn, err := ctx.GetConnectorForHost(host)
    if err != nil { return false, err }
    facts, err := ctx.GetHostFacts(host)
    if err != nil { return false, err }

    for _, pkg := range s.Packages {
        // Assuming StepContext provides a Go context for runner methods
        installed, err := runnerSvc.IsPackageInstalled(ctx.GoContext(), conn, facts, pkg)
        if err != nil { return false, err }
        if !installed {
            ctx.GetLogger().Infof("Precheck for InstallPackages on host %s: package %s is NOT installed.", host.GetName(), pkg)
            return false, nil
        }
        ctx.GetLogger().Infof("Precheck for InstallPackages on host %s: package %s IS already installed.", host.GetName(), pkg)
    }
    ctx.GetLogger().Infof("Precheck for InstallPackages on host %s: All packages %v are already installed.", host.GetName(), s.Packages)
    return true, nil
}

func (s *InstallPackagesStep) Run(ctx runtime.StepContext, host connector.Host) error {
    runnerSvc := ctx.GetRunner()
    conn, err := ctx.GetConnectorForHost(host)
    if err != nil { return err }
    facts, err := ctx.GetHostFacts(host)
    if err != nil { return err }

    ctx.GetLogger().Infof("Running InstallPackages on host %s for packages: %v", host.GetName(), s.Packages)
    // Assuming StepContext provides a Go context for runner methods
    return runnerSvc.InstallPackages(ctx.GoContext(), conn, facts, s.Packages...)
}
