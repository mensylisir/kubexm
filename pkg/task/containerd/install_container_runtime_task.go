package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/resource"
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/command"
	"github.com/mensylisir/kubexm/pkg/task"
)

// InstallContainerRuntimeTask installs and configures the container runtime.
type InstallContainerRuntimeTask struct {
	name        string
	description string
}

// NewInstallContainerRuntimeTask creates a new InstallContainerRuntimeTask.
func NewInstallContainerRuntimeTask() task.Task {
	return &InstallContainerRuntimeTask{
		name:        "InstallContainerRuntime",
		description: "Downloads, installs and configures the container runtime (containerd)",
	}
}

// Name returns the task name.
func (t *InstallContainerRuntimeTask) Name() string {
	return t.name
}

// Description returns the task description.
func (t *InstallContainerRuntimeTask) Description() string {
	return t.description
}

// IsRequired determines if container runtime installation is needed.
func (t *InstallContainerRuntimeTask) IsRequired(ctx task.TaskContext) (bool, error) {
	clusterConfig := ctx.GetClusterConfig()
	if clusterConfig == nil {
		return false, fmt.Errorf("cluster config is nil")
	}

	// Check if container runtime is specified in config
	if clusterConfig.Spec.ContainerRuntime == nil {
		return false, nil // No container runtime configured
	}

	return true, nil
}

// Plan generates an execution fragment for installing the container runtime.
func (t *InstallContainerRuntimeTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	fragment := task.NewExecutionFragment("install-container-runtime")
	logger := ctx.GetLogger().With("task", t.Name())

	clusterConfig := ctx.GetClusterConfig()
	containerRuntimeConfig := clusterConfig.Spec.ContainerRuntime

	// Get all nodes that need container runtime
	allNodes, err := ctx.GetHostsByRole("all")
	if err != nil {
		return nil, fmt.Errorf("failed to get all nodes: %w", err)
	}

	if len(allNodes) == 0 {
		logger.Warn("No nodes found for container runtime installation")
		return task.NewEmptyFragment("install-container-runtime-empty"), nil
	}

	// Create resource handle for containerd binary
	containerdHandle, err := resource.NewRemoteBinaryHandle(
		ctx,
		"containerd",
		containerRuntimeConfig.Version,
		"", // Auto-detect architecture
		"", // Auto-detect OS
		"bin/containerd", // Binary name in archive
		"", // No checksum for now
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create containerd resource handle: %w", err)
	}

	// Generate resource acquisition plan (runs on control node)
	resourceFragment, err := containerdHandle.EnsurePlan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate containerd resource plan: %w", err)
	}

	// Merge resource acquisition into our fragment
	if !resourceFragment.IsEmpty() {
		err = fragment.MergeFragment(resourceFragment)
		if err != nil {
			return nil, fmt.Errorf("failed to merge resource fragment: %w", err)
		}
	}

	// Get the local path of the binary after resource acquisition
	containerdBinaryPath, err := containerdHandle.Path(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get containerd binary path: %w", err)
	}

	logger.Info("Planning container runtime installation", 
		"version", containerRuntimeConfig.Version,
		"node_count", len(allNodes),
		"binary_path", containerdBinaryPath)

	// Create installation steps for each node
	for i, node := range allNodes {
		nodePrefix := fmt.Sprintf("node-%d", i)
		
		// 1. Upload containerd binary
		uploadNodeID := plan.NodeID(fmt.Sprintf("upload-containerd-%s", nodePrefix))
		uploadStep := common.NewUploadFileStep(
			fmt.Sprintf("upload-containerd-%s", node.GetName()),
			containerdBinaryPath,
			"/usr/local/bin/containerd",
			"0755",
			true, // sudo
		)

		uploadNode := &plan.ExecutionNode{
			Name:         fmt.Sprintf("Upload containerd to %s", node.GetName()),
			Step:         uploadStep,
			Hosts:        []connector.Host{node},
			Dependencies: resourceFragment.ExitNodes, // Wait for resource acquisition
			StepName:     uploadStep.Meta().Name,
		}

		_, err := fragment.AddNode(uploadNode, uploadNodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to add upload node for %s: %w", node.GetName(), err)
		}

		// 2. Configure containerd
		configNodeID := plan.NodeID(fmt.Sprintf("configure-containerd-%s", nodePrefix))
		configureStep := common.NewRenderTemplateStep(
			fmt.Sprintf("configure-containerd-%s", node.GetName()),
			"containerd/config.toml.tmpl",
			map[string]interface{}{
				"SystemdCgroup": containerRuntimeConfig.SystemdCgroup,
				"Registry":      clusterConfig.Spec.Registry,
			},
			"/etc/containerd/config.toml",
			"0644",
			true, // sudo
		)

		configNode := &plan.ExecutionNode{
			Name:         fmt.Sprintf("Configure containerd on %s", node.GetName()),
			Step:         configureStep,
			Hosts:        []connector.Host{node},
			Dependencies: []plan.NodeID{uploadNodeID},
			StepName:     configureStep.Meta().Name,
		}

		_, err = fragment.AddNode(configNode, configNodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to add config node for %s: %w", node.GetName(), err)
		}

		// 3. Install and start containerd service
		serviceNodeID := plan.NodeID(fmt.Sprintf("start-containerd-%s", nodePrefix))
		serviceStep := command.NewCommandStep(
			fmt.Sprintf("start-containerd-%s", node.GetName()),
			"systemctl daemon-reload && systemctl enable containerd && systemctl start containerd",
			true,  // sudo
			false, // ignoreError
			0,     // timeout (use default)
			nil,   // env
			0,     // expectedExitCode
			"systemctl is-active containerd", // check command
			false, // checkSudo
			0,     // checkExpectedExitCode
			"systemctl stop containerd && systemctl disable containerd", // rollback
			true,  // rollbackSudo
		)

		serviceNode := &plan.ExecutionNode{
			Name:         fmt.Sprintf("Start containerd service on %s", node.GetName()),
			Step:         serviceStep,
			Hosts:        []connector.Host{node},
			Dependencies: []plan.NodeID{configNodeID},
			StepName:     serviceStep.Meta().Name,
		}

		_, err = fragment.AddNode(serviceNode, serviceNodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to add service node for %s: %w", node.GetName(), err)
		}

		// The service start nodes are the exit nodes for this task
		fragment.ExitNodes = append(fragment.ExitNodes, serviceNodeID)
	}

	// Calculate entry nodes - these are either resource acquisition nodes or upload nodes
	if resourceFragment.IsEmpty() {
		// If no resource acquisition needed, uploads are entry nodes
		for i := range allNodes {
			fragment.EntryNodes = append(fragment.EntryNodes, 
				plan.NodeID(fmt.Sprintf("upload-containerd-node-%d", i)))
		}
	} else {
		// Resource acquisition nodes are the entry nodes
		fragment.EntryNodes = resourceFragment.EntryNodes
	}

	logger.Info("Container runtime installation plan created", "nodes", len(fragment.Nodes))
	return fragment, nil
}

var _ task.Task = (*InstallContainerRuntimeTask)(nil)