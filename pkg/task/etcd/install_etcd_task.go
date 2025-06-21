package etcd

import (
	"fmt"
	"path/filepath" // For joining paths if needed for resource names/keys
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/resource"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/etcd" // Import new etcd steps
	"github.com/mensylisir/kubexm/pkg/task"
	// "github.com/mensylisir/kubexm/pkg/utils" // If any utils are needed
)

// InstallETCDTask defines the task for installing ETCD cluster.
type InstallETCDTask struct {
	task.BaseTask // Embed BaseTask for common functionality
	// Task-specific configurations can be added here if needed,
	// but most should come from ClusterConfiguration via runtime.TaskContext.
}

// NewInstallETCDTask creates a new InstallETCDTask.
func NewInstallETCDTask() task.Task {
	return &InstallETCDTask{
		BaseTask: task.BaseTask{
			TaskName: "InstallETCDCluster",
			TaskDesc: "Installs a new ETCD cluster on designated nodes.",
		},
	}
}

// IsRequired checks if this task is required based on the cluster configuration.
func (t *InstallETCDTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	clusterConfig := ctx.GetClusterConfig()
	if clusterConfig.Spec.Etcd == nil || clusterConfig.Spec.Etcd.Type == v1alpha1.EtcdTypeExternal {
		ctx.GetLogger().Info("ETCD installation is not required (external ETCD or not configured).")
		return false, nil
	}
	// Could add more checks, e.g., if etcd nodes are defined.
	return true, nil
}

