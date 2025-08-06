package packages

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallOfflinePackagesStep struct {
	step.Base
	LocalPackagesPath string
}
type InstallOfflinePackagesStepBuilder struct {
	step.Builder[InstallOfflinePackagesStepBuilder, *InstallOfflinePackagesStep]
}

func NewInstallOfflinePackagesStepBuilder(ctx runtime.Context, instanceName string) *InstallOfflinePackagesStepBuilder {
	s := &InstallOfflinePackagesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Distribute and install offline packages", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 20 * time.Minute
	s.LocalPackagesPath = strings.TrimSuffix(filepath.Join(ctx.GetUploadDir(), "packages.tar.gz"), ".tar.gz")
	b := new(InstallOfflinePackagesStepBuilder).Init(s)
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
			case string(common.ExternalLBTypeKubexmKH):
				packages = append(packages, "keepalived", "haproxy")
			case string(common.ExternalLBTypeKubexmKN):
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

	var checkCmd string
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, err
	}

	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		checkCmd = "dpkg -s %s"
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		checkCmd = "rpm -q %s"
	default:
		logger.Warnf("Unknown package manager, cannot precheck. Assuming installation is needed.")
		return false, nil
	}

	for _, pkg := range packages {
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf(checkCmd, pkg), false); err != nil {
			logger.Infof("Key package '%s' not found. Offline installation is required.", pkg)
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

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.LocalPackagesPath)
	if err != nil || !exists {
		return fmt.Errorf("failed to check remote directory: %w", err)
	}

	if facts.PackageManager == nil || facts.PackageManager.Type == runner.PackageManagerUnknown {
		return fmt.Errorf("cannot perform offline install without a known package manager")
	}
	var installCmd string
	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		// dpkg -i 安装 .deb 文件。-R 表示递归处理目录。
		// apt-get -f install -y 会自动下载并安装缺失的依赖（如果有的话），在纯离线环境，
		// 如果我们的包不全，这里会失败。一个更安全的命令是 `dpkg -i ... && apt-get install -fy --no-download`
		// 但假设我们的包是完整的。
		// `|| true` 确保即使有些包已安装导致 dpkg 报错，也不会中断流程。
		installCmd = fmt.Sprintf("apt install -y %s/*/*/*.deb", s.LocalPackagesPath)

	case runner.PackageManagerYum, runner.PackageManagerDnf:
		installCmd = fmt.Sprintf("%s install -y %s/*/*/*.rpm", facts.PackageManager.Type, s.LocalPackagesPath)

	default:
		return fmt.Errorf("unsupported package manager for offline installation: %s", facts.PackageManager.Type)
	}

	logger.Infof("Executing offline installation command: %s", installCmd)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, installCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to install offline packages: %w", err)
	}

	//logger.Info("Cleaning up temporary package files...")
	//if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("rm -rf %s", s.LocalPackagesPath), s.Sudo); err != nil {
	//	logger.Warnf("Failed to clean up temporary directory %s: %v", s.LocalPackagesPath, err)
	//}

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

	logger.Warnf("Rolling back by removing temporary directory: %s", s.LocalPackagesPath)
	if _, err := runnerSvc.Run(ctx.GoContext(), conn, fmt.Sprintf("rm -rf %s", s.LocalPackagesPath), s.Sudo); err != nil {
		logger.Errorf("Failed to remove temporary directory '%s' during rollback: %v", s.LocalPackagesPath, err)
	}

	return nil
}

var _ step.Step = (*InstallOfflinePackagesStep)(nil)
