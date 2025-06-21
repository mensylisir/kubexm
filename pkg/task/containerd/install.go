package containerd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/resource"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/common" // For common steps like ExtractArchiveStep
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

func (t *InstallContainerdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
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
	// TODO: Get versions, URLs, checksums from cfg.Spec.ContainerRuntime.Containerd or cfg.Spec.Kubernetes / cfg.Spec.Dependencies

	containerdVersion := cfg.Spec.ContainerRuntime.Version // e.g., "1.7.2"
	runcVersion := "v1.1.12" // Example, should come from config
	cniPluginsVersion := "v1.4.0" // Example, should come from config
	arch := "" // Auto-detect by resource handle

	// Containerd Archive
	containerdHandle := resource.NewRemoteBinaryHandle("containerd", containerdVersion, arch,
		"https://github.com/containerd/containerd/releases/download/v{{.Version}}/containerd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		"", "bin/containerd", "", // BinaryPathInArchive is for the main binary, not the whole archive structure
	)
	containerdArchivePlan, err := containerdHandle.EnsurePlan(ctx)
	if err != nil { return nil, err }
	graph.Merge(containerdArchivePlan)
	containerdArchiveLocalPathKey := fmt.Sprintf("resource.%s.downloadedPath", containerdHandle.ID())

	// Runc Binary
	runcHandle := resource.NewRemoteBinaryHandle("runc", strings.TrimPrefix(runcVersion, "v"), arch,
		"https://github.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		"", "runc.{{.Arch}}", "", // BinaryPathInArchive is just the filename itself
	)
	runcBinaryPlan, err := runcHandle.EnsurePlan(ctx) // This handle's EnsurePlan might be simpler (no extract)
	if err != nil { return nil, err }
	graph.Merge(runcBinaryPlan)
	runcBinaryLocalPathKey := fmt.Sprintf("resource.%s.downloadedPath", runcHandle.ID()) // This key points to the runc binary itself

	// CNI Plugins Archive
	cniPluginsHandle := resource.NewRemoteBinaryHandle("cni-plugins", strings.TrimPrefix(cniPluginsVersion, "v"), arch,
		"https://github.com/containernetworking/plugins/releases/download/{{.Version}}/cni-plugins-linux-{{.Arch}}-{{.Version}}.tgz",
		"", "bridge", "", // Dummy BinaryPathInArchive, we care about the archive
	)
	cniPluginsArchivePlan, err := cniPluginsHandle.EnsurePlan(ctx)
	if err != nil { return nil, err }
	graph.Merge(cniPluginsArchivePlan)
	cniPluginsArchiveLocalPathKey := fmt.Sprintf("resource.%s.downloadedPath", cniPluginsHandle.ID())


	// --- 2. Pre-flight checks on all target nodes ---
	loadModulesStep := preflight.NewLoadKernelModulesStep("", []string{"overlay", "br_netfilter"}, true)
	loadModulesNodeID := graph.AddNodePerHost("preflight-load-kernel-modules", targetHosts, loadModulesStep)

	sysctlParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables": "1",
		"net.bridge.bridge-nf-call-ip6tables": "1", // Good to have for IPv6
		"net.ipv4.ip_forward":                "1",
	}
	setSysctlStep := preflight.NewSetSystemConfigStep("", sysctlParams, "/etc/sysctl.d/99-kubernetes-cri.conf", true, true)
	setSysctlNodeID := graph.AddNodePerHost("preflight-set-sysctl-k8s", targetHosts, setSysctlStep)
	// No explicit dependency between modules and sysctl for now, can run in parallel.
	// If ensure resource plans have exit nodes, these could depend on them to ensure downloads finish first.
	// For now, these can start immediately.
	graph.AddEntryNodes(loadModulesNodeID)
	graph.AddEntryNodes(setSysctlNodeID)


	// --- 3. Distribution, Extraction, Installation per node (could be parallelized per node if steps don't conflict) ---
	lastNodeIDs := make(map[string][]plan.NodeID) // Track last step(s) on each node for sequencing

	for _, host := range targetHosts {
		nodePrefix := fmt.Sprintf("containerd-%s-", strings.ReplaceAll(host.GetName(),".","-"))
		currentDeps := []plan.NodeID{
            plan.NodeID(fmt.Sprintf("preflight-load-kernel-modules-%s", strings.ReplaceAll(host.GetName(),".","-"))),
            plan.NodeID(fmt.Sprintf("preflight-set-sysctl-k8s-%s", strings.ReplaceAll(host.GetName(),".","-"))),
        }
		// Add dependencies on resource downloads finishing on control node
		currentDeps = append(currentDeps, containerdArchivePlan.ExitNodes...)
		currentDeps = append(currentDeps, runcBinaryPlan.ExitNodes...)
		currentDeps = append(currentDeps, cniPluginsArchivePlan.ExitNodes...)


		// Containerd
		distContainerdStep := stepContainerd.NewDistributeContainerdArchiveStep(nodePrefix+"DistributeContainerd", containerdArchiveLocalPathKey, "", "", "", true)
		distContainerdNodeID := graph.AddNode(nodePrefix+"dist-containerd", []connector.Host{host}, distContainerdStep, currentDeps...)

		extractContainerdStep := stepContainerd.NewExtractContainerdArchiveStep(nodePrefix+"ExtractContainerd", stepContainerd.ContainerdArchiveRemotePathCacheKey, "", "", "", true, true)
		extractContainerdNodeID := graph.AddNode(nodePrefix+"extract-containerd", []connector.Host{host}, extractContainerdStep, distContainerdNodeID)

		installContainerdStep := stepContainerd.NewInstallContainerdStep(nodePrefix+"InstallContainerdBinaries", stepContainerd.ContainerdExtractedDirCacheKey, nil, "", "", true)
		installContainerdNodeID := graph.AddNode(nodePrefix+"install-containerd-core", []connector.Host{host}, installContainerdStep, extractContainerdNodeID)
		currentDeps = []plan.NodeID{installContainerdNodeID} // Subsequent steps for this host depend on this

		// Runc
		distRuncStep := stepContainerd.NewDistributeRuncBinaryStep(nodePrefix+"DistributeRunc", runcBinaryLocalPathKey, "", "runc", "", true) // Assuming remote name is just "runc"
		distRuncNodeID := graph.AddNode(nodePrefix+"dist-runc", []connector.Host{host}, distRuncStep, currentDeps...) // Depends on containerd install for sequence, or could be parallel

		installRuncStep := stepContainerd.NewInstallRuncBinaryStep(nodePrefix+"InstallRunc", stepContainerd.RuncBinaryRemotePathCacheKey, "/usr/local/sbin/runc", true)
		installRuncNodeID := graph.AddNode(nodePrefix+"install-runc", []connector.Host{host}, installRuncStep, distRuncNodeID)
		currentDeps = append(currentDeps, installRuncNodeID) // Add runc install to current dependencies for this host

		// CNI Plugins
		distCNIStep := stepContainerd.NewDistributeCNIPluginsArchiveStep(nodePrefix+"DistributeCNI", cniPluginsArchiveLocalPathKey, "", "", "", true)
		distCNINodeID := graph.AddNode(nodePrefix+"dist-cni", []connector.Host{host}, distCNIStep, currentDeps...)

		extractCNIStep := stepContainerd.NewExtractCNIPluginsArchiveStep(nodePrefix+"ExtractCNI", stepContainerd.CNIPluginsArchiveRemotePathCacheKey, "/opt/cni/bin", "", true, true)
		extractCNINodeID := graph.AddNode(nodePrefix+"extract-cni", []connector.Host{host}, extractCNIStep, distCNINodeID)
		currentDeps = []plan.NodeID{extractCNINodeID} // Main dependency for next config steps

		// Configuration and Service Management (depends on all binaries being in place)
		// Ensure dependencies for configure step include installContainerdNodeID, installRuncNodeID, extractCNINodeID
		allInstallCompleteDeps := []plan.NodeID{installContainerdNodeID, installRuncNodeID, extractCNINodeID}


		configureStep := stepContainerd.NewConfigureContainerdStep(nodePrefix+"Configure", t.SandboxImage, t.RegistryMirrors, t.InsecureRegistries, t.ContainerdConfigPath, t.RegistryConfigPath, t.ExtraTomlContent, t.UseSystemdCgroup, true)
		configureNodeID := graph.AddNode(nodePrefix+"configure", []connector.Host{host}, configureStep, allInstallCompleteDeps...)

		genServiceStep := stepContainerd.NewGenerateContainerdServiceStep(nodePrefix+"GenerateService", stepContainerd.ContainerdServiceData{}, "", true)
		genServiceNodeID := graph.AddNode(nodePrefix+"gen-service", []connector.Host{host}, genServiceStep, configureNodeID) // Depends on config.toml potentially if service file refers to it

		daemonReloadStep := stepContainerd.NewManageContainerdServiceStep(nodePrefix+"DaemonReload", stepContainerd.ActionDaemonReload, true)
		daemonReloadNodeID := graph.AddNode(nodePrefix+"daemon-reload", []connector.Host{host}, daemonReloadStep, genServiceNodeID)

		enableServiceStep := stepContainerd.NewManageContainerdServiceStep(nodePrefix+"EnableService", stepContainerd.ActionEnable, true)
		enableServiceNodeID := graph.AddNode(nodePrefix+"enable-service", []connector.Host{host}, enableServiceStep, daemonReloadNodeID)

		startServiceStep := stepContainerd.NewManageContainerdServiceStep(nodePrefix+"StartService", stepContainerd.ActionStart, true)
		startServiceNodeID := graph.AddNode(nodePrefix+"start-service", []connector.Host{host}, startServiceStep, enableServiceNodeID)

		verifyStep := stepContainerd.NewVerifyContainerdCrictlStep(nodePrefix+"VerifyCrictl", "", "", false)
		verifyNodeID := graph.AddNode(nodePrefix+"verify-crictl", []connector.Host{host}, verifyStep, startServiceNodeID)

		lastNodeIDs[host.GetName()] = []plan.NodeID{verifyNodeID}
	}

	// Set fragment entry and exit nodes
	// Entry nodes are the initial preflight checks and resource acquisition plans.
	graph.AddEntryNodes(containerdArchivePlan.EntryNodes...)
	graph.AddEntryNodes(runcBinaryPlan.EntryNodes...)
	graph.AddEntryNodes(cniPluginsArchivePlan.EntryNodes...)
	// Preflight checks are already added via AddNodePerHost to graph's entries if they had no deps.

	for _, host := range targetHosts {
		graph.AddExitNodes(lastNodeIDs[host.GetName()]...)
	}
    graph.RemoveDuplicateNodeIDs()

	logger.Info("Containerd installation and configuration plan generated.")
	return graph, nil
}

var _ task.Task = (*InstallContainerdTask)(nil)
