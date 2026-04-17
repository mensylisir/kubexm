package common

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CheckResourceStep checks system resources (disk, memory, CPU).
type CheckResourceStep struct {
	step.Base
	CheckType string // "disk", "memory", "cpu"
	Path      string // path for disk check, "" for other checks
	MinValue  int64
}

type CheckResourceStepBuilder struct {
	step.Builder[CheckResourceStepBuilder, *CheckResourceStep]
}

func NewCheckResourceStepBuilder(ctx runtime.ExecutionContext, instanceName, checkType string) *CheckResourceStepBuilder {
	s := &CheckResourceStep{
		CheckType: checkType,
		Path:      "/",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Check %s resources", instanceName, checkType)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(CheckResourceStepBuilder).Init(s)
}

func (b *CheckResourceStepBuilder) WithPath(path string) *CheckResourceStepBuilder {
	b.Step.Path = path
	return b
}

func (b *CheckResourceStepBuilder) WithMinValue(minValue int64) *CheckResourceStepBuilder {
	b.Step.MinValue = minValue
	return b
}

func (s *CheckResourceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckResourceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CheckResourceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	var cmd, valueRegex string
	switch s.CheckType {
	case "disk":
		cmd = fmt.Sprintf("df -BG %s | tail -1 | awk '{print $4}'", s.Path)
		valueRegex = `(\d+)`
	case "memory":
		cmd = "free -m | grep Mem | awk '{print $2}'"
		valueRegex = `(\d+)`
	case "cpu":
		cmd = "nproc"
		valueRegex = `(\d+)`
	default:
		result.MarkFailed(fmt.Errorf("unknown check type: %s", s.CheckType), "invalid check type")
		return result, fmt.Errorf("unknown check type: %s", s.CheckType)
	}

	logger.Infof("Running: %s", cmd)
	runResult, err := runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to check %s", s.CheckType))
		return result, err
	}

	re := regexp.MustCompile(valueRegex)
	matches := re.FindStringSubmatch(runResult.Stdout)
	if len(matches) < 2 {
		result.MarkFailed(fmt.Errorf("failed to parse %s result", s.CheckType), "parse error")
		return result, fmt.Errorf("failed to parse %s result", s.CheckType)
	}

	value, _ := strconv.ParseInt(matches[1], 10, 64)
	logger.Infof("Check %s: value=%d, min=%d", s.CheckType, value, s.MinValue)

	if value < s.MinValue {
		result.MarkFailed(fmt.Errorf("insufficient %s: got %d, need %d", s.CheckType, value, s.MinValue), "insufficient resources")
		return result, fmt.Errorf("insufficient %s", s.CheckType)
	}

	result.MarkCompleted(fmt.Sprintf("%s check passed: %d", s.CheckType, value))
	return result, nil
}

func (s *CheckResourceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CheckResourceStep)(nil)
