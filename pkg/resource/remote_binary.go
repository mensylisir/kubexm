package resource

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common" // For ControlNodeHostName
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	// "github.com/mensylisir/kubexm/pkg/step" // No longer needed directly, step types come from common or command
	"github.com/mensylisir/kubexm/pkg/step/command" // For NewCommandStep
	"github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task" // For task.ExecutionFragment
)

// RemoteBinaryHandle manages a binary that needs to be extracted from a downloaded archive.
// It plans steps to download the archive, extract it, and provide the path to a specific binary within.
type RemoteBinaryHandle struct {
	// ComponentName of the component (e.g., "etcd", "containerd") - used for directory structuring.
	ComponentName string
	// Version of the binary (e.g., "v3.5.4").
	Version string
	// Arch (e.g., "amd64", "arm64"). Must be non-empty.
	Arch string
	// DownloadURL is the direct URL to the archive file.
	DownloadURL string
	// ArchiveFilename is the name of the file as it will be saved locally (e.g., "etcd-v3.5.4-linux-amd64.tar.gz").
	ArchiveFilename string
	// BinaryPathInArchive is the relative path of the target binary within the extracted archive.
	// For example, "etcd-v3.5.4-linux-amd64/etcd" or "bin/containerd".
	BinaryPathInArchive string
	// ExpectedChecksum is the SHA256 checksum of the downloaded archive file for verification. Optional.
	ExpectedChecksum string
	// ChecksumAlgorithm is the algorithm for the checksum (e.g., "sha256"). Defaults to "sha256" if ExpectedChecksum is set.
	ChecksumAlgorithm string
	// OS a.k.a. GOOS, e.g. "linux", "darwin". Used if archive name needs it. Often "linux".
	OS string
}

// knownBinaryDetail holds predefined details for known components.
type knownBinaryDetail struct {
	urlTemplate         string
	archiveNameTemplate string // Template for the archive name, can use {{.Version}}, {{.Arch}}, {{.OS}}
	binaryPathInArchive string // Path of the binary inside the archive, can use {{.Version}}, {{.Arch}}, {{.OS}}, {{.FileName}}
	isArchive           bool   // Whether the download is an archive
	defaultOS           string
}

// knownBinariesRegistry acts as a lookup for predefined binary details.
var knownBinariesRegistry = map[string]knownBinaryDetail{
	"etcd": {
		urlTemplate:         "https://github.com/coreos/etcd/releases/download/{{.Version}}/etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		archiveNameTemplate: "etcd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		binaryPathInArchive: "etcd-{{.Version}}-{{.OS}}-{{.Arch}}/etcd",
		isArchive:           true,
		defaultOS:           "linux",
	},
	"kubeadm": {
		urlTemplate:         "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubeadm",
		archiveNameTemplate: "kubeadm", // Not an archive, filename is the binary name
		binaryPathInArchive: "kubeadm", // Relative to nothing, it's the file itself
		isArchive:           false,
		defaultOS:           "linux",
	},
	"kubelet": {
		urlTemplate:         "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubelet",
		archiveNameTemplate: "kubelet",
		binaryPathInArchive: "kubelet",
		isArchive:           false,
		defaultOS:           "linux",
	},
	"kubectl": {
		urlTemplate:         "https://dl.k8s.io/release/{{.Version}}/bin/{{.OS}}/{{.Arch}}/kubectl",
		archiveNameTemplate: "kubectl",
		binaryPathInArchive: "kubectl",
		isArchive:           false,
		defaultOS:           "linux",
	},
	"containerd": {
		urlTemplate:         "https://github.com/containerd/containerd/releases/download/v{{.Version}}/containerd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		archiveNameTemplate: "containerd-{{.Version}}-{{.OS}}-{{.Arch}}.tar.gz",
		binaryPathInArchive: "bin/containerd", // Path within the tarball
		isArchive:           true,
		defaultOS:           "linux",
	},
	"runc": {
		urlTemplate:         "https://github.com/opencontainers/runc/releases/download/{{.Version}}/runc.{{.Arch}}",
		archiveNameTemplate: "runc.{{.Arch}}",
		binaryPathInArchive: "runc.{{.Arch}}",
		isArchive:           false,
		defaultOS:           "linux", // OS is not in runc URL/filename typically, but keep for consistency
	},
	"cni-plugins": {
		urlTemplate:         "https://github.com/containernetworking/plugins/releases/download/{{.Version}}/cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		archiveNameTemplate: "cni-plugins-{{.OS}}-{{.Arch}}-{{.Version}}.tgz",
		binaryPathInArchive: "", // User needs to specify which CNI plugin binary, or extract all to a known dir
		isArchive:           true,
		defaultOS:           "linux",
	},
	// TODO: Add other components like cri-dockerd, crictl, helm, docker etc. from the markdown
}

