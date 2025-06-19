package etcd

import (
	// "fmt"
	"github.com/kubexms/kubexms/pkg/config" // For cfg.Spec.Etcd.External types
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/pki"
)

// NewPrepareExternalEtcdPKITask creates a task to prepare local PKI using user-provided external etcd certs.
func NewPrepareExternalEtcdPKITask(
	cfg *config.Cluster, // Used to get paths for external certs
	// clusterPkiBaseDir string, // No longer needed, path comes from cache
) *spec.TaskSpec {

	var caFile, certFile, keyFile string
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.External != nil {
		caFile = cfg.Spec.Etcd.External.CAFile
		certFile = cfg.Spec.Etcd.External.CertFile
		keyFile = cfg.Spec.Etcd.External.KeyFile
	}

	return &spec.TaskSpec{
		Name:      "Prepare PKI for External Etcd",
		LocalNode: true, // This task copies local files to a structured local PKI path
		Steps: []spec.StepSpec{
			// This step now gets the full etcd PKI path from Module Cache (set by SetupEtcdPkiDataContextStep)
			// via its PKIPathToEnsureSharedDataKey (which defaults to pki.DefaultEtcdPKIPathKey).
			&pki.DetermineEtcdPKIPathStepSpec{},
			&pki.PrepareExternalEtcdCertsStepSpec{
				// ExternalEtcdCAFile, CertFile, KeyFile are populated from cfg by the module.
				ExternalEtcdCAFile:   caFile,
				ExternalEtcdCertFile: certFile,
				ExternalEtcdKeyFile:  keyFile,
				// TargetPKIPathSharedDataKey and OutputCopiedFilesListKey use defaults.
			},
		},
	}
}
