package engine

import (
	"context" // Added for runtime.NewContextWithGoContext
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mensylisir/kubexm/pkg/connector" // Updated import path
	"github.com/mensylisir/kubexm/pkg/plan"      // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime"   // Updated import path
	"github.com/mensylisir/kubexm/pkg/step"      // Updated import path
)

// defaultExecutor is the Engine interface's default implementation.
type defaultExecutor struct{}

// NewExecutor creates a new default execution engine.
func NewExecutor() Engine {
	return &defaultExecutor{}
}

func (e *defaultExecutor) Execute(ctx *runtime.Context, p *plan.ExecutionPlan, dryRun bool) (*plan.ExecutionResult, error) {
	result := plan.NewExecutionResult() // Using the helper from plan.go

	if p == nil || len(p.Phases) == 0 {
		result.SetStatus(plan.StatusSuccess) // Use helper
		result.Finalize() // Use helper
		if ctx.Logger != nil { // Check logger for nil
			ctx.Logger.Info("Execution plan is empty, nothing to do.")
		}
		return result, nil
	}

	if dryRun {
		e.dryRun(ctx, p, result) // Pass result to dryRun to populate it
		result.SetStatus(plan.StatusSuccess) // Dry run itself is a success if no errors printing
		result.Finalize()
		if ctx.Logger != nil {
			ctx.Logger.Info("Dry run complete.")
		}
		return result, nil
	}

	if ctx.Logger != nil {
		ctx.Logger.Info("Starting execution of the plan...")
	}
	result.SetStatus(plan.StatusRunning)

	for _, phase := range p.Phases {
		phaseResult := plan.NewPhaseResult(phase.Name) // Use helper
		result.AddPhaseResult(phaseResult)             // Use helper
		phaseResult.SetStatus(plan.StatusRunning)

		// Create a new Go context for this phase that can be cancelled
		// if any action within the phase fails catastrophically.
		// runtime.Context itself has a GoContext, use it as parent.
		g, phaseGoCtx := errgroup.WithContext(ctx.GoContext())

		for _, action := range phase.Actions {
			currentAction := action // Capture range variable for goroutine
			actionResult := plan.NewActionResult(currentAction.Name) // Use helper
			phaseResult.AddActionResult(actionResult) // Use helper
			actionResult.Status = plan.StatusRunning // Set initial status

			g.Go(func() error {
				// Create a new runtime.Context for this action, deriving from the phase's Go context.
				// This is important if runtime.Context carries more than just GoContext.
				actionRuntimeCtx := runtime.NewContextWithGoContext(phaseGoCtx, ctx)

				// Execute the action (which runs steps on hosts)
				err := e.runAction(actionRuntimeCtx, currentAction, actionResult) // Pass actionResult

				// Finalize status of actionResult based on hostResults
				actionResult.Finalize() // This will set Success/Failed/Skipped based on HostResults

				if err != nil {
					// If runAction returned an error, it means the action goroutine itself failed,
					// which is more severe than individual host failures.
					// Mark action as failed, the error will propagate to errgroup.
					actionResult.Status = plan.StatusFailed
					return fmt.Errorf("action '%s' in phase '%s' encountered an error: %w", currentAction.Name, phase.Name, err)
				}
				// If actionResult.Status is already Failed (due to a host failure), errgroup will still get nil here
				// but the phase will be marked as failed later.
				if actionResult.Status == plan.StatusFailed {
				    // Return a sentinel error to stop other actions in the same phase if desired (errgroup behavior)
				    // Or, return nil to allow other actions in the phase to continue, and just record failure.
				    // For now, let errgroup continue other actions unless a setup error occurs in runAction.
				    return nil // fmt.Errorf("action '%s' failed on one or more hosts", currentAction.Name)
				}
				return nil
			})
		} // end actions loop

		if err := g.Wait(); err != nil {
			// This error is from one of the goroutines (e.g., a setup error in runAction or a returned error)
			phaseResult.SetStatus(plan.StatusFailed) // Use helper
			result.SetStatus(plan.StatusFailed)      // Use helper
			result.Finalize()                        // Use helper
			if ctx.Logger != nil {
				ctx.Logger.Error(err, fmt.Sprintf("Error executing phase '%s'", phase.Name))
			}
			return result, fmt.Errorf("error in phase '%s': %w", phase.Name, err)
		}
		phaseResult.Finalize() // Determine phase status from its actions
		if phaseResult.Status == plan.StatusFailed { // If any action failed, the phase failed
		    result.SetStatus(plan.StatusFailed)
		}
	} // end phases loop

	result.Finalize() // Determine overall status
	if ctx.Logger != nil {
		ctx.Logger.Info("Plan execution finished.", "status", result.Status, "duration", result.EndTime.Sub(result.StartTime))
	}
	return result, nil
}

