package multus

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/pkg/errors"
)

type CleanMultusStep struct {
	step.Base
	ReleaseName         string
	Namespace           string
	AdminKubeconfigPath string
}

type CleanMultusStepBuilder struct {
	step.Builder[CleanMultusStepBuilder, *CleanMultusStep]
}

func NewCleanMultusStepBuilder(ctx runtime.Context, instanceName string) *CleanMultusStepBuilder {
	cfg := ctx.GetClusterConfig()
	if cfg.Spec.Network.Multus == nil {
		return nil
	}

	helmProvider := helm.NewHelmProvider(&ctx)
	multusChart := helmProvider.GetChart(string(common.CNITypeMultus))
	if multusChart == nil {
		return nil
	}

	s := &CleanMultusStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Multus CNI Helm release", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	s.ReleaseName = multusChart.ChartName()
	s.Namespace = "kube-system"

	if cfg.Spec.Network.Multus.Installation != nil && cfg.Spec.Network.Multus.Installation.Namespace != "" {
		s.Namespace = cfg.Spec.Network.Multus.Installation.Namespace
	}
	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	b := new(CleanMultusStepBuilder).Init(s)
	return b
}

func (s *CleanMultusStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanMultusStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := runner.LookPath(ctx.GoContext(), conn, "helm"); err != nil {
		logger.Warnf("helm command not found on the target host. Assuming cleanup is not possible or done. %v", err)
		return true, nil
	}
	exists, err := runner.Exists(ctx.GoContext(), conn, s.AdminKubeconfigPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for admin.conf: %w", err)
	}
	if !exists {
		logger.Warnf("admin.conf not found at %s. Assuming cleanup is not possible or done.", s.AdminKubeconfigPath)
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
		logger.Infof("Helm release '%s' not found. Cleanup step is considered complete.", s.ReleaseName)
		return true, nil
	}

	logger.Infof("Helm release '%s' found. Cleanup is required.", s.ReleaseName)
	return false, nil
}

func (s *CleanMultusStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Executing remote Helm uninstall command: %s", cmd)
	output, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		logger.Errorf("Failed to uninstall Multus CNI Helm chart: %v\nOutput: %s", err, output)
		return errors.Wrapf(err, "helm uninstall failed for release %s", s.ReleaseName)
	}

	logger.Info("Multus CNI Helm chart uninstalled successfully.")
	logger.Debugf("Helm uninstall command output:\n%s", output)
	return nil
}

func (s *CleanMultusStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanMultusStep)(nil)
