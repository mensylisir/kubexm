package etcd

import (
	"fmt"

	// "github.com/mensylisir/kubexm/pkg/config" // No longer used
	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host in plan.Action, even if nil
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	// We will use step factories from pkg/step/pki.
	"github.com/mensylisir/kubexm/pkg/step/pki"
	"github.com/mensylisir/kubexm/pkg/task"
	// "github.com/mensylisir/kubexm/pkg/spec" // No longer used
)

// GenerateEtcdPkiTask orchestrates the generation of etcd PKI.
type GenerateEtcdPkiTask struct {
	*task.BaseTask
	AltNameHosts         []pki.HostSpecForAltNames // Used by GenerateEtcdAltNamesStep
	ControlPlaneEndpoint string                  // Used by GenerateEtcdAltNamesStep
	DefaultLBDomain      string                  // Used by GenerateEtcdAltNamesStep
	// Other parameters like PKI base paths, CA details might be sourced from ModuleCache
	// set by SetupEtcdPkiDataContextTask.
}

// NewGenerateEtcdPkiTask creates a new task for generating etcd PKI.
// Parameters like altNameHosts, cpEndpoint, and defaultLBDomain are typically
// derived by the module from the main cluster configuration.
func NewGenerateEtcdPkiTask(
	altNameHosts []pki.HostSpecForAltNames,
	cpEndpoint string,
	defaultLBDomain string,
) task.Interface {
	base := task.NewBaseTask(
		"GenerateEtcdPki",
		"Generates all necessary etcd PKI (CA, member, client certificates).",
		nil,   // This task and its steps are typically local/host-agnostic for planning.
		nil,   // No specific host filter at this task level.
		false, // Not ignoring errors by default.
	)
	return &GenerateEtcdPkiTask{
		BaseTask:             &base,
		AltNameHosts:         altNameHosts,
		ControlPlaneEndpoint: cpEndpoint,
		DefaultLBDomain:      defaultLBDomain,
	}
}

// Name is inherited from BaseTask.
// func (t *GenerateEtcdPkiTask) Name() string { return t.BaseTask.Name() }

// Description is inherited from BaseTask.
// func (t *GenerateEtcdPkiTask) Description() string { return t.BaseTask.Description() }

// IsRequired is inherited from BaseTask (defaults to true).
// This task usually runs if etcd is being set up or PKI needs regeneration.
// func (t *GenerateEtcdPkiTask) IsRequired(ctx runtime.TaskContext) (bool, error) { return true, nil }


// Plan generates the execution plan for creating etcd PKI.
func (t *GenerateEtcdPkiTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")
	execPlan := &plan.ExecutionPlan{}

	// Step 1: Determine/Ensure Etcd PKI Path
	// Assumes pki.NewDetermineEtcdPKIPathStep exists and uses default cache keys if empty strings are passed.
	step1 := pki.NewDetermineEtcdPKIPathStep(
		"", // PKIPathToEnsureSharedDataKey (input from ModuleCache, use default key in step)
		"", // OutputPKIPathSharedDataKey (output to TaskCache, use default key in step)
		"", // Step name (use default in step)
	)

	// Step 2: Generate Etcd AltNames
	// Assumes pki.NewGenerateEtcdAltNamesStep exists.
	step2 := pki.NewGenerateEtcdAltNamesStep(
		t.AltNameHosts,
		t.ControlPlaneEndpoint,
		t.DefaultLBDomain,
		"", // Output key for AltNames (use default in step)
		"", // Step name (use default in step)
	)

	// Step 3: Generate Etcd CA Certificate
	// Assumes pki.NewGenerateEtcdCAStep exists.
	step3 := pki.NewGenerateEtcdCAStep(
		"", // Input PKIPath key (use default from TaskCache)
		"", // Input KubeConf key (use default from ModuleCache)
		"", // Output CA Cert Object key (use default to TaskCache)
		"", // Output CA Cert Path key (use default to TaskCache)
		"", // Output CA Key Path key (use default to TaskCache)
		"", // Step name (use default)
	)

	// Step 4: Generate Etcd Node Certificates (members, clients)
	// Assumes pki.NewGenerateEtcdNodeCertsStep exists.
	step4 := pki.NewGenerateEtcdNodeCertsStep(
		"", // Input PKIPath key (TaskCache)
		"", // Input AltNames key (TaskCache)
		"", // Input CA Cert Object key (TaskCache)
		"", // Input KubeConf key (ModuleCache)
		"", // Input Hosts key (ModuleCache)
		"", // Output Generated Files List key (TaskCache)
		"", // Step name (use default)
	)

	// All these PKI generation steps are typically local operations (no specific remote host target).
	// Actions will have Hosts: nil, indicating they run on the control/local node context.
	actions := []plan.Action{
		{Name: "Determine/Ensure Etcd PKI Path", Step: step1, Hosts: nil},
		{Name: "Generate Etcd Certificate AltNames", Step: step2, Hosts: nil},
		{Name: "Generate Etcd CA", Step: step3, Hosts: nil},
		{Name: "Generate Etcd Node Certificates", Step: step4, Hosts: nil},
	}

	phase := plan.Phase{
		Name:    "Generate Etcd PKI Assets",
		Actions: actions,
	}
	execPlan.Phases = append(execPlan.Phases, phase)

	logger.Info("Planned steps for Etcd PKI generation.")
	return execPlan, nil
}

// Ensure GenerateEtcdPkiTask implements the task.Interface.
var _ task.Interface = (*GenerateEtcdPkiTask)(nil)
