package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployEtcdTask struct {
	task.Base
}

func NewDeployEtcdTask() task.Task {
	return &DeployEtcdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployEtcd",
				Description: "Download, extract, generate certs, distribute, and install etcd",
			},
		},
	}
}

func (t *DeployEtcdTask) Name() string {
	return t.Meta.Name
}

func (t *DeployEtcdTask) Description() string {
	return t.Meta.Description
}

func (t *DeployEtcdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubexm), nil
}

func (t *DeployEtcdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {

	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)

	downloadEtcd := etcd.NewDownloadEtcdStepBuilder(*runtimeCtx, "DownloadEtcd").Build()
	extractEtcd := etcd.NewExtractEtcdStepBuilder(*runtimeCtx, "ExtractEtcd").Build()
	genEtcdCA := etcd.NewGenerateEtcdCAStepBuilder(*runtimeCtx, "GenerateEtcdCA").Build()
	genEtcdCerts := etcd.NewGenerateEtcdCertsStepBuilder(*runtimeCtx, "GenerateEtcdCerts").Build()

	distributeCerts := etcd.NewDistributeEtcdCertsStepBuilder(*runtimeCtx, "DistributeEtcdCerts").Build()
	installEtcd := etcd.NewInstallEtcdStepBuilder(*runtimeCtx, "InstallEtcd").Build()
	configureEtcd := etcd.NewConfigureEtcdStepBuilder(*runtimeCtx, "ConfigureEtcd").Build()
	installService := etcd.NewInstallEtcdServiceStepBuilder(*runtimeCtx, "InstallEtcdService").Build()
	startEtcd := etcd.NewStartEtcdStepBuilder(*runtimeCtx, "StartEtcd").Build()
	checkHealth := etcd.NewCheckEtcdHealthStepBuilder(*runtimeCtx, "CheckEtcdHealth").Build()

	downloadNode := &plan.ExecutionNode{Name: "DownloadEtcd", Step: downloadEtcd, Hosts: []connector.Host{controlNode}}
	extractNode := &plan.ExecutionNode{Name: "ExtractEtcd", Step: extractEtcd, Hosts: []connector.Host{controlNode}}
	genCaNode := &plan.ExecutionNode{Name: "GenerateEtcdCA", Step: genEtcdCA, Hosts: []connector.Host{controlNode}}
	genCertsNode := &plan.ExecutionNode{Name: "GenerateEtcdCerts", Step: genEtcdCerts, Hosts: []connector.Host{controlNode}}

	distributeCertsNode := &plan.ExecutionNode{Name: "DistributeEtcdCerts", Step: distributeCerts, Hosts: etcdHosts}
	installEtcdNode := &plan.ExecutionNode{Name: "InstallEtcd", Step: installEtcd, Hosts: etcdHosts}
	configureEtcdNode := &plan.ExecutionNode{Name: "ConfigureEtcd", Step: configureEtcd, Hosts: etcdHosts}
	installServiceNode := &plan.ExecutionNode{Name: "InstallEtcdService", Step: installService, Hosts: etcdHosts}
	startEtcdNode := &plan.ExecutionNode{Name: "StartEtcd", Step: startEtcd, Hosts: etcdHosts}
	checkHealthNode := &plan.ExecutionNode{Name: "CheckEtcdHealth", Step: checkHealth, Hosts: []connector.Host{etcdHosts[0]}} // 健康检查通常在一个节点上执行即可

	fragment.AddNode(downloadNode)
	fragment.AddNode(extractNode)
	fragment.AddNode(genCaNode)
	fragment.AddNode(genCertsNode)
	fragment.AddNode(distributeCertsNode)
	fragment.AddNode(installEtcdNode)
	fragment.AddNode(configureEtcdNode)
	fragment.AddNode(installServiceNode)
	fragment.AddNode(startEtcdNode)
	fragment.AddNode(checkHealthNode)

	fragment.AddDependency("DownloadEtcd", "ExtractEtcd")
	fragment.AddDependency("GenerateEtcdCA", "GenerateEtcdCerts")

	fragment.AddDependency("ExtractEtcd", "InstallEtcd")
	fragment.AddDependency("GenerateEtcdCerts", "DistributeEtcdCerts")

	fragment.AddDependency("InstallEtcd", "ConfigureEtcd")
	fragment.AddDependency("DistributeEtcdCerts", "ConfigureEtcd")
	fragment.AddDependency("ConfigureEtcd", "InstallEtcdService")
	fragment.AddDependency("InstallEtcdService", "StartEtcd")
	fragment.AddDependency("StartEtcd", "CheckEtcdHealth")

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
