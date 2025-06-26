package etcd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time" // Added for potential timeouts if not handled by steps

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/resource"
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step/etcd" // Specific etcd steps
	"github.com/mensylisir/kubexm/pkg/task"
	// "github.com/mensylisir/kubexm/pkg/runtime" // No longer used in signatures
)

	"github.com/mensylisir/kubexm/pkg/common" // Import common
)

// InstallETCDTask defines the task for installing ETCD cluster (binary deployment).
type InstallETCDTask struct {
	task.BaseTask
}

// NewInstallETCDTask creates a new InstallETCDTask.
func NewInstallETCDTask() task.Task {
	return &InstallETCDTask{
		BaseTask: task.NewBaseTask(
			"InstallETCDCluster",
			"Installs a new ETCD cluster (binary deployment).",
			[]string{v1alpha1.ETCDRole}, // This task primarily targets ETCD role nodes
			nil,                         // No specific host filter beyond role
			false,                       // Not critical to ignore error by default
		),
	}
}

// IsRequired checks if this task is required based on the cluster configuration.
func (t *InstallETCDTask) IsRequired(ctx task.TaskContext) (bool, error) { // Changed to task.TaskContext
	logger := ctx.GetLogger().With("task", t.Name())
	clusterConfig := ctx.GetClusterConfig()
	if clusterConfig.Spec.Etcd == nil {
		logger.Info("ETCD spec not configured, skipping ETCD installation task.")
		return false, nil
	}
	// This task specifically handles binary installation of etcd.
	if clusterConfig.Spec.Etcd.Type != v1alpha1.EtcdTypeKubeXM {
		logger.Info("ETCD installation type is not KubeXM (binary), skipping this task.", "type", clusterConfig.Spec.Etcd.Type)
		return false, nil
	}
	etcdNodes, _ := ctx.GetHostsByRole(v1alpha1.ETCDRole)
	if len(etcdNodes) == 0 {
		logger.Info("No ETCD role nodes defined, skipping ETCD installation task.")
		return false, nil
	}
	logger.Info("ETCD installation is required.")
	return true, nil
}

