package calico

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanCalicoStep struct {
	step.Base
	ReleaseName string
	Namespace   string
}

type CleanCalicoStepBuilder struct {
	step.Builder[CleanCalicoStepBuilder, *CleanCalicoStep]
}

func NewCleanCalicoStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanCalicoStepBuilder {
	s := &CleanCalicoStep{
		ReleaseName: "tigera-operator",
		Namespace:   "tigera-operator",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Uninstall Calico Helm release and cleanup resources", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(CleanCalicoStepBuilder).Init(s)
	return b
}

func (s *CleanCalicoStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanCalicoStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	checkCmd := fmt.Sprintf("helm status %s -n %s", s.ReleaseName, s.Namespace)
	runResult, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err != nil {
		if strings.Contains(strings.ToLower(runResult.Stdout), "release: not found") || strings.Contains(strings.ToLower(err.Error()), "release: not found") {
			logger.Info("Calico Helm release not found. Step is done.")
			return true, nil
		}
		logger.Warnf("Failed to check Helm status, assuming cleanup is required. Error: %v", err)
		return false, nil
	}

	logger.Info("Calico Helm release found. Cleanup is required.")
	return false, nil
}

func (s *CleanCalicoStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	uninstallCmd := fmt.Sprintf("helm uninstall %s -n %s --wait --timeout 5m", s.ReleaseName, s.Namespace)
	logger.Infof("Uninstalling Calico Helm release with command: %s", uninstallCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, uninstallCmd, s.Sudo); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "release: not found") {
			logger.Warnf("Helm uninstall command failed (this may be ok): %v", err)
		}
	}

	logger.Warn("Forcefully deleting Calico CRDs...")
	crdDeleteCmd := "kubectl get crd -o name | grep 'calico.projectcalico.org' | xargs -r kubectl delete"
	if _, err := runner.Run(ctx.GoContext(), conn, crdDeleteCmd, s.Sudo); err != nil {
		logger.Warnf("Failed to delete Calico CRDs (this may be ok if they were already gone): %v", err)
	}

	deleteNsCmd := fmt.Sprintf("kubectl delete namespace %s --ignore-not-found=true", s.Namespace)
	logger.Infof("Ensuring Calico namespace '%s' is deleted", s.Namespace)
	if _, err := runner.Run(ctx.GoContext(), conn, deleteNsCmd, s.Sudo); err != nil {
		logger.Warnf("Failed to delete Calico namespace: %v", err)
	}

	logger.Info("Calico cleanup process finished.")
	result.MarkCompleted("Calico cleanup finished")
	return result, nil
}

func (s *CleanCalicoStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Cleanup step has no rollback action.")
	return nil
}

var _ step.Step = (*CleanCalicoStep)(nil)
