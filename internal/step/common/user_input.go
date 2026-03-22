package common

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type UserInputStep struct {
	step.Base
	Prompt    string
	AssumeYes bool
}

type UserInputStepBuilder struct {
	step.Builder[UserInputStepBuilder, *UserInputStep]
}

func NewUserInputStepBuilder(ctx runtime.ExecutionContext, instanceName, prompt string) *UserInputStepBuilder {
	cs := &UserInputStep{
		Prompt: prompt,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Prompt user for confirmation: %s", instanceName, prompt)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 10 * time.Minute
	return new(UserInputStepBuilder).Init(cs)
}

func (b *UserInputStepBuilder) WithAssumeYes(assumeYes bool) *UserInputStepBuilder {
	b.Step.AssumeYes = assumeYes
	return b
}

func (s *UserInputStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UserInputStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if s.AssumeYes {
		logger.Info("AssumeYes is true, user prompt will be skipped.")
		return true, nil
	}
	return false, nil
}

func (s *UserInputStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())

	if s.AssumeYes {
		logger.Info("AssumeYes is true, proceeding automatically without prompt.")
		result.MarkCompleted("User input skipped (AssumeYes=true)")
		return result, nil
	}

	fmt.Println()
	fmt.Printf("------------------------------------------------------------------\n")
	fmt.Printf("🚨 ACTION REQUIRED: %s\n", s.Prompt)
	fmt.Printf("------------------------------------------------------------------\n")
	fmt.Printf("Enter 'yes' to continue, or 'no' to abort: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to read user input: %v", err))
		return result, err
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "yes" || input == "y" {
		logger.Info("User confirmed to continue.")
		fmt.Println()
		result.MarkCompleted("User confirmed to continue")
		return result, nil
	}

	logger.Error(nil, "User declined to continue. Aborting workflow.")
	fmt.Println()
	result.MarkFailed(fmt.Errorf("user declined confirmation"), "User declined confirmation")
	return result, fmt.Errorf("user declined confirmation")
}

func (s *UserInputStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for UserInputStep is a no-op.")
	return nil
}

var _ step.Step = (*UserInputStep)(nil)
