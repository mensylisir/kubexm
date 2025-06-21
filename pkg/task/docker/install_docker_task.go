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

	// --- 1. Resource Acquisition (cri-dockerd, CNI plugins) ---
	criDockerdVersion := "v0.3.10" // Example, from config
    cniPluginsVersion := "v1.4.0" // Example, from config
	arch := "" // Auto-detect

	criDockerdHandle := resource.NewRemoteBinaryHandle("cri-dockerd", strings.TrimPrefix(criDockerdVersion, "v"), arch,
		"https://github.com/Mirantis/cri-dockerd/releases/download/{{.Version}}/cri-dockerd-{{.Version}}.{{.Arch}}.tgz",
		"", "cri-dockerd/cri-dockerd", "", // BinaryPath is cri-dockerd/cri-dockerd after extraction
	)
	criDockerdArchivePlan, err := criDockerdHandle.EnsurePlan(ctx)
	if err != nil { return nil, err }
	graph.Merge(criDockerdArchivePlan)
	criDockerdArchiveLocalPathKey := fmt.Sprintf("resource.%s.downloadedPath", criDockerdHandle.ID())

	cniPluginsHandle := resource.NewRemoteBinaryHandle("cni-plugins", strings.TrimPrefix(cniPluginsVersion, "v"), arch,
		"https://github.com/containernetworking/plugins/releases/download/{{.Version}}/cni-plugins-linux-{{.Arch}}-{{.Version}}.tgz",
		"", "bridge", "",
	)
	cniPluginsArchivePlan, err := cniPluginsHandle.EnsurePlan(ctx)
	if err != nil { return nil, err }
	graph.Merge(cniPluginsArchivePlan)
	cniPluginsArchiveLocalPathKey := fmt.Sprintf("resource.%s.downloadedPath", cniPluginsHandle.ID())

	// --- 2. Pre-flight checks ---
	loadModulesStep := preflight.NewLoadKernelModulesStep("", []string{"overlay", "br_netfilter"}, true)
	loadModulesNodeID := graph.AddNodePerHost("preflight-load-kernel-modules-docker", targetHosts, loadModulesStep)
	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables": "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
		"net.ipv4.ip_forward":                "1",
	}
	setSysctlStep := preflight.NewSetSystemConfigStep("", sysctlParams, "/etc/sysctl.d/99-kubernetes-cri.conf", true, true)
	setSysctlNodeID := graph.AddNodePerHost("preflight-set-sysctl-k8s-docker", targetHosts, setSysctlStep)
	graph.AddEntryNodes(loadModulesNodeID)
	graph.AddEntryNodes(setSysctlNodeID)


	// --- 3. Installation and Configuration per node ---
	lastNodeIDs := make(map[string][]plan.NodeID)

	for _, host := range targetHosts {
		nodePrefix := fmt.Sprintf("docker-%s-", strings.ReplaceAll(host.GetName(),".","-"))
		currentDeps := []plan.NodeID{
            plan.NodeID(fmt.Sprintf("preflight-load-kernel-modules-docker-%s", strings.ReplaceAll(host.GetName(),".","-"))),
            plan.NodeID(fmt.Sprintf("preflight-set-sysctl-k8s-docker-%s", strings.ReplaceAll(host.GetName(),".","-"))),
        }
		// Add dependencies on resource downloads
		currentDeps = append(currentDeps, criDockerdArchivePlan.ExitNodes...)
		currentDeps = append(currentDeps, cniPluginsArchivePlan.ExitNodes...)

		// Install Docker Engine (package based)
		// TODO: ExtraRepoSetupCmds might be OS-dependent and should come from config or facts.
		installDockerStep := docker.NewInstallDockerEngineStep(nodePrefix+"InstallDocker", nil, nil, true)
		installDockerNodeID := graph.AddNode(nodePrefix+"install-docker", []connector.Host{host}, installDockerStep, currentDeps...)

		// Configure Docker daemon.json
		genDaemonJSONStep := docker.NewGenerateDockerDaemonJSONStep(nodePrefix+"GenDockerDaemonJSON", t.DockerDaemonConfig, "", true)
		genDaemonJSONNodeID := graph.AddNode(nodePrefix+"gen-daemon-json", []connector.Host{host}, genDaemonJSONStep, installDockerNodeID)

		// Manage Docker Service (Restart to apply daemon.json)
		manageDockerRestartStep := docker.NewManageDockerServiceStep(nodePrefix+"RestartDocker", containerdSteps.ActionRestart, true)
		manageDockerRestartNodeID := graph.AddNode(nodePrefix+"restart-docker", []connector.Host{host}, manageDockerRestartStep, genDaemonJSONNodeID)

		enableDockerStep := docker.NewManageDockerServiceStep(nodePrefix+"EnableDocker", containerdSteps.ActionEnable, true)
		enableDockerNodeID := graph.AddNode(nodePrefix+"enable-docker", []connector.Host{host}, enableDockerStep, manageDockerRestartNodeID) // Enable after ensuring it runs with new config

		// CNI Plugins (same as containerd task)
		distCNIStep := containerdSteps.NewDistributeCNIPluginsArchiveStep(nodePrefix+"DistributeCNI", cniPluginsArchiveLocalPathKey, "", "", "", true)
		distCNINodeID := graph.AddNode(nodePrefix+"dist-cni", []connector.Host{host}, distCNIStep, currentDeps...) // Can be parallel to Docker install up to a point

		extractCNIStep := containerdSteps.NewExtractCNIPluginsArchiveStep(nodePrefix+"ExtractCNI", containerdSteps.CNIPluginsArchiveRemotePathCacheKey, "/opt/cni/bin", "", true, true)
		extractCNINodeID := graph.AddNode(nodePrefix+"extract-cni", []connector.Host{host}, extractCNIStep, distCNINodeID)

		// cri-dockerd
		distCriDockerdStep := docker.NewDistributeCriDockerdArchiveStep(nodePrefix+"DistributeCriDockerd", criDockerdArchiveLocalPathKey, "", "", "", true)
		distCriDockerdNodeID := graph.AddNode(nodePrefix+"dist-cri-dockerd", []connector.Host{host}, distCriDockerdStep, currentDeps...) // Parallel

		extractCriDockerdStep := docker.NewExtractCriDockerdArchiveStep(nodePrefix+"ExtractCriDockerd", docker.CriDockerdArchiveRemotePathCacheKey, "", "", "", true, true)
		extractCriDockerdNodeID := graph.AddNode(nodePrefix+"extract-cri-dockerd", []connector.Host{host}, extractCriDockerdStep, distCriDockerdNodeID)

		installCriDockerdStep := docker.NewInstallCriDockerdBinaryStep(nodePrefix+"InstallCriDockerd", docker.CriDockerdExtractedDirCacheKey, "", "", true)
		installCriDockerdNodeID := graph.AddNode(nodePrefix+"install-cri-dockerd", []connector.Host{host}, installCriDockerdStep, extractCriDockerdNodeID)

		// Configure cri-dockerd service (if needed beyond default unit file args)
		// For now, assume default unit file is fine, or args are passed via task spec to modify later.
		// If t.CriDockerdExecStartArgs is populated, use ConfigureCriDockerdServiceStep.
		var lastCriDockerdSetupStep plan.NodeID = installCriDockerdNodeID
		if len(t.CriDockerdExecStartArgs) > 0 {
			configureCriDockerdSvcStep := docker.NewConfigureCriDockerdServiceStep(nodePrefix+"ConfigureCriDockerdSvc", "", t.CriDockerdExecStartArgs, true)
			configureCriDockerdSvcNodeID := graph.AddNode(nodePrefix+"config-cri-dockerd-svc", []connector.Host{host}, configureCriDockerdSvcStep, installCriDockerdNodeID)
			lastCriDockerdSetupStep = configureCriDockerdSvcNodeID
		}

		// Manage cri-dockerd service
		daemonReloadCriDockerdStep := docker.NewManageCriDockerdServiceStep(nodePrefix+"DaemonReloadCriDockerd", containerdSteps.ActionDaemonReload, true)
		daemonReloadCriDockerdNodeID := graph.AddNode(nodePrefix+"daemon-reload-cri-dockerd", []connector.Host{host}, daemonReloadCriDockerdStep, lastCriDockerdSetupStep)

		enableCriDockerdStep := docker.NewManageCriDockerdServiceStep(nodePrefix+"EnableCriDockerd", containerdSteps.ActionEnable, true)
		enableCriDockerdNodeID := graph.AddNode(nodePrefix+"enable-cri-dockerd", []connector.Host{host}, enableCriDockerdStep, daemonReloadCriDockerdNodeID)

		startCriDockerdStep := docker.NewManageCriDockerdServiceStep(nodePrefix+"StartCriDockerd", containerdSteps.ActionStart, true)
		startCriDockerdNodeID := graph.AddNode(nodePrefix+"start-cri-dockerd", []connector.Host{host}, startCriDockerdStep, enableCriDockerdNodeID)

		// Final verification
		// All components (Docker, CNI, cri-dockerd) should be ready before this.
		allServicesReadyDeps := []plan.NodeID{enableDockerNodeID, extractCNINodeID, startCriDockerdNodeID}

		verifyStep := docker.NewVerifyDockerCrictlStep(nodePrefix+"VerifyCrictl", "", "", false)
		verifyNodeID := graph.AddNode(nodePrefix+"verify-crictl", []connector.Host{host}, verifyStep, allServicesReadyDeps...)

		lastNodeIDs[host.GetName()] = []plan.NodeID{verifyNodeID}
	}

	graph.AddEntryNodes(criDockerdArchivePlan.EntryNodes...)
	graph.AddEntryNodes(cniPluginsArchivePlan.EntryNodes...)
	// Pre-flight entry nodes already added.

	for _, host := range targetHosts {
		graph.AddExitNodes(lastNodeIDs[host.GetName()]...)
	}
    graph.RemoveDuplicateNodeIDs()

	logger.Info("Docker & cri-dockerd installation plan generated.")
	return graph, nil
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
