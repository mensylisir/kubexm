package containerd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/resource"
	"github.com/mensylisir/kubexm/pkg/runtime"
	stepContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallContainerdTask installs and configures containerd.
type InstallContainerdTask struct {
	task.BaseTask
	// Configuration for containerd itself, passed to ConfigureContainerdStep
	SandboxImage         string
	RegistryMirrors      map[string]stepContainerd.MirrorConfiguration
	InsecureRegistries   []string
	UseSystemdCgroup     bool
	ExtraTomlContent     string
	ContainerdConfigPath string // e.g., /etc/containerd/config.toml
	RegistryConfigPath   string // e.g., /etc/containerd/certs.d
}

// NewInstallContainerdTask creates a new task for installing and configuring containerd.
func NewInstallContainerdTask(runOnRoles []string) task.Task {
	return &InstallContainerdTask{
		BaseTask: task.NewBaseTask(
			"InstallAndConfigureContainerd",
			"Installs and configures containerd, runc, and CNI plugins.",
			runOnRoles,
			nil,   // HostFilter
			false, // IgnoreError
		),
		// Default values for containerd configuration can be set here or taken from clusterConfig.Spec.ContainerRuntime
		UseSystemdCgroup: true, // Default to true for Kubernetes
	}
}

func (t *InstallContainerdTask) IsRequired(ctx task.TaskContext) (bool, error) {
	clusterConfig := ctx.GetClusterConfig()
	// Only run if containerd is the chosen runtime type
	if clusterConfig.Spec.ContainerRuntime == nil || clusterConfig.Spec.ContainerRuntime.Type != v1alpha1.ContainerdRuntime {
		ctx.GetLogger().Info("Containerd installation is not required (runtime type is not containerd).")
		return false, nil
	}
	return t.BaseTask.IsRequired(ctx)
}

