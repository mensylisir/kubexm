package kubernetes

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/util"
)

// InstallKubernetesBinariesStepSpec defines the configuration for installing kubernetes binaries
type InstallKubernetesBinariesStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// Version specifies the kubernetes version to install
	Version string `json:"version" yaml:"version"`

	// Arch specifies the architecture (amd64, arm64, etc.)
	Arch string `json:"arch,omitempty" yaml:"arch,omitempty"`

	// Components specifies which kubernetes components to install
	Components []string `json:"components,omitempty" yaml:"components,omitempty"`

	// BinaryPath is where to install kubernetes binaries
	BinaryPath string `json:"binaryPath,omitempty" yaml:"binaryPath,omitempty"`

	// DownloadURL is the base URL for downloading kubernetes binaries
	DownloadURL string `json:"downloadURL,omitempty" yaml:"downloadURL,omitempty"`

	// Force indicates whether to force reinstallation
	Force bool `json:"force,omitempty" yaml:"force,omitempty"`

	// NodeRole specifies the node role (master, worker, or both)
	NodeRole string `json:"nodeRole,omitempty" yaml:"nodeRole,omitempty"`
}

// InstallKubernetesBinariesStep implements the Step interface for installing kubernetes binaries
type InstallKubernetesBinariesStep struct {
	spec InstallKubernetesBinariesStepSpec
}

// NewInstallKubernetesBinariesStep creates a new InstallKubernetesBinariesStep
func NewInstallKubernetesBinariesStep(spec InstallKubernetesBinariesStepSpec) *InstallKubernetesBinariesStep {
	// Set default values
	if spec.Arch == "" {
		spec.Arch = util.GetArch()
	}
	if spec.BinaryPath == "" {
		spec.BinaryPath = "/usr/local/bin"
	}
	if spec.DownloadURL == "" {
		spec.DownloadURL = "https://dl.k8s.io/release/%s/bin/linux/%s/%s"
	}
	if len(spec.Components) == 0 {
		// Default components based on node role
		switch spec.NodeRole {
		case "master", "control-plane":
			spec.Components = []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler", "kubectl", "kubelet", "kubeadm"}
		case "worker":
			spec.Components = []string{"kubectl", "kubelet", "kubeadm"}
		default:
			// Install all components
			spec.Components = []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler", "kubectl", "kubelet", "kubeadm"}
		}
	}

	return &InstallKubernetesBinariesStep{spec: spec}
}

// Meta returns the step metadata
func (s *InstallKubernetesBinariesStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if kubernetes binaries are already installed with the correct version
func (s *InstallKubernetesBinariesStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	if s.spec.Force {
		return false, nil // Force reinstallation
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	// Check each component
	for _, component := range s.spec.Components {
		binaryPath := filepath.Join(s.spec.BinaryPath, component)
		
		// Check if binary exists
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, binaryPath)
		if err != nil {
			return false, fmt.Errorf("failed to check if %s exists: %w", component, err)
		}

		if !exists {
			logger.Infof("kubernetes component %s not found at %s", component, binaryPath)
			return false, nil
		}

		// Check version (for components that support --version)
		if s.isVersionCheckSupported(component) {
			cmd := fmt.Sprintf("%s --version", binaryPath)
			output, err := runnerSvc.Run(ctx.GoContext(), conn, cmd)
			if err != nil {
				logger.Warnf("failed to get version for %s: %v", component, err)
				return false, nil
			}

			if !s.isCorrectVersion(output, s.spec.Version) {
				logger.Infof("kubernetes component %s version mismatch, expected %s, found: %s", 
					component, s.spec.Version, output)
				return false, nil
			}
		}
	}

	logger.Infof("kubernetes binaries %s already installed", s.spec.Version)
	return true, nil
}

// Run executes the kubernetes binaries installation
func (s *InstallKubernetesBinariesStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Installing kubernetes binaries %s on host %s", s.spec.Version, host.GetName())

	// Ensure binary directory exists
	err = runner.Mkdirp(ctx.GoContext(), conn, s.spec.BinaryPath, "0755", true)
	if err != nil {
		return fmt.Errorf("failed to create binary directory: %w", err)
	}

	// Download and install each component
	for _, component := range s.spec.Components {
		err = s.installComponent(ctx, runner, conn, component)
		if err != nil {
			return fmt.Errorf("failed to install component %s: %w", component, err)
		}
	}

	logger.Infof("kubernetes binaries %s installed successfully", s.spec.Version)
	return nil
}

// Rollback attempts to remove the installed kubernetes binaries
func (s *InstallKubernetesBinariesStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back kubernetes binaries installation on host %s", host.GetName())

	// Remove each installed component
	for _, component := range s.spec.Components {
		binaryPath := filepath.Join(s.spec.BinaryPath, component)
		
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, binaryPath)
		if err != nil {
			logger.Warnf("failed to check if %s exists during rollback: %v", binaryPath, err)
			continue
		}

		if exists {
			rmCmd := fmt.Sprintf("rm -f %s", binaryPath)
			_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
			if err != nil {
				logger.Warnf("failed to remove %s during rollback: %v", binaryPath, err)
			} else {
				logger.Infof("removed %s", binaryPath)
			}
		}
	}

	logger.Infof("kubernetes binaries rollback completed")
	return nil
}

