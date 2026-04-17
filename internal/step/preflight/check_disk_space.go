package preflight

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
	"github.com/pkg/errors"
)

var _ step.Step = (*CheckDiskSpaceStep)(nil)

// CheckDiskSpaceStep verifies sufficient disk space on critical partitions.
type CheckDiskSpaceStep struct {
	step.Base
	MinDiskSpaceGB *float64
}

type CheckDiskSpaceStepBuilder struct {
	step.Builder[CheckDiskSpaceStepBuilder, *CheckDiskSpaceStep]
}

func NewCheckDiskSpaceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CheckDiskSpaceStepBuilder {
	s := &CheckDiskSpaceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Check disk space", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CheckDiskSpaceStepBuilder).Init(s)
}

func (s *CheckDiskSpaceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// CriticalPaths are the mount points that must have sufficient free space.
var CriticalPaths = []string{
	"/var",      // containerd, kubelet storage
	"/var/lib",   // etcd data, container images
	"/tmp",       // temp files during operations
	"/",          // root filesystem
}

func (s *CheckDiskSpaceStep) checkRequirement(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	skipChecks := ctx.GetClusterConfig().Spec.Preflight.SkipChecks
	for _, checkToSkip := range skipChecks {
		if checkToSkip == "disk_space" {
			logger.Infof("Skipping disk space check because 'disk_space' is in skipChecks list.")
			return nil
		}
	}

	if s.MinDiskSpaceGB == nil {
		logger.Info("Minimum disk space requirement not configured, skipping check.")
		return nil
	}
	minSpaceGB := *s.MinDiskSpaceGB

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return errors.Wrap(err, "failed to get host connector")
	}

	runner := ctx.GetRunner()

	// Use df -BG to get sizes in 1G blocks (Gigabytes)
	// df result: Filesystem  1G-blocks  Used  Available  Use%  Mounted on
	for _, path := range CriticalPaths {
		cmd := fmt.Sprintf("df -BG %s 2>/dev/null | tail -1 | awk '{print $4}'", path)
		runResult, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo)
		if err != nil {
			logger.Warnf("Could not determine free space for '%s', skipping this path: %v", path, err)
			continue
		}

		availGB, err := parseDfOutput(strings.TrimSpace(runResult.Stdout))
		if err != nil {
			logger.Warnf("Could not parse df result for '%s': %v, skipping this path", path, err)
			continue
		}

		logger.Debugf("Checking disk space for '%s': available=%.1fGB, minimum=%.1fGB.", path, availGB, minSpaceGB)

		if availGB < minSpaceGB {
			return errors.Errorf("disk space requirement not met for '%s': minimum required %.1fGB, but only %.1fGB available", path, minSpaceGB, availGB)
		}
	}

	logger.Infof("Disk space check passed for all critical paths (minimum: %.1fGB).", minSpaceGB)
	return nil
}

// parseDfOutput parses "123G" or "50G" from df result and returns the value in GB as float64.
func parseDfOutput(result string) (float64, error) {
	result = strings.TrimSpace(result)
	result = strings.TrimSuffix(result, "G")
	result = strings.TrimSuffix(result, "M")
	result = strings.TrimSuffix(result, "K")

	n, err := strconv.ParseFloat(result, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *CheckDiskSpaceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	checkErr := s.checkRequirement(ctx)
	if checkErr == nil {
		return true, nil
	}
	return false, nil
}

func (s *CheckDiskSpaceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	if err := s.checkRequirement(ctx); err != nil {
		result.MarkFailed(err, "disk space check failed")
		return result, err
	}
	result.MarkCompleted("disk space check passed")
	return result, nil
}

func (s *CheckDiskSpaceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}
