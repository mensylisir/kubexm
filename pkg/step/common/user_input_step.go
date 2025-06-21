package common

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// UserInputStep prompts the user for confirmation.
type UserInputStep struct {
	meta      spec.StepMeta
	Prompt    string
	AssumeYes bool // If true, automatically proceeds without waiting for user input.
}

// NewUserInputStep creates a new UserInputStep.
func NewUserInputStep(instanceName, prompt string, assumeYes bool) step.Step {
	name := instanceName
	if name == "" {
		name = "GetUserConfirmation"
	}
	return &UserInputStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Prompts user for confirmation: '%s'", prompt),
		},
		Prompt:    prompt,
		AssumeYes: assumeYes,
	}
}

func (s *UserInputStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck for UserInputStep returns false if AssumeYes is false, true otherwise.
// If AssumeYes is true, the step is considered "done" as no input is required.
func (s *UserInputStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	if s.AssumeYes {
		ctx.GetLogger().Info("AssumeYes is true, skipping user prompt.", "step", s.meta.Name)
		return true, nil // Done, skip Run
	}
	return false, nil // Not done, Run will prompt
}

// Run prompts the user and waits for input.
func (s *UserInputStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name)
	if s.AssumeYes { // Double check, though Precheck should handle this.
		logger.Info("AssumeYes is true, proceeding automatically.")
		return nil
	}

	fmt.Printf("%s [yes/no]: ", s.Prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		logger.Error(err, "Failed to read user input.")
		return fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "yes" || input == "y" {
		logger.Info("User confirmed.")
		return nil
	}

	logger.Info("User declined.")
	return fmt.Errorf("user declined confirmation")
}

// Rollback for UserInputStep is a no-op.
func (s *UserInputStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	return nil
}

var _ step.Step = (*UserInputStep)(nil)