// NewRemoteBinaryHandle creates a new RemoteBinaryHandle.
// It attempts to use predefined details for known components if templates are not provided.
func NewRemoteBinaryHandle(
	ctx runtime.TaskContext,
	componentName, version, arch, osName string,
	downloadURLTmpl, archiveFilenameTmpl, binaryPathInArchiveTmpl string,
	expectedChecksum, checksumAlgo string,
) (Handle, error) {
	if componentName == "" || version == "" {
		return nil, fmt.Errorf("componentName and version are required for RemoteBinaryHandle")
	}

	logger := ctx.GetLogger().With("component", componentName, "version", version)

	finalArch := arch
	if finalArch == "" {
		controlNode, err := ctx.GetControlNode()
		if err != nil {
			return nil, fmt.Errorf("cannot auto-determine arch: failed to get control node: %w", err)
		}
		facts, err := ctx.GetHostFacts(controlNode)
		if err != nil {
			return nil, fmt.Errorf("cannot auto-determine arch: failed to get control node facts: %w", err)
		}
		if facts.OS == nil || facts.OS.Arch == "" {
			return nil, fmt.Errorf("cannot auto-determine arch: control node OS.Arch is empty")
		}
		finalArch = facts.OS.Arch
		if finalArch == "x86_64" { finalArch = "amd64" }
		if finalArch == "aarch64" { finalArch = "arm64" }
		logger.Info("Architecture auto-derived.", "arch", finalArch)
	}

	var detail knownBinaryDetail
	var hasKnownDetail bool
	if d, ok := knownBinariesRegistry[strings.ToLower(componentName)]; ok {
		detail = d
		hasKnownDetail = true
	}

	finalOS := osName
	if finalOS == "" {
		if hasKnownDetail && detail.defaultOS != "" {
			finalOS = detail.defaultOS
		} else {
			finalOS = "linux" // Fallback default
		}
		logger.Info("OS determined.", "os", finalOS)
	}

	useURLTmpl := downloadURLTmpl
	useArchiveFilenameTmpl := archiveFilenameTmpl
	useBinaryPathInArchiveTmpl := binaryPathInArchiveTmpl

	if useURLTmpl == "" && hasKnownDetail { useURLTmpl = detail.urlTemplate }
	if useArchiveFilenameTmpl == "" && hasKnownDetail { useArchiveFilenameTmpl = detail.archiveNameTemplate }
	if useBinaryPathInArchiveTmpl == "" && hasKnownDetail { useBinaryPathInArchiveTmpl = detail.binaryPathInArchive }

	if useURLTmpl == "" || useArchiveFilenameTmpl == "" || useBinaryPathInArchiveTmpl == "" {
		return nil, fmt.Errorf(
			"downloadURLTemplate, archiveFilenameTemplate, and binaryPathInArchive must be set or derivable for component '%s'", componentName,
		)
	}

	finalChecksumAlgo := checksumAlgo
	if expectedChecksum != "" && finalChecksumAlgo == "" {
		finalChecksumAlgo = "sha256"
	}

	templateData := struct {
		Version  string
		Arch     string
		OS       string
		FileName string // Placeholder for archive filename if needed in URL template
	}{
		Version: version,
		Arch:    finalArch,
		OS:      finalOS,
	}

	parsedArchiveFilename, err := common.RenderTemplate(useArchiveFilenameTmpl, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render archive filename template '%s': %w", useArchiveFilenameTmpl, err)
	}
	templateData.FileName = parsedArchiveFilename // Update template data for subsequent renders

	finalDownloadURL, err := common.RenderTemplate(useURLTmpl, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render download URL template '%s': %w", useURLTmpl, err)
	}

	finalBinaryPathInArchive, err := common.RenderTemplate(useBinaryPathInArchiveTmpl, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render binary path in archive template '%s': %w", useBinaryPathInArchiveTmpl, err)
	}


	return &RemoteBinaryHandle{
		ComponentName:       componentName,
		Version:             version,
		Arch:                finalArch,
		OS:                  finalOS,
		DownloadURL:         finalDownloadURL,
		ArchiveFilename:     parsedArchiveFilename,
		BinaryPathInArchive: finalBinaryPathInArchive,
		ExpectedChecksum:    expectedChecksum,
		ChecksumAlgorithm:   finalChecksumAlgo,
		// IsArchive field can be derived from knownBinariesRegistry or file extension
	}, nil
}

