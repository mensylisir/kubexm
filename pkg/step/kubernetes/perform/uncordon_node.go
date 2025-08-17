package perform

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type UncordonNodeStep struct {
	step.Base
	TargetNodeName string
}

type UncordonNodeStepBuilder struct {
	step.Builder[UncordonNodeStepBuilder, *UncordonNodeStep]
}

func NewUncordonNodeStepBuilder(ctx runtime.Context, instanceName string, targetNodeName string) *UncordonNodeStepBuilder {
	s := &UncordonNodeStep{
		TargetNodeName: targetNodeName,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("Uncordon node '%s' to mark it as schedulable", targetNodeName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(UncordonNodeStepBuilder).Init(s)
	return b
}

func (s *UncordonNodeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UncordonNodeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Precheck")
	logger.Info("Starting precheck for uncordon node...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	checkCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf get node %s", s.TargetNodeName)
	stdout, err := runner.Run(ctx.GoContext(), conn, checkCmd, s.Sudo)
	if err != nil {
		return false, fmt.Errorf("precheck failed: cannot get status of node '%s': %w", s.TargetNodeName, err)
	}

	if !strings.Contains(string(stdout), "SchedulingDisabled") {
		logger.Infof("Node '%s' is already schedulable. Step is done.", s.TargetNodeName)
		return true, nil
	}

	logger.Info("Precheck passed: node is currently cordoned.")
	return false, nil
}

func (s *UncordonNodeStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Uncordoning node '%s'...", s.TargetNodeName)
	uncordonCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf uncordon %s", s.TargetNodeName)

	if _, err := runner.Run(ctx.GoContext(), conn, uncordonCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to uncordon node '%s': %w", s.TargetNodeName, err)
	}

	logger.Infof("Node '%s' uncordoned successfully.", s.TargetNodeName)
	return nil
}

func (s *UncordonNodeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Rollback")
	logger.Warnf("Rolling back by cordoning node '%s'...", s.TargetNodeName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Cannot connect to host for rollback: %v", err)
		return nil
	}

	cordonCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf cordon %s", s.TargetNodeName)

	_, _ = runner.Run(ctx.GoContext(), conn, cordonCmd, s.Sudo)

	logger.Info("Rollback attempt for uncordon finished.")
	return nil
}

var _ step.Step = (*UncordonNodeStep)(nil)
