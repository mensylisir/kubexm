package rbac

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

type ApplyEssentialRBACStep struct {
	step.Base
	APIServerUser string
}

type ApplyEssentialRBACStepBuilder struct {
	step.Builder[ApplyEssentialRBACStepBuilder, *ApplyEssentialRBACStep]
}

func NewApplyEssentialRBACStepBuilder(ctx runtime.ExecutionContext, instanceName string) *ApplyEssentialRBACStepBuilder {
	s := &ApplyEssentialRBACStep{
		APIServerUser: common.KubernetesAPICertCN,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply essential RBAC rules for cluster components", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(ApplyEssentialRBACStepBuilder).Init(s)
	return b
}

func (s *ApplyEssentialRBACStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *ApplyEssentialRBACStep) renderRBAC() (string, error) {
	tmplContent, err := templates.Get("kubernetes/rbac/essential-rbac.yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get essential-rbac.yaml.tmpl: %w", err)
	}
	return templates.Render(tmplContent, s)
}

func (s *ApplyEssentialRBACStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *ApplyEssentialRBACStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	rbacContent, err := s.renderRBAC()
	if err != nil {
		err = fmt.Errorf("failed to render RBAC YAML: %w", err)
		result.MarkFailed(err, "failed to render RBAC YAML")
		return result, err
	}

	remoteTempFile := filepath.Join(ctx.GetUploadDir(), "essential-rbac.yaml")
	if err := helpers.WriteContentToRemote(ctx, conn, rbacContent, remoteTempFile, "0644", false); err != nil {
		err = fmt.Errorf("failed to write temporary RBAC file: %w", err)
		result.MarkFailed(err, "failed to write temporary RBAC file")
		return result, err
	}
	defer runner.Remove(ctx.GoContext(), conn, remoteTempFile, false, false)

	logger.Info("Applying essential RBAC rules using kubectl...")
	applyCmd := fmt.Sprintf("kubectl apply -f %s", remoteTempFile)

	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, applyCmd, s.Sudo); err != nil {
		err = fmt.Errorf("failed to apply essential RBAC rules: %w (stderr: %s)", err, string(stderr))
		result.MarkFailed(err, "failed to apply RBAC rules")
		return result, err
	}

	logger.Info("Essential RBAC rules applied successfully.")
	result.MarkCompleted("RBAC rules applied successfully")
	return result, nil
}

func (s *ApplyEssentialRBACStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	rbacContent, err := s.renderRBAC()
	if err != nil {
		logger.Warnf("Failed to render RBAC YAML for rollback, cannot delete resources: %v", err)
		return nil
	}

	remoteTempFile := filepath.Join(ctx.GetUploadDir(), "essential-rbac-rollback.yaml")
	if err := helpers.WriteContentToRemote(ctx, conn, rbacContent, remoteTempFile, "0644", false); err != nil {
		logger.Warnf("Failed to write temporary RBAC file for rollback: %v", err)
		return nil
	}
	defer runner.Remove(ctx.GoContext(), conn, remoteTempFile, false, false)

	logger.Warn("Rolling back by deleting applied RBAC rules...")
	deleteCmd := fmt.Sprintf("kubectl delete -f %s --ignore-not-found=true", remoteTempFile)
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, deleteCmd, s.Sudo); err != nil {
		logger.Warnf("Failed to delete RBAC rules during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*ApplyEssentialRBACStep)(nil)
