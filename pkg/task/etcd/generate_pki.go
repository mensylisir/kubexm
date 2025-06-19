package etcd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/pki" // For pki.HostSpecForAltNames and step specs
)

// NewGenerateEtcdPKITask creates a task to generate all necessary etcd PKI.
func NewGenerateEtcdPKITask(
	cfg *config.Cluster, // For consistency, though direct fields are passed for some steps
	altNameHosts []pki.HostSpecForAltNames,
	cpEndpoint string,
	defaultLBDomain string,
	// clusterPkiBaseDir string, // Removed: DetermineEtcdPKIPathStep now gets full path from module cache
) *spec.TaskSpec {
	return &spec.TaskSpec{
		Name:      "Generate Etcd PKI",
		LocalNode: true, // PKI generation is a local operation
		Steps: []spec.StepSpec{
			// Step 1: Determine/Ensure Etcd PKI Path
			// This step retrieves the specific etcd PKI path (e.g., .../.kubexm/pki/clusterName/etcd)
			// from the Module Cache (expected to be set by SetupEtcdPkiDataContextTask).
			// It ensures the directory exists and then places this path into the Task Cache
			// for subsequent steps in this task.
			&pki.DetermineEtcdPKIPathStepSpec{
				// PKIPathToEnsureSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (read from Module Cache)
				// OutputPKIPathSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (written to Task Cache)
			},

			// Step 2: Generate Etcd AltNames
			&pki.GenerateEtcdAltNamesStepSpec{
				Hosts:                      altNameHosts, // Directly passed by module
				ControlPlaneEndpointDomain: cpEndpoint,   // Directly passed by module
				DefaultLBDomain:            defaultLBDomain, // Directly passed by module
				// OutputAltNamesSharedDataKey defaults to pki.DefaultEtcdAltNamesKey (written to Task Cache)
			},

			// Step 3: Generate Etcd CA Certificate
			&pki.GenerateEtcdCAStepSpec{
				// PKIPathSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (read from Task Cache)
				// KubeConfSharedDataKey defaults to pki.DefaultKubeConfKey (read from Module Cache)
				// Output keys for CA cert object, cert path, key path use defaults (written to Task Cache)
			},

			// Step 4: Generate Etcd Node Certificates
			&pki.GenerateEtcdNodeCertsStepSpec{
				// Relies on default keys to get data from Module and Task Caches:
				// - PKIPath (Task Cache from DetermineEtcdPKIPathStep)
				// - AltNames (Task Cache from GenerateEtcdAltNamesStep)
				// - CA Cert Object (Task Cache from GenerateEtcdCAStep)
				// - KubeConf (Module Cache from SetupEtcdPkiDataContextTask)
				// - Hosts list (Module Cache from SetupEtcdPkiDataContextTask)
				// OutputGeneratedFilesListKey uses its default (written to Task Cache)
			},
		},
	}
}
