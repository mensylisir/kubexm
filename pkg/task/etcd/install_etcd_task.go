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
	etcdBinaryHandle := resource.NewRemoteBinaryHandle(
		"etcd",
		strings.TrimPrefix(etcdVersion, "v"), // RemoteBinaryHandle might expect version without 'v'
		clusterConfig.Spec.Etcd.Arch, // Can be empty for auto-detect
		"https://github.com/etcd-io/etcd/releases/download/{{.Version}}/etcd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		"", // ArchiveFileName (let handle derive)
		"etcd-{{.Version}}-linux-{{.Arch}}/etcd", // BinaryPathInArchive for etcd
		"", // Checksum (TODO: add checksums)
	)
	etcdctlBinaryHandle := resource.NewRemoteBinaryHandle(
		"etcdctl",
		strings.TrimPrefix(etcdVersion, "v"),
		clusterConfig.Spec.Etcd.Arch,
		"https://github.com/etcd-io/etcd/releases/download/{{.Version}}/etcd-{{.Version}}-linux-{{.Arch}}.tar.gz",
		"",
		"etcd-{{.Version}}-linux-{{.Arch}}/etcdctl",
		"",
	)

	// Plan for ensuring etcd binary archive is on control node
	ensureEtcdArchivePlan, err := etcdBinaryHandle.EnsurePlan(ctx) // etcdctl is in the same archive
	if err != nil {
		return nil, fmt.Errorf("failed to plan etcd binary acquisition: %w", err)
	}

	// Initialize the main execution graph fragment
	graph := task.NewExecutionFragment()
	graph.Merge(ensureEtcdArchivePlan) // ensureEtcdArchivePlan runs on control-node

	// The path to the downloaded archive on the control node will be needed by DistributeEtcdBinaryStep.
	// The RemoteBinaryHandle's EnsurePlan step (GenericDownloadStep) should put this path into cache.
	// Example key: resource.etcd-v3.5.9-amd64-binary.downloadedPath
	// We need to know this key or have the resource handle provide it.
	// For now, assume a known cache key pattern or that the resource handle's `Path` method
	// after EnsurePlan points to the *archive* if that's what needs distributing.
	// Let's assume etcdBinaryHandle.Path(ctx) after EnsurePlan gives the *archive path*.
	// This is a simplification; RemoteBinaryHandle.Path currently gives final binary, not archive.
	// The GenericDownloadStep within RemoteBinaryHandle DOES cache the downloaded archive path.

	// Let's use the cache key from the GenericDownloadStep inside RemoteBinaryHandle
	downloadedArchiveCacheKey := fmt.Sprintf("resource.%s.downloadedPath", etcdBinaryHandle.ID())
	extractedDirCacheKeyOnNode := etcd.ExtractedEtcdDirCacheKey // e.g., "ExtractedEtcdDir"


	// --- 2. PKI Generation (Simplified - assuming certs are generated by a separate PKI task or pre-exist on control node) ---
	// For this task, we'll assume certificate LocalCertificateHandles are used to reference them.
	// The actual generation steps (GenerateCACertStep, GenerateSignedCertStep) would be in a PKITask
	// and would populate the paths that these LocalCertificateHandles point to.

	// --- 3. Orchestrate Steps per ETCD Node ---
	var lastStepNodeIDsOnEachNode = make(map[string]plan.NodeID) // Tracks the last step for each node for sequencing
	allNodes := append(etcdNodes, masterNodes...) // For cert distribution, ensure unique nodes

	// Ensure unique nodes for cert distribution (masters might also be etcd nodes)
    uniqueNodesForCerts := make(map[string]connector.Host)
    for _, node := range allNodes {
        uniqueNodesForCerts[node.GetName()] = node
    }

    // Create a list of unique hosts for cert distribution
    var certDistributionHosts []connector.Host
    for _, h := range uniqueNodesForCerts {
        certDistributionHosts = append(certDistributionHosts, h)
    }

	// Step: Distribute ETCD Certificates to all relevant nodes
	distributeCertsStep := etcd.NewDistributeEtcdCertsStep(
		"DistributeETCDCerts",
		clusterConfig.Name,
		"", // CertBaseDirOnControlNode - let step derive default
		"/etc/etcd/pki", // RemotePKIDir
		v1alpha1.ETCDRole,
		v1alpha1.MasterRole,
		true, // Sudo
	)
	distributeCertsNodeID := graph.AddNodePerHost("distribute-etcd-certs", certDistributionHosts, distributeCertsStep)
	// This step depends on certs being present locally. If certs are generated *by this plan*,
	// then this node would depend on those generation nodes. For now, assume pre-existing/externally generated.
	// If ensureEtcdArchivePlan has exit nodes, make cert distribution depend on them if certs are from archive (unlikely for PKI).
	// For now, assume cert distribution can run in parallel with archive download or after.
    // To be safe, let's make it depend on archive download if there are exit nodes from that plan.
    if len(ensureEtcdArchivePlan.ExitNodes) > 0 {
        for i := range distributeCertsNodeID {
            graph.Nodes[distributeCertsNodeID[i]].Dependencies = append(graph.Nodes[distributeCertsNodeID[i]].Dependencies, ensureEtcdArchivePlan.ExitNodes...)
        }
    }


	initialClusterPeers := []string{}
	for _, node := range etcdNodes {
		// TODO: Get peer URL for each node (needs node IP/hostname and peer port from config)
		// For now, placeholder. This needs proper IP/hostname resolution for each etcdNode.
		nodeFacts, _ := ctx.GetHostFacts(node)
		nodeIP := node.GetAddress() // Or a specific internal IP from facts if available/configured
		if nodeFacts != nil && nodeFacts.IPv4Default != "" { // Example: prefer default IPv4
			// nodeIP = nodeFacts.IPv4Default // This might not be the peer IP; config should be specific
		}
		initialClusterPeers = append(initialClusterPeers, fmt.Sprintf("%s=https://%s:%d", node.GetName(), nodeIP, clusterConfig.Spec.Etcd.PeerPort))
	}
	initialClusterString := strings.Join(initialClusterPeers, ",")


	for i, node := range etcdNodes {
		nodeLogger := logger.With("etcd_node", node.GetName())
		nodeLogger.Info("Planning steps for ETCD node.")

		// Unique prefix for node-specific cache keys and node IDs
		nodePrefix := fmt.Sprintf("etcd-%s-", strings.ReplaceAll(node.GetName(), ".", "-"))

		// Step: Distribute ETCD Binary Archive to this node
		distributeBinaryStep := etcd.NewDistributeEtcdBinaryStep(
			nodePrefix+"DistributeArchive",
			downloadedArchiveCacheKey, // Input: local archive path from control node
			"/tmp/kubexm-archives",    // RemoteTempDir on etcd node
			filepath.Base(etcdBinaryHandle.Path(ctx)+".tar.gz"), // Assuming Path gives binary, so append .tar.gz for archive name
			nodePrefix+etcd.EtcdArchiveRemotePathCacheKey, // Output: remote archive path on this node
			true, // Sudo for mkdirp if needed
		)
		distributeBinaryNodeID := graph.AddNode(nodePrefix+"distribute-archive", []connector.Host{node}, distributeBinaryStep)
		if len(ensureEtcdArchivePlan.ExitNodes) > 0 {
			graph.Nodes[distributeBinaryNodeID].Dependencies = append(graph.Nodes[distributeBinaryNodeID].Dependencies, ensureEtcdArchivePlan.ExitNodes...)
		}
		lastStepNodeIDsOnEachNode[node.GetName()] = distributeBinaryNodeID

		// Step: Extract ETCD Binary Archive on this node
		extractBinaryStep := etcd.NewExtractEtcdBinaryStep(
			nodePrefix+"ExtractArchive",
			nodePrefix+etcd.EtcdArchiveRemotePathCacheKey, // Input: remote archive path from previous step
			"/tmp/kubexm-extracted/"+nodePrefix+"etcd", // TargetExtractBaseDir
			fmt.Sprintf("etcd-%s-linux-%s", strings.TrimPrefix(etcdVersion, "v"), etcdBinaryHandle.(*resource.RemoteBinaryHandle).Arch), // ArchiveInternalSubDir, e.g. etcd-v3.5.9-linux-amd64
			nodePrefix+extractedDirCacheKeyOnNode, // Output: path to dir containing etcd/etcdctl
			false, // Sudo for extraction (usually false for /tmp)
			true,  // RemoveArchiveAfterExtract
		)
		extractBinaryNodeID := graph.AddNode(nodePrefix+"extract-archive", []connector.Host{node}, extractBinaryStep)
		graph.Nodes[extractBinaryNodeID].Dependencies = append(graph.Nodes[extractBinaryNodeID].Dependencies, lastStepNodeIDsOnEachNode[node.GetName()])
		lastStepNodeIDsOnEachNode[node.GetName()] = extractBinaryNodeID

		// Step: Copy etcd and etcdctl to system path
		copyBinariesStep := etcd.NewCopyEtcdBinariesToPathStep(
			nodePrefix+"CopyBinaries",
			nodePrefix+extractedDirCacheKeyOnNode, // Input: path to extracted dir
			"/usr/local/bin", // TargetDir
			strings.TrimPrefix(etcdVersion, "v"), // ExpectedVersion for check
			true, // Sudo
			true, // RemoveSourceAfterCopy (remove the /tmp/kubexm-extracted/etcd/... dir)
		)
		copyBinariesNodeID := graph.AddNode(nodePrefix+"copy-binaries", []connector.Host{node}, copyBinariesStep)
		graph.Nodes[copyBinariesNodeID].Dependencies = append(graph.Nodes[copyBinariesNodeID].Dependencies, lastStepNodeIDsOnEachNode[node.GetName()])
		lastStepNodeIDsOnEachNode[node.GetName()] = copyBinariesNodeID

		// Step: Generate etcd.yaml
		// Construct EtcdNodeConfigData for this node
		nodeIP := node.GetAddress() // This should be the IP for peer/client URLs
		etcdConfigData := etcd.EtcdNodeConfigData{
			Name:                     node.GetName(),
			DataDir:                  filepath.Join(clusterConfig.Spec.Etcd.DataDirBase, "etcd"), // e.g. /var/lib/kubexm/etcd
			WalDir:                   filepath.Join(clusterConfig.Spec.Etcd.DataDirBase, "etcd", "wal"),
			ListenPeerURLs:           fmt.Sprintf("https://%s:%d,https://127.0.0.1:%d", nodeIP, clusterConfig.Spec.Etcd.PeerPort, clusterConfig.Spec.Etcd.PeerPort),
			ListenClientURLs:         fmt.Sprintf("https://%s:%d,https://127.0.0.1:%d", nodeIP, clusterConfig.Spec.Etcd.ClientPort, clusterConfig.Spec.Etcd.ClientPort),
			InitialAdvertisePeerURLs: fmt.Sprintf("https://%s:%d", nodeIP, clusterConfig.Spec.Etcd.PeerPort),
			AdvertiseClientURLs:      fmt.Sprintf("https://%s:%d", nodeIP, clusterConfig.Spec.Etcd.ClientPort),
			InitialCluster:           initialClusterString,
			InitialClusterToken:      clusterConfig.Spec.Etcd.ClusterToken,
			TrustedCAFile:            filepath.Join("/etc/etcd/pki", "ca.pem"),
			CertFile:                 filepath.Join("/etc/etcd/pki", "server.pem"),
			KeyFile:                  filepath.Join("/etc/etcd/pki", "server-key.pem"),
			PeerTrustedCAFile:        filepath.Join("/etc/etcd/pki", "ca.pem"),
			PeerCertFile:             filepath.Join("/etc/etcd/pki", "peer.pem"),
			PeerKeyFile:              filepath.Join("/etc/etcd/pki", "peer-key.pem"),
			// Fill other fields like SnapshotCount, AutoCompactionRetention from clusterConfig.Spec.Etcd if available
		}
		if i == 0 { // First node
			etcdConfigData.InitialClusterState = "new"
		} else {
			etcdConfigData.InitialClusterState = "existing"
		}

		generateConfigStep := etcd.NewGenerateEtcdConfigStep(nodePrefix+"GenerateConfig", etcdConfigData, "", true)
		generateConfigNodeID := graph.AddNode(nodePrefix+"generate-config", []connector.Host{node}, generateConfigStep)
		// Depends on certs being distributed and binaries copied (though not strictly, config could be generated earlier)
		// Let's make it depend on cert distribution for this node and binary copy.
        certNodeForThisHost := plan.NodeID(fmt.Sprintf("distribute-etcd-certs-%s", strings.ReplaceAll(node.GetName(),".","-")))
		graph.Nodes[generateConfigNodeID].Dependencies = append(graph.Nodes[generateConfigNodeID].Dependencies, lastStepNodeIDsOnEachNode[node.GetName()], certNodeForThisHost)
		lastStepNodeIDsOnEachNode[node.GetName()] = generateConfigNodeID

		// Step: Generate etcd.service
		etcdServiceData := etcd.EtcdServiceData{ExecStart: "/usr/local/bin/etcd --config-file=" + etcd.EtcdConfigRemotePath} // Customize as needed
		generateServiceFileStep := etcd.NewGenerateEtcdServiceStep(nodePrefix+"GenerateServiceFile", etcdServiceData, "", true)
		generateServiceNodeID := graph.AddNode(nodePrefix+"generate-service", []connector.Host{node}, generateServiceFileStep)
		graph.Nodes[generateServiceNodeID].Dependencies = append(graph.Nodes[generateServiceNodeID].Dependencies, lastStepNodeIDsOnEachNode[node.GetName()])
		lastStepNodeIDsOnEachNode[node.GetName()] = generateServiceNodeID

		// Step: Daemon Reload
		daemonReloadStep := etcd.NewManageEtcdServiceStep(nodePrefix+"DaemonReload", etcd.ActionDaemonReload, "etcd", true)
		daemonReloadNodeID := graph.AddNode(nodePrefix+"daemon-reload", []connector.Host{node}, daemonReloadStep)
		graph.Nodes[daemonReloadNodeID].Dependencies = append(graph.Nodes[daemonReloadNodeID].Dependencies, lastStepNodeIDsOnEachNode[node.GetName()])
		lastStepNodeIDsOnEachNode[node.GetName()] = daemonReloadNodeID

		// Step: Bootstrap (first node) or Join (subsequent nodes)
		if i == 0 {
			bootstrapStep := etcd.NewBootstrapFirstEtcdNodeStep(nodePrefix+"Bootstrap", "etcd", true)
			bootstrapNodeID := graph.AddNode(nodePrefix+"bootstrap", []connector.Host{node}, bootstrapStep)
			graph.Nodes[bootstrapNodeID].Dependencies = append(graph.Nodes[bootstrapNodeID].Dependencies, lastStepNodeIDsOnEachNode[node.GetName()])
			lastStepNodeIDsOnEachNode[node.GetName()] = bootstrapNodeID
		} else {
			joinStep := etcd.NewJoinEtcdNodeStep(nodePrefix+"Join", "etcd", true)
			joinNodeID := graph.AddNode(nodePrefix+"join", []connector.Host{node}, joinStep)
			graph.Nodes[joinNodeID].Dependencies = append(graph.Nodes[joinNodeID].Dependencies, lastStepNodeIDsOnEachNode[node.GetName()])
			// Joining nodes might need to wait for the first node to be up.
			// This inter-node dependency is complex for a single task's fragment if not all nodes are targets of all steps.
			// For now, join steps depend only on their own node's previous step.
			// True cluster join logic might require a sync point or for the bootstrap node to output a "ready" signal (cache key).
			firstNodeBootstrapID := plan.NodeID(fmt.Sprintf("etcd-%s-bootstrap", strings.ReplaceAll(etcdNodes[0].GetName(), ".", "-")))
			graph.Nodes[joinNodeID].Dependencies = append(graph.Nodes[joinNodeID].Dependencies, firstNodeBootstrapID)
			lastStepNodeIDsOnEachNode[node.GetName()] = joinNodeID
		}
	}

	// Set fragment entry and exit nodes
	// Entry: typically the first nodes of parallel branches (e.g., ensureEtcdArchivePlan's entry, cert distribution's entry)
	// Exit: typically the last nodes of each etcd member's setup (bootstrap/join)
	graph.EntryNodes = append(ensureEtcdArchivePlan.EntryNodes, distributeCertsNodeID...) // Assuming certs can start with archive download
    for _, node := range etcdNodes {
        graph.ExitNodes = append(graph.ExitNodes, lastStepNodeIDsOnEachNode[node.GetName()])
    }
    graph.RemoveDuplicateNodeIDs()


	logger.Info("ETCD installation plan generated.")
	return graph, nil
}

// Ensure InstallETCDTask implements task.Task.
var _ task.Task = &InstallETCDTask{}
