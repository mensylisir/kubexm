package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/module"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/task"
	"github.com/mensylisir/kubexm/pkg/task/kubeadm"
	"github.com/mensylisir/kubexm/pkg/task/kubernetes"
)

type KubernetesModule struct {
	module.Base
}

func NewKubernetesModule(ctx *module.ModuleContext) (module.Interface, error) {
	s := &KubernetesModule{
		Base: module.Base{
			Name: "Kubernetes",
			Desc: "Install and configure Kubernetes cluster",
		},
	}

	k8sType := ctx.GetClusterConfig().Spec.Kubernetes.Type
	var tasks []task.Interface
	var err error

	if k8sType == "kubeadm" {
		var binariesTask, bootstrapTask task.Interface
		binariesTask, err = kubeadm.NewInstallKubeadmBinariesTask(ctx)
		if err == nil {
			bootstrapTask, err = kubeadm.NewBootstrapClusterWithKubeadmTask(ctx)
		}
		tasks = []task.Interface{binariesTask, bootstrapTask}
	} else { // Assume binary
		var binariesTask, pkiTask, bootstrapTask, joinTask task.Interface
		binariesTask, err = kubernetes.NewInstallKubernetesBinariesTask(ctx)
		if err == nil {
			pkiTask, err = kubernetes.NewSetupKubernetesPKITask(ctx)
		}
		if err == nil {
			bootstrapTask, err = kubernetes.NewBootstrapControlPlaneWithBinariesTask(ctx)
		}
		if err == nil {
			joinTask, err = kubernetes.NewJoinNodesWithBinariesTask(ctx)
		}
		tasks = []task.Interface{binariesTask, pkiTask, bootstrapTask, joinTask}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes tasks: %w", err)
	}

	s.SetTasks(tasks)
	return s, nil
}

func (m *KubernetesModule) Execute(ctx *module.ModuleContext) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph(fmt.Sprintf("Module: %s", m.Name))

	var lastTaskExitNodes []plan.NodeID

	for _, t := range m.GetTasks() {
		taskGraph, err := t.Execute(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute task %s in module %s: %w", t.GetName(), m.Name, err)
		}

		if taskGraph.IsEmpty() {
			continue
		}

		p.Merge(taskGraph)

		if len(lastTaskExitNodes) > 0 {
			for _, entryNodeID := range taskGraph.EntryNodes {
				for _, depNodeID := range lastTaskExitNodes {
					p.AddDependency(depNodeID, entryNodeID)
				}
			}
		}

		lastTaskExitNodes = taskGraph.ExitNodes
	}

	p.CalculateEntryAndExitNodes()
	return p, nil
}
