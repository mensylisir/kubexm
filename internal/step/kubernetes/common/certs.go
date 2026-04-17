package common

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyKubeconfigStep copies kubeconfig file to remote host.
type CopyKubeconfigStep struct {
	step.Base
	SourceKubeconfig string
	TargetPath       string
	Mode             string
}

type CopyKubeconfigStepBuilder struct {
	step.Builder[CopyKubeconfigStepBuilder, *CopyKubeconfigStep]
}

func NewCopyKubeconfigStepBuilder(ctx runtime.ExecutionContext, instanceName, sourceKubeconfig, targetPath, mode string) *CopyKubeconfigStepBuilder {
	s := &CopyKubeconfigStep{
		SourceKubeconfig: sourceKubeconfig,
		TargetPath:       targetPath,
		Mode:             mode,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy kubeconfig to %s", instanceName, targetPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CopyKubeconfigStepBuilder).Init(s)
}

func (s *CopyKubeconfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyKubeconfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, err
	}

	if exists {
		return true, nil
	}

	return false, nil
}

func (s *CopyKubeconfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	targetDir := filepath.Dir(s.TargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", false); err != nil {
		result.MarkFailed(err, "failed to create target directory")
		return result, err
	}

	logger.Infof("Copying kubeconfig from %s to %s:%s", s.SourceKubeconfig, ctx.GetHost().GetName(), s.TargetPath)
	if err := runner.CopyFile(ctx.GoContext(), conn, s.SourceKubeconfig, s.TargetPath, false, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to copy kubeconfig")
		return result, err
	}

	logger.Infof("Kubeconfig copied successfully to %s", s.TargetPath)
	result.MarkCompleted(fmt.Sprintf("Kubeconfig copied to %s", s.TargetPath))
	return result, nil
}

func (s *CopyKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.TargetPath)
	runner.Remove(ctx.GoContext(), conn, s.TargetPath, true, false)
	return nil
}

var _ step.Step = (*CopyKubeconfigStep)(nil)

// WaitForKubeAPIStep waits for kube-apiserver to be ready.
type WaitForKubeAPIStep struct {
	step.Base
	Timeout time.Duration
}

type WaitForKubeAPIStepBuilder struct {
	step.Builder[WaitForKubeAPIStepBuilder, *WaitForKubeAPIStep]
}

func NewWaitForKubeAPIStepBuilder(ctx runtime.ExecutionContext, instanceName string) *WaitForKubeAPIStepBuilder {
	s := &WaitForKubeAPIStep{
		Timeout: 5 * time.Minute,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Wait for kube-apiserver to be ready", instanceName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = s.Timeout
	return new(WaitForKubeAPIStepBuilder).Init(s)
}

func (s *WaitForKubeAPIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *WaitForKubeAPIStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *WaitForKubeAPIStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	waitCmd := fmt.Sprintf("kubectl wait --for=condition=Ready --timeout=%s pods -l component=kube-apiserver -n kube-system --request-timeout=%s", s.Timeout, s.Timeout)

	logger.Infof("Waiting for kube-apiserver: %s", waitCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, waitCmd, false); err != nil {
		result.MarkFailed(err, "kube-apiserver not ready")
		return result, err
	}

	logger.Infof("kube-apiserver is ready")
	result.MarkCompleted("kube-apiserver is ready")
	return result, nil
}

func (s *WaitForKubeAPIStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*WaitForKubeAPIStep)(nil)
