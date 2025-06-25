package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/resource"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/docker" // Import new docker steps
	"github.com/mensylisir/kubexm/pkg/step/preflight"
	"github.com/mensylisir/kubexm/pkg/task"
	containerdSteps "github.com/mensylisir/kubexm/pkg/step/containerd" // For ServiceAction type
)

// InstallDockerTask installs Docker and cri-dockerd.
type InstallDockerTask struct {
	task.BaseTask
	// Docker daemon.json configuration
	DockerDaemonConfig docker.DockerDaemonConfig
	// cri-dockerd service configuration (usually command-line args)
	CriDockerdExecStartArgs map[string]string
}

// NewInstallDockerTask creates a new task for installing Docker and cri-dockerd.
func NewInstallDockerTask(runOnRoles []string) task.Task {
	return &InstallDockerTask{
		BaseTask: task.NewBaseTask(
			"InstallAndConfigureDockerCriDockerd",
			"Installs Docker engine, cri-dockerd, and CNI plugins.",
			runOnRoles,
			nil,   // HostFilter
			false, // IgnoreError
		),
		// Default DockerDaemonConfig can be further populated from ClusterConfig
		DockerDaemonConfig: docker.DockerDaemonConfig{
			ExecOpts:      []string{"native.cgroupdriver=systemd"},
			LogDriver:     "json-file",
			LogOpts:       map[string]string{"max-size": "100m"},
			StorageDriver: "overlay2", // Common default
		},
		// Default cri-dockerd args
		CriDockerdExecStartArgs: map[string]string{
			// "--container-runtime-endpoint": "unix:///var/run/docker.sock", // This is often default in unit file.
			// "--network-plugin": "cni", // Default and common for Kubernetes
		},
	}
}

func (t *InstallDockerTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	clusterConfig := ctx.GetClusterConfig()
	if clusterConfig.Spec.ContainerRuntime == nil || clusterConfig.Spec.ContainerRuntime.Type != v1alpha1.DockerRuntime {
		ctx.GetLogger().Info("Docker installation is not required (runtime type is not docker).")
		return false, nil
	}
	return t.BaseTask.IsRequired(ctx)
}

