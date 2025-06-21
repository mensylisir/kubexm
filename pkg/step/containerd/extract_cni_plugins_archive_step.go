package containerd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	// CNIPluginsExtractedDirCacheKey is where the path to the CNI plugins binaries dir will be stored.
	// This is typically /opt/cni/bin after extraction.
	CNIPluginsExtractedDirCacheKey = "CNIPluginsExtractedDir"
)

// ExtractCNIPluginsArchiveStep extracts the CNI plugins archive on a target node, typically to /opt/cni/bin.
type ExtractCNIPluginsArchiveStep struct {
	meta                       spec.StepMeta
	RemoteArchivePathCacheKey  string // Task cache key for the path of the CNI plugins archive on the target node.
	TargetCNIBinDir            string // Directory on the target node where CNI plugins should be extracted (e.g., /opt/cni/bin).
	OutputExtractedPathCacheKey string // Task cache key to store the path to TargetCNIBinDir.
	Sudo                       bool   // Whether to use sudo for extraction operations.
	RemoveArchiveAfterExtract  bool
}

// NewExtractCNIPluginsArchiveStep creates a new ExtractCNIPluginsArchiveStep.
func NewExtractCNIPluginsArchiveStep(instanceName, remotePathCacheKey, targetCNIBinDir, outputPathCacheKey string, sudo, removeArchive bool) step.Step {
	if remotePathCacheKey == "" {
		remotePathCacheKey = CNIPluginsArchiveRemotePathCacheKey
	}
	if targetCNIBinDir == "" {
		targetCNIBinDir = "/opt/cni/bin"
	}
	if outputPathCacheKey == "" {
		outputPathCacheKey = CNIPluginsExtractedDirCacheKey
	}
	name := instanceName
	if name == "" {
		name = "ExtractCNIPluginsArchive"
	}

	return &ExtractCNIPluginsArchiveStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Extracts the CNI plugins archive to %s on the target node.", targetCNIBinDir),
		},
		RemoteArchivePathCacheKey: remotePathCacheKey,
		TargetCNIBinDir:           targetCNIBinDir,
		OutputExtractedPathCacheKey:outputPathCacheKey,
		Sudo:                      sudo, // Extraction to /opt/cni/bin often requires sudo
		RemoveArchiveAfterExtract: removeArchive,
	}
}

func (s *ExtractCNIPluginsArchiveStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ExtractCNIPluginsArchiveStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	// Check if TargetCNIBinDir exists and perhaps if a known CNI plugin binary (e.g., "bridge" or "loopback") is present.
	// A simple check for directory existence might be sufficient for precheck.
	// A more thorough check would list some expected binaries.
	expectedBridgePlugin := filepath.Join(s.TargetCNIBinDir, "bridge")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, expectedBridgePlugin)
	if exists {
		logger.Info("Key CNI plugin (bridge) already exists in target CNI bin directory.", "path", expectedBridgePlugin)
		ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, s.TargetCNIBinDir)
		return true, nil
	}
	logger.Info("CNI plugins not found in target CNI bin directory. Extraction needed.", "expected_at", expectedBridgePlugin)
	return false, nil
}

func (s *ExtractCNIPluginsArchiveStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	remoteArchivePathVal, found := ctx.TaskCache().Get(s.RemoteArchivePathCacheKey)
	if !found {
		return fmt.Errorf("remote CNI plugins archive path not found in task cache with key '%s'", s.RemoteArchivePathCacheKey)
	}
	remoteArchivePath, ok := remoteArchivePathVal.(string)
	if !ok || remoteArchivePath == "" {
		return fmt.Errorf("invalid remote CNI plugins archive path in task cache: got '%v'", remoteArchivePathVal)
	}
	logger.Info("Retrieved remote CNI plugins archive path from cache.", "path", remoteArchivePath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	logger.Info("Ensuring target CNI bin directory exists.", "path", s.TargetCNIBinDir)
	// CNI plugins are often installed by root, so sudo is common for Mkdirp and Extract.
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, s.TargetCNIBinDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create target CNI bin directory %s: %w", s.TargetCNIBinDir, err)
	}

	logger.Info("Extracting CNI plugins archive.", "source", remoteArchivePath, "destinationDir", s.TargetCNIBinDir)
	// CNI plugins are typically a flat list of binaries in the tarball, so they extract directly into TargetCNIBinDir.
	if err := runnerSvc.Extract(ctx.GoContext(), conn, nil, remoteArchivePath, s.TargetCNIBinDir, s.Sudo); err != nil {
		return fmt.Errorf("failed to extract CNI plugins archive %s to %s: %w", remoteArchivePath, s.TargetCNIBinDir, err)
	}
	logger.Info("CNI plugins archive extracted successfully.")

	// Verify a key plugin
	expectedBridgePlugin := filepath.Join(s.TargetCNIBinDir, "bridge")
	exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, expectedBridgePlugin)
	if !exists {
		logger.Error("Post-extraction check failed: CNI bridge plugin not found.", "expected_at", expectedBridgePlugin)
		lsOutput, _, lsErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, fmt.Sprintf("ls -lA %s", s.TargetCNIBinDir), &connector.ExecOptions{Sudo: s.Sudo})
		if lsErr == nil {
			logger.Info("Contents of CNI bin directory", "dir", s.TargetCNIBinDir, "listing", string(lsOutput))
		}
		return fmt.Errorf("CNI bridge plugin not found in %s after extraction", s.TargetCNIBinDir)
	}
	logger.Info("CNI bridge plugin confirmed in target CNI bin directory.", "path", expectedBridgePlugin)


	if s.RemoveArchiveAfterExtract {
		logger.Info("Removing source CNI plugins archive after extraction.", "path", remoteArchivePath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, remoteArchivePath, s.Sudo); err != nil {
			logger.Warn("Failed to remove source CNI plugins archive (best effort).", "path", remoteArchivePath, "error", err)
		}
	}

	ctx.TaskCache().Set(s.OutputExtractedPathCacheKey, s.TargetCNIBinDir)
	return nil
}

func (s *ExtractCNIPluginsArchiveStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback might involve removing the extracted binaries. This can be complex if other CNI plugins were present.
	// A simple rollback is to remove common CNI plugin names known to be in the archive.
	// A more aggressive (but potentially risky) rollback is to remove s.TargetCNIBinDir if it was created by this step.
	// For now, keep it simple: no removal of individual binaries, as we don't know which ones came from *this* archive.
	// If s.TargetCNIBinDir was created *exclusively* for these plugins, it could be removed.
	logger.Info("Rollback for ExtractCNIPluginsArchiveStep is complex and typically not performed by removing individual files from shared /opt/cni/bin. Manual cleanup might be needed if rollback is critical.")
	// To be safer, do not remove TargetCNIBinDir as it's a shared path.

	ctx.TaskCache().Delete(s.OutputExtractedPathCacheKey)
	return nil
}

var _ step.Step = (*ExtractCNIPluginsArchiveStep)(nil)
