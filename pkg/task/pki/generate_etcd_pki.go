package pki

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateEtcdPkiTask struct {
	task.Base
}

func NewGenerateEtcdPkiTask(ctx *task.TaskContext) (task.Interface, error) {
	s := &GenerateEtcdPkiTask{
		Base: task.Base{
			Name:   "GenerateEtcdPki",
			Desc:   "Generate all necessary etcd PKI (CA, member, client certificates)",
			Hosts:  ctx.GetHostsByRole(common.RoleControlPlane), // This task runs on the control-plane node
			Action: new(GenerateEtcdPkiAction),
		},
	}
	return s, nil
}

type GenerateEtcdPkiAction struct {
	task.Action
}

func (a *GenerateEtcdPkiAction) Execute(ctx runtime.Context) (*plan.ExecutionGraph, error) {
	p := plan.NewExecutionGraph("Generate Etcd PKI Phase")

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)

	// 1. Generate Etcd CA
	genCaNode := plan.NodeID("generate-etcd-ca")
	p.AddNode(genCaNode, &plan.ExecutionNode{Step: etcd.NewGenerateCAStep(ctx, genCaNode.String()), Hosts: []connector.Host{controlPlaneHost}})

	// 2. Generate Server/Peer certs for each etcd node
	var certNodes []plan.NodeID
	for _, host := range etcdHosts {
		hostName := host.GetName()

		// Generate server cert
		genServerCertNode := plan.NodeID(fmt.Sprintf("generate-etcd-server-cert-%s", hostName))
		p.AddNode(genServerCertNode, &plan.ExecutionNode{
			Step:         etcd.NewGenerateCertStep(ctx, genServerCertNode.String(), "server"), // type: server
			Hosts:        []connector.Host{controlPlaneHost},
			Dependencies: []plan.NodeID{genCaNode},
		})
		certNodes = append(certNodes, genServerCertNode)

		// Generate peer cert
		genPeerCertNode := plan.NodeID(fmt.Sprintf("generate-etcd-peer-cert-%s", hostName))
		p.AddNode(genPeerCertNode, &plan.ExecutionNode{
			Step:         etcd.NewGenerateCertStep(ctx, genPeerCertNode.String(), "peer"), // type: peer
			Hosts:        []connector.Host{controlPlaneHost},
			Dependencies: []plan.NodeID{genCaNode},
		})
		certNodes = append(certNodes, genPeerCertNode)
	}

	// 3. Generate apiserver-etcd-client certificate
	genApiClientCertNode := plan.NodeID("generate-apiserver-etcd-client-cert")
	p.AddNode(genApiClientCertNode, &plan.ExecutionNode{
		Step:         etcd.NewGenerateCertStep(ctx, genApiClientCertNode.String(), "client"), // type: client
		Hosts:        []connector.Host{controlPlaneHost},
		Dependencies: []plan.NodeID{genCaNode},
	})

	// 4. Distribute all certificates
	distributeCertsNode := plan.NodeID("distribute-etcd-certs")
	allCertsReadyDeps := append(certNodes, genApiClientCertNode)
	p.AddNode(distributeCertsNode, &plan.ExecutionNode{
		Step:         etcd.NewDistributeEtcdCertsStep(ctx, distributeCertsNode.String()),
		Hosts:        etcdHosts,
		Dependencies: allCertsReadyDeps,
	})

	return p, nil
}
