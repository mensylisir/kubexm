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

const (
	// DefaultEtcdPkiDir is the default remote directory for etcd PKI files.
	DefaultEtcdPkiDir = "/etc/etcd/pki"
	// DefaultEtcdServicePath is the default path for the etcd systemd service file.
	DefaultEtcdServicePath = "/etc/systemd/system/etcd.service"
	// DefaultEtcdUsrLocalBin is the default path for etcd binaries.
	DefaultEtcdUsrLocalBin = "/usr/local/bin"
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

	// resourcePrepFragment contains nodes (e.g., DownloadFileStep, ExtractArchiveStep if handle does that)
	// that run on the controlNode to make the etcd archive available.
	resourcePrepFragment, err := etcdArchiveResourceHandle.EnsurePlan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to plan etcd archive acquisition: %w", err)
	}
	// localEtcdArchivePathOnControlNode is the path to the downloaded .tar.gz file on the control node.
	localEtcdArchivePathOnControlNode, err := etcdArchiveResourceHandle.Path(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get local path for etcd archive from resource handle: %w", err)
	}
	// archiveInternalDirName is the name of the directory that's typically at the root of the etcd archive.
	// e.g., "etcd-v3.5.13-linux-amd64". This is used to construct paths to binaries within the extracted archive.
	archiveInternalDirName := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(localEtcdArchivePathOnControlNode), ".tar.gz"), ".tar")


	mainFragment := task.NewExecutionFragment(t.Name())
	if err := mainFragment.MergeFragment(resourcePrepFragment); err != nil {
		// Merging nodes from resourcePrepFragment into the main task fragment.
		return nil, fmt.Errorf("failed to merge resource prep fragment for etcd: %w", err)
	}

	// lastCtrlNodeActivityID tracks the last operation on the control node (e.g., archive download/extraction).
	// Subsequent per-host operations (like uploading this archive) will depend on this.
	lastCtrlNodeActivityID := plan.NodeID("")
	if len(resourcePrepFragment.ExitNodes) > 0 {
		lastCtrlNodeActivityID = resourcePrepFragment.ExitNodes[0]
	}

	// --- 2. PKI Certificate Distribution ---
	// This section plans the distribution of pre-generated etcd certificates
	// from the control node to all relevant target hosts (etcd nodes and master nodes).
	// It assumes certificates were generated by a prior PKI task and are available
	// in standard locations on the control node (e.g., ctx.GetEtcdCertsDir()).
	pkiDistributionSubFragment := task.NewExecutionFragment("DistributeEtcdPKI")
	var pkiDistributionFragmentExitNodes []plan.NodeID // Tracks the last cert upload node for each host receiving certs.

	allCertReceivingHostsMap := make(map[string]connector.Host)
	for _, h := range etcdNodes { allCertReceivingHostsMap[h.GetName()] = h }
	for _, h := range masterNodes { allCertReceivingHostsMap[h.GetName()] = h }

	etcdCertsBaseDirLocal := ctx.GetEtcdCertsDir() // Base directory for certs on control node.

	// All PKI upload operations depend on the etcd archive being ready on the control node.
	pkiNodeInitialDeps := []plan.NodeID{}
	if lastCtrlNodeActivityID != "" { pkiNodeInitialDeps = append(pkiNodeInitialDeps, lastCtrlNodeActivityID) }

	for _, targetHost := range allCertReceivingHostsMap {
		hostPkiPrefix := fmt.Sprintf("upload-pki-%s-", strings.ReplaceAll(targetHost.GetName(), ".", "-"))
		var lastCertUploadedForThisHost plan.NodeID // Tracks the last cert upload for *this specific host*.

		// Define common certs (like ca.pem) and their remote paths/permissions.
		commonCerts := map[string]string{"ca.pem": "0644"}
		for certFile, perm := range commonCerts {
			localPath := filepath.Join(etcdCertsBaseDirLocal, certFile)
			remotePath := filepath.Join(DefaultEtcdPkiDir, certFile) // e.g., /etc/etcd/pki/ca.pem
			nodeIDStr := hostPkiPrefix + strings.ReplaceAll(certFile, ".", "_")

			uploadNode := common.NewUploadFileStep(
				fmt.Sprintf("Upload-%s-to-%s", certFile, targetHost.GetName()), // Instance name for the step
				localPath, remotePath, perm, true, false, // sudo=true, allowMissingSrc=false
			)
			nodeID, _ := pkiDistributionSubFragment.AddNode(&plan.ExecutionNode{
				Name:     uploadNode.Meta().Name, // Use descriptive name from step
				Step:     uploadNode,
				Hosts:    []connector.Host{targetHost},
				Dependencies: pkiNodeInitialDeps, // Depends on control node prep
			}, plan.NodeID(nodeIDStr))
			lastCertUploadedForThisHost = nodeID
		}

		// Upload etcd-node-specific certificates (server.pem, server-key.pem, peer.pem, peer-key.pem)
		if isHostInRole(targetHost, etcdNodes) {
			etcdNodeCerts := map[string]string{ // filename -> permissions
				fmt.Sprintf("%s.pem", targetHost.GetName()):       "0644",
				fmt.Sprintf("%s-key.pem", targetHost.GetName()):   "0600",
				fmt.Sprintf("peer-%s.pem", targetHost.GetName()):  "0644",
				fmt.Sprintf("peer-%s-key.pem", targetHost.GetName()): "0600",
			}
			currentDep := lastCertUploadedForThisHost // Subsequent certs for this host depend on the previous one for this host
			for certFile, perm := range etcdNodeCerts {
				localPath := filepath.Join(etcdCertsBaseDirLocal, certFile)
				remotePath := filepath.Join(DefaultEtcdPkiDir, certFile)
				nodeIDStr := hostPkiPrefix + strings.ReplaceAll(certFile, ".", "_")
				uploadNode := common.NewUploadFileStep("", localPath, remotePath, perm, true, false)
				nodeID, _ := pkiDistributionSubFragment.AddNode(&plan.ExecutionNode{
					Name:     uploadNode.Meta().Name,
					Step:     uploadNode,
					Hosts:    []connector.Host{targetHost},
					Dependencies: []plan.NodeID{currentDep}, // Depends on previous cert for this host
				}, plan.NodeID(nodeIDStr))
				currentDep = nodeID
				lastCertUploadedForThisHost = nodeID
			}
		}

		// Upload master-node-specific certificates (apiserver-etcd-client.pem, apiserver-etcd-client-key.pem)
		if isHostInRole(targetHost, masterNodes) {
			masterCerts := map[string]string{
				"apiserver-etcd-client.pem":     "0644",
				"apiserver-etcd-client-key.pem": "0600",
			}
			currentDep := lastCertUploadedForThisHost // Depends on previous cert for this host (could be CA or etcd-specific)
			for certFile, perm := range masterCerts {
				localPath := filepath.Join(etcdCertsBaseDirLocal, certFile)
				remotePath := filepath.Join(DefaultEtcdPkiDir, certFile) // Masters also store in /etc/etcd/pki for apiserver
				nodeIDStr := hostPkiPrefix + "apiserver_client_" + strings.ReplaceAll(certFile, ".", "_")
				uploadNode := common.NewUploadFileStep("", localPath, remotePath, perm, true, false)
				nodeID, _ := pkiDistributionSubFragment.AddNode(&plan.ExecutionNode{
					Name:     uploadNode.Meta().Name,
					Step:     uploadNode,
					Hosts:    []connector.Host{targetHost},
					Dependencies: []plan.NodeID{currentDep},
				}, plan.NodeID(nodeIDStr))
				currentDep = nodeID
				lastCertUploadedForThisHost = nodeID
			}
		}
		// Collect the very last PKI-related node for this host as an exit point for this host's PKI setup.
		if lastCertUploadedForThisHost != "" {
			pkiDistributionFragmentExitNodes = append(pkiDistributionFragmentExitNodes, lastCertUploadedForThisHost)
		}
	}
	pkiDistributionSubFragment.CalculateEntryAndExitNodes() // Calculate internal entry/exit for this sub-fragment
	if err := mainFragment.MergeFragment(pkiDistributionSubFragment); err != nil {
		return nil, fmt.Errorf("failed to merge PKI distribution sub-fragment: %w", err)
	}

	// --- 3. Orchestrate Installation Steps per ETCD Node ---
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

		uploadArchiveStep := common.NewUploadFileStep(
			fmt.Sprintf("UploadEtcdArchiveTo-%s", etcdHost.GetName()),
			localEtcdArchivePathOnControlNode, remoteEtcdArchivePathOnHost, "0644", true, false,
		)
		uploadArchiveNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
			Name:     uploadArchiveStep.Meta().Name, Step: uploadArchiveStep,
			Hosts:    []connector.Host{etcdHost}, Dependencies: currentHostProcessingDeps,
		}, plan.NodeID(nodeSpecificPrefix+"upload-archive"))
		currentHostLastStepID := uploadArchiveNodeID // Update current dependency for this host's chain

		// Extract Etcd Archive on this etcdHost
		etcdExtractDirOnHost := "/opt/kubexm/etcd-extracted" // TODO: Make this configurable
		extractArchiveStep := common.NewExtractArchiveStep(
			fmt.Sprintf("ExtractEtcdArchiveOn-%s", etcdHost.GetName()),
			remoteEtcdArchivePathOnHost, etcdExtractDirOnHost,
			true, // removeArchiveAfterExtract
			true, // sudo for extraction
		)
		extractArchiveNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
			Name:     extractArchiveStep.Meta().Name, Step: extractArchiveStep,
			Hosts:    []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID},
		}, plan.NodeID(nodeSpecificPrefix+"extract-archive"))
		currentHostLastStepID = extractArchiveNodeID

		// Path to the directory *inside* the extraction that holds etcd/etcdctl binaries
		pathContainingBinariesOnNode := filepath.Join(etcdExtractDirOnHost, archiveInternalDirName)

		// Copy Binaries to System Path (e.g., /usr/local/bin)
		// Using CommandSteps for explicit control over copy and chmod.
		cmdCopyEtcd := fmt.Sprintf("cp -fp %s %s/etcd && chmod +x %s/etcd", filepath.Join(pathContainingBinariesOnNode, "etcd"), DefaultEtcdUsrLocalBin, DefaultEtcdUsrLocalBin)
		cmdCopyEtcdctl := fmt.Sprintf("cp -fp %s %s/etcdctl && chmod +x %s/etcdctl", filepath.Join(pathContainingBinariesOnNode, "etcdctl"), DefaultEtcdUsrLocalBin, DefaultEtcdUsrLocalBin)

		copyEtcdNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
			Name: fmt.Sprintf("CopyEtcdBinaryOn-%s", etcdHost.GetName()),
			Step: common.NewCommandStep("", cmdCopyEtcd, true, false, 0, nil, 0, "", false, 0, "", false),
			Hosts: []connector.Host{etcdHost}, Dependencies: []plan.NodeID{currentHostLastStepID},
		}, plan.NodeID(nodeSpecificPrefix+"copy-etcd"))

		copyEtcdctlNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
			Name: fmt.Sprintf("CopyEtcdctlBinaryOn-%s", etcdHost.GetName()),
			Step: common.NewCommandStep("", cmdCopyEtcdctl, true, false, 0, nil, 0, "", false, 0, "", false),
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
			TrustedCAFile:            filepath.Join(DefaultEtcdPkiDir, "ca.pem"),
			CertFile:                 filepath.Join(DefaultEtcdPkiDir, fmt.Sprintf("%s.pem", etcdHost.GetName())),
			KeyFile:                  filepath.Join(DefaultEtcdPkiDir, fmt.Sprintf("%s-key.pem", etcdHost.GetName())),
			PeerTrustedCAFile:        filepath.Join(DefaultEtcdPkiDir, "ca.pem"),
			PeerCertFile:             filepath.Join(DefaultEtcdPkiDir, fmt.Sprintf("peer-%s.pem", etcdHost.GetName())),
			PeerKeyFile:              filepath.Join(DefaultEtcdPkiDir, fmt.Sprintf("peer-%s-key.pem", etcdHost.GetName())),
			SnapshotCount:            fmt.Sprintf("%d", etcdSpec.GetSnapshotCount()), // Use getters for defaults
			AutoCompactionRetention:  fmt.Sprintf("%d", etcdSpec.GetAutoCompactionRetentionHours()),
			MaxRequestBytes:          fmt.Sprintf("%d", etcdSpec.GetMaxRequestBytes()),
			QuotaBackendBytes:        fmt.Sprintf("%d", etcdSpec.GetQuotaBackendBytes()),
		}
		if i == 0 { etcdConfigData.InitialClusterState = "new" } else { etcdConfigData.InitialClusterState = "existing" }

		generateConfigStep := etcd.NewGenerateEtcdConfigStep(
			fmt.Sprintf("GenerateEtcdConfig-%s", etcdHost.GetName()),
			etcdConfigData, etcd.EtcdConfigRemotePath, true, // sudo=true
		)
		generateConfigNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
			Name: generateConfigStep.Meta().Name, Step: generateConfigStep,
			Hosts: []connector.Host{etcdHost}, Dependencies: configAndServiceSetupDeps,
		}, plan.NodeID(nodeSpecificPrefix+"generate-config"))
		currentHostLastStepID = generateConfigNodeID

		// Generate etcd.service systemd file
		generateServiceStep := etcd.NewGenerateEtcdServiceStep(
			fmt.Sprintf("GenerateEtcdServiceFile-%s", etcdHost.GetName()),
			// TODO: EtcdServiceData may need more fields if template is complex (e.g. User, Group)
			etcd.EtcdServiceData{ExecStartArgs: "--config-file=" + etcd.EtcdConfigRemotePath},
			DefaultEtcdServicePath, true, // sudo=true
		)
		generateServiceNodeID, _ := mainFragment.AddNode(&plan.ExecutionNode{
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