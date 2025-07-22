package kubernetes

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

type CleanKubernetesStep struct {
	step.Base
	InstallPath string
	PurgeData   bool
}

type CleanKubernetesStepBuilder struct {
	step.Builder[CleanKubernetesStepBuilder, *CleanKubernetesStep]
}

func NewCleanKubernetesStepBuilder(ctx runtime.Context, instanceName string) *CleanKubernetesStepBuilder {
	s := &CleanKubernetesStep{
		InstallPath: common.DefaultBinDir,
		PurgeData:   false,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup kubernetes and related components", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(CleanKubernetesStepBuilder).Init(s)
	return b
}

func (b *CleanKubernetesStepBuilder) WithInstallPath(installPath string) *CleanKubernetesStepBuilder {
	b.Step.InstallPath = installPath
	return b
}

func (b *CleanKubernetesStepBuilder) WithPurgeData(purge bool) *CleanKubernetesStepBuilder {
	b.Step.PurgeData = purge
	return b
}

func (s *CleanKubernetesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanKubernetesStep) filesAndDirsToRemove() []string {
	paths := []string{
		filepath.Join(s.InstallPath, "kube-apiserver"),
		filepath.Join(s.InstallPath, "kube-controller-manager"),
		filepath.Join(s.InstallPath, "kube-scheduler"),
		filepath.Join(s.InstallPath, "kubelet"),
		filepath.Join(s.InstallPath, "kube-proxy"),
		filepath.Join(s.InstallPath, "kubectl"),
		common.KubeApiServerDefaultSystemdFile,
		common.KubeControllerManagerDefaultSystemdFile,
		common.KubeSchedulerDefaultSystemdFile,
		common.KubeletDefaultSystemdFile,
		common.KubeProxyDefaultSystemdFile,
		common.KubeConfigDir,
	}

	if s.PurgeData {
		paths = append(paths, "/var/lib/kubelet")
	}

	return paths
}

func (s *CleanKubernetesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	paths := s.filesAndDirsToRemove()
	for _, path := range paths {
		exists, err := runner.Exists(ctx.GoContext(), conn, path)
		if err != nil {
			return false, err
		}
		if exists {
			logger.Infof("Path '%s' still exists. Cleanup is required.", path)
			return false, nil
		}
	}

	logger.Info("All kubernetes related files and directories have been removed. Step is done.")
	return true, nil
}

func (s *CleanKubernetesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
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

	logger.Info("Reloading systemd daemon after cleanup")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
			logger.Errorf("Failed to reload systemd daemon during cleanup: %v", err)
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			logger.Errorf("Failed to reload systemd daemon during cleanup: %v", err)
		}
	}

	return nil
}

func (s *CleanKubernetesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}