func (t *InstallContainerdTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")
	cfg := ctx.GetClusterConfig()

	// --- Populate task fields from ClusterConfiguration ---
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Containerd != nil {
		containerdCfg := cfg.Spec.ContainerRuntime.Containerd
		t.SandboxImage = containerdCfg.SandboxImage
		if t.SandboxImage == "" {
			t.SandboxImage = "registry.k8s.io/pause:3.9" // Default
		}
		// Convert v1alpha1.RegistryMirror to stepContainerd.MirrorConfiguration
		if len(containerdCfg.RegistryMirrors) > 0 {
			t.RegistryMirrors = make(map[string]stepContainerd.MirrorConfiguration)
			for reg, mirrorCfgAlpha := range containerdCfg.RegistryMirrors {
				t.RegistryMirrors[reg] = stepContainerd.MirrorConfiguration{
					Endpoints: mirrorCfgAlpha.Endpoints,
					Rewrite:   mirrorCfgAlpha.Rewrite,
				}
			}
		}
		t.InsecureRegistries = containerdCfg.InsecureRegistries
		t.ContainerdConfigPath = containerdCfg.ConfigPath
		t.RegistryConfigPath = containerdCfg.RegistryConfigPath
		t.ExtraTomlContent = containerdCfg.ExtraTomlContent
		// UseSystemdCgroup is usually true for K8s, can be configurable too.
		// t.UseSystemdCgroup = containerdCfg.UseSystemdCgroup (if such field exists)
	}

	// --- Determine target hosts ---
	targetHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err)
	}
	if len(targetHosts) == 0 {
		logger.Info("No target hosts found, returning empty fragment.")
		return task.NewEmptyFragment(), nil
	}
	// (Deduplication logic for targetHosts can be added if roles might overlap for a single task instance)

	graph := task.NewExecutionFragment()

	// --- 1. Resource Acquisition on Control Node ---
	// Get versions from config
	// TODO: These versions should ideally come from a resolved dependency spec, not hardcoded or simple config fields.
	// For now, assume they are available in cfg.Spec.ContainerRuntime or a similar place.
	containerdVersion := "1.7.13" // Example, replace with config lookup
	runcVersion := "v1.1.12"    // Example
	cniPluginsVersion := "v1.4.0" // Example
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Version != "" {
		containerdVersion = cfg.Spec.ContainerRuntime.Version
	}
	// TODO: Add similar lookups for runcVersion and cniPluginsVersion if they are in cfg.Spec.ContainerRuntime.Dependencies or similar.

	arch := "" // Auto-detect by resource handle

	allLocalResourcePrepFragments := []*task.ExecutionFragment{}

	// Containerd Archive Handle
	containerdArchiveHandle, err := resource.NewRemoteBinaryHandle(ctx,
		"containerd", containerdVersion, arch, "linux",
		"", // BinaryNameInArchive - we want the archive path for upload
		cfg.Spec.ContainerRuntime.GetContainerdChecksum(arch), // Get checksum from config
		"sha256",
	)
	if err != nil { return nil, fmt.Errorf("failed to create containerd archive handle: %w", err) }
	containerdArchivePrepFragment, err := containerdArchiveHandle.EnsurePlan(ctx)
	if err != nil { return nil, fmt.Errorf("failed to plan containerd archive acquisition: %w", err) }
	allLocalResourcePrepFragments = append(allLocalResourcePrepFragments, containerdArchivePrepFragment)
	localContainerdArchivePath := containerdArchiveHandle.(*resource.RemoteBinaryHandle).BinaryInfo().FilePath


	// Runc Binary Handle
	runcHandle, err := resource.NewRemoteBinaryHandle(ctx,
		"runc", strings.TrimPrefix(runcVersion, "v"), arch, "linux",
		"", // BinaryNameInArchive - runc is a direct binary
		cfg.Spec.ContainerRuntime.GetRuncChecksum(arch),
		"sha256",
	)
	if err != nil { return nil, fmt.Errorf("failed to create runc binary handle: %w", err) }
	runcPrepFragment, err := runcHandle.EnsurePlan(ctx)
	if err != nil { return nil, fmt.Errorf("failed to plan runc binary acquisition: %w", err) }
	allLocalResourcePrepFragments = append(allLocalResourcePrepFragments, runcPrepFragment)
	localRuncPath := runcHandle.(*resource.RemoteBinaryHandle).BinaryInfo().FilePath


	// CNI Plugins Archive Handle
	cniPluginsHandle, err := resource.NewRemoteBinaryHandle(ctx,
		"kubecni", strings.TrimPrefix(cniPluginsVersion, "v"), arch, "linux", // "kubecni" is the component name in util.BinaryInfo for CNI plugins
		"", // BinaryNameInArchive - we want the archive path for upload
		cfg.Spec.ContainerRuntime.GetCNIChecksum(arch),
		"sha256",
	)
	if err != nil { return nil, fmt.Errorf("failed to create CNI plugins archive handle: %w", err) }
	cniPluginsPrepFragment, err := cniPluginsHandle.EnsurePlan(ctx)
	if err != nil { return nil, fmt.Errorf("failed to plan CNI plugins archive acquisition: %w", err) }
	allLocalResourcePrepFragments = append(allLocalResourcePrepFragments, cniPluginsPrepFragment)
	localCNIPluginsArchivePath := cniPluginsHandle.(*resource.RemoteBinaryHandle).BinaryInfo().FilePath

	// Merge all local resource preparation fragments. These can run in parallel on the control node.
	mergedLocalResourceFragment := task.NewExecutionFragment(t.Name() + "-LocalResourcePrep")
	for _, frag := range allLocalResourcePrepFragments {
		if err := mergedLocalResourceFragment.MergeFragment(frag); err != nil {
			return nil, fmt.Errorf("failed to merge local resource prep fragment: %w", err)
		}
	}
	mergedLocalResourceFragment.CalculateEntryAndExitNodes() // Should mostly be parallel entries and exits

	// This is the main fragment for the task, starting with local resource prep.
	taskPlanFragment := task.NewExecutionFragment(t.Name())
	if err := taskPlanFragment.MergeFragment(mergedLocalResourceFragment); err != nil {
		return nil, fmt.Errorf("failed to merge local resource fragment into task plan: %w", err)
	}
	controlNodeOpsDoneDependencies := mergedLocalResourceFragment.ExitNodes
	if len(controlNodeOpsDoneDependencies) == 0 && len(mergedLocalResourceFragment.Nodes) > 0 { // If merged fragment had nodes but no explicit exits
		for id := range mergedLocalResourceFragment.Nodes { controlNodeOpsDoneDependencies = append(controlNodeOpsDoneDependencies, id)}
	}


	// --- 2. Pre-flight OS configuration on all target nodes (can run in parallel across hosts) ---
	// These depend on nothing from the local resource prep.
	preflightFragment := task.NewExecutionFragment(t.Name() + "-OSPreflight")

	loadModulesStep := preflight.NewLoadKernelModulesStep("", []string{"overlay", "br_netfilter"}, true)
	for _, host := range targetHosts {
		nodeID := plan.NodeID(fmt.Sprintf("preflight-load-modules-%s", host.GetName()))
		_, _ = preflightFragment.AddNode(&plan.ExecutionNode{
			Name: loadModulesStep.Meta().Name + "-on-" + host.GetName(), Step: loadModulesStep, Hosts: []connector.Host{host},
		})
	}

	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
		"net.ipv4.ip_forward":                 "1",
	}
	setSysctlStep := preflight.NewSetSystemConfigStep("", sysctlParams, "/etc/sysctl.d/99-kubernetes-cri.conf", true, true)
	for _, host := range targetHosts {
		nodeID := plan.NodeID(fmt.Sprintf("preflight-set-sysctl-%s", host.GetName()))
		_, _ = preflightFragment.AddNode(&plan.ExecutionNode{
			Name: setSysctlStep.Meta().Name + "-on-" + host.GetName(), Step: setSysctlStep, Hosts: []connector.Host{host},
		})
	}
	preflightFragment.CalculateEntryAndExitNodes() // All these are parallel entries and exits
	if err := taskPlanFragment.MergeFragment(preflightFragment); err != nil {
		return nil, fmt.Errorf("failed to merge OS preflight fragment: %w", err)
	}
	// No explicit dependency between local resource prep and OS preflight, they can start together.


	// --- 3. Distribution, Extraction, Installation per node ---
	var allHostFinalNodes []plan.NodeID

	for _, host := range targetHosts {
		nodePrefix := fmt.Sprintf("containerd-%s-", strings.ReplaceAll(host.GetName(), ".", "-"))

		// Dependencies for this host: local resource prep AND local OS preflight for this host.
		perHostBaseDependencies := append([]plan.NodeID{}, controlNodeOpsDoneDependencies...)
		perHostBaseDependencies = append(perHostBaseDependencies, plan.NodeID(fmt.Sprintf("preflight-load-modules-%s", host.GetName())))
		perHostBaseDependencies = append(perHostBaseDependencies, plan.NodeID(fmt.Sprintf("preflight-set-sysctl-%s", host.GetName())))
		currentHostLastOpNodeID := plan.NodeID("") // Will track the last operation for sequential steps on this host


		// Upload and Install Containerd
		remoteContainerdArchiveTempPath := filepath.Join("/tmp", filepath.Base(localContainerdArchivePath)) // Example remote temp path
		uploadContainerdArchiveStep := common.NewUploadFileStep(nodePrefix+"UploadContainerdArchive", localContainerdArchivePath, remoteContainerdArchiveTempPath, "0644", true, false)
		uploadContainerdArchiveNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: uploadContainerdArchiveStep.Meta().Name, Step: uploadContainerdArchiveStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(perHostBaseDependencies),
		})
		currentHostLastOpNodeID = uploadContainerdArchiveNodeID

		remoteExtractedContainerdPath := "/opt/kubexm/containerd" // Standard extraction path on remote
		extractContainerdStep := common.NewExtractArchiveStep(nodePrefix+"ExtractContainerdArchive", remoteContainerdArchiveTempPath, remoteExtractedContainerdPath, true, true) // remove archive, sudo
		extractContainerdNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: extractContainerdStep.Meta().Name, Step: extractContainerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = extractContainerdNodeID
		// Task cache key for where InstallContainerdStep can find the extracted files on this host.
		ctx.TaskCache().Set(fmt.Sprintf("%s.%s", stepContainerd.ContainerdExtractedDirCacheKey, host.GetName()), remoteExtractedContainerdPath)


		installContainerdBinariesStep := stepContainerd.NewInstallContainerdStep(nodePrefix+"InstallContainerdBinaries", fmt.Sprintf("%s.%s", stepContainerd.ContainerdExtractedDirCacheKey, host.GetName()), nil, "", "", true)
		installContainerdBinariesNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: installContainerdBinariesStep.Meta().Name, Step: installContainerdBinariesStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = installContainerdBinariesNodeID

		// Upload and Install Runc
		remoteRuncTempPath := filepath.Join("/tmp", filepath.Base(localRuncPath))
		uploadRuncStep := common.NewUploadFileStep(nodePrefix+"UploadRunc", localRuncPath, remoteRuncTempPath, "0755", true, false)
		uploadRuncNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: uploadRuncStep.Meta().Name, Step: uploadRuncStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(perHostBaseDependencies), // Can be parallel to containerd archive upload
		})
		// Task cache key for where InstallRuncBinaryStep can find the uploaded runc binary on this host.
		ctx.TaskCache().Set(fmt.Sprintf("%s.%s", stepContainerd.RuncBinaryRemotePathCacheKey, host.GetName()), remoteRuncTempPath)

		installRuncStep := stepContainerd.NewInstallRuncBinaryStep(nodePrefix+"InstallRuncBinary", fmt.Sprintf("%s.%s", stepContainerd.RuncBinaryRemotePathCacheKey, host.GetName()), "/usr/local/sbin/runc", true)
		installRuncNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: installRuncStep.Meta().Name, Step: installRuncStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadRuncNodeID},
		})
		// currentHostLastOpNodeID should now depend on both containerd binaries AND runc binary being installed.
		currentHostLastOpNodeID = installRuncNodeID // For simplicity, let's chain. A merge node would be more accurate for parallelism.

		// Upload and Install CNI Plugins
		remoteCNIArchiveTempPath := filepath.Join("/tmp", filepath.Base(localCNIPluginsArchivePath))
		uploadCNIStep := common.NewUploadFileStep(nodePrefix+"UploadCNIArchive", localCNIPluginsArchivePath, remoteCNIArchiveTempPath, "0644", true, false)
		uploadCNINodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: uploadCNIStep.Meta().Name, Step: uploadCNIStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(perHostBaseDependencies), // Parallel
		})
		// Task cache key for CNI archive path on remote host.
		ctx.TaskCache().Set(fmt.Sprintf("%s.%s", stepContainerd.CNIPluginsArchiveRemotePathCacheKey, host.GetName()), remoteCNIArchiveTempPath)

		extractCNIStep := stepContainerd.NewExtractCNIPluginsArchiveStep(nodePrefix+"ExtractCNIPlugins", fmt.Sprintf("%s.%s", stepContainerd.CNIPluginsArchiveRemotePathCacheKey, host.GetName()), "/opt/cni/bin", "", true, true) // remove archive, sudo
		extractCNINodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: extractCNIStep.Meta().Name, Step: extractCNIStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadCNINodeID},
		})
		// currentHostLastOpNodeID now depends on containerd, runc, AND CNI plugins being ready.
		// This requires careful dependency management. For now, we'll make subsequent config steps depend on all three install/extract final nodes.
		allBinariesReadyDependencies := []plan.NodeID{installContainerdBinariesNodeID, installRuncNodeID, extractCNINodeID}


		// Configuration and Service Management
		configureStep := stepContainerd.NewConfigureContainerdStep(nodePrefix+"ConfigureContainerd", t.SandboxImage, t.RegistryMirrors, t.InsecureRegistries, t.ContainerdConfigPath, t.RegistryConfigPath, t.ExtraTomlContent, t.UseSystemdCgroup, true)
		configureNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: configureStep.Meta().Name, Step: configureStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(allBinariesReadyDependencies),
		})
		currentHostLastOpNodeID = configureNodeID

		genServiceStep := stepContainerd.NewGenerateContainerdServiceStep(nodePrefix+"GenerateContainerdService", stepContainerd.ContainerdServiceData{}, "", true)
		genServiceNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: genServiceStep.Meta().Name, Step: genServiceStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = genServiceNodeID

		daemonReloadStep := stepContainerd.NewManageContainerdServiceStep(nodePrefix+"DaemonReloadContainerd", stepContainerd.ActionDaemonReload, true)
		daemonReloadNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: daemonReloadStep.Meta().Name, Step: daemonReloadStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = daemonReloadNodeID

		enableServiceStep := stepContainerd.NewManageContainerdServiceStep(nodePrefix+"EnableContainerd", stepContainerd.ActionEnable, true)
		enableServiceNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: enableServiceStep.Meta().Name, Step: enableServiceStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = enableServiceNodeID

		startServiceStep := stepContainerd.NewManageContainerdServiceStep(nodePrefix+"StartContainerd", stepContainerd.ActionStart, true)
		startServiceNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: startServiceStep.Meta().Name, Step: startServiceStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = startServiceNodeID

		verifyStep := stepContainerd.NewVerifyContainerdCrictlStep(nodePrefix+"VerifyContainerdCrictl", "", "", false)
		verifyNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: verifyStep.Meta().Name, Step: verifyStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		allHostFinalNodes = append(allHostFinalNodes, verifyNodeID)
	}

	taskPlanFragment.CalculateEntryAndExitNodes() // This will set EntryNodes based on mergedLocalResourceFragment and preflightFragment, and ExitNodes from allHostFinalNodes.

	logger.Info("Containerd installation and configuration plan generated.")
	return taskPlanFragment, nil
}

var _ task.Task = (*InstallContainerdTask)(nil)
