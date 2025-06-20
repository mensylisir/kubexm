package etcd

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For v1alpha1 constants
	"github.com/mensylisir/kubexm/pkg/config"               // For config.Cluster
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/pki" // For pki data types
	taskEtcdFactory "github.com/mensylisir/kubexm/pkg/task/etcd" // Alias for etcd task factories
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

// NewEtcdModuleSpec creates a module specification for deploying or managing an etcd cluster.
func NewEtcdModuleSpec(cfg *config.Cluster) *spec.ModuleSpec {
	if cfg == nil {
		return &spec.ModuleSpec{
			Name:        "Etcd Cluster Management",
			Description: "Manages the etcd cluster (Error: Missing Configuration)",
			IsEnabled:   "false",
			Tasks:       []*spec.TaskSpec{},
		}
	}

	// --- Determine global parameters from cfg ---
	arch := goruntime.GOARCH // Consider making this configurable or derived from node information
	arch = normalizeArchFunc(arch)

	etcdVersion := "v3.5.0" // Default
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Version != "" {
		etcdVersion = cfg.Spec.Etcd.Version
	}

	zone := "" // Default zone to empty, not currently in cfg.Spec.Global

	clusterName := "kubexms-cluster" // Default
	if cfg.ObjectMeta.Name != "" {
		clusterName = cfg.ObjectMeta.Name
	}

	programBaseDir := "/opt/kubexms/default_run_dir" // Fallback
	if cfg.Spec.Global.WorkDir != "" { // Assuming Global is not nil
		programBaseDir = cfg.Spec.Global.WorkDir
	}
	appFSBaseDir := filepath.Join(programBaseDir, ".kubexm")
	clusterPkiRoot := filepath.Join(appFSBaseDir, "pki", clusterName)

	controlPlaneFQDN := "lb.kubexms.local" // Default
	if cfg.Spec.ControlPlaneEndpoint != nil && cfg.Spec.ControlPlaneEndpoint.Domain != "" {
		controlPlaneFQDN = cfg.Spec.ControlPlaneEndpoint.Domain
	}

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

	kubexmsKubeConfInstance := &pki.KubexmsKubeConf{
		AppFSBaseDir: appFSBaseDir,
		ClusterName:  clusterName,
		PKIDirectory: clusterPkiRoot,
	}

	allTasks := []*spec.TaskSpec{}

	// Task 0: Setup PKI Data Context
	// RunOnRoles is nil for this data context task (local execution)
	setupPkiDataTask, err := taskEtcdFactory.NewSetupEtcdPkiDataContextTaskSpec(
		kubexmsKubeConfInstance, hostSpecsForNodeCerts, "etcd", // etcdSubPath = "etcd"
		"", "", "", // Default cache keys
		nil, // RunOnRoles = nil for local context setup
	)
	if err != nil {
		// Handle error: perhaps return a disabled module or one with an error state
		return &spec.ModuleSpec{
			Name:        "Etcd Cluster Management",
			Description: fmt.Sprintf("Error initializing SetupPkiDataTask: %v", err),
			IsEnabled:   "false",
			Tasks:       []*spec.TaskSpec{},
		}
	}

	isManagedInternalEtcd := false
	if cfg.Spec.Etcd != nil {
		if cfg.Spec.Etcd.Type == v1alpha1.EtcdTypeKubeXMSInternal || cfg.Spec.Etcd.Type == "kubexm" || cfg.Spec.Etcd.Type == "stacked" {
			isManagedInternalEtcd = true
		}
	}

	if isManagedInternalEtcd {
		allTasks = append(allTasks, setupPkiDataTask)

		// Placeholder for NewInstallEtcdBinariesTaskSpec
		// TODO: Replace with actual factory call if NewInstallEtcdBinariesTaskSpec is available
		installBinariesTask := &spec.TaskSpec{
			Name:        "InstallEtcdBinaries",
			Description: fmt.Sprintf("Install etcd binaries version %s for %s (Placeholder)", etcdVersion, arch),
			RunOnRoles:  []string{"etcd", "master", "control-plane"}, // Example roles
			Steps:       []spec.StepSpec{
				// &download.DownloadStepSpec{ URL: ..., Dest: ...},
				// &archive.ExtractStepSpec{ Archive: ..., DestDir: ...},
				// &command.CommandStepSpec{ Command: "cp ... /usr/local/bin/"},
			},
		}
		allTasks = append(allTasks, installBinariesTask)

		// RunOnRoles for PKI generation is typically nil or master roles
		generatePkiTask := taskEtcdFactory.NewGenerateEtcdPkiTaskSpec(hostSpecsForAltNames, controlPlaneFQDN, "lb.kubexms.local", nil)
		allTasks = append(allTasks, generatePkiTask)

		// Placeholder for NewSetupInitialEtcdMemberTaskSpec
		// TODO: Replace with actual factory call
		setupInitialEtcdMemberTask := &spec.TaskSpec{
			Name:        "SetupInitialEtcdMember",
			Description: "Configures the first etcd member (Placeholder)",
			RunOnRoles:  []string{"etcd", "master", "control-plane"}, // Usually on specific nodes
		}
		allTasks = append(allTasks, setupInitialEtcdMemberTask)

	} else if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Type == v1alpha1.EtcdTypeExternal {
		allTasks = append(allTasks, setupPkiDataTask) // Still need PKI context for client certs potentially
		if cfg.Spec.Etcd.External != nil && cfg.Spec.Etcd.External.CAFile != "" {
			prepareExternalPkiTask := taskEtcdFactory.NewPrepareExternalEtcdPKITask(cfg) // Already returns *spec.TaskSpec
			allTasks = append(allTasks, prepareExternalPkiTask)
		}
	}

	// Placeholder for NewValidateEtcdClusterTaskSpec
	// TODO: Replace with actual factory call
	validateEtcdClusterTask := &spec.TaskSpec{
		Name:        "ValidateEtcdClusterHealth",
		Description: "Checks the health of the etcd cluster (Placeholder)",
		RunOnRoles:  []string{"master", "control-plane"}, // Usually run from a control node
	}
	allTasks = append(allTasks, validateEtcdClusterTask)

	return &spec.ModuleSpec{
		Name:        "Etcd Cluster Management",
		Description: "Manages the etcd cluster deployment, PKI, and health.",
		// Condition to enable this module. Evaluated by Executor based on 'cfg'.
		IsEnabled:   "cfg.Spec.Etcd != nil",
		Tasks:       allTasks,
		PreRunHook:  "", // Example: "etcd_prerun_network_checks"
		PostRunHook: "", // Example: "etcd_postrun_cleanup"
	}
}
