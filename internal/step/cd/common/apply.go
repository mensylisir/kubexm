package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ApplyCDManifestStep applies a CD manifest using kubectl.
type ApplyCDManifestStep struct {
	step.Base
	ManifestPath   string
	Namespace      string
	KubeconfigPath string
}

type ApplyCDManifestStepBuilder struct {
	step.Builder[ApplyCDManifestStepBuilder, *ApplyCDManifestStep]
}

func NewApplyCDManifestStepBuilder(ctx runtime.ExecutionContext, instanceName, manifestPath, namespace, kubeconfigPath string) *ApplyCDManifestStepBuilder {
	s := &ApplyCDManifestStep{
		ManifestPath:   manifestPath,
		Namespace:      namespace,
		KubeconfigPath: kubeconfigPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply CD manifest %s", instanceName, manifestPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(ApplyCDManifestStepBuilder).Init(s)
}

func (s *ApplyCDManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ApplyCDManifestStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ApplyCDManifestStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	cmd := fmt.Sprintf("kubectl apply -f %s --kubeconfig %s", s.ManifestPath, s.KubeconfigPath)
	if s.Namespace != "" {
		cmd = fmt.Sprintf("kubectl apply -f %s --namespace %s --kubeconfig %s", s.ManifestPath, s.Namespace, s.KubeconfigPath)
	}

	logger.Infof("Running: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to apply CD manifest")
		return result, err
	}

	logger.Infof("CD manifest %s applied successfully", s.ManifestPath)
	result.MarkCompleted("CD manifest applied")
	return result, nil
}

func (s *ApplyCDManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	deleteCmd := fmt.Sprintf("kubectl delete -f %s --kubeconfig %s --ignore-not-found=true", s.ManifestPath, s.KubeconfigPath)
	if s.Namespace != "" {
		deleteCmd = fmt.Sprintf("kubectl delete -f %s --namespace %s --kubeconfig %s --ignore-not-found=true", s.ManifestPath, s.Namespace, s.KubeconfigPath)
	}

	logger.Warnf("Rolling back: %s", deleteCmd)
	runner.Run(ctx.GoContext(), conn, deleteCmd, s.Base.Sudo)
	return nil
}

var _ step.Step = (*ApplyCDManifestStep)(nil)
