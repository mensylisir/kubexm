package etcd

import (
	// "fmt"
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/pki"
)

// NewPrepareExistingEtcdPKITask creates a task to fetch PKI from an existing etcd cluster.
func NewPrepareExistingEtcdPKITask(
	cfg *config.Cluster, // For consistency and potential future use (e.g. getting RemoteCertDir)
	// clusterPkiBaseDir string, // No longer needed, path comes from cache
) *spec.TaskSpec {
	// The HostFilter for this task (to run on a single etcd node)
	// will be set by the module when it appends this task.
	return &spec.TaskSpec{
		Name: "Prepare PKI from Existing Etcd Cluster",
		// HostFilter will be set by the module
		Steps: []spec.StepSpec{
			// This step now gets the full etcd PKI path from Module Cache (set by SetupEtcdPkiDataContextStep)
			// via its PKIPathToEnsureSharedDataKey (which defaults to pki.DefaultEtcdPKIPathKey).
			&pki.DetermineEtcdPKIPathStepSpec{},
			&pki.FetchExistingEtcdCertsStepSpec{
				// RemoteCertDir uses its default ("/etc/ssl/etcd/ssl") or could be made configurable here from cfg.
				// TargetPKIPathSharedDataKey uses default (pki.DefaultEtcdPKIPathKey), taking output from previous step.
				// OutputFetchedFilesListKey uses its default.
			},
		},
	}
}
