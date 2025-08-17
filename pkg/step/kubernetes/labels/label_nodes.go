package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmLabelNodesStep struct {
	step.Base
	Labels map[string]string
}

type KubeadmLabelNodesStepBuilder struct {
	step.Builder[KubeadmLabelNodesStepBuilder, *KubeadmLabelNodesStep]
}

func NewKubeadmLabelNodesStepBuilder(ctx runtime.Context, instanceName string) *KubeadmLabelNodesStepBuilder {
	s := &KubeadmLabelNodesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Ensure all worker nodes have the correct role label"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmLabelNodesStepBuilder).Init(s)
	return b
}

func (b *KubeadmLabelNodesStepBuilder) WithLabels(labels map[string]string) *KubeadmLabelNodesStepBuilder {
	b.Step.Labels = labels
	return b
}

func (s *KubeadmLabelNodesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmLabelNodesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying 'kubectl' command is available on the execution host...")
	nodeName := ctx.GetHost().GetName()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubectl", s.Sudo); err != nil {
		logger.Errorf("'kubectl' command not found.")
		return false, fmt.Errorf("precheck failed: 'kubectl' command not found on execution host '%s'", ctx.GetHost().GetName())
	}
	allLabeled := true
	for key, value := range s.Labels {
		checkCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf get node %s -o jsonpath='{.metadata.labels.%s}'", nodeName, key)
		stdout, err := ctx.GetRunner().Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
		if err != nil {
			allLabeled = false
			logger.Infof("Worker node '%s' is missing the role label.", nodeName)
			break
		}
		if stdout != value {
			allLabeled = false
			break
		}
	}

	if allLabeled {
		logger.Info("All worker nodes already have the correct role label. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed: 'kubectl' is available and at least one worker needs labeling.")
	return false, nil
}

func (s *KubeadmLabelNodesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	nodeName := ctx.GetHost().GetName()
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	if len(s.Labels) == 0 {
		logger.Info("No labels to apply.")
		return nil
	}

	logger.Info("Applying labels to this node...")

	var labelsStr strings.Builder
	for key, value := range s.Labels {
		labelsStr.WriteString(fmt.Sprintf("%s=%s ", key, value))
	}

	labelCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf label node %s %s --overwrite", nodeName, labelsStr.String())

	if _, err := runner.Run(ctx.GoContext(), conn, labelCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to apply labels to node '%s': %w", nodeName, err)
	}

	logger.Info("Labels applied successfully.")
	return nil
}

func (s *KubeadmLabelNodesStep) Rollback(ctx runtime.ExecutionContext) error {
	nodeName := ctx.GetHost().GetName()
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", nodeName, "phase", "Rollback")

	if len(s.Labels) == 0 {
		logger.Info("No labels were specified, nothing to roll back.")
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Cannot connect to host for rollback: %v", err)
		return nil
	}

	logger.Warn("Rolling back by removing applied labels...")
	var labelsToRemove strings.Builder
	for key := range s.Labels {
		labelsToRemove.WriteString(key + " ")
	}

	removeLabelCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf label node %s %s-", nodeName, labelsToRemove.String())
	_, _ = runner.Run(ctx.GoContext(), conn, removeLabelCmd, s.Sudo)

	logger.Info("Rollback attempt for labels finished.")
	return nil
}

var _ step.Step = (*KubeadmLabelNodesStep)(nil)
