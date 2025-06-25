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
	// These steps are assumed to exist in pkg/step/pki and are responsible for
	// generating certificates on the control node.
	// Their parameters (like output paths, cache keys) need to align with how
	// runtime.Context provides paths (e.g., ctx.GetEtcdCertsDir()).

	fragment := task.NewExecutionFragment(t.Name() + "-Fragment")

	// Step 1: Setup PKI Data Context (e.g., base paths for PKI)
	// This step would populate ModuleCache with necessary base paths or configurations.
	// The actual KubexmsKubeConf and HostSpecForPKI would be derived from ctx.GetClusterConfig()
	// and passed to NewSetupEtcdPkiDataContextStep by the Module or Pipeline that creates this Task.
	// For this task, we assume these are available or derived if needed by sub-steps.
	// Let's assume this step takes a config and populates derived values into cache.
	// For now, we'll simplify and assume paths are directly derived from runtime context by cert generation steps.

	// Step 1: Generate Etcd CA
	// Constructor: NewGenerateCACertStep(instanceName, commonName string, organizations []string, validityDays int, outputDir, baseFilename string)
	caOutputDir := ctx.GetEtcdCertsDir() // All certs go into the standard etcd certs directory on control node
	genCaStepName := fmt.Sprintf("%s-GenerateEtcdCA", t.Name())
	genCaStep := pki.NewGenerateCACertStep(
		genCaStepName,
		"etcd-ca", // Common Name for ETCD CA
		[]string{"kubexm-etcd"}, // Organization
		3650, // Validity 10 years
		caOutputDir,
		"ca-etcd", // Base filename -> ca-etcd.crt, ca-etcd.key
	)
	genCaNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name: genCaStepName, Step: genCaStep, Hosts: controlHost, Dependencies: []plan.NodeID{},
	})

	// Step 2: Generate Server/Peer certs for each etcd node, and client cert for apiserver
	// This is a simplification. A real GenerateEtcdNodeCertsStep would loop through etcd nodes
	// and master nodes (for apiserver-etcd-client) and create individual certs.
	// It would need SANs (from AltNameHosts, CP Endpoint) and CA paths.
	// For now, let's represent this as one conceptual step that depends on the CA.
	// A more detailed implementation would break this into multiple pki.NewGenerateSignedCertStep calls.

	// Example for one etcd node (this would be looped in a real scenario or by a smarter step)
	// This part demonstrates how individual cert generation would be planned.
	// A full implementation of GenerateEtcdNodeCertsStep would encapsulate this looping.
	// For now, let's assume GenerateEtcdPkiTask focuses on CA and one example server cert.
	// A more complete PKI task/module would handle all certs.

	var lastCertGenNodeID = genCaNodeID
	etcdNodes, _ := ctx.GetHostsByRole(common.RoleEtcd) // common.RoleEtcd from pkg/common

	for _, etcdHost := range etcdNodes {
		hostName := etcdHost.GetName()
		// TODO: SANs should be properly gathered for each host, including its IP and hostname.
		// This requires HostSpecForAltNames to be correctly populated and passed or derived.
		// For simplicity, using hostname as CN and only a few SANs.
		serverCertStepName := fmt.Sprintf("%s-GenerateEtcdServerCert-%s", t.Name(), hostName)
		serverCertStep := pki.NewGenerateSignedCertStep(
			serverCertStepName,
			hostName, // CN
			[]string{"kubexm-etcd-server"},
			[]string{hostName, etcdHost.GetAddress()}, // SANs
			365, // Validity
			filepath.Join(caOutputDir, "ca-etcd.crt"),
			filepath.Join(caOutputDir, "ca-etcd.key"),
			caOutputDir, // Output dir
			fmt.Sprintf("%s", hostName), // Base filename for server cert (e.g. node1.crt, node1.key)
			false, true, // Not client, Is server
		)
		serverCertNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name: serverCertStepName, Step: serverCertStep, Hosts: controlHost, Dependencies: []plan.NodeID{genCaNodeID},
		})

		peerCertStepName := fmt.Sprintf("%s-GenerateEtcdPeerCert-%s", t.Name(), hostName)
		peerCertStep := pki.NewGenerateSignedCertStep(
			peerCertStepName,
			hostName, // CN
			[]string{"kubexm-etcd-peer"},
			[]string{hostName, etcdHost.GetAddress()}, // SANs
			365, // Validity
			filepath.Join(caOutputDir, "ca-etcd.crt"),
			filepath.Join(caOutputDir, "ca-etcd.key"),
			caOutputDir,
			fmt.Sprintf("peer-%s", hostName), // Base filename for peer cert
			true, true, // Is client (to other peers), Is server (listens to peers)
		)
		peerCertNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
			Name: peerCertStepName, Step: peerCertStep, Hosts: controlHost, Dependencies: []plan.NodeID{genCaNodeID},
		})
		// Server and Peer certs for a node can be generated in parallel after CA.
		// For a sequence, lastCertGenNodeID would be updated. Here, they parallel genCaNodeID.
		// The final exit node should be a conceptual "all-certs-for-this-host-done".
		// For simplicity, let's make the last one in the loop (if any) the one to track.
		lastCertGenNodeID = peerCertNodeID // Or a merge node if these were parallel.
	}

	// Generate apiserver-etcd-client certificate
	// TODO: SANs for apiserver client cert? Usually just CN based.
	apiserverClientCertStepName := fmt.Sprintf("%s-GenerateApiServerEtcdClientCert", t.Name())
	apiserverClientCertStep := pki.NewGenerateSignedCertStep(
		apiserverClientCertStepName,
		"kube-apiserver-etcd-client", // CN
		[]string{"kubexm-etcd-client"}, // Organization
		nil, // SANs for a client cert are less common unless specific mutual TLS scenarios
		365, // Validity
		filepath.Join(caOutputDir, "ca-etcd.crt"),
		filepath.Join(caOutputDir, "ca-etcd.key"),
		caOutputDir,
		"apiserver-etcd-client", // Base filename
		true, false, // Is client, Not server
	)
	apiserverClientCertNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
		Name: apiserverClientCertStepName, Step: apiserverClientCertStep, Hosts: controlHost, Dependencies: []plan.NodeID{genCaNodeID},
	})


	fragment.EntryNodes = []plan.NodeID{genCaNodeID}
	// Exit nodes are all leaf certificate generation nodes
	fragment.ExitNodes = []plan.NodeID{}
	if lastCertGenNodeID != genCaNodeID { // If any node certs were added
		 for _, node := range fragment.Nodes { // Find all nodes that depend only on CA
			 isLeaf := true
			 for _, otherNode := range fragment.Nodes {
				 if otherNode.Step == node.Step { continue }
				 for _, dep := range otherNode.Dependencies {
					 if dep == plan.NodeID(node.Name) { // if current node is a dependency for another
						isLeaf = false
						break
					 }
				 }
				 if !isLeaf { break }
			 }
			 if isLeaf && plan.NodeID(node.Name) != genCaNodeID { // Exclude CA itself if it's a leaf for some reason
				fragment.ExitNodes = append(fragment.ExitNodes, plan.NodeID(node.Name))
			 }
		 }
	}
	if len(fragment.ExitNodes) == 0 && genCaNodeID != "" { // If only CA was generated
	    fragment.ExitNodes = []plan.NodeID{genCaNodeID}
	}
	fragment.ExitNodes = append(fragment.ExitNodes, apiserverClientCertNodeID)
	fragment.ExitNodes = task.UniqueNodeIDs(fragment.ExitNodes)


	logger.Info("Planned steps for Etcd PKI generation on control-node.")
	return fragment, nil
}

// Ensure GenerateEtcdPkiTask implements the new task.Task interface.
var _ task.Task = (*GenerateEtcdPkiTask)(nil)
