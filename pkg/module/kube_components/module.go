package kube_components

import (
	"fmt"
	"path/filepath"
	goruntime "runtime" // Alias to avoid conflict with kubexms/pkg/runtime
	"os" // For os.Getenv

	"github.com/kubexms/kubexms/pkg/config" // Assumed to have necessary fields
	"github.com/kubexms/kubexms/pkg/spec"
	// Import common steps
	"github.com/kubexms/kubexms/pkg/step/common"
	// Import component download steps
	"github.com/kubexms/kubexms/pkg/step/component_downloads"
)

// normalizeArchFunc ensures consistent architecture naming (amd64, arm64).
func normalizeArchFunc(arch string) string {
	if arch == "x86_64" {
		return "amd64"
	}
	if arch == "aarch64" {
		return "arm64"
	}
	return arch
}

// NewKubernetesComponentsModule creates a module specification for fetching Kubernetes components.
func NewKubernetesComponentsModule(cfg *config.Cluster) *spec.ModuleSpec {
	tasks := []*spec.TaskSpec{}

	// --- Determine global parameters from cfg ---
	arch := cfg.Spec.Arch
	if arch == "" {
		arch = goruntime.GOARCH
	}
	arch = normalizeArchFunc(arch)

	zone := "" // Default (no specific zone for downloads)
	if cfg.Spec.Global != nil && cfg.Spec.Global.Zone != "" {
		zone = cfg.Spec.Global.Zone
	}
	if zone == "" { // Fallback to env var if not in config
		zone = os.Getenv("KKZONE")
	}

	appWorkDir := cfg.WorkDir
	if appWorkDir == "" {
		appWorkDir = "/opt/kubexms/default_workdir" // Fallback
	}
	appArtifactsBaseDir := filepath.Join(appWorkDir, "kubexms") // Base for this app's artifacts

	// --- Kubernetes Components ---
	kubeVersion := ""
	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "" {
		kubeVersion = cfg.Spec.Kubernetes.Version
	} else {
		// If Kubernetes version is essential for this module, consider returning nil or an error
		// or disabling the module if IsEnabled logic doesn't already cover it.
		// For now, allow tasks to be empty if version is not set.
	}

	if kubeVersion != "" {
		kubeComponents := []struct {
			Name         string
			DownloadSpec spec.StepSpec
			IsArchive    bool
			InstallSpecs []spec.StepSpec // Can be multiple binaries from one download/archive
		}{
			{
				Name: "kubeadm",
				DownloadSpec: &component_downloads.DownloadKubeadmStepSpec{
					Version:     kubeVersion,
					Arch:        arch,
					Zone:        zone,
					DownloadDir: filepath.Join(appArtifactsBaseDir, "kubeadm", kubeVersion, arch),
					// Output keys use defaults from DownloadKubeadmStepSpec
				},
				IsArchive: false,
				InstallSpecs: []spec.StepSpec{
					&common.InstallBinaryStepSpec{
						SourcePathSharedDataKey: component_downloads.KubeadmDownloadedPathKey, // Default output key from DownloadKubeadmStepSpec
						SourceIsDirectory:       false,
						TargetDir:               "/usr/local/bin",
						TargetFileName:          "kubeadm",
						Permissions:             "0755",
					},
				},
			},
			{
				Name: "kubelet",
				DownloadSpec: &component_downloads.DownloadKubeletStepSpec{
					Version:     kubeVersion,
					Arch:        arch,
					Zone:        zone,
					DownloadDir: filepath.Join(appArtifactsBaseDir, "kubelet", kubeVersion, arch),
				},
				IsArchive: false,
				InstallSpecs: []spec.StepSpec{
					&common.InstallBinaryStepSpec{
						SourcePathSharedDataKey: component_downloads.KubeletDownloadedPathKey,
						SourceIsDirectory:       false,
						TargetDir:               "/usr/local/bin",
						TargetFileName:          "kubelet",
						Permissions:             "0755",
					},
				},
			},
			{
				Name: "kubectl",
				DownloadSpec: &component_downloads.DownloadKubectlStepSpec{
					Version:     kubeVersion,
					Arch:        arch,
					Zone:        zone,
					DownloadDir: filepath.Join(appArtifactsBaseDir, "kubectl", kubeVersion, arch),
				},
				IsArchive: false,
				InstallSpecs: []spec.StepSpec{
					&common.InstallBinaryStepSpec{
						SourcePathSharedDataKey: component_downloads.KubectlDownloadedPathKey,
						SourceIsDirectory:       false,
						TargetDir:               "/usr/local/bin",
						TargetFileName:          "kubectl",
						Permissions:             "0755",
					},
				},
			},
		}

		for _, comp := range kubeComponents {
			taskSteps := []spec.StepSpec{comp.DownloadSpec}
			if comp.IsArchive { // This path is not taken for kubeadm,kubelet,kubectl currently
				// taskSteps = append(taskSteps, &common.ExtractArchiveStepSpec{...})
			}
			taskSteps = append(taskSteps, comp.InstallSpecs...)

			componentTask := &spec.TaskSpec{
				Name:  fmt.Sprintf("Fetch and Install %s %s (%s)", comp.Name, kubeVersion, arch),
				Steps: taskSteps,
			}
			tasks = append(tasks, componentTask)
		}
	}

	// --- Containerd ---
	containerdVersion := ""
	// Assuming ContainerRuntimeSpec is where containerd version is defined.
	// Adjust path based on actual config structure.
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Version != "" {
		containerdVersion = cfg.Spec.ContainerRuntime.Version
	} else {
		// Fallback or error if containerd version is essential
	}

	if containerdVersion != "" {
		containerdDownloadDir := filepath.Join(appArtifactsBaseDir, "containerd", containerdVersion, arch)
		containerdExtractionDir := filepath.Join(appArtifactsBaseDir, "_extracts", "containerd", containerdVersion, arch)
		// Using a more specific key for containerd's extracted path for clarity, as it contains multiple binaries.
		containerdExtractedDirKey := "ContainerdExtractedArchiveDir"

		containerdTask := &spec.TaskSpec{
			Name: fmt.Sprintf("Fetch and Install containerd %s (%s)", containerdVersion, arch),
			Steps: []spec.StepSpec{
				&component_downloads.DownloadContainerdStepSpec{
					Version:     containerdVersion,
					Arch:        arch,
					Zone:        zone,
					DownloadDir: containerdDownloadDir,
					// OutputFilePathKey defaults to component_downloads.ContainerdDownloadedPathKey
				},
				&common.ExtractArchiveStepSpec{
					ArchivePathSharedDataKey: component_downloads.ContainerdDownloadedPathKey,
					ExtractionDir:            containerdExtractionDir,
					ExtractedDirSharedDataKey: containerdExtractedDirKey,
				},
				// Binaries are typically in a 'bin' subdirectory of the extracted archive.
				&common.InstallBinaryStepSpec{
					SourcePathSharedDataKey: containerdExtractedDirKey, SourceIsDirectory: true, SourceFileName: "bin/containerd",
					TargetDir: "/usr/local/bin", Permissions: "0755",
				},
				&common.InstallBinaryStepSpec{
					SourcePathSharedDataKey: containerdExtractedDirKey, SourceIsDirectory: true, SourceFileName: "bin/ctr",
					TargetDir: "/usr/local/bin", Permissions: "0755",
				},
				&common.InstallBinaryStepSpec{
					SourcePathSharedDataKey: containerdExtractedDirKey, SourceIsDirectory: true, SourceFileName: "bin/containerd-shim",
					TargetDir: "/usr/local/bin", Permissions: "0755",
				},
				&common.InstallBinaryStepSpec{
					SourcePathSharedDataKey: containerdExtractedDirKey, SourceIsDirectory: true, SourceFileName: "bin/containerd-shim-runc-v1",
					TargetDir: "/usr/local/bin", Permissions: "0755",
				},
				&common.InstallBinaryStepSpec{
					SourcePathSharedDataKey: containerdExtractedDirKey, SourceIsDirectory: true, SourceFileName: "bin/containerd-shim-runc-v2",
					TargetDir: "/usr/local/bin", Permissions: "0755",
				},
				&common.InstallBinaryStepSpec{ // runc
					SourcePathSharedDataKey: containerdExtractedDirKey, SourceIsDirectory: true, SourceFileName: "bin/runc",
					TargetDir: "/usr/local/sbin", TargetFileName: "runc", Permissions: "0755", // Often in sbin
				},
			},
		}
		tasks = append(tasks, containerdTask)
	}

	return &spec.ModuleSpec{
		Name: "Kubernetes Components Download & Install",
		IsEnabled: func(currentCfg *config.Cluster) bool {
			// Enable if Kubernetes version is specified, implying it's a K8s cluster setup.
			// Or use a more specific flag like currentCfg.Spec.Components.FetchEnabled.
			return currentCfg != nil && currentCfg.Spec.Kubernetes != nil && currentCfg.Spec.Kubernetes.Version != ""
		},
		Tasks: tasks,
	}
}
