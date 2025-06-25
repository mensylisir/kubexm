package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module" // For module.Module, module.BaseModule, module.ModuleContext
	"github.com/mensylisir/kubexm/pkg/plan"
	// "github.com/mensylisir/kubexm/pkg/runtime" // Removed
	"github.com/mensylisir/kubexm/pkg/task" // For task.Task, task.ExecutionFragment, task.TaskContext
	"github.com/mensylisir/kubexm/pkg/task/greeting"
	"github.com/mensylisir/kubexm/pkg/task/pre"
	// taskPreflight "github.com/mensylisir/kubexm/pkg/task/preflight" // Keep if SystemChecksTask etc. are still used
)

// PreflightModule defines the module for preflight checks and setup.
type PreflightModule struct {
	module.BaseModule // Embed BaseModule
	AssumeYes         bool
}

// NewPreflightModule creates a new PreflightModule.
// It initializes the tasks that this module will orchestrate.
func NewPreflightModule(assumeYes bool) module.Module { // Returns module.Module interface
	// Define the sequence of tasks for this module, using refactored task names/constructors
	moduleTasks := []task.Task{
		greeting.NewGreetingTask(), // Runs first, no dependencies
		// Confirmation can happen early
		pre.NewConfirmTask("InitialConfirmation", "Proceed with KubeXM operations?", assumeYes),
		// SystemChecks can run broadly
		NewSystemChecksTask(nil), // Assuming NewSystemChecksTask takes roles, nil for all relevant
		// Initial OS Setup (Firewall, SELinux, Swap)
		NewInitialNodeSetupTask(), // Uses refactored task
		// Kernel Setup (Modules, Sysctl)
		NewSetupKernelTask(), // Uses refactored task
		// Offline/Artifact related tasks (conditionally run)
		pre.NewVerifyArtifactsTask(),
		pre.NewCreateRepositoryTask(),
	}
	// Note: The old `pre.NewPreTask()` and `taskpreflight.NewNodePreflightChecksTask()`
	// have been replaced by the more granular `NewSystemChecksTask`, `NewInitialNodeSetupTask`, `NewSetupKernelTask`.

	base := module.NewBaseModule("PreflightChecksAndSetup", moduleTasks)
	pm := &PreflightModule{
		BaseModule: base,
		AssumeYes:  assumeYes,
	}
	return pm
}

