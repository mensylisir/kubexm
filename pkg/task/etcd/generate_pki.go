package etcd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/pki"
)

// NewGenerateEtcdPKITask creates a task to generate all necessary etcd PKI.
func NewGenerateEtcdPKITask(
	cfg *config.Cluster, // For consistency, though direct fields are passed
	altNameHosts []pki.HostSpecForAltNames,
	cpEndpoint string,
	defaultLBDomain string,
	// clusterPkiBaseDir string, // No longer needed here, path comes from cache
) *spec.TaskSpec {
	return &spec.TaskSpec{
		Name:      "Generate Etcd PKI",
		LocalNode: true, // PKI generation is a local operation
		Steps: []spec.StepSpec{
			// This step now gets the full etcd PKI path from Module Cache (set by SetupEtcdPkiDataContextStep)
			// via its PKIPathToEnsureSharedDataKey (which defaults to pki.DefaultEtcdPKIPathKey).
			// It then ensures the directory exists and puts the path into Task Cache using OutputPKIPathSharedDataKey.
			&pki.DetermineEtcdPKIPathStepSpec{},
			&pki.GenerateEtcdAltNamesStepSpec{
				Hosts:                      altNameHosts,
				ControlPlaneEndpointDomain: cpEndpoint,
				DefaultLBDomain:            defaultLBDomain,
				// OutputAltNamesSharedDataKey uses default pki.DefaultEtcdAltNamesKey
			},
			&pki.GenerateEtcdCAStepSpec{
				// Relies on default keys to get KubeConf from module cache (via SetupEtcdPkiDataContextTask)
				// and EtcdPKIPath from task cache (from DetermineEtcdPKIPathStep).
			},
			&pki.GenerateEtcdNodeCertsStepSpec{
				// Relies on default keys for KubeConf, Hosts (from SetupEtcdPkiDataContextTask in module cache),
				// EtcdPKIPath (from DetermineEtcdPKIPathStep in task cache),
				// AltNames (from GenerateEtcdAltNamesStep in task cache),
				// CA Cert Object (from GenerateEtcdCAStep in task cache).
			},
		},
	}
}
