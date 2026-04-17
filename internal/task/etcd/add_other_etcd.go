package etcd

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/etcd"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployEtcdTask struct {
	task.Base
}

func NewDeployEtcdTask() task.Task {
	return &DeployEtcdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployEtcd",
				Description: "Extract, generate certs, distribute, and install etcd (assets prepared in Preflight)",
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
	etcdSpec := ctx.GetClusterConfig().Spec.Etcd
	if etcdSpec == nil {
		return false, nil
	}
	return etcdSpec.Type == string(common.EtcdDeploymentTypeKubexm), nil
}

func (t *DeployEtcdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {

	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)

	extractEtcd, err := etcd.NewExtractEtcdStepBuilder(runtimeCtx, "ExtractEtcd").Build()
	if err != nil {
		return nil, err
	}
	distributeCerts, err := etcd.NewDistributeEtcdCertsStepBuilder(runtimeCtx, "DistributeEtcdCerts").Build()
	if err != nil {
		return nil, err
	}
	installEtcd, err := etcd.NewInstallEtcdStepBuilder(runtimeCtx, "InstallEtcd").Build()
	if err != nil {
		return nil, err
	}
	configureEtcd, err := etcd.NewConfigureEtcdStepBuilder(runtimeCtx, "ConfigureEtcd").WithInitialClusterState("existing").Build()
	if err != nil {
		return nil, err
	}
	installService, err := etcd.NewInstallEtcdServiceStepBuilder(runtimeCtx, "InstallEtcdService").Build()
	if err != nil {
		return nil, err
	}
	startEtcd, err := etcd.NewStartEtcdStepBuilder(runtimeCtx, "StartEtcd").Build()
	if err != nil {
		return nil, err
	}
	checkHealth, err := etcd.NewCheckEtcdHealthStepBuilder(runtimeCtx, "CheckEtcdHealth").Build()
	if err != nil {
		return nil, err
	}

	extractNode := &plan.ExecutionNode{Name: "ExtractEtcd", Step: extractEtcd, Hosts: []remotefw.Host{controlNode}}
	distributeCertsNode := &plan.ExecutionNode{Name: "DistributeEtcdCerts", Step: distributeCerts, Hosts: etcdHosts}
	installEtcdNode := &plan.ExecutionNode{Name: "InstallEtcd", Step: installEtcd, Hosts: etcdHosts}
	configureEtcdNode := &plan.ExecutionNode{Name: "ConfigureEtcd", Step: configureEtcd, Hosts: etcdHosts}
	installServiceNode := &plan.ExecutionNode{Name: "InstallEtcdService", Step: installService, Hosts: etcdHosts}
	startEtcdNode := &plan.ExecutionNode{Name: "StartEtcd", Step: startEtcd, Hosts: etcdHosts}
	checkHealthNode := &plan.ExecutionNode{Name: "CheckEtcdHealth", Step: checkHealth, Hosts: []remotefw.Host{etcdHosts[0]}}

	fragment.AddNode(extractNode)
	fragment.AddNode(distributeCertsNode)
	fragment.AddNode(installEtcdNode)
	fragment.AddNode(configureEtcdNode)
	fragment.AddNode(installServiceNode)
	fragment.AddNode(startEtcdNode)
	fragment.AddNode(checkHealthNode)

	fragment.AddDependency("ExtractEtcd", "InstallEtcd")
	fragment.AddDependency("InstallEtcd", "ConfigureEtcd")
	fragment.AddDependency("DistributeEtcdCerts", "ConfigureEtcd")
	fragment.AddDependency("ConfigureEtcd", "InstallEtcdService")
	fragment.AddDependency("InstallEtcdService", "StartEtcd")
	fragment.AddDependency("StartEtcd", "CheckEtcdHealth")

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