// Plan generates the execution fragment for the preflight module.
func (m *PreflightModule) Plan(ctx module.ModuleContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("module", m.Name())
	moduleFragment := task.NewExecutionFragment(m.Name() + "-Fragment")

	taskCtx, ok := ctx.(task.TaskContext)
	if !ok {
		return nil, fmt.Errorf("module context cannot be asserted to task.TaskContext for PreflightModule")
	}

	var previousTaskExitNodes []plan.NodeID

	// Define explicit task instances for clarity in linking
	greetingTask := greeting.NewGreetingTask()
	confirmTask := pre.NewConfirmTask("InitialConfirmation", "Proceed with KubeXM operations?", m.AssumeYes)
	systemChecksTask := NewSystemChecksTask(nil) // Use the constructor from this package if it's defined here, or import preflight.NewSystemChecksTask
	initialNodeSetupTask := NewInitialNodeSetupTask()
	kernelSetupTask := NewSetupKernelTask()
	verifyArtifactsTask := pre.NewVerifyArtifactsTask()
	createRepoTask := pre.NewCreateRepositoryTask()

	// Order and link tasks
	// 1. Greeting
	greetingFrag, err := greetingTask.Plan(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to plan GreetingTask: %w", err) }
	if err := moduleFragment.MergeFragment(greetingFrag); err != nil { return nil, err }
	moduleFragment.EntryNodes = append(moduleFragment.EntryNodes, greetingFrag.EntryNodes...)
	previousTaskExitNodes = greetingFrag.ExitNodes

	// 2. Confirm (depends on Greeting finishing, just for sequence)
	if !m.AssumeYes { // Only plan confirm task if not assuming yes
		confirmTaskRequired, _ := confirmTask.IsRequired(taskCtx) // Should be true if !m.AssumeYes
		if confirmTaskRequired {
			confirmFrag, err := confirmTask.Plan(taskCtx)
			if err != nil { return nil, fmt.Errorf("failed to plan ConfirmTask: %w", err) }
			if err := moduleFragment.MergeFragment(confirmFrag); err != nil { return nil, err }
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, confirmFrag.EntryNodes)
			previousTaskExitNodes = confirmFrag.ExitNodes
		}
	}

	// 3. System Checks (can run after confirmation, or parallel to greeting if confirmation is very first)
	// For simplicity, let's make it depend on confirmation (or greeting if no confirmation)
	systemChecksFrag, err := systemChecksTask.Plan(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to plan SystemChecksTask: %w", err) }
	if err := moduleFragment.MergeFragment(systemChecksFrag); err != nil { return nil, err }
	plan.LinkFragments(moduleFragment, previousTaskExitNodes, systemChecksFrag.EntryNodes)
	// System checks are broad, their exits will be dependencies for many subsequent things.
	systemChecksExits := systemChecksFrag.ExitNodes


	// 4. Initial Node Setup (depends on System Checks being done and confirmation)
	initialSetupFrag, err := initialNodeSetupTask.Plan(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to plan InitialNodeSetupTask: %w", err) }
	if err := moduleFragment.MergeFragment(initialSetupFrag); err != nil { return nil, err }
	plan.LinkFragments(moduleFragment, systemChecksExits, initialSetupFrag.EntryNodes) // Depends on system checks

	// 5. Kernel Setup (depends on Initial Node Setup)
	kernelSetupFrag, err := kernelSetupTask.Plan(taskCtx)
	if err != nil { return nil, fmt.Errorf("failed to plan KernelSetupTask: %w", err) }
	if err := moduleFragment.MergeFragment(kernelSetupFrag); err != nil { return nil, err }
	plan.LinkFragments(moduleFragment, initialSetupFrag.ExitNodes, kernelSetupFrag.EntryNodes)
	previousTaskExitNodes = kernelSetupFrag.ExitNodes // This is now the main line pre-OS-config exit

	// --- Conditional Offline Tasks ---
	// These should be checked with IsRequired and depend on previous setup stages.
	// Example: if clusterConfig.Spec.OfflineMode == true
	offlineMode := false // TODO: Determine from ctx.GetClusterConfig().Spec.SomeOfflineFlag

	if offlineMode {
		verifyArtifactsRequired, _ := verifyArtifactsTask.IsRequired(taskCtx)
		if verifyArtifactsRequired {
			verifyArtifactsFrag, err := verifyArtifactsTask.Plan(taskCtx)
			if err != nil { return nil, fmt.Errorf("failed to plan VerifyArtifactsTask: %w", err) }
			if err := moduleFragment.MergeFragment(verifyArtifactsFrag); err != nil { return nil, err }
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, verifyArtifactsFrag.EntryNodes)
			previousTaskExitNodes = verifyArtifactsFrag.ExitNodes
		}

		createRepoRequired, _ := createRepoTask.IsRequired(taskCtx)
		if createRepoRequired {
			createRepoFrag, err := createRepoTask.Plan(taskCtx)
			if err != nil { return nil, fmt.Errorf("failed to plan CreateRepositoryTask: %w", err) }
			if err := moduleFragment.MergeFragment(createRepoFrag); err != nil { return nil, err }
			plan.LinkFragments(moduleFragment, previousTaskExitNodes, createRepoFrag.EntryNodes)
			previousTaskExitNodes = createRepoFrag.ExitNodes
		}
	}
	// --- End Conditional Offline Tasks ---


	// Recalculate final entry/exit nodes for the entire module fragment
	moduleFragment.CalculateEntryAndExitNodes()

	if len(moduleFragment.Nodes) == 0 {
		logger.Info("Preflight module planned no executable nodes.")
	} else {
		logger.Info("Preflight module planning complete.", "totalNodes", len(moduleFragment.Nodes), "entryNodes", moduleFragment.EntryNodes, "exitNodes", moduleFragment.ExitNodes)
	}

	return moduleFragment, nil
}

// uniqueNodeIDs is now in pkg/plan/graph_plan.go

// Ensure PreflightModule implements the module.Module interface.
var _ module.Module = (*PreflightModule)(nil)
