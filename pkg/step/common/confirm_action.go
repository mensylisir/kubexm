package common

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector" // Host parameter is not used by logic but part of interface
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ConfirmActionStepSpec defines parameters for prompting the user for confirmation.
type ConfirmActionStepSpec struct {
	spec.StepMeta `json:",inline"`

	PromptMessage string `json:"promptMessage,omitempty"` // Required
	AssumeYes     bool   `json:"assumeYes,omitempty"`
	// DefaultChoice string `json:"defaultChoice,omitempty"` // Advanced: e.g., "yes", "no"
	// RetryLimit    int    `json:"retryLimit,omitempty"`    // Advanced
}

// NewConfirmActionStepSpec creates a new ConfirmActionStepSpec.
func NewConfirmActionStepSpec(name, description, promptMessage string) *ConfirmActionStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "User Confirmation"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if promptMessage == "" {
		// This is a required field.
	}

	return &ConfirmActionStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		PromptMessage: promptMessage,
		// AssumeYes defaults to false
	}
}

// Name returns the step's name.
func (s *ConfirmActionStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ConfirmActionStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ConfirmActionStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ConfirmActionStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ConfirmActionStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ConfirmActionStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ConfirmActionStepSpec) populateDefaults(logger runtime.Logger) {
	// AssumeYes defaults to false (Go zero value).
	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Prompts user for confirmation: '%s'", s.PromptMessage)
		if s.AssumeYes {
			s.StepMeta.Description += " (will assume yes)"
		}
	}
}

// Precheck validates inputs and checks AssumeYes flag.
func (s *ConfirmActionStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger) // Host not used by this step's populateDefaults

	if s.PromptMessage == "" {
		return false, fmt.Errorf("PromptMessage must be specified for %s", s.GetName())
	}

	if s.AssumeYes {
		logger.Info("AssumeYes is true, confirmation step will be skipped by Run if called.")
		// This step being "done" means Run doesn't need to prompt.
		// If AssumeYes is true, the action is implicitly confirmed.
		return true, nil
	}

	return false, nil // Needs user interaction in Run phase
}

// Run prompts the user for confirmation.
// IMPORTANT: This implementation uses os.Stdin/os.Stdout and is suitable for local execution
// by the main kubexm process. It will not work as expected if executed on a remote host
// via a standard connector. The Executor would need special handling for this step type.
func (s *ConfirmActionStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName())
	if host != nil { // Log host if provided, though step is conceptually local
		logger = logger.With("host", host.GetName())
	}
	logger = logger.With("phase", "Run")
	s.populateDefaults(logger)


	if s.PromptMessage == "" { // Should have been caught by Precheck if it ran
		return fmt.Errorf("PromptMessage must be specified for %s", s.GetName())
	}

	if s.AssumeYes {
		logger.Info("Confirmation assumed 'yes' due to AssumeYes=true flag.")
		return nil
	}

	// Direct console interaction
	// This makes the step only runnable on the machine where the main CLI tool is interacting with the user.
	// The executor needs to be aware of such steps.
	fmt.Printf("%s [yes/no]: ", s.PromptMessage)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	input = strings.ToLower(strings.TrimSpace(input))

	if input == "yes" || input == "y" {
		logger.Info("User confirmed action.", "input", input)
		return nil
	}
	if input == "no" || input == "n" {
		logger.Warn("User denied action.", "input", input)
		return fmt.Errorf("user aborted: action not confirmed for '%s'", s.PromptMessage)
	}

	// Invalid input (Retry logic could be added here if s.RetryLimit was implemented)
	logger.Error("Invalid input from user.", "input", input)
	return fmt.Errorf("invalid input '%s': please type 'yes' or 'no'", input)
}

// Rollback for ConfirmActionStep is a no-op.
func (s *ConfirmActionStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName())
	if host != nil { logger = logger.With("host", host.GetName()) }
	logger = logger.With("phase", "Rollback")
	logger.Debug("ConfirmActionStep has no rollback action.")
	return nil
}

var _ step.Step = (*ConfirmActionStepSpec)(nil)