func (t *InstallDockerTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name(), "phase", "Plan")
	cfg := ctx.GetClusterConfig()

	// --- Populate task fields from ClusterConfiguration ---
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Docker != nil {
		dockerCfg := cfg.Spec.ContainerRuntime.Docker
		t.DockerDaemonConfig.RegistryMirrors = dockerCfg.RegistryMirrors
		t.DockerDaemonConfig.InsecureRegistries = dockerCfg.InsecureRegistries
		if dockerCfg.CgroupDriver != "" { // User might specify cgroupdriver directly
			t.DockerDaemonConfig.CgroupDriver = dockerCfg.CgroupDriver
			// Ensure exec-opts is cleared or reconciled if CgroupDriver is set
			if dockerCfg.CgroupDriver == "systemd" && !contains(t.DockerDaemonConfig.ExecOpts, "native.cgroupdriver=systemd") {
				 t.DockerDaemonConfig.ExecOpts = append(t.DockerDaemonConfig.ExecOpts, "native.cgroupdriver=systemd")
			} else if dockerCfg.CgroupDriver != "systemd" {
				// remove native.cgroupdriver=systemd from execOpts if user specified other
				var newExecOpts []string
				for _, opt := range t.DockerDaemonConfig.ExecOpts {
					if !strings.HasPrefix(opt, "native.cgroupdriver=") {
						newExecOpts = append(newExecOpts, opt)
					}
				}
				t.DockerDaemonConfig.ExecOpts = newExecOpts
			}
		}
		if dockerCfg.LogDriver != "" { t.DockerDaemonConfig.LogDriver = dockerCfg.LogDriver }
		if len(dockerCfg.LogOpts) > 0 { t.DockerDaemonConfig.LogOpts = dockerCfg.LogOpts }
		if dockerCfg.StorageDriver != "" { t.DockerDaemonConfig.StorageDriver = dockerCfg.StorageDriver }

		// Populate cri-dockerd args if specified in config
		if cfg.Spec.ContainerRuntime.Docker.CriDockerd != nil {
			// Example: t.CriDockerdExecStartArgs["--some-flag"] = cfg.Spec.ContainerRuntime.Docker.CriDockerd.SomeFlagValue
		}
	}


	targetHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...)
	if err != nil { return nil, fmt.Errorf("failed to get hosts for task %s: %w", t.Name(), err) }
	if len(targetHosts) == 0 {
		logger.Info("No target hosts found, returning empty fragment.")
		return task.NewEmptyFragment(), nil
	}

	graph := task.NewExecutionFragment()

	// --- 1. Resource Acquisition on Control Node (cri-dockerd archive, CNI plugins archive) ---
	// Docker engine itself is installed via package manager, so no direct binary resource handle for it.
	criDockerdVersion := "0.3.10" // Example, should come from cfg.Spec.ContainerRuntime.Docker.CriDockerdVersion or similar
	cniPluginsVersion := "v1.4.0" // Example, consistent with containerd task, should come from config
	arch := "" // Auto-detect

	allLocalResourcePrepFragments := []*task.ExecutionFragment{}

	// Cri-dockerd Archive Handle
	criDockerdHandle, err := resource.NewRemoteBinaryHandle(ctx,
		"cri-dockerd", strings.TrimPrefix(criDockerdVersion, "v"), arch, "linux",
		"cri-dockerd", // BinaryNameInArchive: assumes the binary is named 'cri-dockerd' inside a dir like 'cri-dockerd/' in the archive
		cfg.Spec.ContainerRuntime.GetCriDockerdChecksum(arch),
		"sha256",
	)
	if err != nil { return nil, fmt.Errorf("failed to create cri-dockerd archive handle: %w", err) }
	criDockerdPrepFragment, err := criDockerdHandle.EnsurePlan(ctx)
	if err != nil { return nil, fmt.Errorf("failed to plan cri-dockerd archive acquisition: %w", err) }
	allLocalResourcePrepFragments = append(allLocalResourcePrepFragments, criDockerdPrepFragment)
	localCriDockerdArchivePath := criDockerdHandle.(*resource.RemoteBinaryHandle).BinaryInfo().FilePath


	// CNI Plugins Archive Handle (same as in containerd task)
	cniPluginsHandle, err := resource.NewRemoteBinaryHandle(ctx,
		"kubecni", strings.TrimPrefix(cniPluginsVersion, "v"), arch, "linux",
		"", // We need the archive for CNI plugins, not a specific binary from it.
		cfg.Spec.ContainerRuntime.GetCNIChecksum(arch),
		"sha256",
	)
	if err != nil { return nil, fmt.Errorf("failed to create CNI plugins archive handle: %w", err) }
	cniPluginsPrepFragment, err := cniPluginsHandle.EnsurePlan(ctx)
	if err != nil { return nil, fmt.Errorf("failed to plan CNI plugins archive acquisition: %w", err) }
	allLocalResourcePrepFragments = append(allLocalResourcePrepFragments, cniPluginsPrepFragment)
	localCNIPluginsArchivePath := cniPluginsHandle.(*resource.RemoteBinaryHandle).BinaryInfo().FilePath

	// Merge all local resource preparation fragments.
	mergedLocalResourceFragment := task.NewExecutionFragment(t.Name() + "-LocalResourcePrep")
	for _, frag := range allLocalResourcePrepFragments {
		if err := mergedLocalResourceFragment.MergeFragment(frag); err != nil {
			return nil, fmt.Errorf("failed to merge local resource prep fragment for docker task: %w", err)
		}
	}
	mergedLocalResourceFragment.CalculateEntryAndExitNodes()

	taskPlanFragment := task.NewExecutionFragment(t.Name())
	if err := taskPlanFragment.MergeFragment(mergedLocalResourceFragment); err != nil {
		return nil, fmt.Errorf("failed to merge local resource fragment into docker task plan: %w", err)
	}
	controlNodeOpsDoneDependencies := mergedLocalResourceFragment.ExitNodes
	if len(controlNodeOpsDoneDependencies) == 0 && len(mergedLocalResourceFragment.Nodes) > 0 {
		for id := range mergedLocalResourceFragment.Nodes { controlNodeOpsDoneDependencies = append(controlNodeOpsDoneDependencies, id)}
	}

	// --- 2. Pre-flight OS configuration on all target nodes ---
	preflightFragment := task.NewExecutionFragment(t.Name() + "-OSPreflightDocker")
	loadModulesStep := preflight.NewLoadKernelModulesStep("", []string{"overlay", "br_netfilter"}, true)
	for _, host := range targetHosts {
		nodeID := plan.NodeID(fmt.Sprintf("preflight-load-modules-docker-%s", host.GetName()))
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
		nodeID := plan.NodeID(fmt.Sprintf("preflight-set-sysctl-docker-%s", host.GetName()))
		_, _ = preflightFragment.AddNode(&plan.ExecutionNode{
			Name: setSysctlStep.Meta().Name + "-on-" + host.GetName(), Step: setSysctlStep, Hosts: []connector.Host{host},
		})
	}
	preflightFragment.CalculateEntryAndExitNodes()
	if err := taskPlanFragment.MergeFragment(preflightFragment); err != nil {
		return nil, fmt.Errorf("failed to merge OS preflight fragment for docker task: %w", err)
	}

	// --- 3. Installation and Configuration per node ---
	var allHostFinalNodes []plan.NodeID

	for _, host := range targetHosts {
		nodePrefix := fmt.Sprintf("docker-%s-", strings.ReplaceAll(host.GetName(), ".", "-"))
		// Base dependencies for this host's operations: local resources ready AND OS preflight for this host done.
		perHostBaseDependencies := append([]plan.NodeID{}, controlNodeOpsDoneDependencies...)
		perHostBaseDependencies = append(perHostBaseDependencies, plan.NodeID(fmt.Sprintf("preflight-load-modules-docker-%s", host.GetName())))
		perHostBaseDependencies = append(perHostBaseDependencies, plan.NodeID(fmt.Sprintf("preflight-set-sysctl-docker-%s", host.GetName())))
		currentHostLastOpNodeID := plan.NodeID("")


		// Install Docker Engine (package based)
		dockerPackages := cfg.Spec.ContainerRuntime.Docker.Packages // Assuming this field exists in v1alpha1.DockerConfig
		if len(dockerPackages) == 0 { // Provide defaults if not specified
			dockerPackages = []string{"docker-ce", "docker-ce-cli", "containerd.io"} // Note: containerd.io is often a dep for docker-ce
		}
		// TODO: ExtraRepoSetupCmds might be OS-dependent, determined from facts or a more detailed config.
		var extraRepoCmds []string
		// Example: if facts.OS.ID == "ubuntu" { extraRepoCmds = ... }
		installDockerStep := docker.NewInstallDockerEngineStep(nodePrefix+"InstallDockerEngine", dockerPackages, extraRepoCmds, true)
		installDockerNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: installDockerStep.Meta().Name, Step: installDockerStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(perHostBaseDependencies),
		})
		currentHostLastOpNodeID = installDockerNodeID

		// Configure Docker daemon.json
		genDaemonJSONStep := docker.NewGenerateDockerDaemonJSONStep(nodePrefix+"GenerateDockerDaemonJSON", t.DockerDaemonConfig, "", true)
		genDaemonJSONNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: genDaemonJSONStep.Meta().Name, Step: genDaemonJSONStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = genDaemonJSONNodeID

		// Manage Docker Service (Restart to apply daemon.json, then enable)
		manageDockerRestartStep := docker.NewManageDockerServiceStep(nodePrefix+"RestartDockerService", containerdSteps.ActionRestart, true)
		manageDockerRestartNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: manageDockerRestartStep.Meta().Name, Step: manageDockerRestartStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = manageDockerRestartNodeID

		enableDockerStep := docker.NewManageDockerServiceStep(nodePrefix+"EnableDockerService", containerdSteps.ActionEnable, true)
		enableDockerNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: enableDockerStep.Meta().Name, Step: enableDockerStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = enableDockerNodeID // Docker engine is now set up.

		// Upload and Install CNI Plugins (can be parallel to Docker setup to some extent, but needs Docker running for some CNI use cases implicitly)
		// For simplicity, let's make CNI setup depend on Docker being enabled.
		remoteCNIArchiveTempPath := filepath.Join("/tmp", filepath.Base(localCNIPluginsArchivePath))
		uploadCNIStep := common.NewUploadFileStep(nodePrefix+"UploadCNIArchive", localCNIPluginsArchivePath, remoteCNIArchiveTempPath, "0644", true, false)
		uploadCNINodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: uploadCNIStep.Meta().Name, Step: uploadCNIStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(perHostBaseDependencies),
		})
		// Task cache for remote CNI archive path
		ctx.TaskCache().Set(fmt.Sprintf("%s.%s", stepContainerd.CNIPluginsArchiveRemotePathCacheKey, host.GetName()), remoteCNIArchiveTempPath)

		extractCNIStep := stepContainerd.NewExtractCNIPluginsArchiveStep(nodePrefix+"ExtractCNIPlugins", fmt.Sprintf("%s.%s", stepContainerd.CNIPluginsArchiveRemotePathCacheKey, host.GetName()), "/opt/cni/bin", "", true, true)
		extractCNINodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: extractCNIStep.Meta().Name, Step: extractCNIStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadCNINodeID},
		})
		// CNI plugins are now ready on the host.

		// Upload, Extract, Install cri-dockerd
		remoteCriDockerdArchiveTempPath := filepath.Join("/tmp", filepath.Base(localCriDockerdArchivePath))
		uploadCriDockerdStep := common.NewUploadFileStep(nodePrefix+"UploadCriDockerdArchive", localCriDockerdArchivePath, remoteCriDockerdArchiveTempPath, "0644", true, false)
		uploadCriDockerdNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: uploadCriDockerdStep.Meta().Name, Step: uploadCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(perHostBaseDependencies),
		})
		ctx.TaskCache().Set(fmt.Sprintf("%s.%s", docker.CriDockerdArchiveRemotePathCacheKey, host.GetName()), remoteCriDockerdArchiveTempPath)

		extractCriDockerdStep := docker.NewExtractCriDockerdArchiveStep(nodePrefix+"ExtractCriDockerdArchive", fmt.Sprintf("%s.%s", docker.CriDockerdArchiveRemotePathCacheKey, host.GetName()), "", "", "", true, true) // Default extraction path
		extractCriDockerdNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: extractCriDockerdStep.Meta().Name, Step: extractCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadCriDockerdNodeID},
		})
		// Extracted path for cri-dockerd is set in cache by ExtractCriDockerdArchiveStep if it follows the pattern.
		// Assume docker.CriDockerdExtractedDirCacheKey is the key used.
		ctx.TaskCache().Set(fmt.Sprintf("%s.%s", docker.CriDockerdExtractedDirCacheKey, host.GetName()), extractCriDockerdStep.(*docker.ExtractCriDockerdArchiveStep).DefaultExtractionDir)


		installCriDockerdStep := docker.NewInstallCriDockerdBinaryStep(nodePrefix+"InstallCriDockerdBinaries", fmt.Sprintf("%s.%s", docker.CriDockerdExtractedDirCacheKey, host.GetName()), "", "", true)
		installCriDockerdNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: installCriDockerdStep.Meta().Name, Step: installCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{extractCriDockerdNodeID},
		})
		currentHostLastOpNodeID = installCriDockerdNodeID


		// Configure cri-dockerd service if needed
		if len(t.CriDockerdExecStartArgs) > 0 {
			configureCriDockerdSvcStep := docker.NewConfigureCriDockerdServiceStep(nodePrefix+"ConfigureCriDockerdService", "", t.CriDockerdExecStartArgs, true)
			configureCriDockerdSvcNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
				Name: configureCriDockerdSvcStep.Meta().Name, Step: configureCriDockerdSvcStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
			})
			currentHostLastOpNodeID = configureCriDockerdSvcNodeID
		}

		// Manage cri-dockerd service (daemon-reload, enable, start)
		// These depend on Docker service being ready AND CNI plugins being available (for --network-plugin=cni).
		criDockerdServiceDeps := []plan.NodeID{currentHostLastOpNodeID, enableDockerNodeID, extractCNINodeID}

		daemonReloadCriDStep := docker.NewManageCriDockerdServiceStep(nodePrefix+"DaemonReloadCriDockerdSvc", containerdSteps.ActionDaemonReload, true)
		daemonReloadCriDNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: daemonReloadCriDStep.Meta().Name, Step: daemonReloadCriDStep, Hosts: []connector.Host{host}, Dependencies: task.UniqueNodeIDs(criDockerdServiceDeps),
		})
		currentHostLastOpNodeID = daemonReloadCriDNodeID

		enableCriDStep := docker.NewManageCriDockerdServiceStep(nodePrefix+"EnableCriDockerdService", containerdSteps.ActionEnable, true)
		enableCriDNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: enableCriDStep.Meta().Name, Step: enableCriDStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = enableCriDNodeID

		startCriDStep := docker.NewManageCriDockerdServiceStep(nodePrefix+"StartCriDockerdService", containerdSteps.ActionStart, true)
		startCriDNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: startCriDStep.Meta().Name, Step: startCriDStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		currentHostLastOpNodeID = startCriDNodeID

		// Final verification step for this host
		verifyStep := docker.NewVerifyDockerCrictlStep(nodePrefix+"VerifyDockerCrictlSetup", "", "", false)
		verifyNodeID, _ := taskPlanFragment.AddNode(&plan.ExecutionNode{
			Name: verifyStep.Meta().Name, Step: verifyStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{currentHostLastOpNodeID},
		})
		allHostFinalNodes = append(allHostFinalNodes, verifyNodeID)
	}

	taskPlanFragment.CalculateEntryAndExitNodes()

	logger.Info("Docker & cri-dockerd installation plan generated.")
	return taskPlanFragment, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

var _ task.Task = (*InstallDockerTask)(nil)