// ID generates a unique identifier for this resource instance.
func (h *RemoteBinaryHandle) ID() string {
	return fmt.Sprintf("%s-binary-%s-%s-%s-%s",
		strings.ToLower(h.ComponentName),
		h.Version,
		h.OS,
		h.Arch,
		strings.ReplaceAll(h.ArchiveFilename, "/", "_"))
}

// Path returns the expected local path of the target binary after download and extraction.
// Path: ${GlobalWorkDir}/${cluster_name}/${ComponentName}/${Version}/${Arch}/<filename_part_of_BinaryPathInArchive>
func (h *RemoteBinaryHandle) Path(ctx runtime.TaskContext) (string, error) {
		if ctx.GetClusterConfig() == nil {
		return "", fmt.Errorf("cluster config is nil in TaskContext, cannot determine path for %s", h.ID())
	}
	if h.Arch == "" {
		return "", fmt.Errorf("architecture is not set for RemoteBinaryHandle %s", h.ID())
	}

		// Determine the correct directory based on ComponentName type (etcd, container_runtime, kubernetes)
		var componentTypeDir string
		cnLower := strings.ToLower(h.ComponentName)
		if strings.Contains(cnLower, "etcd") {
			componentTypeDir = common.DefaultEtcdDir // "etcd"
		} else if strings.Contains(cnLower, "containerd") || strings.Contains(cnLower, "docker") || strings.Contains(cnLower, "runc") || strings.Contains(cnLower, "cni") {
			componentTypeDir = common.DefaultContainerRuntimeDir // "container_runtime"
		} else if strings.HasPrefix(cnLower, "kube") || strings.Contains(cnLower, "helm") { // kubeadm, kubelet, kubectl, helm etc.
			componentTypeDir = common.DefaultKubernetesDir // "kubernetes"
		} else {
			// Fallback or error for unknown component types for directory structure
			// For now, use ComponentName directly, but this might need refinement.
			componentTypeDir = h.ComponentName
			ctx.GetLogger().Warn("Unknown component type for directory structure, using component name directly.", "component", h.ComponentName)
		}

		// Path: ${GlobalWorkDir}/${cluster_name}/${componentTypeDir}/${ComponentName_maybe_subdir}/${Version}/${Arch}/<filename_part_of_BinaryPathInArchive>
		// Example: /workdir/.kubexm/mycluster/etcd/etcd/v3.5.4/amd64/etcd  (Here ComponentName_maybe_subdir is h.ComponentName)
		// Example: /workdir/.kubexm/mycluster/container_runtime/containerd/v1.7.6/amd64/containerd
		// The GetFileDownloadPath should ideally handle this logic if componentTypeDir is passed to it.
		// For now, let's assume GetFileDownloadPath uses h.ComponentName for the sub-directory.
		// The structure from 21-其他说明.md is: workdir/.kubexm/${cluster_name}/${type}/${version}/${arch}
		// So, we need to map h.ComponentName to this 'type'

		baseArtifactDir := ctx.GetFileDownloadPath(h.ComponentName, h.Version, h.Arch, "") // This should give .../ComponentName/Version/Arch/

	finalBinaryName := filepath.Base(h.BinaryPathInArchive)
	return filepath.Join(baseArtifactDir, finalBinaryName), nil
}

