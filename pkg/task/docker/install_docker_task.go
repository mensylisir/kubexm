package docker

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	stepContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
	"github.com/mensylisir/kubexm/pkg/step/docker"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallDockerTask installs Docker and cri-dockerd.
type InstallDockerTask struct {
	task.BaseTask
}

// NewInstallDockerTask creates a new task for installing Docker and cri-dockerd.
func NewInstallDockerTask() task.Task {
	return &InstallDockerTask{
		BaseTask: task.NewBaseTask(
			"InstallAndConfigureDocker",
			"Installs Docker engine, cri-dockerd, and CNI plugins.",
			[]string{common.RoleMaster, common.RoleWorker},
			nil,
			false,
		),
	}
}

func (t *InstallDockerTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())
	cfg := ctx.GetClusterConfig()

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	// --- 1. Resource Acquisition on Control Node ---
	criDockerdVersion := "v0.3.10" // Example
	cniPluginsVersion := "v1.4.0" // Example

	localCriDockerdPath := filepath.Join(ctx.GetGlobalWorkDir(), fmt.Sprintf("cri-dockerd-%s.amd64.tgz", criDockerdVersion))
	localCniPath := filepath.Join(ctx.GetGlobalWorkDir(), fmt.Sprintf("cni-plugins-linux-amd64-%s.tgz", cniPluginsVersion))

	criDockerdURL := fmt.Sprintf("https://github.com/Mirantis/cri-dockerd/releases/download/%s/cri-dockerd-%s.amd64.tgz", criDockerdVersion, criDockerdVersion)
	cniURL := fmt.Sprintf("https://github.com/containernetworking/plugins/releases/download/%s/cni-plugins-linux-amd64-%s.tgz", cniPluginsVersion, cniPluginsVersion)

	downloadCriDockerdStep := commonstep.NewDownloadFileStep("DownloadCriDockerd", criDockerdURL, localCriDockerdPath, "", "0644", false)
	downloadCniStep := commonstep.NewDownloadFileStep("DownloadCniPlugins", cniURL, localCniPath, "", "0644", false)

	downloadCriDockerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCriDockerd", Step: downloadCriDockerdStep, Hosts: []connector.Host{controlPlaneHost}})
	downloadCniNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCni", Step: downloadCniStep, Hosts: []connector.Host{controlPlaneHost}})

	downloadDependencies := []plan.NodeID{downloadCriDockerdNodeID, downloadCniNodeID}

	// --- 2. Installation and Configuration per node ---
	targetHosts, err := ctx.GetHostsByRole(t.GetRunOnRoles()...)
	if err != nil {
		return nil, err
	}

	var allFinalNodes []plan.NodeID
	for _, host := range targetHosts {
		// Install Docker Engine (via package manager)
		installDockerStep := docker.NewInstallDockerEngineStep("InstallDockerEngine-"+host.GetName(), nil, nil, true)
		installDockerNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "InstallDockerEngine-"+host.GetName(), Step: installDockerStep, Hosts: []connector.Host{host}})

		// Configure Docker daemon.json
		dockerDaemonConfig := docker.DockerDaemonConfig{ExecOpts: []string{"native.cgroupdriver=systemd"}} // Simplified
		genDaemonJSONStep := docker.NewGenerateDockerDaemonJSONStep("GenerateDockerDaemonJSON-"+host.GetName(), dockerDaemonConfig, "", true)
		genDaemonJSONNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "GenerateDockerDaemonJSON-"+host.GetName(), Step: genDaemonJSONStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installDockerNodeID}})

		// Restart and Enable Docker
		restartDockerStep := docker.NewManageDockerServiceStep("RestartDocker-"+host.GetName(), stepContainerd.ActionRestart, true)
		restartDockerNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "RestartDocker-"+host.GetName(), Step: restartDockerStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{genDaemonJSONNodeID}})

		enableDockerStep := docker.NewManageDockerServiceStep("EnableDocker-"+host.GetName(), stepContainerd.ActionEnable, true)
		enableDockerNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "EnableDocker-"+host.GetName(), Step: enableDockerStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{restartDockerNodeID}})

		// Install CNI
		remoteCniPath := filepath.Join("/tmp", filepath.Base(localCniPath))
		uploadCniStep := commonstep.NewUploadFileStep("UploadCni-"+host.GetName(), localCniPath, remoteCniPath, "0644", true, false)
		uploadCniNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "UploadCni-"+host.GetName(), Step: uploadCniStep, Hosts: []connector.Host{host}, Dependencies: downloadDependencies})

		extractCniStep := stepContainerd.NewExtractCNIPluginsArchiveStep("ExtractCni-"+host.GetName(), remoteCniPath, "/opt/cni/bin", "", true, true)
		extractCniNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCni-"+host.GetName(), Step: extractCniStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadCniNodeID}})

		// Install cri-dockerd
		remoteCriDockerdPath := filepath.Join("/tmp", filepath.Base(localCriDockerdPath))
		uploadCriDockerdStep := commonstep.NewUploadFileStep("UploadCriDockerd-"+host.GetName(), localCriDockerdPath, remoteCriDockerdPath, "0644", true, false)
		uploadCriDockerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "UploadCriDockerd-"+host.GetName(), Step: uploadCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: downloadDependencies})

		extractCriDockerdStep := docker.NewExtractCriDockerdArchiveStep("ExtractCriDockerd-"+host.GetName(), remoteCriDockerdPath, "", "", "", true, true)
		extractCriDockerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "ExtractCriDockerd-"+host.GetName(), Step: extractCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{uploadCriDockerdNodeID}})

		installCriDockerdStep := docker.NewInstallCriDockerdBinaryStep("InstallCriDockerd-"+host.GetName(), extractCriDockerdStep.DefaultExtractionDir, "", "", true)
		installCriDockerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "InstallCriDockerd-"+host.GetName(), Step: installCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{extractCriDockerdNodeID}})

		// Start cri-dockerd service
		enableCriDockerdStep := docker.NewManageCriDockerdServiceStep("EnableCriDockerd-"+host.GetName(), stepContainerd.ActionEnable, true)
		enableCriDockerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "EnableCriDockerd-"+host.GetName(), Step: enableCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installCriDockerdNodeID, enableDockerNodeID, extractCniNodeID}})

		startCriDockerdStep := docker.NewManageCriDockerdServiceStep("StartCriDockerd-"+host.GetName(), stepContainerd.ActionStart, true)
		startCriDockerdNodeID, _ := fragment.AddNode(&plan.ExecutionNode{Name: "StartCriDockerd-"+host.GetName(), Step: startCriDockerdStep, Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableCriDockerdNodeID}})

		allFinalNodes = append(allFinalNodes, startCriDockerdNodeID)
	}

	fragment.EntryNodes = downloadDependencies
	fragment.ExitNodes = allFinalNodes

	logger.Info("Docker installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallDockerTask)(nil)
