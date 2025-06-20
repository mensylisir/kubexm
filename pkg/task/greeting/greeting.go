package greeting

import (
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/common" // For PrintOutputStepSpec
)

// NewGreetingsTaskSpec creates a TaskSpec that displays a welcome message and logo.
func NewGreetingsTaskSpec() *spec.TaskSpec {
	logoArt := `
  _  __     _  __ __  __
 | |/ / ___| |/ /|  \/  |
 | ' < / _ \ ' < | |\/| |
 | . \ \  __/ . \| |  | |
 |_|\_\ \___|_|\_\_|  |_|  (KubeXM)

Welcome to KubeXM!
`
	// A slightly different logo attempt, simpler for ASCII representation.
	// If the one in prompt is preferred, can use that.
	// The one in prompt:
	// logoArt := `
	//    __ __ __ __ __ __ ________
	//   |  |  |  |  |  |  |        |
	//   |  |  |  |  |  |  |   __   |
	//   |  |  |  |  |  |  |  |  |  |
	//   |  ` + ` ' ` + ` |  ` + ` ' ` + ` |  |__|  |
	//    ` + ` ____ ` + `'|_______/|________|
	//      |    |
	// K U B E ` + `----` + `' X M
	//    ` + `
	//    `
	// For the above, the backticks need to be handled carefully in a Go raw string literal.
	// The Go parser might interpret internal backticks.
	// Using the simpler one above for now to avoid issues with backticks in backticked strings.

	logoStep := common.NewPrintOutputStepSpec(
		"PrintWelcomeLogo", // Step Name
		"Prints the KubeXM welcome logo and message.", // Step Description
		logoArt, // Content to print
	)
	logoStep.FormatAs = "raw" // Ensure it prints as-is without logger prefixes if possible

	return &spec.TaskSpec{
		Name:        "Greetings",
		Description: "Displays a welcome message and logo to the user.",
		RunOnRoles:  nil, // This task is typically run locally where the CLI is invoked, not on target hosts.
		Steps:       []spec.StepSpec{logoStep},
		IgnoreError: true, // Failure of a greeting message should not halt critical operations.
		// Filter: "", // No specific filter needed, runs locally.
		// Concurrency: 1, // Runs once.
	}
}
