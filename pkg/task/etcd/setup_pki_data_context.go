package etcd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector" // Not directly used by this task's logic, but Plan returns []connector.Host
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/pki" // For NewSetupEtcdPkiDataContextStep and PKI data types
	"github.com/mensylisir/kubexm/pkg/task"     // For task.Interface and task.BaseTask
)

// Define default keys if not imported from a central PKI consts file
const (
	DefaultKubeConfKey = "pkiKubeConfig"
	DefaultHostsKey    = "pkiHosts"
	// DefaultEtcdPKIPathKey is defined in pki package (determine_etcd_pki_path.go).
	// The pki.NewSetupEtcdPkiDataContextStep will use its own default if not provided.
)

// SetupEtcdPkiDataContextTask prepares and caches PKI-related data from the main
// cluster configuration into ModuleCache for use by other PKI steps.
type SetupEtcdPkiDataContextTask struct {
	*task.BaseTask
	KubeConfData *pki.KubexmsKubeConf
	HostsData    []pki.HostSpecForPKI
	EtcdSubPath  string

	// Cache keys (can be empty to use defaults set by the step)
	KubeConfOutputKey    string
	HostsOutputKey       string
	EtcdPkiPathOutputKey string
}

// NewSetupEtcdPkiDataContextTask creates a new task to populate ModuleCache with PKI context.
// KubeConfData and HostsData are typically prepared by the module from the main cluster config.
func NewSetupEtcdPkiDataContextTask(
	kubeConf *pki.KubexmsKubeConf,
	hosts []pki.HostSpecForPKI,
	etcdSubPath string,
	kubeConfKey, hostsKey, etcdPkiPathKey string,
) task.Interface {
	base := task.NewBaseTask(
		"SetupEtcdPkiDataContext",
		"Sets up etcd PKI data (config, hosts, paths) in ModuleCache.",
		nil,   // This task itself doesn't run on specific roles; its step is host-agnostic.
		nil,   // No host filter
		false, // Not ignoring errors
	)
	return &SetupEtcdPkiDataContextTask{
		BaseTask:             &base,
		KubeConfData:         kubeConf,
		HostsData:            hosts,
		EtcdSubPath:          etcdSubPath,
		KubeConfOutputKey:    kubeConfKey,
		HostsOutputKey:       hostsKey,
		EtcdPkiPathOutputKey: etcdPkiPathKey,
	}
}

// Name is inherited from BaseTask.
// func (t *SetupEtcdPkiDataContextTask) Name() string { return t.BaseTask.Name() }

// Description is inherited from BaseTask.
// func (t *SetupEtcdPkiDataContextTask) Description() string { return t.BaseTask.Description() }

// IsRequired is inherited from BaseTask (defaults to true).
// This task is usually always required if PKI setup is part of the module.
// func (t *SetupEtcdPkiDataContextTask) IsRequired(ctx runtime.TaskContext) (bool, error) { return true, nil }


// Plan generates the execution plan to run the SetupEtcdPkiDataContextStep.
func (t *SetupEtcdPkiDataContextTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")
	execPlan := &plan.ExecutionPlan{}

	if t.KubeConfData == nil {
	    return nil, fmt.Errorf("KubeConfData is nil for task %s, cannot plan SetupEtcdPkiDataContextStep", t.Name())
	}
	// HostsData can be nil/empty if not used by subsequent steps for this particular PKI setup.
	// EtcdSubPath can be empty, the step will apply its default.
	// Cache keys can be empty, the step will apply its defaults.

	// The step's populateDefaults will handle empty keys and EtcdSubPath.
	setupStep := pki.NewSetupEtcdPkiDataContextStep(
		t.KubeConfData,
		t.HostsData,
		t.EtcdSubPath,
		t.KubeConfOutputKey,
		t.HostsOutputKey,
		t.EtcdPkiPathOutputKey,
		"", // Step name (can use default from step)
	)

	// This step populates ModuleCache and doesn't run commands on specific target hosts.
	// It's considered a "local" or "control-plane" step.
	// The Engine should handle steps with nil Hosts, typically by running them once locally.
	action := plan.Action{
		Name:  "Populate ModuleCache with Etcd PKI context", // Action description
		Step:  setupStep,
		Hosts: nil, // Step is host-agnostic in its Run method
	}

	phase := plan.Phase{
		Name:    "Initialize Etcd PKI Context",
		Actions: []plan.Action{action},
	}
	execPlan.Phases = append(execPlan.Phases, phase)

	logger.Info("Planned SetupEtcdPkiDataContextStep to populate ModuleCache.")
	return execPlan, nil
}

// Ensure SetupEtcdPkiDataContextTask implements the task.Interface.
var _ task.Interface = (*SetupEtcdPkiDataContextTask)(nil)
