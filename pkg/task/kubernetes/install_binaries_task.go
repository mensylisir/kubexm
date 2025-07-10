package kubernetes

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/resource"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
	// etcdsteps "github.com/mensylisir/kubexm/pkg/step/etcd" // If etcd binaries are also handled here.
)

// InstallBinariesTask distributes and installs Kubernetes binaries (kubelet, kubeadm, kubectl)
// and potentially other core binaries like etcd if managed together.
type InstallBinariesTask struct {
	task.BaseTask
	// KubeVersion string
	// EtcdVersion string // If etcd is handled here
	// TargetDir string // e.g., /usr/local/bin
}

// NewInstallBinariesTask creates a new InstallBinariesTask.
func NewInstallBinariesTask(runOnRoles []string) task.Task {
	return &InstallBinariesTask{
		BaseTask: task.NewBaseTask( // Use NewBaseTask
			"InstallKubernetesCoreBinaries",
			"Distributes and installs kubelet, kubeadm, and kubectl binaries.",
			runOnRoles, // Use passed in roles
			nil,        // HostFilter
			false,      // IgnoreError
		),
	}
}

func (t *InstallBinariesTask) IsRequired(ctx task.TaskContext) (bool, error) {
	// Required if Kubernetes is being installed/managed by this tool.
	// Could check ClusterConfig.Spec.Kubernetes.Type or similar.
	// For now, assume if this task is in a pipeline, it's required.
	if len(t.BaseTask.RunOnRoles) == 0 { // If no roles specified for this task instance
		ctx.GetLogger().Info("No target roles specified for InstallKubernetesCoreBinaries task, skipping.")
		return false, nil
	}
	return true, nil
}

