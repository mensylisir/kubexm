package registry

import (
	"fmt"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/task"
	registry "github.com/mensylisir/kubexm/internal/step/registry"
)

// ===================================================================
// Registry Tasks - wraps existing registry step implementations
// ===================================================================

// InstallRegistryTask installs the registry binary on registry-role hosts.
type InstallRegistryTask struct {
	task.Base
}

func NewInstallRegistryTask() task.Task {
	return &InstallRegistryTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "InstallRegistry",
				Description: "Install registry binary on registry-role hosts",
			},
		},
	}
}

func (t *InstallRegistryTask) Name() string        { return t.Meta.Name }
func (t *InstallRegistryTask) Description() string { return t.Meta.Description }

func (t *InstallRegistryTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		// Also check if registry is enabled in config
		cfg := ctx.GetClusterConfig()
		if cfg != nil && cfg.Spec.Registry != nil && cfg.Spec.Registry.LocalDeployment != nil {
			return true, nil
		}
	}
	return len(hosts) > 0, nil
}

func (t *InstallRegistryTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		installStep, err := registry.NewInstallRegistryStepBuilder(
			hostCtx, fmt.Sprintf("InstallRegistry-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry install step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("InstallRegistry-%s", host.GetName()),
			Step:  installStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// GenerateRegistryConfigTask generates the registry config.yml locally.
type GenerateRegistryConfigTask struct {
	task.Base
}

func NewGenerateRegistryConfigTask() task.Task {
	return &GenerateRegistryConfigTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateRegistryConfig",
				Description: "Generate registry config.yml locally",
			},
		},
	}
}

func (t *GenerateRegistryConfigTask) Name() string        { return t.Meta.Name }
func (t *GenerateRegistryConfigTask) Description() string { return t.Meta.Description }

func (t *GenerateRegistryConfigTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *GenerateRegistryConfigTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	// Config generation is a single-node step (local, not host-specific)
	host := hosts[0]
	hostCtx := runtime.ForHost(execCtx, host)

	configStep, err := registry.NewGenerateRegistryConfigStepBuilder(
		hostCtx, "GenerateRegistryConfig").Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create registry config generate step: %w", err)
	}
	nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  "GenerateRegistryConfig",
		Step:  configStep,
		Hosts: []remotefw.Host{host},
	})
	fragment.EntryNodes = []plan.NodeID{nodeID}
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// DistributeRegistryConfigTask distributes the registry config to registry-role hosts.
type DistributeRegistryConfigTask struct {
	task.Base
}

func NewDistributeRegistryConfigTask() task.Task {
	return &DistributeRegistryConfigTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DistributeRegistryConfig",
				Description: "Distribute registry config.yml to registry-role hosts",
			},
		},
	}
}

func (t *DistributeRegistryConfigTask) Name() string        { return t.Meta.Name }
func (t *DistributeRegistryConfigTask) Description() string { return t.Meta.Description }

func (t *DistributeRegistryConfigTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *DistributeRegistryConfigTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		distStep, err := registry.NewDistributeRegistryConfigStepBuilder(
			hostCtx, fmt.Sprintf("DistributeRegistryConfig-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry config distribute step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("DistributeRegistryConfig-%s", host.GetName()),
			Step:  distStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// SetupRegistryServiceTask sets up the registry systemd service on registry-role hosts.
type SetupRegistryServiceTask struct {
	task.Base
}

func NewSetupRegistryServiceTask() task.Task {
	return &SetupRegistryServiceTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "SetupRegistryService",
				Description: "Setup registry systemd service on registry-role hosts",
			},
		},
	}
}

func (t *SetupRegistryServiceTask) Name() string        { return t.Meta.Name }
func (t *SetupRegistryServiceTask) Description() string { return t.Meta.Description }

func (t *SetupRegistryServiceTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *SetupRegistryServiceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		setupStep, err := registry.NewSetupRegistryServiceStepBuilder(
			hostCtx, fmt.Sprintf("SetupRegistryService-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry service setup step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("SetupRegistryService-%s", host.GetName()),
			Step:  setupStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// StartRegistryServiceTask enables and starts the registry service on registry-role hosts.
type StartRegistryServiceTask struct {
	task.Base
}

func NewStartRegistryServiceTask() task.Task {
	return &StartRegistryServiceTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "StartRegistryService",
				Description: "Enable and start registry service on registry-role hosts",
			},
		},
	}
}

