package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ApplyManifestStep applies a manifest file using kubectl.
type ApplyManifestStep struct {
	step.Base
	ManifestPath   string
	Namespace      string
	KubeconfigPath string
	IgnoreNotFound bool
}

type ApplyManifestStepBuilder struct {
	step.Builder[ApplyManifestStepBuilder, *ApplyManifestStep]
}

func NewApplyManifestStepBuilder(ctx runtime.ExecutionContext, instanceName, manifestPath, namespace, kubeconfigPath string) *ApplyManifestStepBuilder {
	s := &ApplyManifestStep{
		ManifestPath:   manifestPath,
		Namespace:      namespace,
		KubeconfigPath: kubeconfigPath,
		IgnoreNotFound: true,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply manifest %s", instanceName, manifestPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(ApplyManifestStepBuilder).Init(s)
}

func (b *ApplyManifestStepBuilder) WithIgnoreNotFound(ignore bool) *ApplyManifestStepBuilder {
	b.Step.IgnoreNotFound = ignore
	return b
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

	cmd := fmt.Sprintf("kubectl apply -f %s --kubeconfig %s", s.ManifestPath, s.KubeconfigPath)
	if s.Namespace != "" {
		cmd = fmt.Sprintf("kubectl apply -f %s --namespace %s --kubeconfig %s", s.ManifestPath, s.Namespace, s.KubeconfigPath)
	}

	logger.Infof("Running: %s", cmd)
	if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to apply manifest")
		return result, err
	}

	logger.Infof("Manifest %s applied successfully", s.ManifestPath)
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

	deleteCmd := fmt.Sprintf("kubectl delete -f %s --kubeconfig %s", s.ManifestPath, s.KubeconfigPath)
	if s.Namespace != "" {
		deleteCmd = fmt.Sprintf("kubectl delete -f %s --namespace %s --kubeconfig %s", s.ManifestPath, s.Namespace, s.KubeconfigPath)
	}
	if s.IgnoreNotFound {
		deleteCmd += " --ignore-not-found=true"
	}

	logger.Warnf("Rolling back by deleting manifest: %s", deleteCmd)
	runner.Run(ctx.GoContext(), conn, deleteCmd, s.Base.Sudo)
	return nil
}

var _ step.Step = (*ApplyManifestStep)(nil)
