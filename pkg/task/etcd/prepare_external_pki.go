package etcd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/pki"
)

// NewPrepareExternalEtcdPKITask creates a task to prepare local PKI using user-provided external etcd certs.
func NewPrepareExternalEtcdPKITask(
	cfg *config.Cluster, // Used to get paths for external certs
	// clusterPkiBaseDir string, // Removed: DetermineEtcdPKIPathStep now gets full path from module cache
) *spec.TaskSpec {

	var caFile, certFile, keyFile string
	// Ensure Etcd and External fields are not nil before accessing them
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.External != nil {
		caFile = cfg.Spec.Etcd.External.CAFile
		certFile = cfg.Spec.Etcd.External.CertFile
		keyFile = cfg.Spec.Etcd.External.KeyFile
	}
	// If paths are empty, PrepareExternalEtcdCertsStepSpec's Execute method handles it.

	return &spec.TaskSpec{
		Name:      "Prepare PKI for External Etcd",
		LocalNode: true, // This task copies local files to a structured local PKI path
		Steps: []spec.StepSpec{
			// Step 1: Determine/Ensure Local Etcd PKI Path
			// Retrieves the specific etcd PKI path from Module Cache (set by SetupEtcdPkiDataContextTask),
			// ensures the directory exists locally, and places path into Task Cache.
			&pki.DetermineEtcdPKIPathStepSpec{
				// PKIPathToEnsureSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (read from Module Cache)
				// OutputPKIPathSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (written to Task Cache)
			},

			// Step 2: Copy User-Provided External Etcd Certificates
			&pki.PrepareExternalEtcdCertsStepSpec{
				ExternalEtcdCAFile:   caFile,   // Sourced from cfg by this constructor
				ExternalEtcdCertFile: certFile, // Sourced from cfg by this constructor
				ExternalEtcdKeyFile:  keyFile,  // Sourced from cfg by this constructor
				// TargetPKIPathSharedDataKey defaults to pki.DefaultEtcdPKIPathKey (read from Task Cache).
				// OutputCopiedFilesListKey uses its default.
			},
		},
	}
}
