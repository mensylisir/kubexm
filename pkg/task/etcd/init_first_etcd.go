package etcd

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployFirstEtcdTask struct {
	task.Base
}

func NewDeployFirstEtcdTask() task.Task {
	return &DeployFirstEtcdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployEtcdForFirstEtcd",
				Description: "Download, extract, generate certs, distribute, and install etcd",
			},
		},
	}
}

func (t *DeployFirstEtcdTask) Name() string {
	return t.Meta.Name
}

func (t *DeployFirstEtcdTask) Description() string {
	return t.Meta.Description
}

func (t *DeployFirstEtcdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubexm), nil
}

func (t *DeployFirstEtcdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {

	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)

	extractEtcd := etcd.NewExtractEtcdStepBuilder(*runtimeCtx, "ExtractEtcd").Build()
	distributeCerts := etcd.NewDistributeEtcdCertsStepBuilder(*runtimeCtx, "DistributeEtcdCerts").Build()
	installEtcd := etcd.NewInstallEtcdStepBuilder(*runtimeCtx, "InstallEtcd").Build()
	configureEtcd := etcd.NewConfigureEtcdStepBuilder(*runtimeCtx, "ConfigureEtcd").WithInitialClusterState("new").Build()
	installService := etcd.NewInstallEtcdServiceStepBuilder(*runtimeCtx, "InstallEtcdService").Build()
	startEtcd := etcd.NewStartEtcdStepBuilder(*runtimeCtx, "StartEtcd").Build()
	checkHealth := etcd.NewCheckEtcdHealthStepBuilder(*runtimeCtx, "CheckEtcdHealth").Build()

	extractNode := &plan.ExecutionNode{Name: "ExtractEtcd", Step: extractEtcd, Hosts: []connector.Host{controlNode}}
	distributeCertsNode := &plan.ExecutionNode{Name: "DistributeEtcdCerts", Step: distributeCerts, Hosts: etcdHosts}
	installEtcdNode := &plan.ExecutionNode{Name: "InstallEtcd", Step: installEtcd, Hosts: etcdHosts}
	configureEtcdNode := &plan.ExecutionNode{Name: "ConfigureEtcd", Step: configureEtcd, Hosts: etcdHosts}
	installServiceNode := &plan.ExecutionNode{Name: "InstallEtcdService", Step: installService, Hosts: etcdHosts}
	startEtcdNode := &plan.ExecutionNode{Name: "StartEtcd", Step: startEtcd, Hosts: etcdHosts}
	checkHealthNode := &plan.ExecutionNode{Name: "CheckEtcdHealth", Step: checkHealth, Hosts: []connector.Host{etcdHosts[0]}}

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

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Adding download steps for Etcd.")
		downloadEtcd := etcd.NewDownloadEtcdStepBuilder(*runtimeCtx, "DownloadEtcd").Build()
		downloadNode := &plan.ExecutionNode{Name: "DownloadEtcd", Step: downloadEtcd, Hosts: []connector.Host{controlNode}}
		fragment.AddNode(downloadNode)
		fragment.AddDependency("DownloadEtcd", "ExtractEtcd")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download steps for Etcd.")
	}

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
