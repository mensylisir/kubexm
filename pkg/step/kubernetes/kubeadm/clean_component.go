package kubeadm

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanKubeComponentsStep struct {
	step.Base
	InstallPath string
}

type CleanKubeComponentsStepBuilder struct {
	step.Builder[CleanKubeComponentsStepBuilder, *CleanKubeComponentsStep]
}

func NewCleanKubeComponentsStepBuilder(ctx runtime.Context, instanceName string) *CleanKubeComponentsStepBuilder {
	s := &CleanKubeComponentsStep{
		InstallPath: common.DefaultBinDir,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Clean kubelet, kubectl, kubeadm and related service files", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CleanKubeComponentsStepBuilder).Init(s)
	return b
}

func (s *CleanKubeComponentsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanKubeComponentsStep) filesAndDirsToRemove() []string {
	return []string{
		filepath.Join(s.InstallPath, "kubelet"),
		filepath.Join(s.InstallPath, "kubectl"),
		filepath.Join(s.InstallPath, "kubeadm"),

		common.DefaultKubeletServiceFile,
		common.KubeletSystemdDropinDirTarget,
	}
}

func (s *CleanKubeComponentsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	logger.Info("Stopping and disabling kubelet service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, "kubelet.service"); err != nil {
		logger.Warnf("Failed to stop kubelet service (it might not exist): %v", err)
	}
	if err := runner.DisableService(ctx.GoContext(), conn, facts, "kubelet.service"); err != nil {
		logger.Warnf("Failed to disable kubelet service (it might not exist): %v", err)
	}

	paths := s.filesAndDirsToRemove()
	for _, path := range paths {
		logger.Warnf("Removing path: %s", path)
		if err := runner.Remove(ctx.GoContext(), conn, path, s.Sudo, true); err != nil {
			if !strings.Contains(err.Error(), "no such file or directory") {
				logger.Errorf("Failed to remove '%s', manual cleanup may be required. Error: %v", path, err)
			}
		}
	}

	logger.Info("Reloading systemd daemon after cleanup...")
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		logger.Errorf("Failed to reload systemd daemon during cleanup: %v", err)
	}

	return nil
}

func (s *CleanKubeComponentsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get current host connector: %v", err)
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts to check service status: %v", err)
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	allClean := true
	for _, path := range s.filesAndDirsToRemove() {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			logger.Warnf("Failed to check existence of '%s': %v", path, err)
			return false, err
		}
		if exists {
			allClean = false
			break
		}
	}
	if !allClean {
		logger.Warn("Some kube components are still present. Step needs to run.")
		return false, nil
	}

	isActive, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, "kubelet.service")
	if err != nil {
		return true, nil
	}
	if isActive {
		return false, nil
	}
	return true, nil
}

func (s *CleanKubeComponentsStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CleanKubeComponentsStep)(nil)
