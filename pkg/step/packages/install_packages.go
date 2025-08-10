package packages

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallPackagesStep struct {
	step.Base
}
type InstallPackagesStepBuilder struct {
	step.Builder[InstallPackagesStepBuilder, *InstallPackagesStep]
}

func NewInstallPackagesStepBuilder(ctx runtime.Context, instanceName string) *InstallPackagesStepBuilder {
	s := &InstallPackagesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install base and conditional packages", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute
	b := new(InstallPackagesStepBuilder).Init(s)
	return b
}
func (s *InstallPackagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallPackagesStep) getRequiredPackages(ctx runtime.ExecutionContext) ([]string, error) {
	cluster := ctx.GetClusterConfig()
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return nil, fmt.Errorf("failed to get host facts: %w", err)
	}

	packages := []string{
		"socat",
		"conntrack",
		"ipset",
		"ebtables",
		"chrony",
		"ipvsadm",
	}

	switch ctx.GetClusterConfig().Spec.Kubernetes.ContainerRuntime.Type {
	case common.RuntimeTypeCRIO:
		packages = append(packages, "crio")
	case common.RuntimeTypeIsula:
		packages = append(packages, "isula")
	}

	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		packages = append(packages, "open-iscsi")
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		packages = append(packages, "iscsi-initiator-utils")
	default:
		ctx.GetLogger().Warnf("iSCSI package name unknown for package manager '%s', skipping installation.", facts.PackageManager.Type)
	}

	haSpec := cluster.Spec.ControlPlaneEndpoint.HighAvailability
	if haSpec != nil && haSpec.Enabled != nil && *haSpec.Enabled &&
		haSpec.External != nil && haSpec.External.Enabled != nil && *haSpec.External.Enabled {

		isLBNode := false
		for _, role := range ctx.GetHost().GetRoles() {
			if role == common.RoleLoadBalancer {
				isLBNode = true
				break
			}
		}

		if isLBNode {
			switch haSpec.External.Type {
			case "kubexm-kh":
				packages = append(packages, "keepalived", "haproxy")
			case "kubexm-xn":
				packages = append(packages, "keepalived", "nginx")
			}
		}
	}

	systemSpec := cluster.Spec.System
	if systemSpec != nil {
		logger := ctx.GetLogger()
		switch facts.PackageManager.Type {
		case runner.PackageManagerApt:
			if len(systemSpec.Debs) > 0 {
				logger.Infof("Adding custom DEB packages from spec: %v", systemSpec.Debs)
				packages = append(packages, systemSpec.Debs...)
			}

		case runner.PackageManagerYum, runner.PackageManagerDnf:
			if len(systemSpec.RPMs) > 0 {
				logger.Infof("Adding custom RPM packages from spec: %v", systemSpec.RPMs)
				packages = append(packages, systemSpec.RPMs...)
			}
		}
	}

	return packages, nil
}

func (s *InstallPackagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	packages, err := s.getRequiredPackages(ctx)
	if err != nil {
		return false, err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, err
	}
	if facts.PackageManager == nil || facts.PackageManager.PkgQueryCmd == "" {
		return false, fmt.Errorf("package manager or query command not detected")
	}

	for _, pkg := range packages {
		queryCmd := fmt.Sprintf(facts.PackageManager.PkgQueryCmd, pkg)
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, queryCmd, false); err != nil {
			logger.Infof("Required package '%s' is not installed. Step needs to run.", pkg)
			return false, nil
		}
	}

	logger.Info("All required packages are already installed.")
	return true, nil
}

func (s *InstallPackagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	packagesToInstall, err := s.getRequiredPackages(ctx)
	if err != nil {
		return err
	}

	if len(packagesToInstall) == 0 {
		logger.Info("No packages required for installation on this host.")
		return nil
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return err
	}
	pkgManager := facts.PackageManager

	var missingPackages []string
	for _, pkg := range packagesToInstall {
		queryCmd := fmt.Sprintf(pkgManager.PkgQueryCmd, pkg)
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, queryCmd, false); err != nil {
			missingPackages = append(missingPackages, pkg)
		}
	}

	if len(missingPackages) == 0 {
		logger.Info("All required packages were already installed. Nothing to do.")
		return nil
	}

	logger.Infof("Packages to be installed: %s", strings.Join(missingPackages, ", "))

	installCmd := fmt.Sprintf(pkgManager.InstallCmd, strings.Join(missingPackages, " "))

	if pkgManager.UpdateCmd != "" {
		logger.Infof("Executing package manager update command: %s", pkgManager.UpdateCmd)
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, pkgManager.UpdateCmd, s.Sudo); err != nil {
			return fmt.Errorf("package manager update command failed: %w", err)
		}
	}

	logger.Infof("Executing installation command: %s", installCmd)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	logger.Info("All required packages have been installed successfully.")
	return nil
}

func (s *InstallPackagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for InstallPackagesStep is a no-op.")
	return nil
}

var _ step.Step = (*InstallPackagesStep)(nil)
