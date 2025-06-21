package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/pki" // Assuming this is the correct path for PKI steps
	"github.com/mensylisir/kubexm/pkg/task"
)

// GenerateEtcdPkiTask orchestrates the generation of etcd PKI.
type GenerateEtcdPkiTask struct {
	*task.BaseTask
	AltNameHosts         []pki.HostSpecForAltNames // Used by GenerateEtcdAltNamesStep
	ControlPlaneEndpoint string                  // Used by GenerateEtcdAltNamesStep
	DefaultLBDomain      string                  // Used by GenerateEtcdAltNamesStep
}

// NewGenerateEtcdPkiTask creates a new task for generating etcd PKI.
func NewGenerateEtcdPkiTask(
	altNameHosts []pki.HostSpecForAltNames,
	cpEndpoint string,
	defaultLBDomain string,
) task.Task { // Return task.Task
	base := task.NewBaseTask(
		"GenerateEtcdPki",
		"Generates all necessary etcd PKI (CA, member, client certificates).",
		[]string{common.ControlNodeRole}, // Explicitly target control-node role for these local steps
		nil,
		false,
	)
	return &GenerateEtcdPkiTask{
		BaseTask:             &base,
		AltNameHosts:         altNameHosts,
		ControlPlaneEndpoint: cpEndpoint,
		DefaultLBDomain:      defaultLBDomain,
	}
}

// Plan generates the execution fragment for creating etcd PKI.
func (t *GenerateEtcdPkiTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)

	// Get the control node host
	controlNodeHosts, err := ctx.GetHostsByRole(common.ControlNodeRole)
	if err != nil {
		return nil, fmt.Errorf("failed to get control node host for PKI generation: %w", err)
	}
	if len(controlNodeHosts) == 0 {
		return nil, fmt.Errorf("control node host with role '%s' not found", common.ControlNodeRole)
	}
	// PKI steps typically run on one control node, even if multiple are defined (though unusual).
	// Taking the first one.
	controlHost := []connector.Host{controlNodeHosts[0]}
	controlHostName := []string{controlNodeHosts[0].GetName()}

	// Step 1: Determine/Ensure Etcd PKI Path
	// Assuming PKI step constructors now take an instance name as their last string argument.
	step1_determinePath := pki.NewDetermineEtcdPKIPathStep(
		"", // PKIPathToEnsureSharedDataKey
		"", // OutputPKIPathSharedDataKey
		"DetermineEtcdPKIPath", // Instance name for the step
	)
	node1_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), step1_determinePath.Meta().Name))
	nodes[node1_ID] = &plan.ExecutionNode{
		Name:         step1_determinePath.Meta().Name,
		Step:         step1_determinePath,
		Hosts:        controlHost,
		HostNames:    controlHostName,
		StepName:     step1_determinePath.Meta().Name,
		Dependencies: []plan.NodeID{},
	}

	// Step 2: Generate Etcd AltNames
	step2_altNames := pki.NewGenerateEtcdAltNamesStep(
		t.AltNameHosts,
		t.ControlPlaneEndpoint,
		t.DefaultLBDomain,
		"", // Output key for AltNames
		"GenerateEtcdCertAltNames", // Instance name
	)
	node2_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), step2_altNames.Meta().Name))
	nodes[node2_ID] = &plan.ExecutionNode{
		Name:         step2_altNames.Meta().Name,
		Step:         step2_altNames,
		Hosts:        controlHost,
		HostNames:    controlHostName,
		StepName:     step2_altNames.Meta().Name,
		Dependencies: []plan.NodeID{node1_ID}, // Depends on PKI path being determined
	}

	// Step 3: Generate Etcd CA Certificate
	step3_genCA := pki.NewGenerateEtcdCAStep(
		"", // Input PKIPath key
		"", // Input KubeConf key
		"", // Output CA Cert Object key
		"", // Output CA Cert Path key
		"", // Output CA Key Path key
		"GenerateEtcdCA", // Instance name
	)
	node3_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), step3_genCA.Meta().Name))
	nodes[node3_ID] = &plan.ExecutionNode{
		Name:         step3_genCA.Meta().Name,
		Step:         step3_genCA,
		Hosts:        controlHost,
		HostNames:    controlHostName,
		StepName:     step3_genCA.Meta().Name,
		Dependencies: []plan.NodeID{node1_ID, node2_ID}, // Depends on PKI path and AltNames
	}

	// Step 4: Generate Etcd Node Certificates (members, clients)
	step4_genNodeCerts := pki.NewGenerateEtcdNodeCertsStep(
		"", // Input PKIPath key
		"", // Input AltNames key
		"", // Input CA Cert Object key
		"", // Input KubeConf key
		"", // Input Hosts key
		"", // Output Generated Files List key
		"GenerateEtcdNodeCertificates", // Instance name
	)
	node4_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), step4_genNodeCerts.Meta().Name))
	nodes[node4_ID] = &plan.ExecutionNode{
		Name:         step4_genNodeCerts.Meta().Name,
		Step:         step4_genNodeCerts,
		Hosts:        controlHost,
		HostNames:    controlHostName,
		StepName:     step4_genNodeCerts.Meta().Name,
		Dependencies: []plan.NodeID{node1_ID, node2_ID, node3_ID}, // Depends on PKI path, AltNames, and CA
	}

	logger.Info("Planned steps for Etcd PKI generation on control-node.")
	return &task.ExecutionFragment{
		Nodes:      nodes,
		EntryNodes: []plan.NodeID{node1_ID},
		ExitNodes:  []plan.NodeID{node4_ID},
	}, nil
}

// Ensure GenerateEtcdPkiTask implements the new task.Task interface.
var _ task.Task = (*GenerateEtcdPkiTask)(nil)
