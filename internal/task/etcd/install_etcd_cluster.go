package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/step"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/etcd"
	etcdcertsstep "github.com/mensylisir/kubexm/internal/step/etcd"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployEtcdClusterTask struct {
	task.Base
}

func NewDeployEtcdClusterTask() task.Task {
	return &DeployEtcdClusterTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployEtcdCluster",
				Description: "Generates PKI and deploys a full Etcd cluster in a rolling fashion",
			},
		},
	}
}

func (t *DeployEtcdClusterTask) Name() string {
	return t.Meta.Name
}

func (t *DeployEtcdClusterTask) Description() string {
	return t.Meta.Description
}

func (t *DeployEtcdClusterTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	etcdSpec := ctx.GetClusterConfig().Spec.Etcd
	if etcdSpec == nil {
		return false, nil
	}
	
	// Only deploy etcd when type is kubexm (binary deployment)
	// For kubeadm type, etcd is managed by kubeadm
	// For external/exists type, skip deployment as etcd already exists
	etcdType := etcdSpec.Type
	return etcdType == string(common.EtcdDeploymentTypeKubexm), nil
}

func (t *DeployEtcdClusterTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []remotefw.Host{controlNode}

	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdHosts) == 0 {
		return nil, fmt.Errorf("no etcd hosts found for task %s", t.Name())
	}

	generateEtcdCA, err := etcdcertsstep.NewGenerateEtcdCAStepBuilder(runtimeCtx, "GenerateEtcdCA").Build()
	if err != nil {
		return nil, err
	}
	generateEtcdCerts, err := etcdcertsstep.NewGenerateEtcdCertsStepBuilder(runtimeCtx, "GenerateEtcdCerts").Build()
	if err != nil {
		return nil, err
	}

	generateCANode := &plan.ExecutionNode{Name: "GenerateEtcdCA", Step: generateEtcdCA, Hosts: controlNodeList}
	generateCertsNode := &plan.ExecutionNode{Name: "GenerateEtcdCerts", Step: generateEtcdCerts, Hosts: controlNodeList}

	generateCAID, _ := fragment.AddNode(generateCANode)
	generateCertsID, _ := fragment.AddNode(generateCertsNode)
	fragment.AddDependency(generateCAID, generateCertsID)

	extractEtcdStep, err := etcd.NewExtractEtcdStepBuilder(runtimeCtx, "ExtractEtcd").Build()
	if err != nil {
		return nil, err
	}
	extractEtcdNode := &plan.ExecutionNode{Name: "ExtractEtcd", Step: extractEtcdStep, Hosts: controlNodeList}
	extractEtcdID, _ := fragment.AddNode(extractEtcdNode)

	localPrepExitPoints := []plan.NodeID{generateCertsID, extractEtcdID}

	var lastNodeExitPoint plan.NodeID = ""

	for i, host := range etcdHosts {
		hostName := host.GetName()
		hostList := []remotefw.Host{host}

		distributeCertsStep, err := etcd.NewDistributeEtcdCertsStepBuilder(runtimeCtx, fmt.Sprintf("DistributeEtcdCertsTo%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		installEtcdStep, err := etcd.NewInstallEtcdStepBuilder(runtimeCtx, fmt.Sprintf("InstallEtcdOn%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		var configureEtcdStep step.Step
		if i == 0 {
			configureEtcdStep, err = etcd.NewConfigureEtcdStepBuilder(runtimeCtx, fmt.Sprintf("ConfigureEtcdNewOn%s", hostName)).WithInitialClusterState("new").Build()
			if err != nil {
				return nil, err
			}
		} else {
			configureEtcdStep, err = etcd.NewConfigureEtcdStepBuilder(runtimeCtx, fmt.Sprintf("ConfigureEtcdJoinOn%s", hostName)).WithInitialClusterState("existing").Build()
			if err != nil {
				return nil, err
			}
		}

		installServiceStep, err := etcd.NewInstallEtcdServiceStepBuilder(runtimeCtx, fmt.Sprintf("InstallEtcdServiceOn%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		startEtcdStep, err := etcd.NewStartEtcdStepBuilder(runtimeCtx, fmt.Sprintf("StartEtcdOn%s", hostName)).Build()
		if err != nil {
			return nil, err
		}
		checkHealthStep, err := etcd.NewCheckEtcdHealthStepBuilder(runtimeCtx, fmt.Sprintf("CheckEtcdHealthOn%s", hostName)).Build()
		if err != nil {
			return nil, err
		}

		distributeCertsNode := &plan.ExecutionNode{Name: fmt.Sprintf("DistributeEtcdCerts_%s", hostName), Step: distributeCertsStep, Hosts: hostList}
		installEtcdNode := &plan.ExecutionNode{Name: fmt.Sprintf("InstallEtcd_%s", hostName), Step: installEtcdStep, Hosts: hostList}
		configureEtcdNode := &plan.ExecutionNode{Name: fmt.Sprintf("ConfigureEtcd_%s", hostName), Step: configureEtcdStep, Hosts: hostList}
		installServiceNode := &plan.ExecutionNode{Name: fmt.Sprintf("InstallEtcdService_%s", hostName), Step: installServiceStep, Hosts: hostList}
		startEtcdNode := &plan.ExecutionNode{Name: fmt.Sprintf("StartEtcd_%s", hostName), Step: startEtcdStep, Hosts: hostList}
		checkHealthNode := &plan.ExecutionNode{Name: fmt.Sprintf("CheckEtcdHealth_%s", hostName), Step: checkHealthStep, Hosts: hostList}

		distributeCertsID, _ := fragment.AddNode(distributeCertsNode)
		installEtcdID, _ := fragment.AddNode(installEtcdNode)
		configureEtcdID, _ := fragment.AddNode(configureEtcdNode)
		installServiceID, _ := fragment.AddNode(installServiceNode)
		startEtcdID, _ := fragment.AddNode(startEtcdNode)
		checkHealthID, _ := fragment.AddNode(checkHealthNode)

		if i == 0 {
			for _, prepExitID := range localPrepExitPoints {
				fragment.AddDependency(prepExitID, distributeCertsID)
				fragment.AddDependency(prepExitID, installEtcdID)
			}
		}

		fragment.AddDependency(installEtcdID, configureEtcdID)
		fragment.AddDependency(distributeCertsID, configureEtcdID)
		fragment.AddDependency(configureEtcdID, installServiceID)
		fragment.AddDependency(installServiceID, startEtcdID)
		fragment.AddDependency(startEtcdID, checkHealthID)

		if lastNodeExitPoint != "" {
			fragment.AddDependency(lastNodeExitPoint, distributeCertsID)
			fragment.AddDependency(lastNodeExitPoint, installEtcdID)
		}

		lastNodeExitPoint = checkHealthID
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}

var _ task.Task = (*DeployEtcdClusterTask)(nil)
