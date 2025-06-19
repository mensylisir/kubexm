package etcd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/pki"
)

// NewSetupEtcdPkiDataContextTask creates a task to populate the module cache with PKI context.
func NewSetupEtcdPkiDataContextTask(
	cfg *config.Cluster, // Not directly used by task constructor, but passed for consistency or future use
	kubexmsKubeConf *pki.KubexmsKubeConf,
	hostsForPki []pki.HostSpecForPKI,
	// clusterPkiBaseDir is now part of kubexmsKubeConf.PKIDirectory
) *spec.TaskSpec {
	return &spec.TaskSpec{
		Name:      "Setup Etcd PKI Data Context",
		LocalNode: true, // This task runs locally
		Steps: []spec.StepSpec{
			&pki.SetupEtcdPkiDataContextStepSpec{
				KubeConfForCache:    kubexmsKubeConf,
				HostsForPKIForCache: hostsForPki,
				// EtcdPkiPathForCache field removed from spec as per previous refinement,
				// path determination is handled by DetermineEtcdPKIPathStep.
				// The PKIDirectory in KubeConfForCache serves as the base for DetermineEtcdPKIPathStep.

				// Output keys will use defaults defined in SetupEtcdPkiDataContextStepSpec.PopulateDefaults()
				// e.g., KubeConfOutputKey: pki.DefaultKubeConfKey
				//       HostsForPKIOutputKey: pki.DefaultHostsKey
			},
		},
	}
}
