package etcd

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type CleanEtcdTask struct {
	task.Base
}

func NewCleanEtcdTask() task.Task {
	return &CleanEtcdTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "CleanEtcd",
				Description: "Stop, disable, and remove etcd and its related components",
			},
		},
	}
}

func (t *CleanEtcdTask) Name() string {
	return t.Meta.Name
}

func (t *CleanEtcdTask) Description() string {
	return t.Meta.Description
}

func (t *CleanEtcdTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubexm), nil
}

func (t *CleanEtcdTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {

	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	if len(etcdHosts) == 0 {
		return fragment, nil
	}

	stopEtcd := etcd.NewStopEtcdStepBuilder(*runtimeCtx, "StopEtcd").Build()
	disableEtcd := etcd.NewDisableEtcdStepBuilder(*runtimeCtx, "DisableEtcd").Build()
	removeEtcd := etcd.NewRemoveEtcdMemberStepBuilder(*runtimeCtx, "RemoveEtcd").Build()
	cleanEtcdFiles := etcd.NewCleanupEtcdStepBuilder(*runtimeCtx, "CleanEtcdFiles").Build()
	stopNode := &plan.ExecutionNode{Name: "StopEtcd", Step: stopEtcd, Hosts: etcdHosts}
	disableNode := &plan.ExecutionNode{Name: "DisableEtcd", Step: disableEtcd, Hosts: etcdHosts}
	removeNode := &plan.ExecutionNode{Name: "RemoveEtcd", Step: removeEtcd, Hosts: etcdHosts}
	cleanFilesNode := &plan.ExecutionNode{Name: "CleanEtcdFiles", Step: cleanEtcdFiles, Hosts: etcdHosts}

	fragment.AddNode(stopNode)
	fragment.AddNode(disableNode)
	fragment.AddNode(removeNode)
	fragment.AddNode(cleanFilesNode)

	fragment.AddDependency("StopEtcd", "DisableEtcd")
	fragment.AddDependency("DisableEtcd", "RemoveEtcd")
	fragment.AddDependency("RemoveEtcd", "CleanEtcdFiles")

	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}
