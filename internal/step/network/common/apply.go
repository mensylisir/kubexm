package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ApplyManifestStep applies a Kubernetes manifest using kubectl.
type ApplyManifestStep struct {
	step.Base
	ManifestContent string
	Namespace       string
}

type ApplyManifestStepBuilder struct {
	step.Builder[ApplyManifestStepBuilder, *ApplyManifestStep]
}

func NewApplyManifestStepBuilder(ctx runtime.ExecutionContext, instanceName, manifestContent, namespace string) *ApplyManifestStepBuilder {
	s := &ApplyManifestStep{
		ManifestContent: manifestContent,
		Namespace:       namespace,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply Kubernetes manifest to namespace %s", instanceName, namespace)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(ApplyManifestStepBuilder).Init(s)
}

func (s *ApplyManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ApplyManifestStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ApplyManifestStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	// Write manifest to a temp file
	tempFile := fmt.Sprintf("/tmp/%s.yaml", s.Base.Meta.Name)
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(s.ManifestContent), tempFile, "0644", s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to write manifest to temp file")
		return result, err
	}

	// Apply the manifest
	applyCmd := fmt.Sprintf("kubectl apply -f %s", tempFile)
	if s.Namespace != "" {
		applyCmd = fmt.Sprintf("kubectl apply -f %s -n %s", tempFile, s.Namespace)
	}

	logger.Infof("Applying manifest: %s", applyCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, applyCmd, false); err != nil {
		result.MarkFailed(err, "failed to apply manifest")
		return result, err
	}

	logger.Infof("Manifest applied successfully")
	result.MarkCompleted("Manifest applied")
	return result, nil
}

func (s *ApplyManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	tempFile := fmt.Sprintf("/tmp/%s.yaml", s.Base.Meta.Name)
	deleteCmd := fmt.Sprintf("kubectl delete -f %s --ignore-not-found", tempFile)
	if s.Namespace != "" {
		deleteCmd = fmt.Sprintf("kubectl delete -f %s -n %s --ignore-not-found", tempFile, s.Namespace)
	}

	logger.Warnf("Rolling back by deleting manifest: %s", deleteCmd)
	runner.Run(ctx.GoContext(), conn, deleteCmd, false)
	return nil
}

var _ step.Step = (*ApplyManifestStep)(nil)
