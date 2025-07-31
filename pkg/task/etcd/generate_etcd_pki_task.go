package etcd

import (
	"crypto/x509"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// GenerateEtcdPkiTask orchestrates the generation of etcd PKI.
type GenerateEtcdPkiTask struct {
	task.BaseTask
}

// NewGenerateEtcdPkiTask creates a new task for generating etcd PKI.
func NewGenerateEtcdPkiTask() task.Task {
	return &GenerateEtcdPkiTask{
		BaseTask: task.NewBaseTask(
			"GenerateEtcdPki",
			"Generates all necessary etcd PKI (CA, member, client certificates).",
			[]string{common.ControlNodeRole},
			nil,
			false,
		),
	}
}

func (t *GenerateEtcdPkiTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	controlPlaneHost, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane host for PKI generation: %w", err)
	}

	etcdCertDir := ctx.GetEtcdCertsDir()

	// Step 1: Generate Etcd CA
	caStep := commonstep.NewGenerateCAStepBuilder(ctx, "GenerateEtcdCA").
		WithLocalCertsDir(etcdCertDir).
		WithCertFileName(common.EtcdCaPemFileName).
		WithKeyFileName(common.EtcdCaKeyPemFileName).
		WithCADuration(10 * 365 * 24 * time.Hour).
		Build()

	genCaNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:  caStep.Meta().Name,
		Step:  caStep,
		Hosts: []connector.Host{controlPlaneHost},
	})

	// Step 2: Generate Server/Peer certs for each etcd node
	etcdNodes, err := ctx.GetHostsByRole(common.RoleEtcd)
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd nodes for task %s: %w", t.Name(), err)
	}

	var allCertNodeIDs []plan.NodeID

	for _, etcdHost := range etcdNodes {
		hostName := etcdHost.GetName()
		hostIP := etcdHost.GetAddress()

		// Server Cert
		serverCertStep := commonstep.NewGenerateCertsStepBuilder(ctx, fmt.Sprintf("GenerateEtcdServerCert-%s", hostName)).
			WithLocalCertsDir(etcdCertDir).
			WithCaCertFileName(common.EtcdCaPemFileName).
			WithCaKeyFileName(common.EtcdCaKeyPemFileName).
			WithCert(fmt.Sprintf("%s.pem", hostName)).
			WithCertKey(fmt.Sprintf("%s-key.pem", hostName)).
			WithCommonName(hostName).
			WithOrganization([]string{"kubexm-etcd-server"}).
			WithHosts([]string{hostName, hostIP}).
			WithUsages([]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}).
			Build()

		serverCertNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:         serverCertStep.Meta().Name,
			Step:         serverCertStep,
			Hosts:        []connector.Host{controlPlaneHost},
			Dependencies: []plan.NodeID{genCaNodeID},
		})
		allCertNodeIDs = append(allCertNodeIDs, serverCertNodeID)

		// Peer Cert
		peerCertStep := commonstep.NewGenerateCertsStepBuilder(ctx, fmt.Sprintf("GenerateEtcdPeerCert-%s", hostName)).
			WithLocalCertsDir(etcdCertDir).
			WithCaCertFileName(common.EtcdCaPemFileName).
			WithCaKeyFileName(common.EtcdCaKeyPemFileName).
			WithCert(fmt.Sprintf("peer-%s.pem", hostName)).
			WithCertKey(fmt.Sprintf("peer-%s-key.pem", hostName)).
			WithCommonName(hostName).
			WithOrganization([]string{"kubexm-etcd-peer"}).
			WithHosts([]string{hostName, hostIP}).
			WithUsages([]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}).
			Build()

		peerCertNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name:         peerCertStep.Meta().Name,
			Step:         peerCertStep,
			Hosts:        []connector.Host{controlPlaneHost},
			Dependencies: []plan.NodeID{genCaNodeID},
		})
		allCertNodeIDs = append(allCertNodeIDs, peerCertNodeID)
	}

	// Step 3: Generate apiserver-etcd-client certificate
	apiClientCertStep := commonstep.NewGenerateCertsStepBuilder(ctx, "GenerateApiServerEtcdClientCert").
		WithLocalCertsDir(etcdCertDir).
		WithCaCertFileName(common.EtcdCaPemFileName).
		WithCaKeyFileName(common.EtcdCaKeyPemFileName).
		WithCert("apiserver-etcd-client.pem").
		WithCertKey("apiserver-etcd-client-key.pem").
		WithCommonName("kube-apiserver-etcd-client").
		WithOrganization([]string{"system:masters"}).
		WithUsages([]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}).
		Build()

	apiClientCertNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name:         apiClientCertStep.Meta().Name,
		Step:         apiClientCertStep,
		Hosts:        []connector.Host{controlPlaneHost},
		Dependencies: []plan.NodeID{genCaNodeID},
	})
	allCertNodeIDs = append(allCertNodeIDs, apiClientCertNodeID)

	fragment.EntryNodes = []plan.NodeID{genCaNodeID}
	fragment.ExitNodes = allCertNodeIDs

	logger.Info("Planned steps for Etcd PKI generation on control-plane node.")
	return fragment, nil
}

var _ task.Task = (*GenerateEtcdPkiTask)(nil)
