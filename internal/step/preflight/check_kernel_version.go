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

var _ step.Step = (*CheckKernelVersionStep)(nil)

// CheckKernelVersionStep verifies the kernel version meets minimum requirements.
type CheckKernelVersionStep struct {
	step.Base
	MinVersion *string
}

type CheckKernelVersionStepBuilder struct {
	step.Builder[CheckKernelVersionStepBuilder, *CheckKernelVersionStep]
}

func NewCheckKernelVersionStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CheckKernelVersionStepBuilder {
	s := &CheckKernelVersionStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Check kernel version", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CheckKernelVersionStepBuilder).Init(s)
}

func (s *CheckKernelVersionStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckKernelVersionStep) checkRequirement(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	skipChecks := ctx.GetClusterConfig().Spec.Preflight.SkipChecks
	for _, checkToSkip := range skipChecks {
		if checkToSkip == "kernel_version" {
			logger.Infof("Skipping kernel version check because 'kernel_version' is in skipChecks list.")
			return nil
		}
	}

	if s.MinVersion == nil {
		logger.Info("Minimum kernel version not configured, skipping check.")
		return nil
	}
	minVersion := *s.MinVersion

	actualVersion, err := s.getActualKernelVersion(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to determine kernel version")
	}

	if !versionMeetsMinimum(actualVersion, minVersion) {
		return errors.Errorf("kernel version requirement not met: minimum required is '%s', but found '%s'", minVersion, actualVersion)
	}

	logger.Infof("Kernel version check passed: '%s' >= '%s'.", actualVersion, minVersion)
	return nil
}

func (s *CheckKernelVersionStep) getActualKernelVersion(ctx runtime.ExecutionContext) (string, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return "", errors.Wrap(err, "failed to get host connector")
	}

	runner := ctx.GetRunner()
	runResult, err := runner.Run(ctx.GoContext(), conn, "uname -r", s.Sudo)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute 'uname -r'")
	}

	// uname -r returns e.g. "3.10.0-1127.el7.x86_64"
	version := strings.TrimSpace(runResult.Stdout)
	// Strip distro-specific suffixes like ".el7.x86_64"
	if idx := strings.Index(version, "-"); idx > 0 {
		version = version[:idx]
	}

	logger.Debugf("Detected kernel version: %s", version)
	return version, nil
}

// versionMeetsMinimum checks if actual >= minimum using semantic version comparison.
func versionMeetsMinimum(actual, minimum string) bool {
	actualParts := parseKernelVersion(actual)
	minParts := parseKernelVersion(minimum)

	for i := 0; i < 3; i++ {
		if actualParts[i] < minParts[i] {
			return false
		}
		if actualParts[i] > minParts[i] {
			return true
		}
	}
	return true
}

func parseKernelVersion(v string) [3]int {
	var parts [3]int
	segments := strings.Split(v, ".")
	for i := 0; i < len(segments) && i < 3; i++ {
		if n, err := strconv.Atoi(segments[i]); err == nil {
			parts[i] = n
		}
	}
	return parts
}

func (s *CheckKernelVersionStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	checkErr := s.checkRequirement(ctx)
	if checkErr == nil {
		return true, nil
	}
	return false, nil
}

func (s *CheckKernelVersionStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	if err := s.checkRequirement(ctx); err != nil {
		result.MarkFailed(err, "kernel version check failed")
		return result, err
	}
	result.MarkCompleted("kernel version check passed")
	return result, nil
}

func (s *CheckKernelVersionStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No action to roll back for a check-only step.")
	return nil
}
