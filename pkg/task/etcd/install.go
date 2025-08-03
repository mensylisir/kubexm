package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type InstallEtcdTask struct {
	task.Base
}

func NewInstallEtcdTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &InstallEtcdTask{
		Base: task.Base{
			Name:   "InstallEtcd",
			Desc:   "Install and configure etcd on the etcd nodes",
			Hosts:  ctx.GetHostsByRole(common.RoleEtcd),
			Action: new(InstallEtcdAction),
		},
	}
	return s, nil
}

type InstallEtcdAction struct {
	task.Action
}

func (a *InstallEtcdAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Install Etcd Phase")

	etcdHosts := a.GetHosts()
	if len(etcdHosts) == 0 {
		return p, nil
	}

	// This task assumes that etcd binaries have been downloaded and PKI has been distributed by previous tasks.
	for _, host := range etcdHosts {
		hostName := host.GetName()

		installNode := plan.NodeID(fmt.Sprintf("install-etcd-%s", hostName))
		p.AddNode(installNode, &plan.ExecutionNode{Step: etcd.NewInstallEtcdStep(ctx, installNode.String()), Hosts: []connector.Host{host}})

		setupDirNode := plan.NodeID(fmt.Sprintf("setup-etcd-dir-%s", hostName))
		p.AddNode(setupDirNode, &plan.ExecutionNode{Step: etcd.NewSetupEtcdDirStep(ctx, setupDirNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installNode}})

		configureNode := plan.NodeID(fmt.Sprintf("configure-etcd-%s", hostName))
		p.AddNode(configureNode, &plan.ExecutionNode{Step: etcd.NewConfigureEtcdStep(ctx, configureNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{setupDirNode}})

		installSvcNode := plan.NodeID(fmt.Sprintf("install-etcd-svc-%s", hostName))
		p.AddNode(installSvcNode, &plan.ExecutionNode{Step: etcd.NewInstallEtcdServiceStep(ctx, installSvcNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{configureNode}})

		enableNode := plan.NodeID(fmt.Sprintf("enable-etcd-%s", hostName))
		p.AddNode(enableNode, &plan.ExecutionNode{Step: etcd.NewEnableEtcdStep(ctx, enableNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{installSvcNode}})

		startNode := plan.NodeID(fmt.Sprintf("start-etcd-%s", hostName))
		p.AddNode(startNode, &plan.ExecutionNode{Step: etcd.NewStartEtcdStep(ctx, startNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{enableNode}})

		checkHealthNode := plan.NodeID(fmt.Sprintf("check-etcd-health-%s", hostName))
		p.AddNode(checkHealthNode, &plan.ExecutionNode{Step: etcd.NewCheckHealthStep(ctx, checkHealthNode.String()), Hosts: []connector.Host{host}, Dependencies: []plan.NodeID{startNode}})
	}

	return p, nil
}
