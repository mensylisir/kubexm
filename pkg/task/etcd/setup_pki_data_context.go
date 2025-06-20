package etcd

import (
	"fmt"
	// "path/filepath" // No longer needed directly here

	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki" // For NewSetupEtcdPkiDataContextStep and PKI data types
	// "github.com/mensylisir/kubexm/pkg/config" // Example if config needed for params
)

// NewSetupEtcdPkiDataContextTaskSpec creates a new TaskSpec for setting up etcd PKI data
// (like config, host details, paths) into the ModuleCache for subsequent PKI steps.
// Parameters:
//   kubeConfData: KubexmsKubeConf object containing PKI generation parameters.
//   hostsData: Slice of HostSpecForPKI providing details for each host needing PKI.
//   etcdSubPath: Specific sub-path for etcd PKI under the main PKI directory.
//   kubeConfKey: Cache key for storing KubeConfData. Uses step's default if empty.
//   hostsKey: Cache key for storing HostsData. Uses step's default if empty.
//   etcdPkiPathKey: Cache key for storing the resolved etcd PKI path. Uses step's default if empty.
//   runOnRoles: Specifies which host roles this task should target. For a data context setup task
//               that only populates cache, this is typically nil, as the step itself is host-agnostic
//               and runs locally from the orchestrator's perspective.
func NewSetupEtcdPkiDataContextTaskSpec(
	kubeConfData *pki.KubexmsKubeConf,
	hostsData []pki.HostSpecForPKI,
	etcdSubPath string,
	kubeConfKey, hostsKey, etcdPkiPathKey string,
	runOnRoles []string, // Typically nil for this type of task
) (*spec.TaskSpec, error) {

	if kubeConfData == nil {
		return nil, fmt.Errorf("KubeConfData cannot be nil for NewSetupEtcdPkiDataContextTaskSpec")
	}

	// The step itself will handle default cache keys if empty strings are passed.
	setupStep := pki.NewSetupEtcdPkiDataContextStep(
		kubeConfData,
		hostsData,
		etcdSubPath,
		kubeConfKey,    // KubeConfOutputKey
		hostsKey,       // HostsOutputKey
		etcdPkiPathKey, // EtcdPkiPathOutputKey
		"",             // Step name (use default from step)
	)

	return &spec.TaskSpec{
		Name:        "SetupEtcdPkiDataContext",
		Description: "Sets up etcd PKI data (config, hosts, paths) in ModuleCache for other PKI steps.",
		RunOnRoles:  runOnRoles, // Usually nil, step is host-agnostic (populates cache)
		Steps:       []spec.StepSpec{setupStep},
		IgnoreError: false, // Critical for PKI setup
		// Filter: "",
		// Concurrency: 1, // Only one instance of this data setup needed
	}, nil
}
