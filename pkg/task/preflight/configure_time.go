package preflight

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/os"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/chrony"
	"github.com/mensylisir/kubexm/pkg/task"
)

type ConfigureTimeTask struct {
	task.Base
}

func NewConfigureTimeTask() task.Task {
	return &ConfigureTimeTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ConfigureNTP",
				Description: "Configures chrony on all nodes based on the NTPServers spec and verifies time synchronization",
			},
		},
	}
}

func (t *ConfigureTimeTask) Name() string {
	return t.Meta.Name
}

func (t *ConfigureTimeTask) Description() string {
	return t.Meta.Description
}

func (t *ConfigureTimeTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	clusterSpec := ctx.GetClusterConfig().Spec
	if clusterSpec.System != nil && len(clusterSpec.System.NTPServers) > 0 {
		return true, nil
	}
	ctx.GetLogger().Info("Skipping NTP configuration: No NTPServers defined in the cluster configuration.")
	return false, nil
}

func (t *ConfigureTimeTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	allHosts := ctx.GetHostsByRole("")
	ntpServers := ctx.GetClusterConfig().Spec.System.NTPServers

	ntpServerHostNames := make(map[string]bool)
	for _, server := range ntpServers {
		ntpServerHostNames[server] = true
	}

	var lastNodeExitPoint plan.NodeID = ""

	for _, host := range allHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		configureTimezoneStep := os.NewConfigureTimezoneStepBuilder(*runtimeCtx, fmt.Sprintf("ConfigureTimezoneFor%s", hostName)).Build()

		var configureStep step.Step
		if ntpServerHostNames[hostName] {
			configureStep = chrony.NewConfigureChronyAsServerStepBuilder(*runtimeCtx, fmt.Sprintf("ConfigureChronyServerOn%s", hostName)).Build()
		} else {
			configureStep = chrony.NewConfigureChronyAsClientStepBuilder(*runtimeCtx, fmt.Sprintf("ConfigureChronyClientOn%s", hostName)).Build()
		}

		enableStep := chrony.NewEnableChronyStepBuilder(*runtimeCtx, fmt.Sprintf("EnableChronyOn%s", hostName)).Build()
		restartStep := chrony.NewRestartChronyStepBuilder(*runtimeCtx, fmt.Sprintf("RestartChronyOn%s", hostName)).Build()
		verifyStep := chrony.NewVerifyTimeSyncStepBuilder(*runtimeCtx, fmt.Sprintf("VerifyTimeSyncOn%s", hostName)).Build()

		configureTimezoneNode := &plan.ExecutionNode{Name: fmt.Sprintf("ConfigureTimezoneFor%s", hostName), Step: configureTimezoneStep, Hosts: hostList}
		configureNode := &plan.ExecutionNode{Name: fmt.Sprintf("ConfigureChronyFor%s", hostName), Step: configureStep, Hosts: hostList}
		enableNode := &plan.ExecutionNode{Name: fmt.Sprintf("EnableChronyFor%s", hostName), Step: enableStep, Hosts: hostList}
		restartNode := &plan.ExecutionNode{Name: fmt.Sprintf("RestartChronyFor%s", hostName), Step: restartStep, Hosts: hostList}
		verifyNode := &plan.ExecutionNode{Name: fmt.Sprintf("VerifyTimeSyncFor%s", hostName), Step: verifyStep, Hosts: hostList}

		configureTimezoneID, _ := fragment.AddNode(configureTimezoneNode)
		configureID, _ := fragment.AddNode(configureNode)
		enableID, _ := fragment.AddNode(enableNode)
		restartID, _ := fragment.AddNode(restartNode)
		verifyID, _ := fragment.AddNode(verifyNode)

		fragment.AddDependency(configureTimezoneID, configureID)
		fragment.AddDependency(configureID, enableID)
		fragment.AddDependency(enableID, restartID)
		fragment.AddDependency(restartID, verifyID)
		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, configureID)
		}
		lastNodeExitPoint = verifyID
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*ConfigureTimeTask)(nil)
