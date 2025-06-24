package common

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// PrintMessageStep prints a message to the console.
// It's intended to run on the control node.
type PrintMessageStep struct {
	meta    spec.StepMeta
	Message string
}

// NewPrintMessageStep creates a new PrintMessageStep.
func NewPrintMessageStep(instanceName, message string) step.Step {
	name := instanceName
	if name == "" {
		name = "PrintMessage"
	}
	return &PrintMessageStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Prints a message to the console.",
		},
		Message: message,
	}
}

func (s *PrintMessageStep) Meta() *spec.StepMeta {
	return &s.meta
}

// Precheck for PrintMessageStep always returns false, as it's meant to always execute.
func (s *PrintMessageStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	return false, nil
}

// Run prints the message. It assumes this step is targeted at the control node
// and prints to the application's standard output.
func (s *PrintMessageStep) Run(ctx step.StepContext, host connector.Host) error {
	// The logger from StepContext could be used, but for direct user messages,
	// fmt.Println or a dedicated UI service in the runtime context might be more appropriate.
	// Using fmt.Println for direct console output.
	// The host parameter is part of the interface but might be ignored if this step
	// is always local to where kubexm is running.
	// For steps on control node, host would be the controlHost object.

	// If we want it in logs as well:
	// logger := ctx.GetLogger().With("step", s.meta.Name)
	// logger.Info(s.Message)

	fmt.Println(s.Message)
	return nil
}

// Rollback for PrintMessageStep is a no-op.
func (s *PrintMessageStep) Rollback(ctx step.StepContext, host connector.Host) error {
	return nil
}

var _ step.Step = (*PrintMessageStep)(nil)