func (t *InstallBinariesTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	taskFragment := task.NewExecutionFragment(t.Name())

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg.Spec.Kubernetes == nil || clusterCfg.Spec.Kubernetes.Version == "" {
		return nil, fmt.Errorf("Kubernetes version not specified in configuration for task %s", t.Name())
	}
	k8sVersion := clusterCfg.Spec.Kubernetes.Version

	targetHosts, err := ctx.GetHostsByRole(t.BaseTask.RunOnRoles...)
	if err != nil {
		return nil, fmt.Errorf("failed to get target hosts for task %s: %w", t.Name(), err)
	}
	if len(targetHosts) == 0 {
		logger.Info("No target hosts found for roles, skipping binary installation.", "roles", t.BaseTask.RunOnRoles)
		return task.NewEmptyFragment(), nil
	}

	binariesToInstall := []string{"kubeadm", "kubelet", "kubectl"}
	// TODO: Add CNI binaries if they are distributed this way (e.g. "bridge", "host-local", "loopback")
	// For now, CNI plugins are handled by containerd/docker tasks or CNI specific tasks.

	var allLocalPrepExitNodes []plan.NodeID

	// 1. Resource Acquisition for each binary on Control Node
	localBinaryPaths := make(map[string]string) // Store local path for each binary

	for _, binaryName := range binariesToInstall {
		logger.Debug("Planning resource acquisition for binary.", "binary", binaryName, "version", k8sVersion)
		// Arch can be determined per host, but for download, typically use control node's arch or a common one.
		// For simplicity, assuming common arch or handle auto-detects.
		// The resource handle will use control node's arch if not specified.
		binaryHandle, err := resource.NewRemoteBinaryHandle(ctx,
			binaryName, k8sVersion, "", "", // Arch and OS can be auto-detected by handle
			binaryName, // BinaryNameInArchive (kubeadm, kubelet, kubectl are usually direct or in a /bin dir)
			"", "",     // No checksums for now, add if available from a manifest
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote binary handle for %s: %w", binaryName, err)
		}

		prepFragment, err := binaryHandle.EnsurePlan(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to plan resource acquisition for %s: %w", binaryName, err)
		}
		if err := taskFragment.MergeFragment(prepFragment); err != nil {
			return nil, fmt.Errorf("failed to merge prep fragment for %s: %w", binaryName, err)
		}
		allLocalPrepExitNodes = append(allLocalPrepExitNodes, prepFragment.ExitNodes...)

		localPath, err := binaryHandle.Path(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get local path for prepared binary %s: %w", binaryName, err)
		}
		localBinaryPaths[binaryName] = localPath
	}
	allLocalPrepExitNodes = plan.UniqueNodeIDs(allLocalPrepExitNodes)

	// 2. Distribution and Installation on Target Hosts
	var allInstallExitNodes []plan.NodeID

	for _, host := range targetHosts {
		hostArch := host.GetArch() // Assuming connector.Host has GetArch() from facts
		if hostArch == "" { // Fallback if fact not available or host is local control node before full facts
			cn, _ := ctx.GetControlNode()
			facts, _ := ctx.GetHostFacts(cn)
			if facts != nil && facts.OS != nil { hostArch = facts.OS.Arch } else { hostArch = "amd64" } // Ultimate fallback
		}


		var lastStepOnHost plan.NodeID = "" // Not strictly needed if all binary installs on a host are parallel

		for _, binaryName := range binariesToInstall {
			nodePrefix := fmt.Sprintf("%s-%s-%s-", binaryName, host.GetName(), hostArch)
			localPath := localBinaryPaths[binaryName] // Path on control node

			if localPath == "" { // Should not happen if prep was successful
				return nil, fmt.Errorf("local path for binary %s not found after resource prep", binaryName)
			}

			remoteTempPath := filepath.Join("/tmp", binaryName+"-"+k8sVersion)
			finalInstallPath := filepath.Join(common.KubeBinDir, binaryName) // e.g., /usr/local/bin/kubelet

			// Upload binary
			uploadStep := commonstep.NewUploadFileStep(nodePrefix+"Upload", localPath, remoteTempPath, "0755", false, false) // sudo=false for /tmp
			uploadNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
				Name:         uploadStep.Meta().Name,
				Step:         uploadStep,
				Hosts:        []connector.Host{host},
				Dependencies: allLocalPrepExitNodes, // Depends on all binaries being ready locally
			})

			// Install (move and chmod)
			// Ensure KubeBinDir exists
			mkdirCmd := fmt.Sprintf("mkdir -p %s", common.KubeBinDir)
			ensureDirStep := commonstep.NewCommandStep(nodePrefix+"EnsureBinDir", mkdirCmd, true, false, 0, nil, 0, "", false, 0, "", false)
			ensureDirNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
				Name: ensureDirStep.Meta().Name,
				Step: ensureDirStep,
				Hosts: []connector.Host{host},
				Dependencies: []plan.NodeID{uploadNodeID}, // Depends on its own binary's upload
			})

			installCmd := fmt.Sprintf("mv %s %s && chmod +x %s", remoteTempPath, finalInstallPath, finalInstallPath)
			installStep := commonstep.NewCommandStep(nodePrefix+"Install", installCmd, true, false, 0, nil, 0, "", false, 0, "", false)
			installNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{
				Name:         installStep.Meta().Name,
				Step:         installStep,
				Hosts:        []connector.Host{host},
				Dependencies: []plan.NodeID{ensureDirNodeID},
			})
			allInstallExitNodes = append(allInstallExitNodes, installNodeID)
		}
	}

	taskFragment.CalculateEntryAndExitNodes()
	if taskFragment.IsEmpty() {
		logger.Info("InstallKubernetesCoreBinariesTask planned no executable nodes.")
	} else {
		logger.Info("InstallKubernetesCoreBinariesTask planning complete.", "entryNodes", taskFragment.EntryNodes, "exitNodes", taskFragment.ExitNodes)
	}
	return taskFragment, nil
}

var _ task.Task = (*InstallBinariesTask)(nil)
