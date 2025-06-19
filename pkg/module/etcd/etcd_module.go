package etcd

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/runtime" // For runtime.Host in HostFilter
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/pki"
	taskEtcd "github.com/kubexms/kubexms/pkg/task/etcd"
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
		arch = goruntime.GOARCH
	}
	arch = normalizeArchFunc(arch)

	etcdVersion := "v3.5.0"
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Version != "" {
		etcdVersion = cfg.Spec.Etcd.Version
	}

	zone := ""
	if cfg.Spec.Global != nil && cfg.Spec.Global.Zone != "" {
		zone = cfg.Spec.Global.Zone
	}

	clusterName := "kubexms-cluster"
	if cfg.Metadata.Name != "" {
		clusterName = cfg.Metadata.Name
	}

	programExecutableDir := cfg.WorkDir // cfg.WorkDir is assumed to be <program_executable_directory>
	if programExecutableDir == "" {
		programExecutableDir = "/opt/kubexms/default_run_dir" // Fallback
	}
	appFSBaseDir := filepath.Join(programExecutableDir, ".kubexm") // <executable_dir>/.kubexm

	// Cluster-specific PKI root directory.
	clusterPkiRoot := filepath.Join(appFSBaseDir, "pki", clusterName)

	controlPlaneFQDN := "lb.kubexms.local"
	if cfg.Spec.ControlPlaneEndpoint != nil && cfg.Spec.ControlPlaneEndpoint.Domain != "" {
		controlPlaneFQDN = cfg.Spec.ControlPlaneEndpoint.Domain
	}

	// --- Prepare HostSpec lists for PKI steps ---
	var hostSpecsForAltNames []pki.HostSpecForAltNames
	var hostSpecsForNodeCerts []pki.HostSpecForPKI
	for _, chost := range cfg.Spec.Hosts {
		hostSpecsForAltNames = append(hostSpecsForAltNames, pki.HostSpecForAltNames{
			Name:            chost.Name,
			InternalAddress: chost.InternalAddress,
		})
		hostSpecsForNodeCerts = append(hostSpecsForNodeCerts, pki.HostSpecForPKI{
			Name:  chost.Name,
			Roles: chost.Roles,
		})
	}

	// --- Prepare KubexmsKubeConf for PKI steps ---
	kubexmsKubeConfInstance := &pki.KubexmsKubeConf{
		AppFSBaseDir: appFSBaseDir,       // <executable_dir>/.kubexm
		ClusterName:  clusterName,
		PKIDirectory: clusterPkiRoot,   // <executable_dir>/.kubexm/pki/clusterName
	}

	// --- Define Tasks ---
	allTasks := []*spec.TaskSpec{}

	// Task 0: Setup PKI Data Context
	// This task populates KubeConf, Hosts list, and the specific EtcdPkiPath into Module Cache.
	setupPkiDataTask := taskEtcd.NewSetupEtcdPkiDataContextTask(cfg, kubexmsKubeConfInstance, hostSpecsForNodeCerts)

	// --- Conditional Task Assembly ---
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Managed {
		allTasks = append(allTasks, setupPkiDataTask) // Setup data context first for all PKI scenarios

		if cfg.Spec.Etcd.Type == "external" {
			// For external etcd, we only prepare local PKI files if specified by user.
			// No binary installation by this module.
			if cfg.Spec.Etcd.External != nil && cfg.Spec.Etcd.External.CAFile != "" {
				allTasks = append(allTasks, taskEtcd.NewPrepareExternalEtcdPKITask(cfg))
			} else {
				// Log warning or handle error: external etcd chosen but no cert paths provided.
				// The PrepareExternalEtcdCertsStep itself might error if paths are empty and it tries to use them.
			}
		} else { // Internal Etcd (new or existing)
			allTasks = append(allTasks, taskEtcd.NewInstallEtcdBinariesTask(cfg, etcdVersion, arch, zone, appFSBaseDir))

			if cfg.Spec.Etcd.Existing {
				existingPkiTask := taskEtcd.NewPrepareExistingEtcdPKITask(cfg)
				// TODO: Set HostFilter on existingPkiTask to target a single etcd node for fetching.
				// Example: existingPkiTask.HostFilter = spec.FirstHostWithRole("etcd")
				allTasks = append(allTasks, existingPkiTask)
			} else {
				allTasks = append(allTasks, taskEtcd.NewGenerateEtcdPKITask(cfg, hostSpecsForAltNames, controlPlaneFQDN, "lb.kubexms.local"))
			}

			setupInitialEtcdMemberTaskSpec := &spec.TaskSpec{
				Name: "Setup Initial Etcd Member (Placeholder Spec)",
			}
			allTasks = append(allTasks, setupInitialEtcdMemberTaskSpec)
		}
	}

	validateEtcdClusterTaskSpec := &spec.TaskSpec{
		Name: "Validate Etcd Cluster Health (Placeholder Spec)",
	}
	allTasks = append(allTasks, validateEtcdClusterTaskSpec)

	return &spec.ModuleSpec{
		Name: "Etcd Cluster Management",
		IsEnabled: func(currentCfg *config.Cluster) bool {
			if currentCfg != nil && currentCfg.Spec.Etcd != nil && currentCfg.Spec.Etcd.Managed {
				return true
			}
			return false
		},
		Tasks: allTasks,
		PreRun: nil,
		PostRun: nil,
	}
}
