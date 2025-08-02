package kubernetes

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
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

	// 1. Generate the main Certificate Authority (CA) on the control node.
	genCA := certs.NewGenerateCAStep(ctx, "GenerateClusterCA")
	p.AddNode("gen-ca", &plan.ExecutionNode{Step: genCA, Hosts: []connector.Host{controlPlaneHost}})

	// 2. Distribute the public CA certificate to all nodes.
	distributeCA := certs.NewDistributionCAStep(ctx, "DistributeClusterCA")
	p.AddNode("distribute-ca", &plan.ExecutionNode{Step: distributeCA, Hosts: allHosts, Dependencies: []plan.NodeID{"gen-ca"}})

	// 3. Generate all other certificates on the control node, signed by the CA.
	// In a real implementation, you might generate certs for each component.
	// This is a simplified representation.
	genCerts := certs.NewGenerateCertsStep(ctx, "GenerateAllCerts")
	p.AddNode("gen-certs", &plan.ExecutionNode{Step: genCerts, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"gen-ca"}})

	// 4. Distribute the generated component certificates to the relevant nodes.
	distributeCerts := certs.NewDistributionCertsStep(ctx, "DistributeAllCerts")
	p.AddNode("distribute-certs", &plan.ExecutionNode{Step: distributeCerts, Hosts: allHosts, Dependencies: []plan.NodeID{"gen-certs"}})

	// 5. Generate Kubeconfig files for components that need them (controller-manager, scheduler, admin).
	genKubeconfigs := certs.NewGenerateKubeconfigStep(ctx, "GenerateAllKubeconfigs")
	p.AddNode("gen-kubeconfigs", &plan.ExecutionNode{Step: genKubeconfigs, Hosts: []connector.Host{controlPlaneHost}, Dependencies: []plan.NodeID{"gen-certs"}})

	// 6. Distribute the kubeconfig files.
	distributeKubeconfigs := certs.NewDistributionKubeconfigStep(ctx, "DistributeAllKubeconfigs")
	p.AddNode("distribute-kubeconfigs", &plan.ExecutionNode{Step: distributeKubeconfigs, Hosts: allHosts, Dependencies: []plan.NodeID{"gen-kubeconfigs"}})

	return p, nil
}
