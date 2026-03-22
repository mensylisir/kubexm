package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

const (
	masterLabel          = "node-role.kubernetes.io/master"
	controlPlaneLabel    = "node-role.kubernetes.io/control-plane"
	controlPlaneTaintKey = "node-role.kubernetes.io/control-plane"
)

type KubeadmLabelControlPlaneNodeStep struct {
	step.Base
}

type KubeadmLabelControlPlaneNodeStepBuilder struct {
	step.Builder[KubeadmLabelControlPlaneNodeStepBuilder, *KubeadmLabelControlPlaneNodeStep]
}

func NewKubeadmLabelControlPlaneNodeStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmLabelControlPlaneNodeStepBuilder {
	s := &KubeadmLabelControlPlaneNodeStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Ensure the current control-plane node has correct labels and taints"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmLabelControlPlaneNodeStepBuilder).Init(s)
	return b
}

func (s *KubeadmLabelControlPlaneNodeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmLabelControlPlaneNodeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying 'kubectl' command is available...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubectl", s.Sudo); err != nil {
		logger.Errorf("'kubectl' command not found.")
		return false, fmt.Errorf("precheck failed: 'kubectl' command not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: 'kubectl' command is available.")
	return false, nil
}

func (s *KubeadmLabelControlPlaneNodeStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	nodeName := ctx.GetHost().GetName()
	log := logger.With("node", nodeName)
	log.Info("Ensuring node has correct labels and taints...")

	log.Info("Applying labels...")
	labelCmd1 := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf label node %s %s='' --overwrite", nodeName, masterLabel)
	if _, err := runner.Run(ctx.GoContext(), conn, labelCmd1, s.Sudo); err != nil {
		err = fmt.Errorf("failed to apply label '%s' to node '%s': %w", masterLabel, nodeName, err)
		result.MarkFailed(err, "failed to apply label")
		return result, err
	}

	labelCmd2 := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf label node %s %s='' --overwrite", nodeName, controlPlaneLabel)
	if _, err := runner.Run(ctx.GoContext(), conn, labelCmd2, s.Sudo); err != nil {
		err = fmt.Errorf("failed to apply label '%s' to node '%s': %w", controlPlaneLabel, nodeName, err)
		result.MarkFailed(err, "failed to apply label")
		return result, err
	}
	log.Info("Labels applied successfully.")

	log.Info("Applying NoSchedule taint...")
	taintCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf taint nodes %s %s:NoSchedule --overwrite", nodeName, controlPlaneTaintKey)
	if _, err := runner.Run(ctx.GoContext(), conn, taintCmd, s.Sudo); err != nil {
		if !strings.Contains(err.Error(), "taint \""+controlPlaneTaintKey+"\" already exists") {
			err = fmt.Errorf("failed to apply taint to node '%s': %w", nodeName, err)
			result.MarkFailed(err, "failed to apply taint")
			return result, err
		}
		log.Info("Taint already exists.")
	} else {
		log.Info("Taint applied successfully.")
	}

	log.Info("Node has been configured with correct labels and taints.")
	result.MarkCompleted("labels and taints configured successfully")
	return result, nil
}

func (s *KubeadmLabelControlPlaneNodeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for labeling/tainting is not performed automatically to avoid unintended side effects (e.g., causing pods to be scheduled on the master).")
	return nil
}

var _ step.Step = (*KubeadmLabelControlPlaneNodeStep)(nil)
