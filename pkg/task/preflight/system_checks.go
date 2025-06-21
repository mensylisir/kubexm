package preflight

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For config type
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	stepPreflight "github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

// SystemChecksTask performs common system preflight checks.
type SystemChecksTask struct {
	taskName   string
	taskDesc   string
	runOnRoles []string // Roles this task should run on. Empty means all nodes (or as per module).
	cfg        *v1alpha1.Cluster
}

// NewSystemChecksTask creates a new SystemChecksTask.
func NewSystemChecksTask(cfg *v1alpha1.Cluster, roles []string) task.Task {
	return &SystemChecksTask{
		taskName:   "SystemPreflightChecks",
		taskDesc:   "Performs common system preflight checks like CPU, memory, and swap.",
		runOnRoles: roles, // Can be empty, IsRequired or Plan can default to all hosts.
		cfg:        cfg,
	}
}

// Name returns the name of the task.
func (t *SystemChecksTask) Name() string {
	return t.taskName
}

// IsRequired determines if this task needs to run.
// For preflight checks, this is generally true for all relevant nodes.
func (t *SystemChecksTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// If runOnRoles is empty, the module might decide to run it on all hosts.
	// Or, this task could be designed to always be "required" and target all hosts if roles are empty.
	// For simplicity, if roles are defined, check if any hosts match. If no roles, assume not required by default here,
	// deferring to module logic to call it for all hosts if needed.
	if len(t.runOnRoles) == 0 {
		// A system check task might be intended for all nodes by default if no roles.
		// Let's assume for now it's required if it reaches the Plan stage,
		// and host targeting will handle applicability.
		return true, nil
	}
	for _, role := range t.runOnRoles {
		hosts, err := ctx.GetHostsByRole(role)
		if err != nil {
			return false, fmt.Errorf("failed to get hosts for role '%s' in task %s: %w", role, t.Name(), err)
		}
		if len(hosts) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// Plan generates the execution fragment for system checks.
func (t *SystemChecksTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID

	targetHosts, err := t.determineTargetHosts(ctx)
	if err != nil {
		return nil, err
	}
	if len(targetHosts) == 0 {
		logger.Info("No target hosts for system checks task based on roles or defaults.")
		return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
	}

	// Default values for checks
	minCores := 2
	minMemoryMB := uint64(2048) // 2GB
	runDisableSwapStep := true // By default, try to disable swap
	sudoForSwap := true        // Default sudo for swap operations

	if t.cfg != nil && t.cfg.Spec.Preflight != nil {
		if t.cfg.Spec.Preflight.MinCPUCores > 0 {
			minCores = t.cfg.Spec.Preflight.MinCPUCores
		}
		if t.cfg.Spec.Preflight.MinMemoryMB > 0 {
			minMemoryMB = t.cfg.Spec.Preflight.MinMemoryMB
		}
		runDisableSwapStep = t.cfg.Spec.Preflight.DisableSwap
		// Sudo for swap might also be configurable if needed:
		// sudoForSwap = t.cfg.Spec.Preflight.SudoForSwapActions (assuming such a field)
	}

	// Node 1: CheckCPU
	// Assuming step constructors: NewCheckCPUStep(instanceName, minCores, sudo)
	cpuCheckStep := stepPreflight.NewCheckCPUStep("SystemCPUCheck", minCores, false) // CPU check itself usually doesn't need sudo
	nodeIDCPUCheck := plan.NodeID(fmt.Sprintf("%s-cpu-check", t.Name()))
	nodes[nodeIDCPUCheck] = &plan.ExecutionNode{
		Name:  "SystemCPUCheck",
		Step:  cpuCheckStep,
		Hosts: targetHosts,
	}
	entryNodes = append(entryNodes, nodeIDCPUCheck)

	// Node 2: CheckMemory
	memCheckStep := stepPreflight.NewCheckMemoryStep("SystemMemoryCheck", minMemoryMB, false) // Memory check also usually no sudo
	nodeIDMemoryCheck := plan.NodeID(fmt.Sprintf("%s-memory-check", t.Name()))
	nodes[nodeIDMemoryCheck] = &plan.ExecutionNode{
		Name:  "SystemMemoryCheck",
		Step:  memCheckStep,
		Hosts: targetHosts,
	}
	entryNodes = append(entryNodes, nodeIDMemoryCheck) // Can run in parallel with CPU check

	if runDisableSwapStep {
		disableSwapStep := stepPreflight.NewDisableSwapStep("DisableSwapMemory", sudoForSwap)
		nodeIDDisableSwap := plan.NodeID(fmt.Sprintf("%s-disable-swap", t.Name()))
		nodes[nodeIDDisableSwap] = &plan.ExecutionNode{
			Name:  "DisableSwapMemory",
			Step:  disableSwapStep,
			Hosts: targetHosts,
			// No explicit dependencies on CPU/Mem checks unless specified as a requirement
		}
		entryNodes = append(entryNodes, nodeIDDisableSwap) // Can run in parallel
	}

	// All these checks can typically be entry points and also exit points if they are independent.
	// If there were a final "report checks" step, that would be the single exit node.
	exitNodes = append(exitNodes, entryNodes...) // Copy all entry nodes to exit nodes

	logger.Info("Planned system preflight checks.", "hostCount", len(targetHosts), "nodesCount", len(nodes))
	return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
}

// determineTargetHosts helper function for this task
func (t *SystemChecksTask) determineTargetHosts(ctx runtime.TaskContext) ([]connector.Host, error) {
	var targetHosts []connector.Host
	var err error

	if len(t.runOnRoles) > 0 {
		for _, role := range t.runOnRoles {
			hosts, err := ctx.GetHostsByRole(role)
			if err != nil {
				return nil, fmt.Errorf("failed to get hosts for role '%s' in task %s: %w", role, t.Name(), err)
			}
			targetHosts = append(targetHosts, hosts...)
		}
		// Deduplicate
		uniqueHosts := make(map[string]connector.Host)
		for _, h := range targetHosts {
			uniqueHosts[h.GetName()] = h
		}
		targetHosts = []connector.Host{}
		for _, h := range uniqueHosts {
			targetHosts = append(targetHosts, h)
		}
	} else {
		// If no roles are specified for this task, assume it applies to all known hosts.
		// This behavior might be module-dependent or task-specific.
		// For preflight checks, running on all hosts is a common scenario.
		targetHosts, err = ctx.GetAllHosts() // Assumes TaskContext has GetAllHosts()
		if err != nil {
			return nil, fmt.Errorf("failed to get all hosts for task %s: %w", t.Name(), err)
		}
	}
	return targetHosts, nil
}

var _ task.Task = (*SystemChecksTask)(nil)