// Plan generates the execution plan (fragment) for installing ETCD.
func (t *InstallETCDTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) { // Changed to task.TaskContext
	logger := ctx.GetLogger().With("task", t.Name())
	logger.Info("Planning ETCD installation (binary type)...")

	clusterConfig := ctx.GetClusterConfig()
	etcdSpec := clusterConfig.Spec.Etcd

	etcdNodes, err := ctx.GetHostsByRole(v1alpha1.ETCDRole)
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd nodes for task %s: %w", t.Name(), err)
	}
	// masterNodes are needed for distributing apiserver-etcd-client certificates.
	masterNodes, _ := ctx.GetHostsByRole(v1alpha1.MasterRole) // Error can be ignored if no masters, cert part will be skipped for them.

	// --- 1. Resource Acquisition: Etcd Archive on Control Node ---
	etcdVersion := etcdSpec.Version
	if etcdVersion == "" {
		etcdVersion = "v3.5.13" // TODO: Define this default in a centralized constants or configuration defaults package.
		logger.Info("ETCD version not specified, using default.", "version", etcdVersion)
	}

	// Create a handle for the ETCD archive. The resource handle will manage
	// downloading this archive to the control node.
	etcdArchiveResourceHandle, err := resource.NewRemoteBinaryHandle(ctx, // Pass task.TaskContext
		"etcd",        // Component name for util.BinaryInfo
		etcdVersion,   // Resolved version
		etcdSpec.Arch, // Target architecture, can be empty for auto-detect by handle based on control node
		"",            // Target OS, can be empty for default (linux)
		"",            // BinaryNameInArchive: empty, so handle's Path() will point to the archive file itself.
		"",            // ExpectedChecksum (TODO: populate from a reliable source)
		"",            // ChecksumAlgorithm
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd archive resource handle: %w", err)
	}

	// resourcePrepFragment contains steps to download/extract etcd locally on the control node.
	resourcePrepFragment, err := etcdArchiveResourceHandle.EnsurePlan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan etcd archive acquisition: %w", err)
	}
	// localEtcdArchivePathOnControlNode is the path to the downloaded .tar.gz file on the control node.
	// Note: etcdArchiveResourceHandle.Path() now gives the path to the *final primary artifact*
	// For an archive where no specific BinaryNameInArchive was set, this Path() would give the *extracted directory*.
	// If we need the archive path itself for upload, we might need a specific method from the handle,
	// or the handle's BinaryInfo.FilePath.
	// For this task, we need to upload the ARCHIVE, then extract on remote.
	// So, we need the path to the *archive file* on the control node.
	// The current `resource.NewRemoteBinaryHandle` with empty BinaryNameInArchive will have `Path` point to the archive itself
	// if the `RemoteBinaryHandle.Path` is adjusted or if we use `binaryInfo.FilePath` from the handle.
	// Let's assume `etcdArchiveResourceHandle.binaryInfo.FilePath` gives the local archive path.
	// This detail depends on the exact implementation of RemoteBinaryHandle's Path() and its internal binaryInfo.

	// To be precise with the "local-first" model:
	// 1. Resource Handle ensures `etcd-vX.Y.Z-linux-arch.tar.gz` is in `$(pwd)/.kubexm/cluster/etcd/vX.Y.Z/arch/`
	// 2. Resource Handle also ensures it's extracted locally, e.g. to `.../arch/extracted_etcd-vX.Y.Z.../`
	//    and `handle.Path()` might point to `.../extracted_etcd.../etcd` (the binary).
	// However, this task needs to upload the *archive* and extract remotely.
	// So, the resource handle for "etcd-archive" should have `Path()` return the local archive path.
	// And a *separate* resource handle for "etcd-binary" (if needed directly by control node) would point to the extracted binary.

	// For InstallETCDTask (binary type), the flow is:
	// 1. Ensure ETCD archive is on control node (via etcdArchiveResourceHandle.EnsurePlan).
	//    `localEtcdArchivePathOnControlNode` should be this path.
	// 2. PKI generation/preparation on control node (by a separate PKITask, results in local cert paths).
	// 3. This task then:
	//    a. Uploads the ETCD archive to each etcdNode.
	//    b. Uploads relevant certs to each etcdNode and masterNode.
	//    c. On each etcdNode: extracts archive, copies binaries, generates config, generates service, starts service.

	// Let's refine the resource handle usage:
	// The `etcdArchiveResourceHandle` as defined (empty BinaryNameInArchive) via `NewRemoteBinaryHandle`
	// will have its `Path(ctx)` return the *extraction directory* if `IsArchive` is true and `BinaryNameInArchive` is empty.
	// To get the archive path itself, we should use `etcdArchiveResourceHandle.binaryInfo.FilePath`.
	// This is a bit of an internal detail. A cleaner way might be for the Handle interface
	// to have `GetDownloadArtifactPath()` and `GetPrimaryArtifactPath()`.
	// For now, we'll assume `etcdArchiveResourceHandle.binaryInfo.FilePath` is accessible or a similar method exists.
	// The `resource.NewRemoteBinaryHandle`'s `Path` method was updated to return extraction dir if BinaryNameInArchive is empty.
	// We need the *archive path* for upload.
	// A quick fix: create the handle such that its `Path` returns the archive path.
	// This means `BinaryNameInArchive` for `etcdArchiveResourceHandle` should effectively be the archive's own filename,
	// or `IsArchive` should be treated specially by `Path`.
	// Let's assume we get `binaryInfo` from the handle to get the archive path for upload.
	if etcdArchiveResourceHandle == nil || etcdArchiveResourceHandle.(*resource.RemoteBinaryHandle).BinaryInfo() == nil { // Type assertion to access BinaryInfo
		return nil, fmt.Errorf("internal error: etcdArchiveResourceHandle or its binaryInfo is nil")
	}
	localEtcdArchivePathOnControlNode := etcdArchiveResourceHandle.(*resource.RemoteBinaryHandle).BinaryInfo().FilePath


	archiveInternalDirName := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(localEtcdArchivePathOnControlNode), ".tar.gz"), ".tar")

	// This fragment will contain all steps for this task.
	taskFragment := task.NewExecutionFragment(t.Name())
	// Merge the local resource preparation steps first.
	if err := taskFragment.MergeFragment(resourcePrepFragment); err != nil {
		return nil, fmt.Errorf("failed to merge etcd archive resource prep fragment: %w", err)
	}
	// All subsequent remote operations will depend on the exit nodes of resourcePrepFragment.
	controlNodePrepDoneDependencies := resourcePrepFragment.ExitNodes
	if len(controlNodePrepDoneDependencies) == 0 && len(resourcePrepFragment.Nodes) > 0 {
		// If prep fragment had nodes but no explicit exit nodes (e.g. single node fragment), use all its nodes as deps.
		for id := range resourcePrepFragment.Nodes { controlNodePrepDoneDependencies = append(controlNodePrepDoneDependencies, id)}
	}


	// --- PKI Generation & Distribution ---
	// This task assumes PKI generation happens on the control node.
	// A dedicated PKI generation task (e.g., GenerateEtcdPkiTask from etcd/generate_pki.go)
	// should run before this task, populating localEtcdCertsBaseDir.
	// For this task, we focus on *distributing* those certs.

	localEtcdCertsBaseDir := ctx.GetEtcdCertsDir() // Path on control node where PKI task saved certs.

	// Create a sub-fragment for all certificate uploads.
	// These uploads can happen in parallel for different hosts, and for different certs on the same host after CA.
	certsUploadFragment := task.NewExecutionFragment("UploadEtcdCertificates")
	var allCertUploadExitNodes []plan.NodeID


	for _, targetHost := range etcdNodes { // Certs for etcd nodes
		hostPkiPrefix := fmt.Sprintf("upload-etcd-certs-%s-", targetHost.GetName())
		certsForEtcdNode := map[string]string{ // cert_file_name_on_control_node -> remote_permissions
			"ca.pem":                          "0644",
			fmt.Sprintf("%s.pem", targetHost.GetName()):       "0644", // server cert
			fmt.Sprintf("%s-key.pem", targetHost.GetName()):   "0600", // server key
			fmt.Sprintf("peer-%s.pem", targetHost.GetName()):  "0644", // peer cert
			fmt.Sprintf("peer-%s-key.pem", targetHost.GetName()): "0600", // peer key
		}
		var lastUploadOnHost plan.NodeID
		for certFile, perm := range certsForEtcdNode {
			localPath := filepath.Join(localEtcdCertsBaseDir, certFile)
			remotePath := filepath.Join(common.EtcdDefaultPKIDir, certFile) // Use common constant
			nodeName := fmt.Sprintf("Upload-%s-to-%s", certFile, targetHost.GetName())
			uploadStep := commonstep.NewUploadFileStep(nodeName, localPath, remotePath, perm, true, false) // sudo=true, commonstep alias

			nodeID, _ := certsUploadFragment.AddNode(&plan.ExecutionNode{
				Name:         uploadStep.Meta().Name,
				Step:         uploadStep,
				Hosts:        []connector.Host{targetHost},
				Dependencies: controlNodePrepDoneDependencies, // Depends on local PKI generation being notionally complete
			})
			if lastUploadOnHost != "" { // Chain uploads for the same host for simplicity, though could be parallel
				certsUploadFragment.AddDependency(lastUploadOnHost, nodeID)
			}
			lastUploadOnHost = nodeID
		}
		if lastUploadOnHost != "" {
			allCertUploadExitNodes = append(allCertUploadExitNodes, lastUploadOnHost)
		}
	}

	for _, targetHost := range masterNodes { // Certs for master nodes (apiserver client)
		// Avoid re-uploading CA if master is also an etcd node (handled by map key uniqueness if merging nodes directly)
		hostPkiPrefix := fmt.Sprintf("upload-apiserver-etcd-client-certs-%s-", targetHost.GetName())
		certsForMasterNode := map[string]string{
			"ca.pem":                          "0644", // Ensure CA is on masters too
			"apiserver-etcd-client.pem":       "0644",
			"apiserver-etcd-client-key.pem": "0600",
		}
		var lastUploadOnHost plan.NodeID
		// Check if CA was already uploaded if this master is also an etcd node.
		// This requires a bit more complex tracking or ensuring UploadFileStep is idempotent.
		// For simplicity, we might re-upload CA or rely on UploadFileStep's precheck.

		for certFile, perm := range certsForMasterNode {
			// Skip CA if already planned for this host as an etcd node (more robust check needed if merging nodes)
			isEtcdNode := false
			for _, en := range etcdNodes { if en.GetName() == targetHost.GetName() { isEtcdNode = true; break } }
			if certFile == "ca.pem" && isEtcdNode {
				// Find the existing CA upload node for this host and use it as dependency
				// This logic is complex here. Simpler: let UploadStep's Precheck handle existing files.
				// Or, ensure PKI generation task creates a single set of certs locally, and this task just uploads.
			}

			localPath := filepath.Join(localEtcdCertsBaseDir, certFile)
			remotePath := filepath.Join(DefaultEtcdPkiDir, certFile) // Masters might store etcd client certs in a similar path
			nodeName := fmt.Sprintf("Upload-APIServerEtcdClient-%s-to-%s", certFile, targetHost.GetName())
			uploadStep := common.NewUploadFileStep(nodeName, localPath, remotePath, perm, true, false)

			nodeID, _ := certsUploadFragment.AddNode(&plan.ExecutionNode{
				Name:         uploadStep.Meta().Name,
				Step:         uploadStep,
				Hosts:        []connector.Host{targetHost},
				Dependencies: controlNodePrepDoneDependencies,
			})
			if lastUploadOnHost != "" {
				certsUploadFragment.AddDependency(lastUploadOnHost, nodeID)
			}
			lastUploadOnHost = nodeID
		}
		if lastUploadOnHost != "" {
			allCertUploadExitNodes = append(allCertUploadExitNodes, lastUploadOnHost)
		}
	}
	certsUploadFragment.CalculateEntryAndExitNodes()
	if err := taskFragment.MergeFragment(certsUploadFragment); err != nil {
		return nil, fmt.Errorf("failed to merge PKI distribution fragment: %w", err)
	}
	// All subsequent remote etcd setup steps will depend on these cert uploads being done *for that specific host*.

	// --- Per-Host ETCD Installation ---
	initialClusterPeers := []string{}
	etcdClientPort := etcdSpec.GetClientPort()      // Uses getter for default port if not set
	etcdPeerPortResolved := etcdSpec.GetPeerPort() // Uses getter

	for _, node := range etcdNodes {
		// TODO: For InitialCluster, it's crucial to use the correct peer IP.
		// This might be node.InternalAddress if defined and different, or an IP from node facts
		// that is routable within the cluster for peer communication.
		// Defaulting to node.GetAddress() for now.
		nodePeerAddr := node.GetAddress()
		initialClusterPeers = append(initialClusterPeers, fmt.Sprintf("%s=https://%s:%d", node.GetName(), nodePeerAddr, etcdPeerPortResolved))
	}
	initialClusterString := strings.Join(initialClusterPeers, ",")

	var allEtcdNodeServiceReadyNodeIDs []plan.NodeID // Collects final operational node for each etcd host
	var firstEtcdNodeBootstrapSuccessNodeID plan.NodeID // Tracks the bootstrap success of the first etcd node

	for i, etcdHost := range etcdNodes {
		nodeSpecificPrefix := fmt.Sprintf("etcd-%s-", strings.ReplaceAll(etcdHost.GetName(), ".", "-"))

		// Base dependencies for this host's etcd setup:
		// 1. Etcd archive ready on control node (lastCtrlNodeActivityID)
		// 2. PKI certs for *this specific host* are uploaded.
		currentHostProcessingDeps := []plan.NodeID{}
		if lastCtrlNodeActivityID != "" {
			currentHostProcessingDeps = append(currentHostProcessingDeps, lastCtrlNodeActivityID)
		}
		// Find the last PKI upload node ID for this specific etcdHost from the pkiDistributionSubFragment
		thisHostLastPkiNodeID, foundPkiForHost := findLastPkiNodeForHost(pkiDistributionSubFragment, etcdHost.GetName(), etcdNodes, masterNodes)
		if foundPkiForHost {
			currentHostProcessingDeps = append(currentHostProcessingDeps, thisHostLastPkiNodeID)
		} else {
			logger.Warn("Could not determine specific PKI dependency node for etcd host, etcd setup might fail or use incorrect certs.", "host", etcdHost.GetName())
			// This is a potential issue. For robustness, one might make it depend on all pkiDistributionGlobalExitNodeIDs,
			// or ensure findLastPkiNodeForHost is reliable.
		}

		// Upload Etcd Archive to this etcdHost
		remoteTempArchiveDir := "/tmp/kubexm-etcd-archives" // TODO: Make this configurable or use a host-specific temp path
		remoteEtcdArchivePathOnHost := filepath.Join(remoteTempArchiveDir, filepath.Base(localEtcdArchivePathOnControlNode))

		uploadArchiveStep := commonstep.NewUploadFileStep( // commonstep alias
			fmt.Sprintf("UploadEtcdArchiveTo-%s", etcdHost.GetName()),
			localEtcdArchivePathOnControlNode, remoteEtcdArchivePathOnHost, "0644", true, false,
		)
		uploadArchiveNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{ // Use taskFragment
			Name:     uploadArchiveStep.Meta().Name, Step: uploadArchiveStep,
			Hosts:    []connector.Host{etcdHost}, Dependencies: currentHostProcessingDeps,
		}, plan.NodeID(nodeSpecificPrefix+"upload-archive"))
		currentHostLastStepID := uploadArchiveNodeID // Update current dependency for this host's chain

		// Extract Etcd Archive on this etcdHost
		etcdExtractDirOnHost := "/opt/kubexm/etcd-extracted" // TODO: Make this configurable
		extractArchiveStep := commonstep.NewExtractArchiveStep( // commonstep alias
			fmt.Sprintf("ExtractEtcdArchiveOn-%s", etcdHost.GetName()),
			remoteEtcdArchivePathOnHost, etcdExtractDirOnHost,
			true, // removeArchiveAfterExtract
			true, // sudo for extraction
		)
		extractArchiveNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{ // Use taskFragment
			Name:     extractArchiveStep.Meta().Name, Step: extractArchiveStep,
			Hosts:    []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID},
		}, plan.NodeID(nodeSpecificPrefix+"extract-archive"))
		currentHostLastStepID = extractArchiveNodeID

		// Path to the directory *inside* the extraction that holds etcd/etcdctl binaries
		pathContainingBinariesOnNode := filepath.Join(etcdExtractDirOnHost, archiveInternalDirName)

		// Copy Binaries to System Path (e.g., /usr/local/bin)
		// Using CommandSteps for explicit control over copy and chmod.
		cmdCopyEtcd := fmt.Sprintf("cp -fp %s %s/etcd && chmod +x %s/etcd", filepath.Join(pathContainingBinariesOnNode, "etcd"), common.EtcdDefaultBinDir, common.EtcdDefaultBinDir)
		cmdCopyEtcdctl := fmt.Sprintf("cp -fp %s %s/etcdctl && chmod +x %s/etcdctl", filepath.Join(pathContainingBinariesOnNode, "etcdctl"), common.EtcdDefaultBinDir, common.EtcdDefaultBinDir)

		copyEtcdNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{ // Use taskFragment
			Name: fmt.Sprintf("CopyEtcdBinaryOn-%s", etcdHost.GetName()),
			Step: commonstep.NewCommandStep("", cmdCopyEtcd, true, false, 0, nil, 0, "", false, 0, "", false), // commonstep alias
			Hosts: []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID},
		}, plan.NodeID(nodeSpecificPrefix+"copy-etcd"))

		copyEtcdctlNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{ // Use taskFragment
			Name: fmt.Sprintf("CopyEtcdctlBinaryOn-%s", etcdHost.GetName()),
			Step: commonstep.NewCommandStep("", cmdCopyEtcdctl, true, false, 0, nil, 0, "", false, 0, "", false), // commonstep alias
			Hosts: []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID}, // Can run parallel to etcd copy
		}, plan.NodeID(nodeSpecificPrefix+"copy-etcdctl"))

		// Next steps depend on both binaries being copied.
		configAndServiceSetupDeps := []plan.NodeID{copyEtcdNodeID, copyEtcdctlNodeID}
		// Also ensure PKI certs for this host are in place before generating config that uses them.
		if foundPkiForHost { // If we successfully identified the last PKI node for this host
			configAndServiceSetupDeps = append(configAndServiceSetupDeps, thisHostLastPkiNodeID)
		}


		// Generate etcd.yaml configuration file
		nodeIP := etcdHost.GetAddress() // TODO: Use internal IP for listen/advertise if available and configured
		etcdConfigData := etcd.EtcdNodeConfigData{
			Name:                     etcdHost.GetName(), DataDir: etcdSpec.GetDataDir(),
			ListenPeerURLs:           fmt.Sprintf("https://%s:%d,https://127.0.0.1:%d", nodeIP, etcdPeerPortResolved, etcdPeerPortResolved),
			ListenClientURLs:         fmt.Sprintf("https://%s:%d,https://127.0.0.1:%d", nodeIP, etcdClientPort, etcdClientPort),
			InitialAdvertisePeerURLs: fmt.Sprintf("https://%s:%d", nodeIP, etcdPeerPortResolved),
			AdvertiseClientURLs:      fmt.Sprintf("https://%s:%d", nodeIP, etcdClientPort),
			InitialCluster:           initialClusterString, InitialClusterToken: etcdSpec.ClusterToken,
			TrustedCAFile:            filepath.Join(common.EtcdDefaultPKIDir, common.CACertFileName), // Use common constants
			CertFile:                 filepath.Join(common.EtcdDefaultPKIDir, fmt.Sprintf("%s.pem", etcdHost.GetName())),
			KeyFile:                  filepath.Join(common.EtcdDefaultPKIDir, fmt.Sprintf("%s-key.pem", etcdHost.GetName())),
			PeerTrustedCAFile:        filepath.Join(common.EtcdDefaultPKIDir, common.CACertFileName), // Use common constants
			PeerCertFile:             filepath.Join(common.EtcdDefaultPKIDir, fmt.Sprintf("peer-%s.pem", etcdHost.GetName())),
			PeerKeyFile:              filepath.Join(common.EtcdDefaultPKIDir, fmt.Sprintf("peer-%s-key.pem", etcdHost.GetName())),
			SnapshotCount:            fmt.Sprintf("%d", etcdSpec.GetSnapshotCount()), // Use getters for defaults
			AutoCompactionRetention:  fmt.Sprintf("%d", etcdSpec.GetAutoCompactionRetentionHours()),
			MaxRequestBytes:          fmt.Sprintf("%d", etcdSpec.GetMaxRequestBytes()),
			QuotaBackendBytes:        fmt.Sprintf("%d", etcdSpec.GetQuotaBackendBytes()),
		}
		if i == 0 { etcdConfigData.InitialClusterState = "new" } else { etcdConfigData.InitialClusterState = "existing" }

		generateConfigStep := etcdstep.NewGenerateEtcdConfigStep( // etcdstep alias
			fmt.Sprintf("GenerateEtcdConfig-%s", etcdHost.GetName()),
			etcdConfigData, common.EtcdDefaultConfFile, true, // Use common constant for remote path
		)
		generateConfigNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{ // Use taskFragment
			Name: generateConfigStep.Meta().Name, Step: generateConfigStep,
			Hosts: []connector.Host{etcdHost}, Dependencies: configAndServiceSetupDeps,
		}, plan.NodeID(nodeSpecificPrefix+"generate-config"))
		currentHostLastStepID = generateConfigNodeID

		// Generate etcd.service systemd file
		generateServiceStep := etcdstep.NewGenerateEtcdServiceStep( // etcdstep alias
			fmt.Sprintf("GenerateEtcdServiceFile-%s", etcdHost.GetName()),
			// TODO: EtcdServiceData may need more fields if template is complex (e.g. User, Group)
			etcdstep.EtcdServiceData{ExecStartArgs: "--config-file=" + common.EtcdDefaultConfFile}, // Use common constant
			common.EtcdDefaultSystemdFile, true, // sudo=true, Use common constant
		)
		generateServiceNodeID, _ := taskFragment.AddNode(&plan.ExecutionNode{ // Use taskFragment
			Name: generateServiceStep.Meta().Name, Step: generateServiceStep,
			Hosts: []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID},
		}, plan.NodeID(nodeSpecificPrefix+"generate-service"))
		currentHostLastStepID = generateServiceNodeID

		// Systemctl Daemon Reload
		daemonReloadStep := etcd.NewManageEtcdServiceStep(
			fmt.Sprintf("DaemonReloadEtcd-%s", etcdHost.GetName()),
			etcd.ActionDaemonReload, "etcd", true, // sudo=true
		)
		daemonReloadNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
			Name: daemonReloadStep.Meta().Name, Step: daemonReloadStep,
			Hosts: []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID},
		}, plan.NodeID(nodeSpecificPrefix+"daemon-reload"))
		currentHostLastStepID = daemonReloadNodeID

		// Bootstrap (if first node) or Join (for subsequent nodes)
		if i == 0 {
			bootstrapStep := etcd.NewBootstrapFirstEtcdNodeStep(
				fmt.Sprintf("BootstrapEtcd-%s", etcdHost.GetName()), "etcd", true,
			)
			bootstrapNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
				Name: bootstrapStep.Meta().Name, Step: bootstrapStep,
				Hosts: []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID},
			}, plan.NodeID(nodeSpecificPrefix+"bootstrap"))
			currentHostLastStepID = bootstrapNodeID
			firstEtcdNodeBootstrapSuccessNodeID = currentHostLastStepID // Save this for other nodes to depend on
		} else {
			joinDeps := []plan.NodeID{currentHostLastStepID}
			if firstEtcdNodeBootstrapSuccessNodeID != "" { // Ensure first node has bootstrapped successfully
				joinDeps = append(joinDeps, firstEtcdNodeBootstrapSuccessNodeID)
			}
			joinStep := etcd.NewJoinEtcdNodeStep(
				fmt.Sprintf("JoinEtcd-%s", etcdHost.GetName()), "etcd", true,
			)
			joinNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
				Name: joinStep.Meta().Name, Step: joinStep,
				Hosts: []connector.Host{etcdHost}, Dependencies: joinDeps,
			}, plan.NodeID(nodeSpecificPrefix+"join"))
			currentHostLastStepID = joinNodeID
		}
		allEtcdNodeServiceReadyNodeIDs = append(allEtcdNodeServiceReadyNodeIDs, currentHostLastStepID)
	}

	mainFragment.CalculateEntryAndExitNodes() // Recalculate after all nodes and internal dependencies are added
	logger.Info("ETCD installation task planning complete.", "total_nodes", len(mainFragment.Nodes))
	return mainFragment, nil
}

