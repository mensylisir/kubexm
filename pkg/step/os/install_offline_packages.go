package os

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallOfflinePackagesStep struct {
	step.Base
	LocalPackagesDir string
}
type InstallOfflinePackagesStepBuilder struct {
	step.Builder[InstallOfflinePackagesStepBuilder, *InstallOfflinePackagesStep]
	localPackagesDir string
}

func NewInstallOfflinePackagesStepBuilder(ctx runtime.Context, instanceName string) *InstallOfflinePackagesStepBuilder {
	s := &InstallOfflinePackagesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute and install offline packages", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute

	b := new(InstallOfflinePackagesStepBuilder).Init(s)
	b.localPackagesDir = "packages"
	return b
}

func (b *InstallOfflinePackagesStepBuilder) WithLocalPackagesDir(dir string) *InstallOfflinePackagesStepBuilder {
	b.Step.LocalPackagesDir = dir
	return b
}

func (s *InstallOfflinePackagesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallOfflinePackagesStep) getRequiredPackages(ctx runtime.ExecutionContext) ([]string, error) {
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

func (s *InstallOfflinePackagesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	for _, pkg := range packages {
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("which %s", pkg), false); err != nil {
			logger.Infof("Key package binary '%s' not found. Offline installation is required.", pkg)
			return false, nil
		}
	}

	logger.Info("All key packages seem to be installed. Skipping offline installation.")
	return true, nil
}

func (s *InstallOfflinePackagesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts to determine offline package: %w", err)
	}

	localTarballName := fmt.Sprintf("packages-%s-%s-%s.tar.gz", facts.OS.ID, facts.OS.VersionID, facts.OS.Arch)
	localPackagePath := filepath.Join(s.LocalPackagesDir, localTarballName)

	// 2.
	remoteTempDir := "/tmp/kubexm_packages_offline"
	remotePackagePath := filepath.Join(remoteTempDir, "packages.tar.gz")

	logger.Infof("Uploading offline package '%s' to host...", localTarballName)

	if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteTempDir, remoteTempDir), s.Sudo); err != nil {
		return fmt.Errorf("failed to prepare remote directory: %w", err)
	}

	if err := runnerSvc.Upload(ctx.GoContext(), conn, localPackagePath, remotePackagePath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload offline package: %w", err)
	}

	logger.Info("Extracting offline packages on host...")
	extractCmd := fmt.Sprintf("tar -zxf %s -C %s", remotePackagePath, remoteTempDir)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, extractCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to extract offline package: %w", err)
	}

	logger.Info("Starting local installation from extracted packages...")

	packageInstallDir := filepath.Join(remoteTempDir, facts.OS.ID, facts.OS.VersionID, facts.OS.Arch)
	var installCmd string

	if facts.PackageManager == nil || facts.PackageManager.Type == runner.PackageManagerUnknown {
		return fmt.Errorf("cannot perform offline install without a known package manager")
	}

	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		// dpkg -i 安装 .deb 文件。-R 表示递归处理目录。
		// apt-get -f install -y 会自动下载并安装缺失的依赖（如果有的话），在纯离线环境，
		// 如果我们的包不全，这里会失败。一个更安全的命令是 `dpkg -i ... && apt-get install -fy --no-download`
		// 但假设我们的包是完整的。
		// `|| true` 确保即使有些包已安装导致 dpkg 报错，也不会中断流程。
		installCmd = fmt.Sprintf("dpkg -R -i %s || apt-get install -fy", packageInstallDir)

	case runner.PackageManagerYum, runner.PackageManagerDnf:
		installCmd = fmt.Sprintf("%s install -y %s/*.rpm", facts.PackageManager.Type, packageInstallDir)

	default:
		return fmt.Errorf("unsupported package manager for offline installation: %s", facts.PackageManager.Type)
	}

	logger.Infof("Executing offline installation command: %s", installCmd)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to install offline packages: %w", err)
	}

	logger.Info("Cleaning up temporary package files...")
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("rm -rf %s", remoteTempDir), s.Sudo); err != nil {
		logger.Warnf("Failed to clean up temporary directory %s: %v", remoteTempDir, err)
	}

	logger.Info("All required packages have been installed successfully from offline source.")
	return nil
}

func (s *InstallOfflinePackagesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remoteTempDir := "/tmp/kubexm_packages_offline"
	logger.Warnf("Rolling back by removing temporary directory: %s", remoteTempDir)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("rm -rf %s", remoteTempDir), s.Sudo); err != nil {
		logger.Errorf("Failed to remove temporary directory '%s' during rollback: %v", remoteTempDir, err)
	}

	return nil
}

var _ step.Step = (*InstallOfflinePackagesStep)(nil)