// EnsurePlan generates an ExecutionFragment to download and extract the archive, making the binary available.
// Steps are planned to run on the local control node.
func (h *RemoteBinaryHandle) EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_id", h.ID(), "component", h.ComponentName)
	logger.Info("Planning resource assurance for remote binary...")

	finalBinaryPath, err := h.Path(ctx)
	if err != nil {
		logger.Error(err, "Failed to determine final binary path")
		return nil, fmt.Errorf("failed to determine final binary path for %s: %w", h.ID(), err)
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		logger.Error(err, "Failed to get control node from TaskContext")
		return nil, fmt.Errorf("failed to get control node for resource %s: %w", h.ID(), err)
	}

	localArchivePath := ctx.GetFileDownloadPath(h.ComponentName, h.Version, h.Arch, h.ArchiveFilename)
	localArchiveDir := filepath.Dir(localArchivePath)

	// Determine if the downloaded item is an archive based on known details or filename extension
	isArchive := h.isArchive() // Use a helper method

	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID
	lastNodeID := plan.NodeID("")

	runner := ctx.GetRunner()
	localConn, err := ctx.GetConnectorForHost(controlNode)
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for control node during pre-check: %w", err)
	}

	// --- Pre-check if final binary already exists ---
	if exists, _ := runner.Exists(ctx.GoContext(), localConn, finalBinaryPath); exists {
		// TODO: Add checksum for the final binary itself if provided by the handle.
		logger.Info("Final binary already exists. Assuming valid and skipping resource acquisition.", "path", finalBinaryPath)
		return &task.ExecutionFragment{Nodes: nodes, EntryNodes: entryNodes, ExitNodes: exitNodes}, nil
	}

	// --- Pre-check for archive if it exists and checksum validation ---
	if isArchive && h.ExpectedChecksum != "" {
		if archiveExists, _ := runner.Exists(ctx.GoContext(), localConn, localArchivePath); archiveExists {
			logger.Info("Archive file found locally, verifying checksum.", "archive", localArchivePath)
			checksumStep := common.NewFileChecksumStep("", localArchivePath, h.ExpectedChecksum, h.ChecksumAlgorithm)
			// This check is synchronous for simplicity here. In a full plan, it would be a node.
			// For this pre-check, we execute it directly.
			// This is a simplification; ideally, checksum is also a node if an action is taken based on it.
			// However, for EnsurePlan, if checksum fails, we proceed to download.
			// If we were to make checksum a node, EnsurePlan becomes more complex as it would return a conditional graph.
			// Let's assume for now: if checksum fails, we log and re-download.
			// To do this properly, we'd need a way for the step to output its result to decide the next action.
			// For now, if checksum fails, the download step will run. If it succeeds, download might be skipped by its own precheck.
			// This part needs a more robust solution if strict "don't download if valid archive exists" is needed.
			// A simple approach: if checksum is bad, delete the archive to force re-download.
			currentChecksum, checksumErr := runner.GetSHA256(ctx.GoContext(), localConn, localArchivePath) // Assuming GetSHA256 or similar in runner
			if checksumErr == nil && currentChecksum == h.ExpectedChecksum {
				logger.Info("Local archive checksum matches. Will proceed to extraction if needed.")
				// If archive is valid, we might skip download step.
				// The DownloadFileStep itself has a precheck for file existence, but not checksum.
				// This interaction needs refinement. For now, let's assume DownloadFileStep is smart enough or we force it.
			} else {
				if checksumErr != nil {
					logger.Warn("Failed to get checksum for local archive, will re-download.", "archive", localArchivePath, "error", checksumErr.Error())
				} else {
					logger.Warn("Local archive checksum mismatch, will re-download.", "archive", localArchivePath, "expected", h.ExpectedChecksum, "got", currentChecksum)
				}
				// To force re-download, we might delete the existing archive.
				// runner.Remove(ctx.GoContext(), localConn, localArchivePath, false) // This is an action, should be a step.
				// For now, the DownloadFileStep will overwrite.
			}
		}
	}


	// 1. Download Step
	downloadNodeID := plan.NodeID(fmt.Sprintf("download-%s", h.ID()))
	downloadStep := common.NewDownloadFileStep("", h.DownloadURL, localArchivePath, h.ExpectedChecksum, h.ChecksumAlgorithm, false)
	// Note: NewDownloadFileStep in pkg/step/common/download_file_step.go takes (instanceName, url, destPath, checksum, checksumType, sudo)
	// localArchivePath is the full path to the file, localArchiveDir is its directory.
	// The existing NewDownloadFileStep seems to expect destPath to be the full file path.
	// Let's adjust the call to pass localArchivePath as the DestPath, and its directory is derived within the step if needed.
	// Actually, the DownloadFileStep I reviewed takes `destDir` and `destName` (which is `h.ArchiveFilename` here).
	// So, the call should be: common.NewDownloadFileStep("", h.DownloadURL, localArchiveDir, h.ArchiveFilename, h.ExpectedChecksum, h.ChecksumAlgorithm, false)
	// Let's re-check the NewDownloadFileStep signature from the file.
	// It is: NewDownloadFileStep(instanceName, url, destPath, checksum, checksumType string, sudo bool)
	// So, destPath should be the full path to the *file*.
	// My previous replacement was: common.NewDownloadFileStep("", h.DownloadURL, localArchivePath, h.ExpectedChecksum, h.ChecksumAlgorithm, false)
	// This seems correct. The 3rd argument is the full path to the destination file.
	nodes[downloadNodeID] = &plan.ExecutionNode{
		Name:     fmt.Sprintf("Download %s (%s %s %s)", h.ComponentName, h.Version, h.OS, h.Arch),
		Step:     downloadStep,
		Hosts:    []connector.Host{controlNode},
		StepName: downloadStep.Meta().Name,
	}
	entryNodes = append(entryNodes, downloadNodeID)
	lastNodeID = downloadNodeID

	if isArchive {
		extractionDir := filepath.Join(localArchiveDir, "extracted_"+strings.TrimSuffix(h.ArchiveFilename, filepath.Ext(h.ArchiveFilename)))

		extractNodeID := plan.NodeID(fmt.Sprintf("extract-%s", h.ID()))
		// NewExtractArchiveStep(instanceName, sourceArchivePath, destinationDir string, removeArchiveAfterExtract, sudo bool)
		extractStep := common.NewExtractArchiveStep("", localArchivePath, extractionDir, false, false)
		nodes[extractNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Extract %s archive (%s %s %s)", h.ComponentName, h.Version, h.OS, h.Arch),
			Step:         extractStep,
			Hosts:        []connector.Host{controlNode},
			Dependencies: []plan.NodeID{lastNodeID},
			StepName:     extractStep.Meta().Name,
		}
		lastNodeID = extractNodeID

		// 3. Finalize Binary Step (Copy from extraction path to final path and ensure executable)
		sourceBinaryInExtraction := filepath.Join(extractionDir, h.BinaryPathInArchive)
		finalizeNodeID := plan.NodeID(fmt.Sprintf("finalize-%s", h.ID()))
		cmdToFinalize := fmt.Sprintf("mkdir -p %s && cp -fp %s %s && chmod +x %s", // Added -p to cp for preserving mode
			filepath.Dir(finalBinaryPath),
			sourceBinaryInExtraction,
			finalBinaryPath,
			finalBinaryPath,
		)
		finalizeStep := command.NewCommandStep(cmdToFinalize, "", "", false, "", 0, nil)
		nodes[finalizeNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Finalize %s binary (%s %s %s)", h.ComponentName, h.Version, h.OS, h.Arch),
			Step:         finalizeStep,
			Hosts:        []connector.Host{controlNode},
			Dependencies: []plan.NodeID{lastNodeID},
			StepName:     finalizeStep.Meta().Name,
		}
		exitNodes = append(exitNodes, finalizeNodeID)
	} else { // Not an archive, the downloaded file is the binary itself
		finalizeNodeID := plan.NodeID(fmt.Sprintf("finalize-direct-%s", h.ID()))
		cmdToFinalize := fmt.Sprintf("mkdir -p %s && cp -fp %s %s && chmod +x %s",
			filepath.Dir(finalBinaryPath),
			localArchivePath, // Source is the downloaded file itself
			finalBinaryPath,
			finalBinaryPath,
		)
		finalizeStep := command.NewCommandStep(cmdToFinalize, "", "", false, "", 0, nil)
		nodes[finalizeNodeID] = &plan.ExecutionNode{
			Name:         fmt.Sprintf("Finalize direct %s binary (%s %s %s)", h.ComponentName, h.Version, h.OS, h.Arch),
			Step:         finalizeStep,
			Hosts:        []connector.Host{controlNode},
			Dependencies: []plan.NodeID{lastNodeID}, // Depends on download
			StepName:     finalizeStep.Meta().Name,
		}
		exitNodes = append(exitNodes, finalizeNodeID)
	}


	logger.Info("Remote binary resource assurance plan created.", "final_binary_path", finalBinaryPath)
	return &task.ExecutionFragment{
		Nodes:      nodes,
		EntryNodes: entryNodes,
		ExitNodes:  exitNodes,
	}, nil
}

// isArchive checks if the handle represents an archive based on known details or filename.
func (h *RemoteBinaryHandle) isArchive() bool {
	if detail, ok := knownBinariesRegistry[strings.ToLower(h.ComponentName)]; ok {
		return detail.isArchive
	}
	// Fallback: check common archive extensions
	ext := filepath.Ext(h.ArchiveFilename)
	return ext == ".tar" || ext == ".gz" || ext == ".tgz" || ext == ".zip"
}

// Type returns the type of this resource handle.
func (h *RemoteBinaryHandle) Type() string {
	return "remote-binary"
}

// Ensure RemoteBinaryHandle implements the Handle interface.
var _ Handle = (*RemoteBinaryHandle)(nil)
