package etcd

import (
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki"
)

// NewPrepareExistingEtcdPKITask creates a task to fetch PKI from an existing etcd cluster.
func NewPrepareExistingEtcdPKITask(
	cfg *config.Cluster, // For consistency and potential future use
	// clusterPkiBaseDir string, // Removed: DetermineEtcdPKIPathStep now gets full path from module cache
) *spec.TaskSpec {
	// The HostFilter for this task (to run on a single etcd node)
	// will be set by the module when it appends this task.
	return &spec.TaskSpec{
		Name: "Prepare PKI from Existing Etcd Cluster",
		// HostFilter will be set by the module
		Steps: []spec.StepSpec{
			// Step 1: Determine/Ensure Local Etcd PKI Path
			// Retrieves the specific etcd PKI path from Module Cache (set by SetupEtcdPkiDataContextTask),
			// ensures the directory exists locally, and places path into Task Cache.
			&pki.DetermineEtcdPKIPathStepSpec{
				// PKIPathToEnsureSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (read from Module Cache)
				// OutputPKIPathSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (written to Task Cache)
			},

			// Step 2: Fetch Existing Etcd Certificates from a remote Etcd node
			&pki.FetchExistingEtcdCertsStepSpec{
				// RemoteCertDir uses its default ("/etc/ssl/etcd/ssl") or could be made configurable.
				// TargetPKIPathSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (read from Task Cache).
				// OutputFetchedFilesListKey uses its default (written to Task Cache).
			},
		},
	}
}