// installComponent downloads and installs a single kubernetes component
func (s *InstallKubernetesBinariesStep) installComponent(ctx step.StepContext, runner runner.Runner, conn connector.Connector, component string) error {
	logger := ctx.GetLogger()
	
	// Construct download URL
	downloadURL := fmt.Sprintf(s.spec.DownloadURL, s.spec.Version, s.spec.Arch, component)
	downloadPath := fmt.Sprintf("/tmp/%s-%s", component, s.spec.Version)
	
	logger.Infof("Downloading %s from %s", component, downloadURL)
	
	// Download the binary
	err := runner.DownloadFile(ctx.GoContext(), conn, downloadURL, downloadPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", component, err)
	}

	// Move to final location
	finalPath := filepath.Join(s.spec.BinaryPath, component)
	moveCmd := fmt.Sprintf("mv %s %s", downloadPath, finalPath)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, moveCmd)
	if err != nil {
		return fmt.Errorf("failed to move %s to final location: %w", component, err)
	}

	// Set executable permissions
	chmodCmd := fmt.Sprintf("chmod +x %s", finalPath)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, chmodCmd)
	if err != nil {
		return fmt.Errorf("failed to set executable permissions for %s: %w", component, err)
	}

	logger.Infof("installed %s successfully", component)
	return nil
}

// isVersionCheckSupported checks if a component supports --version flag
func (s *InstallKubernetesBinariesStep) isVersionCheckSupported(component string) bool {
	supportedComponents := map[string]bool{
		"kubectl":                 true,
		"kubelet":                 true,
		"kubeadm":                 true,
		"kube-apiserver":          true,
		"kube-controller-manager": true,
		"kube-scheduler":          true,
		"kube-proxy":              true,
	}
	return supportedComponents[component]
}

// isCorrectVersion checks if the version output matches the expected version
func (s *InstallKubernetesBinariesStep) isCorrectVersion(output, expectedVersion string) bool {
	// The output format varies by component, but generally contains the version
	// Examples:
	// kubectl: "Client Version: version.Info{Major:"1", Minor:"25", GitVersion:"v1.25.4"...}"
	// kubelet: "Kubernetes v1.25.4"
	// kubeadm: "kubeadm version: &version.Info{Major:"1", Minor:"25", GitVersion:"v1.25.4"...}"
	
	// Simple check: see if the expected version is contained in the output
	return containsString(output, expectedVersion)
}

// containsString checks if a string contains a substring (case-insensitive)
func containsString(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	
	// Convert to lowercase for case-insensitive comparison
	sLower := toLowerCase(s)
	substrLower := toLowerCase(substr)
	
	return findString(sLower, substrLower) >= 0
}

// toLowerCase converts a string to lowercase
func toLowerCase(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			result[i] = s[i] + 32 // Convert to lowercase
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// findString finds the index of substr in s
func findString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(s) < len(substr) {
		return -1
	}
	
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}