// Helper function to check if a host name is in a slice of connector.Host
func stringSliceContains(slice []connector.Host, hostName string) bool {
	for _, h := range slice {
		if h.GetName() == hostName {
			return true
		}
	}
	return false
}

func isHostInRole(host connector.Host, roleHosts []connector.Host) bool {
    for _, rh := range roleHosts {
        if rh.GetName() == host.GetName() {
            return true
        }
    }
    return false
}

// findLastPkiNodeForHost attempts to find the NodeID of the last PKI-related operation for a given host
// within the pkiDistributionSubFragment. This is used to set fine-grained dependencies.
// Note: This helper relies on naming conventions for PKI nodes. A more robust system might involve
// the PKI fragment explicitly defining its exit nodes per host.
func findLastPkiNodeForHost(pkiFragment *task.ExecutionFragment, hostName string, etcdNodes, masterNodes []connector.Host) (plan.NodeID, bool) {
    // Determine the type of certs this host should receive as the "last" one in the sequence.
    var lastExpectedCertFileBaseName string
    hostIsEtcdNode := isHostInRole(connector.NewHostFromSpec(v1alpha1.HostSpec{Name: hostName}), etcdNodes)
    hostIsMasterNode := isHostInRole(connector.NewHostFromSpec(v1alpha1.HostSpec{Name: hostName}), masterNodes)

    if hostIsEtcdNode {
        // For etcd nodes, the peer key is typically among the last etcd-specific certs.
        lastExpectedCertFileBaseName = fmt.Sprintf("peer-%s-key.pem", hostName)
    } else if hostIsMasterNode {
        // For master nodes (that are not also etcd nodes), the apiserver client key for etcd is last.
        lastExpectedCertFileBaseName = "apiserver-etcd-client-key.pem"
    } else {
        // If the host is neither etcd nor master but received certs (e.g. just CA), CA is last.
        lastExpectedCertFileBaseName = "ca.pem"
    }

	// Construct the expected NodeID based on the naming convention used in the PKI distribution loop:
	// hostPkiPrefix + strings.ReplaceAll(certFile, ".", "_")
	// OR hostPkiPrefix + "apiserver_client_" + strings.ReplaceAll(certFile, ".", "_")

	nodeIDPrefixPart := fmt.Sprintf("upload-pki-%s-", strings.ReplaceAll(hostName, ".", "-"))
	var expectedNodeIDStr string

	if strings.HasPrefix(lastExpectedCertFileBaseName, "apiserver-etcd-client") {
		expectedNodeIDStr = nodeIDPrefixPart + "apiserver_client_" + strings.ReplaceAll(lastExpectedCertFileBaseName, ".", "_")
	} else {
		expectedNodeIDStr = nodeIDPrefixPart + strings.ReplaceAll(lastExpectedCertFileBaseName, ".", "_")
	}

	nodeID := plan.NodeID(expectedNodeIDStr)
	if _, exists := pkiFragment.Nodes[nodeID]; exists {
		return nodeID, true
	}

    // Fallback: if the specific "last" cert node isn't found (e.g. master that is also etcd node, where peer key was truly last),
    // try to find the CA cert upload node for this host, as that's a common baseline.
	if lastExpectedCertFileBaseName != "ca.pem" { // Avoid re-checking if CA was already the target
		caNodeID := plan.NodeID(nodeIDPrefixPart + strings.ReplaceAll("ca.pem", ".", "_"))
		if _, exists := pkiFragment.Nodes[caNodeID]; exists {
			return caNodeID, true
		}
	}

	return "", false // Indicate specific PKI node for this host not reliably found
}


// Ensure InstallETCDTask implements task.Task.
var _ task.Task = &InstallETCDTask{}