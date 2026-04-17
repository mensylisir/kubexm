package hybridnet

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/util/helm"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanHybridnetStep struct {
	step.Base
	ReleaseName         string
	Namespace           string
	AdminKubeconfigPath string
}

type CleanHybridnetStepBuilder struct {
	step.Builder[CleanHybridnetStepBuilder, *CleanHybridnetStep]
}

func NewCleanHybridnetStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanHybridnetStepBuilder {
	helmProvider := helm.NewHelmProvider(ctx)
	hybridnetChart := helmProvider.GetChart(string(common.CNITypeHybridnet))
	if hybridnetChart == nil {
		return nil
	}

	s := &CleanHybridnetStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Hybridnet Helm release", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	s.ReleaseName = hybridnetChart.ChartName()
	s.Namespace = "kube-system"
	s.AdminKubeconfigPath = filepath.Join(common.KubernetesConfigDir, common.AdminKubeconfigFileName)

	b := new(CleanHybridnetStepBuilder).Init(s)
	return b
}

func (s *CleanHybridnetStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanHybridnetStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	runResult, err := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
	if err != nil {
		logger.Infof("Helm release '%s' not found. Cleanup step is considered complete.", s.ReleaseName)
		return true, nil
	}

	var status helmStatusOutput
	if jsonErr := json.Unmarshal([]byte(runResult.Stdout), &status); jsonErr != nil {
		logger.Warnf("Found helm release '%s', but failed to parse its status. Proceeding with uninstall. %v", s.ReleaseName, jsonErr)
		return false, nil
	}

	logger.Infof("Helm release '%s' found in status '%s'. Cleanup is required.", s.ReleaseName, status.Info.Status)
	return false, nil
}

func (s *CleanHybridnetStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf(
		"helm uninstall %s --namespace %s --kubeconfig %s --wait",
		s.ReleaseName,
		s.Namespace,
		s.AdminKubeconfigPath,
	)

	logger.Infof("Executing remote Helm uninstall command: %s", cmd)
	runResult, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
	if err != nil {
		logger.Errorf("Failed to uninstall Hybridnet Helm chart: %v\nOutput: %s", err, runResult.Stdout)
		result.MarkFailed(err, fmt.Sprintf("helm uninstall failed for release %s", s.ReleaseName))
		return result, err
	}

	logger.Info("Hybridnet Helm chart uninstalled successfully.")
	logger.Debugf("Helm uninstall command output:\n%s", runResult.Stdout)
	result.MarkCompleted("Hybridnet Helm chart uninstalled successfully")
	return result, nil
}

func (s *CleanHybridnetStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanHybridnetStep)(nil)
