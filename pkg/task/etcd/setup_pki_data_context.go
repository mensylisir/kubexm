package etcd

import (
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki" // For pki.KubexmsKubeConf, pki.HostSpecForPKI, pki.SetupEtcdPkiDataContextStepSpec
)

// NewSetupEtcdPkiDataContextTask creates a task to populate the module cache with PKI context.
// This task is intended to run locally.
func NewSetupEtcdPkiDataContextTask(
	cfg *config.Cluster, // Passed for consistency, though not directly used by this specific task constructor
	kubexmsKubeConf *pki.KubexmsKubeConf, // This is the main data source, prepared by the module
	hostsForPki []pki.HostSpecForPKI,     // Also prepared by the module
) *spec.TaskSpec {

	// The SetupEtcdPkiDataContextStepSpec will have its fields (KubeConfToCache, HostsToCache, EtcdSpecificSubPath)
	// populated. Its PopulateDefaults method will set default cache keys and the default EtcdSpecificSubPath ("etcd").
	// The executor of this step will then:
	// 1. Use KubeConfToCache.AppFSBaseDir, KubeConfToCache.ClusterName, and spec.EtcdSpecificSubPath
	//    to determine the final etcd PKI path.
	// 2. Store KubeConfToCache into Module Cache using spec.KubeConfOutputKey (default: pki.DefaultKubeConfKey).
	// 3. Store HostsToCache into Module Cache using spec.HostsOutputKey (default: pki.DefaultHostsKey).
	// 4. Store the derived etcd PKI path into Module Cache using spec.EtcdPkiPathOutputKey (default: pki.DefaultEtcdPKIPathKey).

	setupStepSpec := &pki.SetupEtcdPkiDataContextStepSpec{
		KubeConfToCache:    kubexmsKubeConf,
		HostsToCache:       hostsForPki,
		// EtcdSpecificSubPath will use its default "etcd" via PopulateDefaults in the step.
		// Output keys (KubeConfOutputKey, HostsOutputKey, EtcdPkiPathOutputKey) will also use their defaults.
	}

	return &spec.TaskSpec{
		Name:      "Setup Etcd PKI Data Context",
		LocalNode: true, // This task runs locally on the control node
		Steps:     []spec.StepSpec{setupStepSpec},
	}
}
