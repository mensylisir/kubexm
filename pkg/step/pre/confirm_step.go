package pre

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ConfirmStep struct {
	step.Base
	Prompt    string
	AssumeYes bool
}

type ConfirmStepBuilder struct {
	step.Builder[ConfirmStepBuilder, *ConfirmStep]
}

func NewConfirmStepBuilder(ctx runtime.Context, instanceName, prompt string) *ConfirmStepBuilder {
	cs := &ConfirmStep{
		Prompt: prompt,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Prompt user for confirmation: %s", instanceName, prompt)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 10 * time.Minute
	return new(ConfirmStepBuilder).Init(cs)
}

func (b *ConfirmStepBuilder) WithAssumeYes(assumeYes bool) *ConfirmStepBuilder {
	b.Step.AssumeYes = assumeYes
	return b
}

func (s *ConfirmStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfirmStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if s.AssumeYes {
		logger.Info("AssumeYes is true, user prompt will be skipped.")
		return true, nil
	}
	return false, nil
}

func (s *ConfirmStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if s.AssumeYes {
		logger.Info("AssumeYes is true, proceeding automatically without prompt.")
		return nil
	}

	fmt.Println()
	fmt.Printf("------------------------------------------------------------------\n")
	fmt.Printf("🚨 ACTION REQUIRED: %s\n", s.Prompt)
	fmt.Printf("------------------------------------------------------------------\n")
	fmt.Printf("Enter 'yes' to continue, or 'no' to abort: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "yes" || input == "y" {
		logger.Info("User confirmed to continue.")
		fmt.Println()
		return nil
	}

	logger.Error("User declined to continue. Aborting workflow.")
	fmt.Println()
	return fmt.Errorf("user declined confirmation")
}

func (s *ConfirmStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for ConfirmStep is a no-op.")
	return nil
}

var _ step.Step = (*ConfirmStep)(nil)
