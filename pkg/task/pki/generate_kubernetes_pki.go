package pki

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/kubernetes/certs"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateKubernetesPkiTask struct {
	task.Base
}

func NewGenerateKubernetesPkiTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &GenerateKubernetesPkiTask{
		Base: task.Base{
			Name:   "GenerateKubernetesPki",
			Desc:   "Generate all necessary Kubernetes PKI (CA, component certs, etc.)",
			Hosts:  ctx.GetHostsByRole(common.RoleControlPlane),
			Action: new(GenerateKubernetesPkiAction),
		},
	}
	return s, nil
}

type GenerateKubernetesPkiAction struct {
	task.Action
}

func (a *GenerateKubernetesPkiAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Generate Kubernetes PKI Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	allHosts := a.GetHosts()

	// 1. Generate the main Certificate Authority (CA) on the control node.
	genCaNode := plan.NodeID("generate-k8s-ca")
	p.AddNode(genCaNode, &plan.ExecutionNode{Step: certs.NewGenerateCAStep(ctx, genCaNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	// 2. Distribute the public CA certificate to all nodes.
	distributeCaNode := plan.NodeID("distribute-k8s-ca")
	p.AddNode(distributeCaNode, &plan.ExecutionNode{Step: certs.NewDistributionCAStep(ctx, distributeCaNode.String()), Hosts: allHosts, Dependencies: []plan.NodeID{genCaNode}})

	// 3. Generate all other certificates on the control node, signed by the CA.
	genCertsNode := plan.NodeID("generate-k8s-certs")
	p.AddNode(genCertsNode, &plan.ExecutionNode{Step: certs.NewGenerateCertsStep(ctx, genCertsNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{genCaNode}})

	// 4. Distribute the generated component certificates to the relevant nodes.
	distributeCertsNode := plan.NodeID("distribute-k8s-certs")
	p.AddNode(distributeCertsNode, &plan.ExecutionNode{Step: certs.NewDistributionCertsStep(ctx, distributeCertsNode.String()), Hosts: allHosts, Dependencies: []plan.NodeID{genCertsNode}})

	// 5. Generate Kubeconfig files for components and users.
	genKubeconfigsNode := plan.NodeID("generate-kubeconfigs")
	p.AddNode(genKubeconfigsNode, &plan.ExecutionNode{Step: certs.NewGenerateKubeconfigStep(ctx, genKubeconfigsNode.String()), Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{genCertsNode}})

	// 6. Distribute the kubeconfig files.
	distributeKubeconfigsNode := plan.NodeID("distribute-kubeconfigs")
	p.AddNode(distributeKubeconfigsNode, &plan.ExecutionNode{Step: certs.NewDistributionKubeconfigStep(ctx, distributeKubeconfigsNode.String()), Hosts: allHosts, Dependencies: []plan.NodeID{genKubeconfigsNode}})

	return p, nil
}
