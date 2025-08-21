package argocd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
)

type CleanArgoCDStep struct {
	step.Base
	ReleaseName         string
	Namespace           string
	AdminKubeconfigPath string
}

type CleanArgoCDStepBuilder struct {
	step.Builder[CleanArgoCDStepBuilder, *CleanArgoCDStep]
}

func NewCleanArgoCDStepBuilder(ctx runtime.Context, instanceName string) *CleanArgoCDStepBuilder {
	helmProvider := helm.NewHelmProvider(&ctx)
	chart := helmProvider.GetChart("argocd")
	if chart == nil {
		return nil // 如果 BOM 中没有 argocd 的信息，则无法进行清理
	}

	s := &CleanArgoCDStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Argo CD Helm release", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	s.ReleaseName = "argocd"
	s.Namespace = "argocd-system"
	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	b := new(CleanArgoCDStepBuilder).Init(s)
	return b
}

func (s *CleanArgoCDStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanArgoCDStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		logger.Warn(err, "helm command not found on the target host. Assuming cleanup is not possible or done.")
		return true, nil
	}
	exists, err := runner.Exists(ctx.GoContext(), conn, s.AdminKubeconfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for admin.conf: %w", err)
	}
	if !exists {
		logger.Warn("admin.conf not found. Assuming cleanup is not possible or done.", "path", s.AdminKubeconfigPath)
		return true, nil
	}

	statusCmd := fmt.Sprintf(
		"helm status %s --namespace %s --kubeconfig %s -o json",
		s.ReleaseName,
		s.Namespace,
		s.AdminKubeconfigPath,
	)

	_, err = runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
	if err != nil {
		logger.Info("Helm release not found. Cleanup step is considered complete.", "release", s.ReleaseName)
		return true, nil
	}

	logger.Info("Helm release found. Cleanup is required.", "release", s.ReleaseName)
	return false, nil
}

func (s *CleanArgoCDStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(
		"helm uninstall %s --namespace %s --kubeconfig %s --wait",
		s.ReleaseName,
		s.Namespace,
		s.AdminKubeconfigPath,
	)

	logger.Info("Executing remote Helm uninstall command.", "command", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		logger.Error(err, "Failed to uninstall Argo CD Helm chart.", "output", output)
	}

	logger.Info("Argo CD Helm chart uninstalled successfully.")
	return nil
}

func (s *CleanArgoCDStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step. No action taken.")
	return nil
}

var _ step.Step = (*CleanArgoCDStep)(nil)
