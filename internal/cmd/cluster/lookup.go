package cluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/config"
)

// legacyClustersDir returns the legacy hardcoded clusters directory path.
func legacyClustersDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".kubexm", "clusters"), nil
}

// LoadClusterConfig attempts to load a cluster config by name.
// It tries the runtime-consistent path first, then falls back to the legacy path.
func LoadClusterConfig(clusterName string) (*v1alpha1.Cluster, error) {
	// Try legacy path first (most users will have clusters there)
	legacyBase, err := legacyClustersDir()
	if err != nil {
		return nil, err
	}
	legacyConfigPath := filepath.Join(legacyBase, clusterName, "config.yaml")
	if clusterConfig, err := config.ParseFromFile(legacyConfigPath); err == nil {
		return clusterConfig, nil
	}

	// Fall back: try to find config at workDir/clusterName/config.yaml
	// by iterating known workDir candidates
	knownWorkDirs := []string{
		filepath.Join(os.Getenv("HOME"), ".kubexm"),
		"/tmp/kubexm",
		".kubexm",
	}
	for _, workDir := range knownWorkDirs {
		candidatePath := filepath.Join(workDir, clusterName, "config.yaml")
		if clusterConfig, err := config.ParseFromFile(candidatePath); err == nil {
			return clusterConfig, nil
		}
	}

	return nil, fmt.Errorf("cluster config not found for '%s' (searched: %s and known work directories)", clusterName, legacyConfigPath)
}

// FindKubeconfig attempts to find the kubeconfig file for a given cluster.
// It tries the runtime-consistent path first, then falls back to the legacy path.
func FindKubeconfig(clusterName string) (string, error) {
	// Try legacy path first
	legacyBase, err := legacyClustersDir()
	if err != nil {
		return "", err
	}
	legacyPath := filepath.Join(legacyBase, clusterName, "admin.kubeconfig")
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, nil
	}

	// Try known workDir candidates
	knownWorkDirs := []string{
		filepath.Join(os.Getenv("HOME"), ".kubexm"),
		"/tmp/kubexm",
		".kubexm",
	}
	for _, workDir := range knownWorkDirs {
		candidate := filepath.Join(workDir, clusterName, "admin.kubeconfig")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("kubeconfig not found for cluster '%s' (searched: %s and known work directories)", clusterName, legacyPath)
}

// ListClustersDirs returns all cluster directories found in both legacy and runtime paths.
func ListClustersDirs() ([]string, error) {
	var allDirs []string
	seen := make(map[string]bool)

	// Legacy path
	legacyBase, err := legacyClustersDir()
	if err == nil {
		if entries, err := os.ReadDir(legacyBase); err == nil {
			for _, e := range entries {
				if e.IsDir() && !seen[e.Name()] {
					allDirs = append(allDirs, filepath.Join(legacyBase, e.Name()))
					seen[e.Name()] = true
				}
			}
		}
	}

	// Known workDir candidates (skip if same as legacy)
	knownWorkDirs := []string{
		"/tmp/kubexm",
		".kubexm",
	}
	for _, workDir := range knownWorkDirs {
		if workDir == legacyBase {
			continue
		}
		if entries, err := os.ReadDir(workDir); err == nil {
			for _, e := range entries {
				if e.IsDir() && !seen[e.Name()] {
					clusterDir := filepath.Join(workDir, e.Name())
					// Verify it's a cluster dir (has config or pki subdir)
					if _, statErr := os.Stat(filepath.Join(clusterDir, "config.yaml")); statErr == nil {
						allDirs = append(allDirs, clusterDir)
						seen[e.Name()] = true
					}
				}
			}
		}
	}

	return allDirs, nil
}
