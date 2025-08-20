package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/etcd"
	etcdcertsstep "github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
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
	return ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubexm), nil
}

func (t *DeployEtcdClusterTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	controlNodeList := []connector.Host{controlNode}

	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdHosts) == 0 {
		return nil, fmt.Errorf("no etcd hosts found for task %s", t.Name())
	}

	generateEtcdCA := etcdcertsstep.NewGenerateEtcdCAStepBuilder(*runtimeCtx, "GenerateEtcdCA").Build()
	generateEtcdCerts := etcdcertsstep.NewGenerateEtcdCertsStepBuilder(*runtimeCtx, "GenerateEtcdCerts").Build()

	generateCANode := &plan.ExecutionNode{Name: "GenerateEtcdCA", Step: generateEtcdCA, Hosts: controlNodeList}
	generateCertsNode := &plan.ExecutionNode{Name: "GenerateEtcdCerts", Step: generateEtcdCerts, Hosts: controlNodeList}

	generateCAID, _ := fragment.AddNode(generateCANode)
	generateCertsID, _ := fragment.AddNode(generateCertsNode)
	fragment.AddDependency(generateCAID, generateCertsID)

	extractEtcdStep := etcd.NewExtractEtcdStepBuilder(*runtimeCtx, "ExtractEtcd").Build()
	extractEtcdNode := &plan.ExecutionNode{Name: "ExtractEtcd", Step: extractEtcdStep, Hosts: controlNodeList}
	extractEtcdID, _ := fragment.AddNode(extractEtcdNode)

	localPrepExitPoints := []plan.NodeID{generateCertsID, extractEtcdID}

	if !ctx.IsOfflineMode() {
		downloadEtcdStep := etcd.NewDownloadEtcdStepBuilder(*runtimeCtx, "DownloadEtcd").Build()
		downloadNode := &plan.ExecutionNode{Name: "DownloadEtcd", Step: downloadEtcdStep, Hosts: controlNodeList}
		downloadID, _ := fragment.AddNode(downloadNode)
		fragment.AddDependency(downloadID, extractEtcdID)
		localPrepExitPoints = []plan.NodeID{generateCertsID, downloadID}
	}

	var lastNodeExitPoint plan.NodeID = ""

	for i, host := range etcdHosts {
		hostName := host.GetName()
		hostList := []connector.Host{host}

		distributeCertsStep := etcd.NewDistributeEtcdCertsStepBuilder(*runtimeCtx, fmt.Sprintf("DistributeEtcdCertsTo%s", hostName)).Build()
		installEtcdStep := etcd.NewInstallEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("InstallEtcdOn%s", hostName)).Build()

		var configureEtcdStep step.Step
		if i == 0 {
			configureEtcdStep = etcd.NewConfigureEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("ConfigureEtcdNewOn%s", hostName)).WithInitialClusterState("new").Build()
		} else {
			configureEtcdStep = etcd.NewConfigureEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("ConfigureEtcdJoinOn%s", hostName)).WithInitialClusterState("existing").Build()
		}

		installServiceStep := etcd.NewInstallEtcdServiceStepBuilder(*runtimeCtx, fmt.Sprintf("InstallEtcdServiceOn%s", hostName)).Build()
		startEtcdStep := etcd.NewStartEtcdStepBuilder(*runtimeCtx, fmt.Sprintf("StartEtcdOn%s", hostName)).Build()
		checkHealthStep := etcd.NewCheckEtcdHealthStepBuilder(*runtimeCtx, fmt.Sprintf("CheckEtcdHealthOn%s", hostName)).Build()

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
