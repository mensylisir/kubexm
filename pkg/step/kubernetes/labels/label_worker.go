// FILE: pkg/kubeadm/step_label_worker_node.go

package kubeadm

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	workerLabel = "node-role.kubernetes.io/worker"
)

type KubeadmLabelWorkerNodeStep struct {
	step.Base
	TargetWorkerNodeName string
}

type KubeadmLabelWorkerNodeStepBuilder struct {
	step.Builder[KubeadmLabelWorkerNodeStepBuilder, *KubeadmLabelWorkerNodeStep]
}

func NewKubeadmLabelWorkerNodeStepBuilder(ctx runtime.Context, instanceName string, targetWorkerNodeName string) *KubeadmLabelWorkerNodeStepBuilder {
	s := &KubeadmLabelWorkerNodeStep{
		TargetWorkerNodeName: targetWorkerNodeName,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("Ensure worker node '%s' has the correct role label", targetWorkerNodeName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmLabelWorkerNodeStepBuilder).Init(s)
	return b
}

func (s *KubeadmLabelWorkerNodeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmLabelWorkerNodeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying 'kubectl' command is available...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubectl", s.Sudo); err != nil {
		logger.Errorf("'kubectl' command not found.")
		return false, fmt.Errorf("precheck failed: 'kubectl' command not found on execution host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: 'kubectl' command is available.")
	return false, nil
}

func (s *KubeadmLabelWorkerNodeStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	log := logger.With("target_node", s.TargetWorkerNodeName)
	log.Info("Applying worker role label...")

	labelCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf label node %s %s='' --overwrite", s.TargetWorkerNodeName, workerLabel)

	if _, err := runner.Run(ctx.GoContext(), conn, labelCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to apply label '%s' to worker node '%s': %w", workerLabel, s.TargetWorkerNodeName, err)
	}

	log.Info("Worker role label applied successfully.")
	return nil
}

func (s *KubeadmLabelWorkerNodeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back by removing worker label from node '%s'...", s.TargetWorkerNodeName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Cannot connect to host for rollback: %v", err)
		return nil
	}

	removeLabelCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf label node %s %s-", s.TargetWorkerNodeName, workerLabel)

	_, _ = runner.Run(ctx.GoContext(), conn, removeLabelCmd, s.Sudo)

	logger.Info("Attempted to remove worker label.")
	return nil
}

var _ step.Step = (*KubeadmLabelWorkerNodeStep)(nil)
