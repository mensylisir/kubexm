package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/pki" // Assuming this is the correct path for PKI steps
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/pki" // Assuming this is the correct path for PKI steps
	"github.com/mensylisir/kubexm/pkg/task"
)

// GenerateEtcdPkiTask orchestrates the generation of etcd PKI.
type GenerateEtcdPkiTask struct {
	taskName             string
	taskDesc             string
	runOnRoles           []string
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
	return &GenerateEtcdPkiTask{
		taskName:             "GenerateEtcdPki",
		taskDesc:             "Generates all necessary etcd PKI (CA, member, client certificates).",
		runOnRoles:           []string{common.ControlNodeRole}, // Explicitly target control-node role
		AltNameHosts:         altNameHosts,
		ControlPlaneEndpoint: cpEndpoint,
		DefaultLBDomain:      defaultLBDomain,
	}
}

// Name returns the name of the task.
func (t *GenerateEtcdPkiTask) Name() string {
	return t.taskName
}

// IsRequired determines if the task needs to run.
// For PKI generation on control-node, it's typically always required if this task is part of the plan.
func (t *GenerateEtcdPkiTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// This task runs on control-node. Check if control-node exists.
	hosts, err := ctx.GetHostsByRole(common.ControlNodeRole)
	if err != nil {
		return false, fmt.Errorf("failed to get hosts for role '%s' in task %s: %w", common.ControlNodeRole, t.Name(), err)
	}
	return len(hosts) > 0, nil
}

// Plan generates the execution fragment for creating etcd PKI.
func (t *GenerateEtcdPkiTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	nodes := make(map[plan.NodeID]*plan.ExecutionNode)

	controlNodeHosts, err := ctx.GetHostsByRole(common.ControlNodeRole)
	if err != nil {
		return nil, fmt.Errorf("failed to get control node host for PKI generation: %w", err)
	}
	if len(controlNodeHosts) == 0 {
		return nil, fmt.Errorf("control node host with role '%s' not found", common.ControlNodeRole)
	}
	controlHost := []connector.Host{controlNodeHosts[0]} // PKI steps run on one control node.

	// Node Names (also used as part of NodeID for uniqueness within task)
	determinePathNodeName := "DetermineEtcdPKIPath"
	altNamesNodeName := "GenerateEtcdCertAltNames"
	genCANodeName := "GenerateEtcdCA"
	genNodeCertsNodeName := "GenerateEtcdNodeCertificates"

	// Step 1: Determine/Ensure Etcd PKI Path
	// Assuming PKI step constructors are updated: NewXYZStep(instanceName, ...params, sudoBool)
	// For local PKI ops, sudo might not be needed if paths are user-writable (e.g. in global work dir).
	// Let's assume sudo:false for these local PKI steps.
	step1_determinePath := pki.NewDetermineEtcdPKIPathStep(
		determinePathNodeName, // instanceName for step's Meta
		"",                    // PKIPathToEnsureSharedDataKey
		"",                    // OutputPKIPathSharedDataKey
		false,                 // sudo
	)
	node1_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), determinePathNodeName))
	nodes[node1_ID] = &plan.ExecutionNode{
		Name:         determinePathNodeName,
		Step:         step1_determinePath,
		Hosts:        controlHost,
		Dependencies: []plan.NodeID{},
	}

	// Step 2: Generate Etcd AltNames
	step2_altNames := pki.NewGenerateEtcdAltNamesStep(
		altNamesNodeName, // instanceName
		t.AltNameHosts,
		t.ControlPlaneEndpoint,
		t.DefaultLBDomain,
		"",    // Output key for AltNames
		false, // sudo
	)
	node2_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), altNamesNodeName))
	nodes[node2_ID] = &plan.ExecutionNode{
		Name:         altNamesNodeName,
		Step:         step2_altNames,
		Hosts:        controlHost,
		Dependencies: []plan.NodeID{node1_ID},
	}

	// Step 3: Generate Etcd CA Certificate
	step3_genCA := pki.NewGenerateEtcdCAStep(
		genCANodeName, // instanceName
		"",            // Input PKIPath key
		"",            // Input KubeConf key (if needed by step, else remove)
		"",            // Output CA Cert Object key
		"",            // Output CA Cert Path key
		"",            // Output CA Key Path key
		false,         // sudo
	)
	node3_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), genCANodeName))
	nodes[node3_ID] = &plan.ExecutionNode{
		Name:         genCANodeName,
		Step:         step3_genCA,
		Hosts:        controlHost,
		Dependencies: []plan.NodeID{node1_ID, node2_ID},
	}

	// Step 4: Generate Etcd Node Certificates (members, clients)
	step4_genNodeCerts := pki.NewGenerateEtcdNodeCertsStep(
		genNodeCertsNodeName, // instanceName
		"",                   // Input PKIPath key
		"",                   // Input AltNames key
		"",                   // Input CA Cert Object key
		"",                   // Input KubeConf key (if needed)
		"",                   // Input Hosts key (for which nodes to gen certs, if applicable beyond altnames)
		"",                   // Output Generated Files List key
		false,                // sudo
	)
	node4_ID := plan.NodeID(fmt.Sprintf("%s-%s", t.Name(), genNodeCertsNodeName))
	nodes[node4_ID] = &plan.ExecutionNode{
		Name:         genNodeCertsNodeName,
		Step:         step4_genNodeCerts,
		Hosts:        controlHost,
		Dependencies: []plan.NodeID{node1_ID, node2_ID, node3_ID},
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