// runAction executes a single action and populates the actionResult.
func (e *defaultExecutor) runAction(ctx *runtime.Context, action plan.Action, actionResult *plan.ActionResult) error {
	var actionErr error
	g, actionGoCtx := errgroup.WithContext(ctx.GoContext())

	for _, host := range action.Hosts {
		currentHost := host // Capture range variable
		g.Go(func() error {
			// Create a new runtime.Context for this specific step execution on a host
			stepRuntimeCtx := runtime.NewContextWithGoContext(actionGoCtx, ctx)

			hostResult := e.runStepOnHost(stepRuntimeCtx, currentHost, action.Step)
			actionResult.SetHostResult(currentHost.GetName(), hostResult) // Use helper

			if hostResult.Status == plan.StatusFailed {
				// Store the first error encountered to potentially return, but continue other hosts
				if actionErr == nil {
					actionErr = fmt.Errorf("step '%s' failed on host '%s': %s", action.Step.Name(), currentHost.GetName(), hostResult.Message)
				}
				// Decide if one host failure should cancel others in the same action.
				// errgroup.WithContext will handle cancellation if this goroutine returns an error.
				// For now, let's return the error to signal failure for this host.
				return fmt.Errorf(hostResult.Message)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
	    // This means at least one step execution on a host failed and returned an error.
	    // actionErr would also be set.
	    // The actionResult's status will be determined by its Finalize method based on all hostResults.
		return err // Propagate the error to mark the action goroutine in Execute as failed.
	}
	return nil // All hosts in the action completed without returning a direct error to the errgroup.
}

// runStepOnHost executes a step on a single host and returns its result.
func (e *defaultExecutor) runStepOnHost(ctx *runtime.Context, host connector.Host, s step.Step) *plan.HostResult {
	hr := &plan.HostResult{HostName: host.GetName(), Status: plan.StatusRunning}
	// runtime.Context itself should be usable as StepContext, or provide one.
	// This depends on the design of runtime.Context and StepContext.
	// The issue shows runtime.Context.NewStepContext().
	stepCtx := ctx.NewStepContext() // Assuming this method exists on *runtime.Context

	// Precheck
	if ctx.Logger != nil {
		ctx.Logger.V(1).Info("Running Precheck for step", "step", s.Name(), "host", host.GetName())
	}
	isDone, err := s.Precheck(stepCtx, host)
	if err != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Precheck failed for step '%s': %v", s.Name(), err)
		if ctx.Logger != nil {
			ctx.Logger.Error(err, hr.Message, "step", s.Name(), "host", host.GetName())
		}
		return hr
	}
	if isDone {
		hr.Status = plan.StatusSkipped
		hr.Skipped = true
		hr.Message = fmt.Sprintf("Skipped: Precheck condition for step '%s' already met.", s.Name())
		if ctx.Logger != nil {
			ctx.Logger.Info(hr.Message, "step", s.Name(), "host", host.GetName())
		}
		return hr
	}

	// Run
	if ctx.Logger != nil {
		ctx.Logger.Info("Running step", "step", s.Name(), "description", s.Description(), "host", host.GetName())
	}
	err = s.Run(stepCtx, host)
	if err != nil {
		hr.Status = plan.StatusFailed
		hr.Message = fmt.Sprintf("Run failed for step '%s': %v", s.Name(), err)
		if cmdErr, ok := err.(*connector.CommandError); ok { // Check if it's a CommandError
			hr.Stdout = cmdErr.Stdout
			hr.Stderr = cmdErr.Stderr
		}
		if ctx.Logger != nil {
			ctx.Logger.Error(err, hr.Message, "step", s.Name(), "host", host.GetName())
		}

		// Attempt Rollback
		if ctx.Logger != nil {
			ctx.Logger.Info("Attempting rollback for step", "step", s.Name(), "host", host.GetName())
		}
		if rbErr := s.Rollback(stepCtx, host); rbErr != nil {
			rbMsg := fmt.Sprintf("Rollback failed for step '%s' after run error: %v", s.Name(), rbErr)
			hr.Message = fmt.Sprintf("%s. %s", hr.Message, rbMsg) // Append rollback failure to original message
			if ctx.Logger != nil {
				ctx.Logger.Error(rbErr, "Rollback failed", "step", s.Name(), "host", host.GetName())
			}
		} else {
			if ctx.Logger != nil {
				ctx.Logger.Info("Rollback successful for step", "step", s.Name(), "host", host.GetName())
			}
		}
		return hr
	}

	hr.Status = plan.StatusSuccess
	hr.Message = fmt.Sprintf("Step '%s' executed successfully.", s.Name())
	if ctx.Logger != nil {
		ctx.Logger.Info(hr.Message, "step", s.Name(), "host", host.GetName())
	}
	return hr
}

// dryRun prints the execution plan.
// It now also populates the result structure with phases and actions, marking them as skipped.
func (e *defaultExecutor) dryRun(ctx *runtime.Context, p *plan.ExecutionPlan, result *plan.ExecutionResult) {
	if ctx.Logger != nil {
		ctx.Logger.Info("--- Dry Run Execution Plan ---")
	}
	fmt.Println("--- Dry Run Execution Plan ---")

	for _, phase := range p.Phases {
		phaseResult := plan.NewPhaseResult(phase.Name)
		phaseResult.SetStatus(plan.StatusSkipped) // Mark phase as skipped for dry run
		result.AddPhaseResult(phaseResult)

		if ctx.Logger != nil {
			ctx.Logger.Info("Phase", "name", phase.Name)
		}
		fmt.Printf("  Phase: %s\n", phase.Name)

		for _, action := range phase.Actions {
			actionResult := plan.NewActionResult(action.Name)
			actionResult.Status = plan.StatusSkipped // Mark action as skipped
			phaseResult.AddActionResult(actionResult)

			if ctx.Logger != nil {
				ctx.Logger.Info("  Action", "name", action.Name, "step", action.Step.Name(), "description", action.Step.Description())
			}
			fmt.Printf("    Action: %s (Step: %s - %s)\n", action.Name, action.Step.Name(), action.Step.Description())

			hostNames := []string{}
			for _, h := range action.Hosts {
				hostNames = append(hostNames, h.GetName())
				// For dry run, populate host results as skipped
				hr := &plan.HostResult{
					HostName: h.GetName(),
					Status:   plan.StatusSkipped,
					Message:  "Dry run: Skipped.",
					Skipped:  true,
				}
				actionResult.SetHostResult(h.GetName(), hr)
			}
			if ctx.Logger != nil {
				ctx.Logger.Info("    Hosts", "names", hostNames)
			}
			fmt.Printf("      Hosts: %v\n", hostNames)
			actionResult.Finalize() // Finalize action based on (skipped) host results
		}
		phaseResult.Finalize() // Finalize phase based on (skipped) action results
	}
	if ctx.Logger != nil {
		ctx.Logger.Info("--- End of Dry Run ---")
	}
	fmt.Println("--- End of Dry Run ---")
}
