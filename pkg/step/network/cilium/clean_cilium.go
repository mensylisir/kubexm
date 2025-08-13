package cilium

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanCiliumNodeStateStep struct {
	step.Base
}

type CleanCiliumNodeStateStepBuilder struct {
	step.Builder[CleanCiliumNodeStateStepBuilder, *CleanCiliumNodeStateStep]
}

func NewCleanCiliumNodeStateStepBuilder(ctx runtime.Context, instanceName string) *CleanCiliumNodeStateStepBuilder {
	s := &CleanCiliumNodeStateStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Cleanup Cilium eBPF maps and runtime files on the node", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = true
	s.Base.Timeout = 3 * time.Minute

	b := new(CleanCiliumNodeStateStepBuilder).Init(s)
	return b
}

func (s *CleanCiliumNodeStateStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanCiliumNodeStateStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	ciliumRunDir := "/run/cilium"
	exists, err := runner.Exists(ctx.GoContext(), conn, ciliumRunDir)
	if err != nil {
		return false, fmt.Errorf("failed to check for directory '%s': %w", ciliumRunDir, err)
	}

	if !exists {
		logger.Info("Cilium runtime directory not found. Node state is clean. Step is done.")
		return true, nil
	}

	logger.Info("Cilium runtime directory found. Node cleanup is required.")
	return false, nil
}

func (s *CleanCiliumNodeStateStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Cleaning up Cilium BPF maps and mounts on the node...")
	cleanupScript := `
set -e
if mount | grep -q /run/cilium/bpffs; then
  echo "Unmounting Cilium BPF filesystem..."
  umount /run/cilium/bpffs
fi
echo "Removing Cilium directories..."
rm -rf /run/cilium
rm -rf /var/run/cilium
`
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupScript, s.Sudo); err != nil {
		logger.Warnf("Cilium node cleanup script failed (this may be ok if directories were already gone): %v", err)
	}

	logger.Info("Cilium node state cleanup finished.")
	return nil
}

func (s *CleanCiliumNodeStateStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CleanCiliumNodeStateStep)(nil)
