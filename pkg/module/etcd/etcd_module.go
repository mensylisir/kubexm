package etcd

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"

	// "github.com/kubexms/kubexms/pkg/config" // No longer used
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.Host in HostFilter and ClusterRuntime
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For v1alpha1 constants if needed
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki"
	taskEtcd "github.com/mensylisir/kubexm/pkg/task/etcd"
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
func NewEtcdModule(clusterRt *runtime.ClusterRuntime) *spec.ModuleSpec {
	if clusterRt == nil || clusterRt.ClusterConfig == nil {
		return &spec.ModuleSpec{
			Name:      "Etcd Cluster Management (Error: Missing Configuration)",
			IsEnabled: func(_ *runtime.ClusterRuntime) bool { return false },
			Tasks:     []*spec.TaskSpec{},
		}
	}
	cfg := clusterRt.ClusterConfig // cfg is *v1alpha1.Cluster

	// --- Determine global parameters from cfg ---
	// TODO: Re-evaluate architecture detection. cfg.Spec.Arch removed.
	// Consider deriving from host list or a new global config if diverse archs are supported.
	arch := goruntime.GOARCH
	arch = normalizeArchFunc(arch)

	etcdVersion := "v3.5.0" // Default, consider making this configurable via EtcdConfig
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Version != "" {
		etcdVersion = cfg.Spec.Etcd.Version
	}

	zone := "" // Default zone to empty
	// TODO: Re-evaluate zone. v1alpha1.GlobalSpec does not have Zone.
	// if cfg.Spec.Global != nil && cfg.Spec.Global.Zone != "" {
	// 	zone = cfg.Spec.Global.Zone
	// }

	clusterName := "kubexms-cluster" // Default cluster name
	if cfg.ObjectMeta.Name != "" {
		clusterName = cfg.ObjectMeta.Name
	}

	programBaseDir := "/opt/kubexms/default_run_dir" // Fallback
	if cfg.Spec.Global != nil && cfg.Spec.Global.WorkDir != "" {
		programBaseDir = cfg.Spec.Global.WorkDir
	}
	// appFSBaseDir is <executable_dir>/.kubexm
	appFSBaseDir := filepath.Join(programBaseDir, ".kubexm")

	// Cluster-specific PKI root directory.
	clusterPkiRoot := filepath.Join(appFSBaseDir, "pki", clusterName)

	controlPlaneFQDN := "lb.kubexms.local" // Default CPlane FQDN
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
		AppFSBaseDir:   appFSBaseDir,    // <executable_dir>/.kubexm
		ClusterName:    clusterName,
		PKIDirectory:   clusterPkiRoot,  // <executable_dir>/.kubexm/pki/clusterName
	}

	// --- Define Tasks ---
	allTasks := []*spec.TaskSpec{}

	// Task 0: Setup PKI Data Context
	setupPkiDataTask := taskEtcd.NewSetupEtcdPkiDataContextTask(cfg, kubexmsKubeConfInstance, hostSpecsForNodeCerts)

	// --- Conditional Task Assembly ---
	// Check if Etcd config exists and Type indicates it's managed internally
	isManagedInternalEtcd := false
	if cfg.Spec.Etcd != nil {
		// Assuming EtcdTypeKubeXMSInternal means it's managed by this system.
		// The prompt mentioned "kubexm" as a default type; ensure constants align.
		// For now, using EtcdTypeKubeXMSInternal as per existing code structure.
		if cfg.Spec.Etcd.Type == v1alpha1.EtcdTypeKubeXMSInternal || cfg.Spec.Etcd.Type == "kubexm" || cfg.Spec.Etcd.Type == "stacked" {
			isManagedInternalEtcd = true
		}
	}

	if isManagedInternalEtcd {
		allTasks = append(allTasks, setupPkiDataTask)
		allTasks = append(allTasks, taskEtcd.NewInstallEtcdBinariesTask(cfg, etcdVersion, arch, zone, appFSBaseDir))

		// TODO: Re-evaluate logic for "Existing" Etcd.
		// The field cfg.Spec.Etcd.Existing was removed. Determine how to detect an existing setup if needed.
		// Perhaps by checking DataDir on hosts or other means. For now, assume new setup.
		allTasks = append(allTasks, taskEtcd.NewGenerateEtcdPKITask(cfg, hostSpecsForAltNames, controlPlaneFQDN, "lb.kubexms.local"))

		setupInitialEtcdMemberTaskSpec := &spec.TaskSpec{
	Name: "Setup Initial Etcd Member (Placeholder Spec)",
	}
		allTasks = append(allTasks, setupInitialEtcdMemberTaskSpec)

	} else if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Type == v1alpha1.EtcdTypeExternal {
		// External Etcd: still need PKI data context for clients, but also prepare external PKI if specified
		allTasks = append(allTasks, setupPkiDataTask)
		if cfg.Spec.Etcd.External != nil && cfg.Spec.Etcd.External.CAFile != "" {
			allTasks = append(allTasks, taskEtcd.NewPrepareExternalEtcdPKITask(cfg))
		} else {
			// Consider logging a warning: External etcd specified but no CA/Cert info provided for secure client connections.
		}
	}


	validateEtcdClusterTaskSpec := &spec.TaskSpec{
		Name: "Validate Etcd Cluster Health (Placeholder Spec)",
	}
	allTasks = append(allTasks, validateEtcdClusterTaskSpec)
	// Removed duplicated block here

	return &spec.ModuleSpec{
		Name: "Etcd Cluster Management",
		IsEnabled: func(cr *runtime.ClusterRuntime) bool {
			if cr == nil || cr.ClusterConfig == nil {
				return false // Cannot determine if Etcd is configured
			}
			// Enable if Etcd spec exists, regardless of type (external still needs client setup/validation).
			// The tasks themselves will differ based on Etcd.Type.
			return cr.ClusterConfig.Spec.Etcd != nil
		},
		Tasks: allTasks,
		PreRun: nil,
		PostRun: nil,
	}
}
