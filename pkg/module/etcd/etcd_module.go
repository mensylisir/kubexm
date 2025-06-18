package etcd

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"

	"github.com/kubexms/kubexms/pkg/config" // Assumed to have necessary fields
	"github.com/kubexms/kubexms/pkg/spec"
	etcdsteps "github.com/kubexms/kubexms/pkg/step/etcd"
	"github.com/kubexms/kubexms/pkg/step/pki"
	// commonsteps "github.com/kubexms/kubexms/pkg/step/common" // For common.ExtractArchiveStepSpec etc. if used in future
)

// normalizeArchFunc ensures consistent architecture naming (amd64, arm64).
func normalizeArchFunc(arch string) string {
	if arch == "x86_64" {
		return "amd64"
	}
	if arch == "aarch64" {
		return "arm64"
	}
	return arch
}

// NewEtcdModule creates a module specification for deploying or managing an etcd cluster.
func NewEtcdModule(cfg *config.Cluster) *spec.ModuleSpec {
	// --- Determine global parameters from cfg ---
	arch := cfg.Spec.Arch
	if arch == "" {
		arch = goruntime.GOARCH // Default to host architecture of the KubeXMS process
	}
	arch = normalizeArchFunc(arch)

	etcdVersion := "v3.5.0" // Default, overridden by cfg if present
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Version != "" {
		etcdVersion = cfg.Spec.Etcd.Version
	}

	zone := "" // Default (no specific zone for downloads)
	if cfg.Spec.Global != nil && cfg.Spec.Global.Zone != "" {
		zone = cfg.Spec.Global.Zone
	}

	clusterName := "kubexms-cluster" // Default
	if cfg.Metadata.Name != "" {
		clusterName = cfg.Metadata.Name
	}

	appWorkDir := cfg.WorkDir // e.g., /opt/kubexms or a user-specified directory
	if appWorkDir == "" {
		appWorkDir = "/opt/kubexms/default_workdir" // Fallback if not set in cfg
	}

	// Base directory for all PKI material for this cluster.
	// This path is local to where the KubeXMS process runs and generates initial PKI.
	clusterPkiBaseDir := filepath.Join(appWorkDir, "kubexms-pki", clusterName) // Changed "kubexms/pki" to "kubexms-pki"

	controlPlaneFQDN := "lb.kubexms.local" // Default
	if cfg.Spec.ControlPlaneEndpoint != nil && cfg.Spec.ControlPlaneEndpoint.Domain != "" {
		controlPlaneFQDN = cfg.Spec.ControlPlaneEndpoint.Domain
	}

	// --- Prepare HostSpec lists for PKI steps ---
	var hostSpecsForAltNames []pki.HostSpecForAltNames
	var hostSpecsForNodeCerts []pki.HostSpecForPKI
	for _, chost := range cfg.Spec.Hosts { // Assuming cfg.Spec.Hosts is of type []config.HostSpec
		hostSpecsForAltNames = append(hostSpecsForAltNames, pki.HostSpecForAltNames{
			Name:            chost.Name,
			InternalAddress: chost.InternalAddress,
		})
		hostSpecsForNodeCerts = append(hostSpecsForNodeCerts, pki.HostSpecForPKI{
			Name:  chost.Name,
			Roles: chost.Roles, // Assuming config.HostSpec has a Roles []string field
		})
	}

	// --- Prepare KubexmsKubeConf stub for PKI steps ---
	kubexmsKubeConfInstance := &pki.KubexmsKubeConf{
		ClusterName:  clusterName,
		PKIDirectory: clusterPkiBaseDir, // This is the base for all cluster PKI. Steps will add subpaths.
	}

	// --- Define Tasks ---
	etcdTaskSpecs := []*spec.TaskSpec{}

	// Task 0: Setup PKI Data Context (runs locally to populate module cache)
	// This task makes KubeConf and HostLists available to subsequent PKI steps in this module.
	setupPkiDataContextTask := &spec.TaskSpec{
		Name:      "Setup Etcd PKI Data Context",
		LocalNode: true, // Indicates this task runs locally where KubeXMS process is running.
		Steps: []spec.StepSpec{
			&pki.SetupEtcdPkiDataContextStepSpec{
				KubeConfForCache:    kubexmsKubeConfInstance,
				HostsForPKIForCache: hostSpecsForNodeCerts,
				// EtcdPkiPathForCache is NOT set here; DetermineEtcdPKIPathStep will handle it using KubeConfForCache.PKIDirectory as base.
				// HostsForAltNamesForCache is also not set here, as GenerateEtcdAltNamesStepSpec.Hosts is populated directly.
			},
		},
	}

	// Task for installing etcd binaries
	installEtcdBinariesTask := &spec.TaskSpec{
		Name: fmt.Sprintf("Provision etcd %s for %s", etcdVersion, arch),
		Steps: []spec.StepSpec{
			&etcdsteps.DownloadEtcdArchiveStepSpec{
				Version:     etcdVersion,
				Arch:        arch,
				Zone:        zone,
				DownloadDir: filepath.Join(appWorkDir, "etcd-binaries", etcdVersion, arch), // Standardized artifact download path
			},
			&etcdsteps.ExtractEtcdArchiveStepSpec{
				// ArchivePathSharedDataKey uses default from DownloadEtcdStepSpec's output.
				ExtractionDir: filepath.Join(appWorkDir, "_artifact_extracts", "etcd", etcdVersion, arch), // Standardized extraction path
				// ExtractedDirSharedDataKey uses its default ("extractedPath").
			},
			&etcdsteps.InstallEtcdFromDirStepSpec{
				// SourcePathSharedDataKey uses default from ExtractArchiveStepSpec's output.
				// TargetDir uses its default ("/usr/local/bin").
			},
			&etcdsteps.CleanupEtcdInstallationStepSpec{}, // Cleans up based on SharedData from download/extract.
		},
	}

	// --- PKI Task Definitions ---
	generateEtcdPKITask := &spec.TaskSpec{
		Name:      "Generate New Etcd PKI",
		LocalNode: true,
		Steps: []spec.StepSpec{
			&pki.DetermineEtcdPKIPathStepSpec{
				// This step now reads KubeConf from cache (populated by setupPkiDataContextTask)
				// and uses KubexmsKubeConf.PKIDirectory as its base to construct the etcd-specific PKI path.
				// BaseWorkDirSharedDataKey defaults to pki.DefaultKubeConfKey.
				// EtcdPKISubPath defaults to "pki/etcd".
				// OutputPKIPathSharedDataKey defaults to pki.DefaultEtcdPKIPathKey.
			},
			&pki.GenerateEtcdAltNamesStepSpec{
				ControlPlaneEndpointDomain: controlPlaneFQDN,
				Hosts:                      hostSpecsForAltNames, // Directly populated by the module.
			},
			&pki.GenerateEtcdCAStepSpec{
				// Relies on default SharedData/TaskCache keys to get PKIPath (from DetermineEtcdPKIPathStep)
				// and KubeConf (from setupPkiDataContextTask).
			},
			&pki.GenerateEtcdNodeCertsStepSpec{
				// Relies on default SharedData/TaskCache keys for PKIPath, AltNames, CA Cert Object, KubeConf, Hosts.
			},
		},
	}

	prepareExistingEtcdPKITask := &spec.TaskSpec{
		Name: "Prepare PKI from Existing Internal Etcd Cluster",
		// HostFilter: &spec.HostFilter{Roles: []string{"etcd"}, Strategy: spec.PickFirst}, // Example
		Steps: []spec.StepSpec{
			&pki.DetermineEtcdPKIPathStepSpec{
				// BaseWorkDir will be derived from KubeConf in cache.
			},
			&pki.FetchExistingEtcdCertsStepSpec{
				// Uses defaults for RemoteCertDir, TargetPKIPathSharedDataKey, OutputFetchedFilesListKey.
				// TargetPKIPath comes from DetermineEtcdPKIPathStep's output.
			},
		},
	}

	prepareExternalEtcdPKITask := &spec.TaskSpec{
		Name:      "Prepare PKI using User-Provided External Etcd Certificates",
		LocalNode: true,
		Steps: []spec.StepSpec{
			&pki.DetermineEtcdPKIPathStepSpec{
				// BaseWorkDir will be derived from KubeConf in cache.
			},
			&pki.PrepareExternalEtcdCertsStepSpec{
				ExternalEtcdCAFile:   cfg.Spec.Etcd.External.CAFile,   // Assumes cfg.Spec.Etcd.External is valid if this task runs
				ExternalEtcdCertFile: cfg.Spec.Etcd.External.CertFile,
				ExternalEtcdKeyFile:  cfg.Spec.Etcd.External.KeyFile,
				// TargetPKIPathSharedDataKey and OutputCopiedFilesListKey use defaults.
			},
		},
	}

	// --- Conditional Task Assembly ---
	if cfg.Spec.Etcd == nil || !cfg.Spec.Etcd.Managed {
		// If Etcd is not configured or not managed, do nothing related to etcd.
		// The IsEnabled function for the module will handle this.
	} else {
		// Always run the PKI data context setup task first if any PKI operation is needed.
		// This task itself is local and populates the module cache.
		etcdTaskSpecs = append(etcdTaskSpecs, setupPkiDataContextTask)

		if cfg.Spec.Etcd.Type == "external" {
			if cfg.Spec.Etcd.External == nil || cfg.Spec.Etcd.External.CAFile == "" {
				// Configuration error: external etcd specified but no cert paths provided.
				// Module should probably error out or this task will fail.
				// For now, add the task; it will fail if paths are empty and required.
				etcdTaskSpecs = append(etcdTaskSpecs, prepareExternalEtcdPKITask)
				// No binary installation for external etcd.
			} else {
				etcdTaskSpecs = append(etcdTaskSpecs, prepareExternalEtcdPKITask)
			}
		} else { // Internal Etcd
			etcdTaskSpecs = append(etcdTaskSpecs, installEtcdBinariesTask) // Install binaries for internal etcd

			if cfg.Spec.Etcd.Existing { // Using an existing internal etcd cluster
				etcdTaskSpecs = append(etcdTaskSpecs, prepareExistingEtcdPKITask)
				// TODO: Add tasks for distributing fetched certs to other nodes if necessary.
			} else { // New internal etcd cluster, generate PKI from scratch.
				etcdTaskSpecs = append(etcdTaskSpecs, generateEtcdPKITask)
			}

			// Placeholder for other internal etcd tasks (setup members, join, validate)
			setupInitialEtcdMemberTaskSpec := &spec.TaskSpec{
				Name: "Setup Initial Etcd Member (Placeholder Spec)",
				// This task would use prepared PKI and etcd binaries.
				// Needs HostFilter for initial member(s).
			}
			etcdTaskSpecs = append(etcdTaskSpecs, setupInitialEtcdMemberTaskSpec)
		}
	}

	// Common validation task, runs if module is enabled.
	validateEtcdClusterTaskSpec := &spec.TaskSpec{
		Name: "Validate Etcd Cluster Health (Placeholder Spec)",
	}
	etcdTaskSpecs = append(etcdTaskSpecs, validateEtcdClusterTaskSpec)

	return &spec.ModuleSpec{
		Name: "Etcd Cluster Management",
		IsEnabled: func(currentCfg *config.Cluster) bool {
			if currentCfg != nil && currentCfg.Spec.Etcd != nil && currentCfg.Spec.Etcd.Managed {
				return true
			}
			return false
		},
		Tasks: etcdTaskSpecs,
		PreRun: func(ctx runtime.Context) error {
			// This PreRun (if it existed at module execution level, not definition)
			// could be where KubeConf and Hosts lists are placed into ctx.Module().
			// However, the current plan uses a dedicated Step (SetupEtcdPkiDataContextStep)
			// as the first step in tasks that need this data.
			// If PreRun is for the *entire module before any task*, it's a good place.
			// For now, using the explicit setup task.
			return nil
		},
		PostRun: nil,
	}
}
