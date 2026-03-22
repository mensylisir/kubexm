package perform

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CordonNodeStep struct {
	step.Base
	TargetNodeName string
}

type CordonNodeStepBuilder struct {
	step.Builder[CordonNodeStepBuilder, *CordonNodeStep]
}

func NewCordonNodeStepBuilder(ctx runtime.ExecutionContext, instanceName string, targetNodeName string) *CordonNodeStepBuilder {
	s := &CordonNodeStep{
		TargetNodeName: targetNodeName,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("Cordon node '%s' to mark it as unschedulable", targetNodeName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(CordonNodeStepBuilder).Init(s)
	return b
}

func (s *CordonNodeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CordonNodeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Precheck")
	logger.Info("Starting precheck for cordon node...")

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

	if strings.Contains(string(stdout), "SchedulingDisabled") {
		logger.Infof("Node '%s' is already cordoned. Step is done.", s.TargetNodeName)
		return true, nil
	}

	logger.Info("Precheck passed: node is currently schedulable.")
	return false, nil
}

func (s *CordonNodeStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	logger.Infof("Cordoning node '%s'...", s.TargetNodeName)
	cordonCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf cordon %s", s.TargetNodeName)

	if _, err := runner.Run(ctx.GoContext(), conn, cordonCmd, s.Sudo); err != nil {
		err = fmt.Errorf("failed to cordon node '%s': %w", s.TargetNodeName, err)
		result.MarkFailed(err, "failed to cordon node")
		return result, err
	}

	logger.Infof("Node '%s' cordoned successfully.", s.TargetNodeName)
	result.MarkCompleted("node cordoned successfully")
	return result, nil
}

func (s *CordonNodeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Rollback")
	logger.Warnf("Rolling back by uncordoning node '%s'...", s.TargetNodeName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Cannot connect to host for rollback: %v", err)
		return nil
	}

	uncordonCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf uncordon %s", s.TargetNodeName)

	if _, err := runner.Run(ctx.GoContext(), conn, uncordonCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to uncordon node '%s' during rollback. Manual intervention may be needed. Error: %v", s.TargetNodeName, err)
	}

	logger.Info("Rollback attempt for cordon finished.")
	return nil
}

var _ step.Step = (*CordonNodeStep)(nil)
