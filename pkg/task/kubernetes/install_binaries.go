package kubernetes

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallBinariesTask distributes and installs Kubernetes binaries.
type InstallBinariesTask struct {
	task.BaseTask
}

// NewInstallBinariesTask creates a new InstallBinariesTask.
func NewInstallBinariesTask() task.Task {
	return &InstallBinariesTask{
		BaseTask: task.NewBaseTask(
			"InstallKubernetesBinaries",
			"Distributes and installs kubelet, kubeadm, and kubectl binaries.",
			[]string{common.RoleMaster, common.RoleWorker}, // This task runs on all nodes
			nil,
			false,
		),
	}
}

func (t *InstallBinariesTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	k8sVersion := ctx.GetClusterConfig().Spec.Kubernetes.Version
	if k8sVersion == "" {
		return nil, fmt.Errorf("kubernetes version is not specified in the cluster configuration")
	}

	binaries := []string{"kubeadm", "kubelet", "kubectl"}
	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for task %s: %w", t.Name(), err)
	}

	// 1. Download all binaries to the control plane node first.
	var downloadExitNodes []plan.NodeID
	localPaths := make(map[string]string)

	for _, binary := range binaries {
		// This is a simplified URL. A real implementation would need a more robust way
		// to get the correct URL based on version and architecture.
		url := fmt.Sprintf("https://storage.googleapis.com/kubernetes-release/release/%s/bin/linux/amd64/%s", k8sVersion, binary)
		localPath := filepath.Join(ctx.GetGlobalWorkDir(), binary)
		localPaths[binary] = localPath

		downloadStep := commonstep.NewDownloadFileStep(
			fmt.Sprintf("Download-%s", binary),
			url,
			localPath,
			"", // checksum
			"0755",
			false, // sudo
		)
		downloadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  downloadStep.Meta().Name,
			Step:  downloadStep,
			Hosts: []connector.Host{controlPlaneHost},
		})
		downloadExitNodes = append(downloadExitNodes, downloadNodeID)
	}

	// 2. Distribute and install binaries on all target nodes.
	targetHosts, err := ctx.GetHostsByRole(t.GetRunOnRoles()...)
	if err != nil {
		return nil, err
	}

	var installExitNodes []plan.NodeID
	for _, host := range targetHosts {
		for _, binary := range binaries {
			localPath := localPaths[binary]
			remotePath := filepath.Join("/usr/local/bin", binary)

			uploadStep := commonstep.NewUploadFileStep(
				fmt.Sprintf("Upload-%s-to-%s", binary, host.GetName()),
				localPath,
				remotePath,
				"0755", // executable permission
				true,   // sudo
				false,
			)
			uploadNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
				Name:         uploadStep.Meta().Name,
				Step:         uploadStep,
				Hosts:        []connector.Host{host},
				Dependencies: downloadExitNodes,
			})
			installExitNodes = append(installExitNodes, uploadNodeID)
		}
	}

	fragment.EntryNodes = downloadExitNodes
	fragment.ExitNodes = installExitNodes

	logger.Info("Kubernetes binaries installation task planning complete.")
	return fragment, nil
}

var _ task.Task = (*InstallBinariesTask)(nil)
