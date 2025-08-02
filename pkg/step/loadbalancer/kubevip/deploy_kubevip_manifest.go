package kubevip

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DeployKubeVipManifestStep struct {
	step.Base
}

func NewDeployKubeVipManifestStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *DeployKubeVipManifestStep] {
	s := &DeployKubeVipManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Deploy kube-vip static pod manifest"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *DeployKubeVipManifestStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	manifestContent, ok := ctx.Get("kube-vip.yaml")
	if !ok {
		return fmt.Errorf("kube-vip.yaml not found in context")
	}

	manifestBytes, ok := manifestContent.([]byte)
	if !ok {
		return fmt.Errorf("kube-vip.yaml in context is not of type []byte")
	}

	remotePath := filepath.Join(common.KubernetesManifestsDir, "kube-vip.yaml")
	logger.Infof("Deploying kube-vip static pod manifest to %s", remotePath)

	if err := runner.WriteFile(ctx.GoContext(), conn, manifestBytes, remotePath, "0644", true); err != nil {
		return fmt.Errorf("failed to deploy kube-vip static pod manifest: %w", err)
	}

	return nil
}

func (s *DeployKubeVipManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	remotePath := filepath.Join(common.KubernetesManifestsDir, "kube-vip.yaml")
	logger.Warnf("Rolling back by removing %s", remotePath)

	if err := runner.Remove(ctx.GoContext(), conn, remotePath, true, false); err != nil {
		logger.Errorf("Failed to remove kube-vip manifest during rollback: %v", err)
	}

	return nil
}

func (s *DeployKubeVipManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
