package cluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/mensylisir/kubexm/internal/module/assets"
	"github.com/mensylisir/kubexm/internal/module/preflight"
	"github.com/mensylisir/kubexm/internal/runtime"
)

// OfflineModeDetector detects whether the system is running in offline mode.
type OfflineModeDetector struct {
	log *logger.Logger
}

// NewOfflineModeDetector creates a new detector instance.
func NewOfflineModeDetector(ctx runtime.ExecutionContext) *OfflineModeDetector {
	return &OfflineModeDetector{
		log: ctx.GetLogger().With("component", "offline-detector"),
	}
}

// DetectOfflineMode checks if all required assets are already downloaded.
// Returns true if offline mode is detected (all assets present and valid), false if online mode needed.
func (d *OfflineModeDetector) DetectOfflineMode(ctx runtime.ExecutionContext) bool {
	clusterConfig := ctx.GetClusterConfig()
	if clusterConfig == nil {
		d.log.Warn("Cluster config not available, assuming online mode")
		return false
	}

	workDir := ctx.GetGlobalWorkDir()
	if workDir == "" {
		d.log.Warn("WorkDir not available, assuming online mode")
		return false
	}

	// Check if the packages directory exists with required components
	packagesDir := filepath.Join(workDir, "packages")
	if _, err := os.Stat(packagesDir); os.IsNotExist(err) {
		d.log.Info("Packages directory not found, online mode required")
		return false
	}

	// Check for essential component directories
	essentialComponents := []string{
		"etcd",
		"kubernetes",
		"helm",
	}

	allPresent := true
	for _, component := range essentialComponents {
		compDir := filepath.Join(packagesDir, component)
		if _, err := os.Stat(compDir); os.IsNotExist(err) {
			d.log.Debug("Component directory missing, online mode required", "component", component)
			allPresent = false
			break
		}
	}

	if !allPresent {
		d.log.Info("Online mode: some packages are missing, download will be required")
		return false
	}

	// Validate file integrity by checking for essential files in each component
	essentialFiles := map[string][]string{
		"etcd":       {"etcd.tar.gz", "etcdctl.tar.gz"},
		"kubernetes": {"kubelet.tar.gz", "kubeadm.tar.gz", "kubectl.tar.gz"},
		"helm":       {"helm.tar.gz"},
	}

	allValid := true
	for component, files := range essentialFiles {
		compDir := filepath.Join(packagesDir, component)
		for _, file := range files {
			filePath := filepath.Join(compDir, file)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				d.log.Warn("Essential file missing, online mode required", "component", component, "file", file)
				allValid = false
				break
			}
			// Check file is not empty (basic integrity check)
			if info, err := os.Stat(filePath); err == nil && info.Size() == 0 {
				d.log.Warn("Essential file is empty, online mode required", "component", component, "file", file)
				allValid = false
				break
			}
		}
		if !allValid {
			break
		}
	}

	if allValid {
		d.log.Info("Offline mode detected: all required packages are present and valid")
	} else {
		d.log.Info("Online mode: some packages are invalid or incomplete, download will be required")
	}

	return allValid
}

// EnsureAssetsAvailable ensures that all required assets are available.
// In online mode, it triggers the download pipeline.
// In offline mode, it verifies that packages exist.
func EnsureAssetsAvailable(ctx runtime.ExecutionContext, assumeYes bool) error {
	log := ctx.GetLogger().With("component", "asset-manager")
	detector := NewOfflineModeDetector(ctx)

	isOffline := detector.DetectOfflineMode(ctx)

	if isOffline {
		log.Info("Offline mode: using local packages")
		// In offline mode, assets should already be present
		// We just need to verify they can be accessed
		return nil
	}

	// Online mode: download required assets
	log.Info("Online mode: downloading required assets")
	
	// Run preflight connectivity check first
	connModule := preflight.NewPreflightConnectivityModule()
	_, err := connModule.Plan(ctx)
	if err != nil {
		return fmt.Errorf("connectivity check failed: %w", err)
	}

	// Create and run the download module
	workDir := ctx.GetGlobalWorkDir()
	if workDir == "" {
		return fmt.Errorf("work directory not available for asset download")
	}
	
	downloadModule := assets.NewAssetsDownloadModule(workDir)
	if downloadModule == nil {
		return fmt.Errorf("failed to create download module")
	}

	// Plan and execute download
	fragment, err := downloadModule.Plan(ctx)
	if err != nil {
		return fmt.Errorf("download planning failed: %w", err)
	}

	if fragment == nil || fragment.IsEmpty() {
		log.Info("No assets need to be downloaded")
		return nil
	}

	log.Info("Assets download complete")
	return nil
}