func (t *StartRegistryServiceTask) Name() string        { return t.Meta.Name }
func (t *StartRegistryServiceTask) Description() string { return t.Meta.Description }

func (t *StartRegistryServiceTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *StartRegistryServiceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		enableStep, err := registry.NewEnableRegistryServiceStepBuilder(
			hostCtx, fmt.Sprintf("EnableRegistryService-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry enable step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("EnableRegistryService-%s", host.GetName()),
			Step:  enableStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// StopRegistryServiceTask stops the registry service on registry-role hosts.
type StopRegistryServiceTask struct {
	task.Base
}

func NewStopRegistryServiceTask() task.Task {
	return &StopRegistryServiceTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "StopRegistryService",
				Description: "Stop registry service on registry-role hosts",
			},
		},
	}
}

func (t *StopRegistryServiceTask) Name() string        { return t.Meta.Name }
func (t *StopRegistryServiceTask) Description() string { return t.Meta.Description }

func (t *StopRegistryServiceTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *StopRegistryServiceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		stopStep, err := registry.NewStopRegistryServiceStepBuilder(
			hostCtx, fmt.Sprintf("StopRegistryService-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry stop step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("StopRegistryService-%s", host.GetName()),
			Step:  stopStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// DisableRegistryServiceTask disables the registry service on registry-role hosts.
type DisableRegistryServiceTask struct {
	task.Base
}

func NewDisableRegistryServiceTask() task.Task {
	return &DisableRegistryServiceTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DisableRegistryService",
				Description: "Disable registry service on registry-role hosts",
			},
		},
	}
}

func (t *DisableRegistryServiceTask) Name() string        { return t.Meta.Name }
func (t *DisableRegistryServiceTask) Description() string { return t.Meta.Description }

func (t *DisableRegistryServiceTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *DisableRegistryServiceTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		disableStep, err := registry.NewDisableRegistryServiceStepBuilder(
			hostCtx, fmt.Sprintf("DisableRegistryService-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry disable step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("DisableRegistryService-%s", host.GetName()),
			Step:  disableStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// RemoveRegistryArtifactsTask removes registry binary, config and service files.
type RemoveRegistryArtifactsTask struct {
	task.Base
}

func NewRemoveRegistryArtifactsTask() task.Task {
	return &RemoveRegistryArtifactsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RemoveRegistryArtifacts",
				Description: "Remove registry binary, config and service files from registry-role hosts",
			},
		},
	}
}

func (t *RemoveRegistryArtifactsTask) Name() string        { return t.Meta.Name }
func (t *RemoveRegistryArtifactsTask) Description() string { return t.Meta.Description }

func (t *RemoveRegistryArtifactsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *RemoveRegistryArtifactsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		removeStep, err := registry.NewRemoveRegistryArtifactsStepBuilder(
			hostCtx, fmt.Sprintf("RemoveRegistryArtifacts-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry remove step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RemoveRegistryArtifacts-%s", host.GetName()),
			Step:  removeStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

// RemoveRegistryDataTask removes registry data directory.
type RemoveRegistryDataTask struct {
	task.Base
}

func NewRemoveRegistryDataTask() task.Task {
	return &RemoveRegistryDataTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "RemoveRegistryData",
				Description: "Remove registry data directory from registry-role hosts",
			},
		},
	}
}

func (t *RemoveRegistryDataTask) Name() string        { return t.Meta.Name }
func (t *RemoveRegistryDataTask) Description() string { return t.Meta.Description }

func (t *RemoveRegistryDataTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	return len(hosts) > 0, nil
}

func (t *RemoveRegistryDataTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	execCtx := ctx.ForTask(t.Name())
	hosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(hosts) == 0 {
		return fragment, nil
	}

	var entryNodes []plan.NodeID
	for _, host := range hosts {
		hostCtx := runtime.ForHost(execCtx, host)

		removeDataStep, err := registry.NewRemoveRegistryDataStepBuilder(
			hostCtx, fmt.Sprintf("RemoveRegistryData-%s", host.GetName())).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create registry data remove step for %s: %w", host.GetName(), err)
		}
		nodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:  fmt.Sprintf("RemoveRegistryData-%s", host.GetName()),
			Step:  removeDataStep,
			Hosts: []remotefw.Host{host},
		})
		entryNodes = append(entryNodes, nodeID)
	}

	fragment.EntryNodes = plan.UniqueNodeIDs(entryNodes)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
