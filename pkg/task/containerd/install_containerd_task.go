package containerd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	stepContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallContainerdTask installs and configures containerd.
type InstallContainerdTask struct {
	task.BaseTask
}

// NewInstallContainerdTask creates a new task for installing and configuring containerd.
func NewInstallContainerdTask() task.Task {
	return &InstallContainerdTask{
		BaseTask: task.NewBaseTask(
			"InstallAndConfigureContainerd",
			"Installs and configures containerd, runc, and CNI plugins.",
			[]string{common.RoleMaster, common.RoleWorker},
			nil,
			false,
		),
	}
}

func (t *InstallContainerdTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())
	cfg := ctx.GetClusterConfig()

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	// --- 1. Resource Acquisition on Control Node ---
	containerdVersion := "1.7.13" // Example, should come from config
	runcVersion := "v1.1.12"      // Example
	cniPluginsVersion := "v1.4.0" // Example
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Version != "" {
		containerdVersion = cfg.Spec.ContainerRuntime.Version
	}

	// Define local paths for downloaded files
	localContainerdPath := filepath.Join(ctx.GetGlobalWorkDir(), fmt.Sprintf("containerd-%s-linux-amd64.tar.gz", containerdVersion))
	localRuncPath := filepath.Join(ctx.GetGlobalWorkDir(), "runc.amd64")
	localCniPath := filepath.Join(ctx.GetGlobalWorkDir(), fmt.Sprintf("cni-plugins-linux-amd64-%s.tgz", cniPluginsVersion))

	// Simplified URLs, a real implementation would have better logic
	containerdURL := fmt.Sprintf("https://github.com/containerd/containerd/releases/download/v%s/containerd-%s-linux-amd64.tar.gz", containerdVersion, containerdVersion)
	runcURL := fmt.Sprintf("https://github.com/opencontainers/runc/releases/download/%s/runc.amd64", runcVersion)
	cniURL := fmt.Sprintf("https://github.com/containernetworking/plugins/releases/download/%s/cni-plugins-linux-amd64-%s.tgz", cniPluginsVersion, cniPluginsVersion)

	// Create download steps
	downloadContainerdStep := commonstep.NewDownloadFileStep("DownloadContainerd", containerdURL, localContainerdPath, "", "0644", false)
	downloadRuncStep := commonstep.NewDownloadFileStep("DownloadRunc", runcURL, localRuncPath, "", "0755", false)
	downloadCniStep := commonstep.NewDownloadFileStep("DownloadCniPlugins", cniURL, localCniPath, "", "0644", false)

	downloadContainerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "DownloadContainerd", Step: downloadContainerdStep, Hosts: []connector.Host{controlPlaneHost}})
	downloadRuncNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "DownloadRunc", Step: downloadRuncStep, Hosts: []connector.Host{controlPlaneHost}})
	downloadCniNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCni", Step: downloadCniStep, Hosts: []connector.Host{controlPlaneHost}})

	downloadDependencies := []plan.NodeID{downloadContainerdNodeID, downloadRuncNodeID, downloadCniNodeID}

	// --- 2. Distribution, Extraction, Installation per node ---
	targetHosts, err := ctx.GetHostsByRole(t.GetRunOnRoles()...)
	if err != nil {
		return nil, err
	}

	var allFinalNodes []plan.NodeID
	for _, host := range targetHosts {
		// Upload binaries
		remoteContainerdPath := filepath.Join("/tmp", filepath.Base(localContainerdPath))
		remoteRuncPath := filepath.Join("/tmp", filepath.Base(localRuncPath))
		remoteCniPath := filepath.Join("/tmp", filepath.Base(localCniPath))

		uploadContainerdStep := commonstep.NewUploadFileStep("UploadContainerd-"+host.GetName(), localContainerdPath, remoteContainerdPath, "0644", true, false)
		uploadRuncStep := commonstep.NewUploadFileStep("UploadRunc-"+host.GetName(), localRuncPath, remoteRuncPath, "0755", true, false)
		uploadCniStep := commonstep.NewUploadFileStep("UploadCni-"+host.GetName(), localCniPath, remoteCniPath, "0644", true, false)

		uploadContainerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "UploadContainerd-"+host.GetName(), Step: uploadContainerdStep, Hosts: []connector.Host{host}, Dependencies: downloadDependencies})
		uploadRuncNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "UploadRunc-"+host.GetName(), Step: uploadRuncStep, Hosts: []connector.Host{host}, Dependencies: downloadDependencies})
		uploadCniNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "UploadCni-"+host.GetName(), Step: uploadCniStep, Hosts: []connector.Host{host}, Dependencies: downloadDependencies})

		// Install/Extract
		extractContainerdStep := stepContainerd.NewExtractContainerdStep("ExtractContainerd-"+host.GetName(), remoteContainerdPath, "/usr/local", true)
		installRuncStep := stepContainerd.NewInstallRuncBinaryStep("InstallRunc-"+host.GetName(), remoteRuncPath, "/usr/local/sbin/runc", true)
		extractCniStep := stepContainerd.NewExtractCNIPluginsArchiveStep("ExtractCni-"+host.GetName(), remoteCniPath, "/opt/cni/bin", "", true, true)

		extractContainerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "ExtractContainerd-"+host.GetName(), Step: extractContainerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadContainerdNodeID}})
		installRuncNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "InstallRunc-"+host.GetName(), Step: installRuncStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadRuncNodeID}})
		extractCniNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCni-"+host.GetName(), Step: extractCniStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadCniNodeID}})

		binariesReadyDeps := []plan.NodeID{extractContainerdNodeID, installRuncNodeID, extractCniNodeID}

		// Configure and start service
		configureStep := stepContainerd.NewConfigureContainerdStep("ConfigureContainerd-"+host.GetName(), "registry.k8s.io/pause:3.9", nil, nil, "", "", "", true, true)
		configureNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureContainerd-"+host.GetName(), Step: configureStep, Hosts: []connector.Host{host}, Dependencies: binariesReadyDeps})

		genServiceStep := stepContainerd.NewGenerateContainerdServiceStep("GenerateContainerdService-"+host.GetName(), stepContainerd.ContainerdServiceData{}, "", true)
		genServiceNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "GenerateContainerdService-"+host.GetName(), Step: genServiceStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{configureNodeID}})

		enableServiceStep := stepContainerd.NewManageContainerdServiceStep("EnableContainerd-"+host.GetName(), stepContainerd.ActionEnable, true)
		enableServiceNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "EnableContainerd-"+host.GetName(), Step: enableServiceStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{genServiceNodeID}})

		startServiceStep := stepContainerd.NewManageContainerdServiceStep("StartContainerd-"+host.GetName(), stepContainerd.ActionStart, true)
		startServiceNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "StartContainerd-"+host.GetName(), Step: startServiceStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableServiceNodeID}})

		allFinalNodes = append(allFinalNodes, startServiceNodeID)
	}

	fragment.EntryNodes = downloadDependencies
	fragment.ExitNodes = allFinalNodes

	logger.Info("Containerd installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallContainerdTask)(nil)
