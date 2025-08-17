package perform

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DrainNodeStep struct {
	step.Base
	TargetNodeName string
	GracePeriod    int
	DrainTimeout   time.Duration
}

type DrainNodeStepBuilder struct {
	step.Builder[DrainNodeStepBuilder, *DrainNodeStep]
}

func NewDrainNodeStepBuilder(ctx runtime.Context, instanceName string, targetNodeName string) *DrainNodeStepBuilder {
	s := &DrainNodeStep{
		TargetNodeName: targetNodeName,
		GracePeriod:    -1,
		DrainTimeout:   5 * time.Minute,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("Drain pods from node '%s'", targetNodeName)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = s.DrainTimeout + 1*time.Minute

	b := new(DrainNodeStepBuilder).Init(s)
	return b
}

func (s *DrainNodeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DrainNodeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Precheck")
	logger.Info("Starting precheck for drain node...")

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
		return false, fmt.Errorf("precheck failed: node '%s' must be cordoned before draining", s.TargetNodeName)
	}

	logger.Info("Precheck passed: node is cordoned.")
	return false, nil
}

func (s *DrainNodeStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Draining pods from node '%s' (timeout: %v)...", s.TargetNodeName, s.DrainTimeout)
	drainCmd := fmt.Sprintf("kubectl --kubeconfig /etc/kubernetes/admin.conf drain %s "+
		"--ignore-daemonsets --delete-local-data --force --grace-period=%d --timeout=%s",
		s.TargetNodeName, s.GracePeriod, s.DrainTimeout.String())

	if _, err := runner.Run(ctx.GoContext(), conn, drainCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to drain node '%s': %w", s.TargetNodeName, err)
	}

	logger.Infof("Node '%s' drained successfully.", s.TargetNodeName)
	return nil
}

func (s *DrainNodeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "exec_host", ctx.GetHost().GetName(), "target_node", s.TargetNodeName, "phase", "Rollback")
	logger.Warn("Rollback for drain is not an active operation. The node will remain cordoned until the 'Uncordon' step is run.")
	return nil
}

var _ step.Step = (*DrainNodeStep)(nil)
