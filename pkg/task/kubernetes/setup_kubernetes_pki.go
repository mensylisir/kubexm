package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/certs"
	"github.com/mensylisir/kubexm/pkg/task"
)

type SetupKubernetesPKITask struct {
	task.Base
}

func NewSetupKubernetesPKITask(ctx *task.TaskContext) (task.Interface, error) {
	s := &SetupKubernetesPKITask{
		Base: task.Base{
			Name:   "SetupKubernetesPKI",
			Desc:   "Generate and distribute all Kubernetes PKI certificates",
			Hosts:  ctx.GetHosts(),
			Action: new(SetupKubernetesPKIAction),
		},
	}
	return s, nil
}

type SetupKubernetesPKIAction struct {
	task.Action
}

func (a *SetupKubernetesPKIAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Setup Kubernetes PKI Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	allHosts := a.GetHosts()

	genCaNode := plan.NodeID("gen-ca")
	p.AddNode(genCaNode, &plan.ExecutionNode{Step: certs.NewGenerateCAStep(ctx, genCaNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	distributeCaNode := plan.NodeID("distribute-ca")
	p.AddNode(distributeCaNode, &plan.ExecutionNode{Step: certs.NewDistributionCAStep(ctx, distributeCaNode.String()), Hosts: allHosts, Dependencies: []plan.NodeID{genCaNode}})

	genCertsNode := plan.NodeID("gen-certs")
	p.AddNode(genCertsNode, &plan.ExecutionNode{Step: certs.NewGenerateCertsStep(ctx, genCertsNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{genCaNode}})

	distributeCertsNode := plan.NodeID("distribute-certs")
	p.AddNode(distributeCertsNode, &plan.ExecutionNode{Step: certs.NewDistributionCertsStep(ctx, distributeCertsNode.String()), Hosts: allHosts, Dependencies: []plan.NodeID{genCertsNode}})

	genKubeconfigsNode := plan.NodeID("gen-kubeconfigs")
	p.AddNode(genKubeconfigsNode, &plan.ExecutionNode{Step: certs.NewGenerateKubeconfigStep(ctx, genKubeconfigsNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{genCertsNode}})

	distributeKubeconfigsNode := plan.NodeID("distribute-kubeconfigs")
	p.AddNode(distributeKubeconfigsNode, &plan.ExecutionNode{Step: certs.NewDistributionKubeconfigStep(ctx, distributeKubeconfigsNode.String()), Hosts: allHosts, Dependencies: []plan.NodeID{genKubeconfigsNode}})

	return p, nil
}
