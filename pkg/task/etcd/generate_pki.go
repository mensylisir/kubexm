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
		pki.NewGenerateEtcdCAStep(
			"", // InputPKIPathKey (use default from TaskCache)
			"", // InputKubeConfKey (use default from ModuleCache) - KubeConf seems misnamed here if it's for CA config
			"", // OutputCACertObjectKey (use default to TaskCache) - e.g., "EtcdCACertObject"
			"EtcdCACertPath", // OutputCACertPathKey (use default to TaskCache) - Forcing a known key for chaining
			"EtcdCAKeyPath",  // OutputCAKeyPathKey (use default to TaskCache) - Forcing a known key for chaining
			"", // Step name (use default)
		),
		// Step 3.1: Convert generated Etcd CA to PEM format
		// This step will read "EtcdCACertPath" and "EtcdCAKeyPath" from TaskCache
		// and write its output paths (e.g., "ca.pem", "ca-key.pem") also to TaskCache
		// using default keys like pki.DefaultCertPemCacheKey if not specified.
		func() spec.StepSpec {
			// Use default target paths, let populateDefaults in ConvertCertsToPemStepSpec handle them
			// Default output cache keys (pki.DefaultCertPemCacheKey, pki.DefaultKeyPemCacheKey) will be used by the step
			// if CertPemCacheKey and KeyPemCacheKey are not set here.
			convertSpec := pki.NewConvertCertsToPemStepSpec(
				"Convert Etcd CA to PEM", // name
				"Converts the generated Etcd CA certificate and key to PEM format.", // description
				"", // sourceCertPath (use cache)
				"", // sourceKeyPath (use cache)
				"", // targetCertPemPath (defaulted by step)
				"", // targetKeyPemPath (defaulted by step)
			)
			// Configure to read from TaskCache keys written by GenerateEtcdCAStep
			convertSpec.SourceCertPathTaskCacheKey = "EtcdCACertPath"
			convertSpec.SourceKeyPathTaskCacheKey = "EtcdCAKeyPath"
			// Optionally, set output keys if subsequent steps need these PEM paths from TaskCache specifically
			// convertSpec.CertPemCacheKey = "EtcdCaCertPemPath" // Example if needed in TaskCache explicitly
			// convertSpec.KeyPemCacheKey = "EtcdCaKeyPemPath"   // Example if needed in TaskCache explicitly
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
