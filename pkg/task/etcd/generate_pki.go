package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki" // Assuming step factories return spec.StepSpec
	// "github.com/mensylisir/kubexm/pkg/config" // Example if config needed for params
)

// NewGenerateEtcdPkiTaskSpec creates a new TaskSpec for generating etcd PKI.
// Parameters:
//   altNameHosts: List of host specifications for SANs in certificates.
//   cpEndpoint: The control plane endpoint (VIP or DNS).
//   defaultLBDomain: Default load balancer domain, used if host specs don't provide FQDNs.
//   runOnRoles: Specifies which host roles this task should target. For PKI generation,
//               this might be nil or specific to a master/control-plane role, as these
//               steps are often executed locally on a node orchestrating the setup.
//
// Note: Many parameters for the individual PKI steps (like PKI paths, CA details)
// are expected to be resolved by the steps themselves, potentially using a shared
// ModuleCache or TaskCache populated by a preceding setup step (e.g., SetupEtcdPkiDataContextTask).
// The empty string arguments for keys in step constructors indicate that the steps
// should use their internally defined default cache keys.
func NewGenerateEtcdPkiTaskSpec(
	altNameHosts []pki.HostSpecForAltNames,
	cpEndpoint string,
	defaultLBDomain string,
	runOnRoles []string, // Typically nil or master/control-plane roles
) *spec.TaskSpec {

	taskSteps := []spec.StepSpec{
		// Step 1: Determine/Ensure Etcd PKI Path
		pki.NewDetermineEtcdPKIPathStep(
			"", // PKIPathToEnsureSharedDataKey (input from ModuleCache, use default key in step)
			"", // OutputPKIPathSharedDataKey (output to TaskCache, use default key in step)
			"", // Step name (use default in step)
		),
		// Step 2: Generate Etcd AltNames
		pki.NewGenerateEtcdAltNamesStep(
			altNameHosts,
			cpEndpoint,
			defaultLBDomain,
			"", // Output key for AltNames (use default in step)
			"", // Step name (use default in step)
		),
		// Step 3: Generate Etcd CA Certificate
		// This step is assumed to be equivalent to GenerateCAWithCRTStepSpec for the Etcd CA,
		// meaning it will write its output paths to ModuleCache using the standard keys
		// "ClusterRootCACertPath" and "ClusterRootCAKeyPath".
		// The specific NewGenerateEtcdCAStep factory might need to be adjusted internally
		// if it doesn't already do this (e.g., by calling NewGenerateCAWithCRTStepSpec).
		// We remove explicit output key overrides here to rely on its standard behavior.
		pki.NewGenerateEtcdCAStep(
			"", // InputPKIPathKey (from TaskCache, set by DetermineEtcdPKIPathStep)
			"", // InputKubeConfKey (from ModuleCache, set by SetupEtcdPkiDataContextTask)
			"", // OutputCACertObjectKey (to TaskCache, step's default)
			"", // OutputCACertPathKey (to ModuleCache by the step, e.g. pki.ClusterRootCACertPathKey)
			"", // OutputCAKeyPathKey (to ModuleCache by the step, e.g. pki.ClusterRootCAKeyPathKey)
			"Generate Etcd CA", // Step name
		),
		// Step 3.1: Convert generated Etcd CA to PEM format
		// This step will read "ClusterRootCACertPath" and "ClusterRootCAKeyPath" from ModuleCache.
		// Its outputs (PEM paths) will go to StepCache using its default keys (pki.DefaultCertPemCacheKey, etc.)
		func() spec.StepSpec {
			convertSpec := pki.NewConvertCertsToPemStepSpec(
				"Convert Etcd CA to PEM", // name
				"Converts the generated Etcd CA certificate and key to PEM format.", // description
				"", // sourceCertPath (use cache)
				"", // sourceKeyPath (use cache)
				"", // targetCertPemPath (will be defaulted by step)
				"", // targetKeyPemPath (will be defaulted by step)
			)
			// Configure to read from ModuleCache keys written by the CA generation step
			convertSpec.SourceCertPathModuleCacheKey = "ClusterRootCACertPath" // Standard key from GenerateCAWithCRTStepSpec
			convertSpec.SourceKeyPathModuleCacheKey  = "ClusterRootCAKeyPath"  // Standard key from GenerateCAWithCRTStepSpec

			// Output for these PEM files will be in StepCache by default, using:
			// convertSpec.CertPemCacheKey = pki.DefaultCertPemCacheKey (value: "ConvertedCertPemPath")
			// convertSpec.KeyPemCacheKey  = pki.DefaultKeyPemCacheKey  (value: "ConvertedKeyPemPath")
			// If these need to be in TaskCache or ModuleCache for other steps/tasks,
			// further configuration or steps would be needed.
			return convertSpec
		}(),
		// Step 4: Generate Etcd Node Certificates (members, clients)
		pki.NewGenerateEtcdNodeCertsStep(
			"", // InputPKIPathKey (TaskCache)
			"", // InputAltNamesKey (TaskCache)
			"", // InputCACertObjectKey (TaskCache)
			"", // InputKubeConfKey (ModuleCache) - KubeConf seems misnamed here
			"", // InputHostsKey (ModuleCache)
			"", // OutputGeneratedFilesListKey (TaskCache)
			"", // Step name (use default)
		),
	}

	return &spec.TaskSpec{
		Name:        "GenerateEtcdPki",
		Description: "Generates all necessary etcd PKI (CA, member, client certificates).",
		RunOnRoles:  runOnRoles, // Often nil or ["master", "control-plane"]
		Steps:       taskSteps,
		IgnoreError: false,
		// Filter: "", // No specific filter defined
		// Concurrency: 1, // PKI generation is often sequential and local
	}
}