// Plan generates the execution plan (fragment) for installing ETCD.
func (t *InstallETCDTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	clusterConfig := ctx.GetClusterConfig()

	etcdNodes, err := ctx.GetHostsByRole(v1alpha1.ETCDRole) // Assuming role name is defined in v1alpha1
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd nodes: %w", err)
	}
	if len(etcdNodes) == 0 {
		return nil, fmt.Errorf("no etcd nodes found in the configuration for task %s", t.Name())
	}
    masterNodes, err := ctx.GetHostsByRole(v1alpha1.MasterRole)
	if err != nil {
		// Non-fatal if no masters, but cert distribution might be partial
		logger.Warn("No master nodes found, etcd client certs for apiserver might not be distributed by this task if intended for masters.", "error", err)
	}


	// --- 1. Resource Acquisition on Control Node ---
	etcdVersion := clusterConfig.Spec.Etcd.Version
	if etcdVersion == "" {
		etcdVersion = "v3.5.9" // Default, or get from a constants file
		logger.Info("ETCD version not specified, using default.", "version", etcdVersion)
	}
	// Arch will be auto-detected by the resource handle if not specified.
	// TODO: Get URL template and archive/binary names from config or constants.
	// Use a single handle for the ETCD archive, which contains both etcd and etcdctl.
	etcdArchiveHandle := resource.NewRemoteBinaryArchiveHandle(
		"etcd-archive", // A logical name for this resource handle
		strings.TrimPrefix(etcdVersion, "v"),
		clusterConfig.Spec.Etcd.Arch,
		"https://github.com/etcd-io/etcd/releases/download/{{.Version}}/etcd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		"", // ArchiveFileName (let handle derive)
		// Define the binaries expected within this archive and their relative paths
		map[string]string{
			"etcd":    "etcd-{{.Version}}-linux-{{.Arch}}/etcd",
			"etcdctl": "etcd-{{.Version}}-linux-{{.Arch}}/etcdctl",
		},
		"", // Checksum (TODO: add checksums)
	)

	// Plan for ensuring etcd archive is downloaded and extracted on the control node.
	// This fragment contains steps like DownloadFile and ExtractArchive running on the control node.
	resourcePrepFragment, err := etcdArchiveHandle.EnsurePlan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan ETCD archive acquisition: %w", err)
	}

	// Initialize the main execution graph fragment for this task
	fragment := task.NewExecutionFragment()

	// Merge nodes from resource preparation (download, extract on control node)
	for nodeID, node := range resourcePrepFragment.Nodes {
		fragment.Nodes[nodeID] = node
	}
	// The entry points for the overall task fragment are initially the entry points of the resource prep.
	fragment.EntryNodes = append(fragment.EntryNodes, resourcePrepFragment.EntryNodes...)

	// After resourcePrepFragment.EnsurePlan(), the paths to the *extracted binaries on the control node*
	// should be available via etcdArchiveHandle.GetLocalPath("etcd", ctx) and ...("etcdctl", ctx).
	// Or, they are stored in cache by keys like "resource.etcd-archive.extracted.etcd"
	localEtcdBinaryPathOnControlNode, err := etcdArchiveHandle.GetLocalPath("etcd", ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get local path for etcd binary after EnsurePlan: %w", err)
	}
	localEtcdctlBinaryPathOnControlNode, err := etcdArchiveHandle.GetLocalPath("etcdctl", ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get local path for etcdctl binary after EnsurePlan: %w", err)
	}

	// This is the cache key for the directory *on the target etcd node* where etcd/etcdctl will be after extraction *on that node*.
	extractedDirCacheKeyOnTargetNode := etcd.ExtractedEtcdDirCacheKey // e.g., "ExtractedEtcdDir"


	// --- 2. PKI Generation (Conceptual - assumed done by a preceding PKI task) ---
	// Paths to locally available CA cert, etcd server certs, peer certs, client certs
	// would be retrieved from ModuleCache or specific resource handles.
	// For example:
	// caCertPath := ctx.ModuleCache().GetString("pki.ca.cert_path")
	// etcdServerCertPath := ctx.ModuleCache().GetString(fmt.Sprintf("pki.etcd.server.%s.cert_path", node.GetName()))
	// ... and so on for keys.
	// This task will then use these local paths as sources for DistributeEtcdCertsStep.

	// --- 3. Orchestrate Steps per ETCD Node ---
	var lastNodeIDProcessedOnControlNode plan.NodeID
	if len(resourcePrepFragment.ExitNodes) > 0 {
        // Assuming single exit node from resource prep for simplicity, or choose one if multiple.
        // This represents "all resources ready on control node".
		lastNodeIDProcessedOnControlNode = resourcePrepFragment.ExitNodes[0]
	}


	allNodesForCerts := append([]connector.Host{}, etcdNodes...) // Start with etcdNodes
	if masterNodes != nil {
		allNodesForCerts = append(allNodesForCerts, masterNodes...)
	}
	// Ensure unique nodes for cert distribution
	uniqueHostMap := make(map[string]connector.Host)
	for _, host := range allNodesForCerts {
		uniqueHostMap[host.GetName()] = host
	}
	var certDistributionTargetHosts []connector.Host
	for _, host := range uniqueHostMap {
		certDistributionTargetHosts = append(certDistributionTargetHosts, host)
	}

	// Step: Distribute ETCD Certificates to all relevant nodes
	// This step would take local paths to certs (from PKI task output) as parameters.
	// For this example, we assume the step knows how to find them or they are at default paths.
	distributeCertsStepName := "DistributeETCDCertsToAllRequiredNodes"
	distributeCertsStepInstance := etcd.NewDistributeEtcdCertsStep(
		distributeCertsStepName,
		clusterConfig.Name, // clusterName for pathing conventions
		ctx.GetWorkDir(),   // Base directory on control node where certs might be found (e.g. workdir/clusterName/pki/etcd)
		"/etc/etcd/pki",    // RemotePKIDir on target nodes
		v1alpha1.ETCDRole,  // Hint for which certs etcd nodes need
		v1alpha1.MasterRole,// Hint for which certs master nodes need (apiserver-etcd-client)
		true,               // Sudo
	)

	// Create one node for distributing all certs (conceptually, can be parallelized by step internally if safe)
	// Or, create per-host nodes if the step is designed per-host.
	// Assuming DistributeEtcdCertsStep handles multiple hosts internally or we create per-host nodes.
	// For simplicity, let's assume one overarching cert distribution node.
	// If this step itself generates per-host nodes, this is different.
	// Let's assume for now it's a single logical node that distributes to all its target hosts.
	// This is not ideal for the DAG model if the step doesn't return a fragment.
	// Better: DistributeEtcdCertsStep is applied per host, or it's a task itself.
	// Sticking to the current step model:

	var certDistributionNodeIDs []plan.NodeID
    if len(certDistributionTargetHosts) > 0 {
        for _, host := range certDistributionTargetHosts {
            nodeID := plan.NodeID(fmt.Sprintf("distribute-etcd-certs-%s", strings.ReplaceAll(host.GetName(), ".", "-")))
            node := &plan.ExecutionNode{
                Name:        fmt.Sprintf("%s-on-%s", distributeCertsStepName, host.GetName()),
                Step:        distributeCertsStepInstance, // The same step instance applied to different hosts
                Hosts:       []connector.Host{host},
                StepName:    distributeCertsStepInstance.Meta().Name,
                Dependencies: []plan.NodeID{},
            }
            if lastNodeIDProcessedOnControlNode != "" { // Depends on resources (and potentially PKI gen) being ready locally
                node.Dependencies = append(node.Dependencies, lastNodeIDProcessedOnControlNode)
            }
            fragment.Nodes[nodeID] = node
            certDistributionNodeIDs = append(certDistributionNodeIDs, nodeID)
        }
    }


	initialClusterPeers := []string{}
	for _, node := range etcdNodes {
		nodeIP := node.GetAddress() // TODO: Use a specific internal peer IP from config/facts
		initialClusterPeers = append(initialClusterPeers, fmt.Sprintf("%s=https://%s:%d", node.GetName(), nodeIP, clusterConfig.Spec.Etcd.PeerPort))
	}
	initialClusterString := strings.Join(initialClusterPeers, ",")

    var allEtcdNodeFinalStepIDs []plan.NodeID
    var firstEtcdNodeBootstrapSuccessNodeID plan.NodeID

	for i, node := range etcdNodes {
		nodeLogger := logger.With("etcd_node", node.GetName())
		nodeLogger.Info("Planning steps for ETCD node.")
		nodeSpecificPrefix := fmt.Sprintf("etcd-%s-", strings.ReplaceAll(node.GetName(),".","-"))

		currentDependencyNodeID := lastNodeIDProcessedOnControlNode // Start depending on control node resource prep

		// Step: Distribute ETCD Binary (etcd specifically) to this node
        // This step takes the *local path on control node* to the specific binary.
		distributeEtcdActualBinaryStep := etcd.NewDistributeSingleBinaryStep( // Assuming a new or refactored step
			nodeSpecificPrefix+"DistributeEtcdBin",
			localEtcdBinaryPathOnControlNode, // Source path on control node
			"/usr/local/bin/etcd",            // Target path on ETCD node
			"0755",
			true, // Sudo
		)
		distributeEtcdBinNodeID := plan.NodeID(nodeSpecificPrefix+"distribute-etcd-bin")
		fragment.Nodes[distributeEtcdBinNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Distribute-etcd-binary-to-%s", node.GetName()),
			Step:         distributeEtcdActualBinaryStep,
			Hosts:        []connector.Host{node},
			StepName:     distributeEtcdActualBinaryStep.Meta().Name,
			Dependencies: []plan.NodeID{currentDependencyNodeID},
		}
		currentDependencyNodeID = distributeEtcdBinNodeID

		// Step: Distribute ETCD Binary (etcdctl specifically) to this node
		distributeEtcdctlActualBinaryStep := etcd.NewDistributeSingleBinaryStep(
			nodeSpecificPrefix+"DistributeEtcdctlBin",
			localEtcdctlBinaryPathOnControlNode, // Source path on control node
			"/usr/local/bin/etcdctl",            // Target path on ETCD node
			"0755",
			true, // Sudo
		)
		distributeEtcdctlBinNodeID := plan.NodeID(nodeSpecificPrefix+"distribute-etcdctl-bin")
		fragment.Nodes[distributeEtcdctlBinNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Distribute-etcdctl-binary-to-%s", node.GetName()),
			Step:         distributeEtcdctlActualBinaryStep,
			Hosts:        []connector.Host{node},
			StepName:     distributeEtcdctlActualBinaryStep.Meta().Name,
			Dependencies: []plan.NodeID{ /* Can run in parallel with etcd bin distribution if desired, or sequentially */
			    plan.NodeID(nodeSpecificPrefix+"distribute-etcd-bin"), // Example: make it sequential for simplicity
            },
		}
        // If distributing etcd & etcdctl in parallel, both would depend on lastNodeIDProcessedOnControlNode.
        // If sequential like above, currentDependencyNodeID updates. For now, let's make them parallel from control node's readiness.
        fragment.Nodes[distributeEtcdctlBinNodeID].Dependencies = []plan.NodeID{lastNodeIDProcessedOnControlNode}
        // The next step will depend on *both* etcd and etcdctl being distributed.
        binariesDistributedNodeID := plan.NodeID(nodeSpecificPrefix + "binaries-distributed-marker")
        fragment.Nodes[binariesDistributedNodeID] = &plan.ExecutionNode{
            Name: fmt.Sprintf("Marker-EtcdBinariesDistributed-%s", node.GetName()),
            Step: task.NewNoOpStep(fmt.Sprintf("Marker-EtcdBinariesDistributed-%s", node.GetName())), // A NoOp step
            Hosts: []connector.Host{node},
            StepName: "NoOp",
            Dependencies: []plan.NodeID{distributeEtcdBinNodeID, distributeEtcdctlBinNodeID },
        }
		currentDependencyNodeID = binariesDistributedNodeID


		// Step: Generate etcd.yaml config
		nodeIP := node.GetAddress() // This should be the IP for peer/client URLs
		etcdConfigData := etcd.EtcdNodeConfigData{
			Name:                     node.GetName(),
			DataDir:                  filepath.Join(clusterConfig.Spec.Etcd.DataDirBase, "etcd"),
			WalDir:                   filepath.Join(clusterConfig.Spec.Etcd.DataDirBase, "etcd", "wal"),
			ListenPeerURLs:           fmt.Sprintf("https://%s:%d,https://127.0.0.1:%d", nodeIP, clusterConfig.Spec.Etcd.PeerPort, clusterConfig.Spec.Etcd.PeerPort),
			ListenClientURLs:         fmt.Sprintf("https://%s:%d,https://127.0.0.1:%d", nodeIP, clusterConfig.Spec.Etcd.ClientPort, clusterConfig.Spec.Etcd.ClientPort),
			InitialAdvertisePeerURLs: fmt.Sprintf("https://%s:%d", nodeIP, clusterConfig.Spec.Etcd.PeerPort),
			AdvertiseClientURLs:      fmt.Sprintf("https://%s:%d", nodeIP, clusterConfig.Spec.Etcd.ClientPort),
			InitialCluster:           initialClusterString,
			InitialClusterToken:      clusterConfig.Spec.Etcd.ClusterToken,
			TrustedCAFile:            filepath.Join("/etc/etcd/pki", "ca.pem"), // Assuming this path from DistributeCerts
			CertFile:                 filepath.Join("/etc/etcd/pki", fmt.Sprintf("%s.pem", node.GetName())), // Node specific server cert
			KeyFile:                  filepath.Join("/etc/etcd/pki", fmt.Sprintf("%s-key.pem", node.GetName())),
			PeerTrustedCAFile:        filepath.Join("/etc/etcd/pki", "ca.pem"),
			PeerCertFile:             filepath.Join("/etc/etcd/pki", fmt.Sprintf("peer-%s.pem", node.GetName())),// Node specific peer cert
			PeerKeyFile:              filepath.Join("/etc/etcd/pki", fmt.Sprintf("peer-%s-key.pem", node.GetName())),
		}
		if i == 0 { etcdConfigData.InitialClusterState = "new" } else { etcdConfigData.InitialClusterState = "existing" }

		generateConfigStepInstance := etcd.NewGenerateEtcdConfigStep(nodeSpecificPrefix+"GenerateEtcdConfig", etcdConfigData, "/etc/etcd/etcd.yaml", true)
		generateConfigNodeID := plan.NodeID(nodeSpecificPrefix + "generate-etcd-config")
        certNodeForThisHostID := plan.NodeID(fmt.Sprintf("distribute-etcd-certs-%s", strings.ReplaceAll(node.GetName(),".","-")))
		fragment.Nodes[generateConfigNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Generate-etcd.yaml-for-%s", node.GetName()),
			Step:         generateConfigStepInstance,
			Hosts:        []connector.Host{node},
			StepName:     generateConfigStepInstance.Meta().Name,
			Dependencies: []plan.NodeID{currentDependencyNodeID, certNodeForThisHostID}, // Depends on binaries and certs being on the node
		}
		currentDependencyNodeID = generateConfigNodeID

		// Step: Generate etcd.service systemd file
		etcdServiceData := etcd.EtcdServiceData{ExecStart: "/usr/local/bin/etcd --config-file=/etc/etcd/etcd.yaml"}
		generateServiceInstance := etcd.NewGenerateEtcdServiceStep(nodeSpecificPrefix+"GenerateEtcdService", etcdServiceData, "/etc/systemd/system/etcd.service", true)
		generateServiceNodeID := plan.NodeID(nodeSpecificPrefix + "generate-etcd-service")
		fragment.Nodes[generateServiceNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Generate-etcd.service-for-%s", node.GetName()),
			Step:         generateServiceInstance,
			Hosts:        []connector.Host{node},
			StepName:     generateServiceInstance.Meta().Name,
			Dependencies: []plan.NodeID{currentDependencyNodeID},
		}
		currentDependencyNodeID = generateServiceNodeID

		// Step: Systemctl Daemon Reload
		daemonReloadInstance := etcd.NewManageEtcdServiceStep(nodeSpecificPrefix+"DaemonReloadEtcd", etcd.ActionDaemonReload, "etcd", true)
		daemonReloadNodeID := plan.NodeID(nodeSpecificPrefix + "daemon-reload-etcd")
		fragment.Nodes[daemonReloadNodeID] = &plan.ExecutionNode{
			Name: fmt.Sprintf("DaemonReload-for-etcd-on-%s", node.GetName()),
			Step: daemonReloadInstance,
			Hosts: []connector.Host{node},
			StepName: daemonReloadInstance.Meta().Name,
			Dependencies: []plan.NodeID{currentDependencyNodeID},
		}
		currentDependencyNodeID = daemonReloadNodeID

		// Step: Start and Enable ETCD service (Bootstrap or Join logic within these steps or separate steps)
		if i == 0 { // First ETCD node
			bootstrapInstance := etcd.NewBootstrapFirstEtcdNodeStep(nodeSpecificPrefix+"BootstrapEtcd", "etcd", true) // This step would handle start & enable
			bootstrapNodeID := plan.NodeID(nodeSpecificPrefix + "bootstrap-etcd")
			fragment.Nodes[bootstrapNodeID] = &plan.ExecutionNode{
				Name: fmt.Sprintf("Bootstrap-etcd-on-%s", node.GetName()),
				Step: bootstrapInstance,
				Hosts: []connector.Host{node},
				StepName: bootstrapInstance.Meta().Name,
				Dependencies: []plan.NodeID{currentDependencyNodeID},
			}
			currentDependencyNodeID = bootstrapNodeID
            firstEtcdNodeBootstrapSuccessNodeID = bootstrapNodeID // Capture for other nodes to depend on
		} else { // Subsequent ETCD nodes
			joinInstance := etcd.NewJoinEtcdNodeStep(nodeSpecificPrefix+"JoinEtcd", "etcd", true) // This step would handle start & enable
			joinNodeID := plan.NodeID(nodeSpecificPrefix + "join-etcd")
			dependenciesForJoin := []plan.NodeID{currentDependencyNodeID}
			if firstEtcdNodeBootstrapSuccessNodeID != "" {
				dependenciesForJoin = append(dependenciesForJoin, firstEtcdNodeBootstrapSuccessNodeID)
			}
			fragment.Nodes[joinNodeID] = &plan.ExecutionNode{
				Name: fmt.Sprintf("Join-etcd-on-%s", node.GetName()),
				Step: joinInstance,
				Hosts: []connector.Host{node},
				StepName: joinInstance.Meta().Name,
				Dependencies: dependenciesForJoin,
			}
			currentDependencyNodeID = joinNodeID
		}
        allEtcdNodeFinalStepIDs = append(allEtcdNodeFinalStepIDs, currentDependencyNodeID)
	}

	// Determine overall fragment Entry and Exit nodes
	// EntryNodes are already set from resourcePrepFragment.EntryNodes
    // ExitNodes are the final steps on each ETCD node.
    fragment.ExitNodes = task.UniqueNodeIDs(allEtcdNodeFinalStepIDs)
    // Also, if cert distribution nodes have no downstream dependencies within this task, they are also exit nodes.
    // This part needs careful thought based on actual graph structure.
    // For now, assume final ETCD operational state on each node are the primary exits.

	logger.Info("ETCD installation plan generated.")
	return fragment, nil
}

// Ensure InstallETCDTask implements task.Task.
var _ task.Task = &InstallETCDTask{}
