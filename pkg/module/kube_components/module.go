package kube_components

import (
	"fmt"
	goruntime "runtime" // Alias to avoid conflict with kubexms/pkg/runtime

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	// Import common steps
	"github.com/kubexms/kubexms/pkg/step/common"
	// Import component download steps
	"github.com/kubexms/kubexms/pkg/step/component_downloads"
	// Assuming runtime.Context is passed to steps, not goruntime
)

// normalizeArch ensures consistent architecture naming (amd64, arm64).
func normalizeArch(arch string) string {
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

	// Determine architecture
	arch := cfg.Spec.Arch // Assuming cfg.Spec.Arch stores the desired target architecture
	if arch == "" {
		arch = goruntime.GOARCH // Default to host architecture of the control plane if not specified
	}
	arch = normalizeArch(arch)

	// Assumed config paths (these would need to be defined in your actual config.ClusterSpec)
	etcdVersion := "v3.5.0"         // Placeholder, replace with cfg.Spec.Etcd.Version
	kubeVersion := "v1.23.5"        // Placeholder, replace with cfg.Spec.Kubernetes.Version
	containerdVersion := "1.6.4"    // Placeholder, replace with cfg.Spec.Containerd.Version
	zone := ""                      // Placeholder, replace with cfg.Spec.Zone or similar global setting

	// If cfg and its sub-specs are not nil, use values from there.
	// This is a basic way to handle potentially nil config parts.
	// A more robust solution would involve default application in config unmarshalling or dedicated config managers.
	if cfg != nil {
		if cfg.Spec.Arch != "" {
			arch = normalizeArch(cfg.Spec.Arch)
		}
		// Example of how zone might be configured, e.g. a global setting or per component type
		// if cfg.Spec.Global != nil && cfg.Spec.Global.Zone != "" {
		// 	zone = cfg.Spec.Global.Zone
		// }

		if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Version != "" {
			etcdVersion = cfg.Spec.Etcd.Version
		}
		if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "" {
			kubeVersion = cfg.Spec.Kubernetes.Version
		}
		// Assuming ContainerdSpec might be nested differently, e.g., under Kubernetes or a CR spec.
		// For this example, let's assume a direct path if it were defined.
		// if cfg.Spec.Containerd != nil && cfg.Spec.Containerd.Version != "" {
		// 	containerdVersion = cfg.Spec.Containerd.Version
		// }
		// For now, using hardcoded defaults above if specific cfg paths are not fully fleshed out.
	}


	// === Task for etcd ===
	// Using default keys from each step's spec for SharedData communication.
	// These keys are typically like "EtcdDownloadedPath", "EtcdExtractedDir", etc.
	// The component_downloads steps define these as constants.
	// common.ExtractArchiveStepSpec defaults its input key to common.DefaultDownloadedFilePathKey.
	// common.InstallBinaryStepSpec defaults its input key to common.DefaultExtractedPathKey.
	// We need to ensure these defaults align or override them.

	// For clarity, explicitly set SharedData keys if defaults are not perfectly aligned or obvious.
	// For etcd, DownloadEtcdStepSpec will use `component_downloads.EtcdDownloadedPathKey` as its output.
	// ExtractArchiveStepSpec needs this as input.
	// InstallBinaryStepSpec needs ExtractArchiveStepSpec's output as its input.

	etcdTask := &spec.TaskSpec{
		Name: fmt.Sprintf("Fetch etcd %s (%s)", etcdVersion, arch),
		Steps: []spec.StepSpec{
			&component_downloads.DownloadEtcdStepSpec{
				Version: etcdVersion, Arch: arch, Zone: zone,
				// OutputFilePathKey defaults to component_downloads.EtcdDownloadedPathKey
			},
			&common.ExtractArchiveStepSpec{
				ArchivePathSharedDataKey: component_downloads.EtcdDownloadedPathKey, // Input from previous step
				// ExtractionDir: by default, will be a unique dir like /tmp/kubexms_extracts/etcd-v3.5.0.tar.gz-extract-<ts>
				ExtractedDirSharedDataKey: common.DefaultExtractedPathKey, // Output for next step (generic key)
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: common.DefaultExtractedPathKey, // Input from extraction
				SourceIsDirectory:       true,
				SourceFileName:          "etcd", // etcd binary is directly in extracted dir (e.g. etcd-v3.5.0-linux-amd64/etcd)
				                                // This assumes the ExtractedDirSharedDataKey points to the *root* of extracted content,
				                                // e.g. /tmp/.../etcd-v3.5.0-linux-amd64/
				TargetDir:               "/usr/local/bin",
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: common.DefaultExtractedPathKey, // Input from extraction
				SourceIsDirectory:       true,
				SourceFileName:          "etcdctl",
				TargetDir:               "/usr/local/bin",
			},
		},
	}
	tasks = append(tasks, etcdTask)

	// === Task for kubeadm ===
	kubeadmTask := &spec.TaskSpec{
		Name: fmt.Sprintf("Fetch kubeadm %s (%s)", kubeVersion, arch),
		Steps: []spec.StepSpec{
			&component_downloads.DownloadKubeadmStepSpec{
				Version: kubeVersion, Arch: arch, Zone: zone,
				// OutputFilePathKey defaults to component_downloads.KubeadmDownloadedPathKey
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: component_downloads.KubeadmDownloadedPathKey, // Input from download
				SourceIsDirectory:       false, // kubeadm is downloaded as a direct binary
				// SourceFileName is not needed if SourceIsDirectory is false and key points to the file
				TargetDir:               "/usr/local/bin",
				TargetFileName:          "kubeadm", // Ensure it's named kubeadm
			},
		},
	}
	tasks = append(tasks, kubeadmTask)

	// === Task for kubelet ===
	kubeletTask := &spec.TaskSpec{
		Name: fmt.Sprintf("Fetch kubelet %s (%s)", kubeVersion, arch),
		Steps: []spec.StepSpec{
			&component_downloads.DownloadKubeletStepSpec{
				Version: kubeVersion, Arch: arch, Zone: zone,
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: component_downloads.KubeletDownloadedPathKey,
				SourceIsDirectory:       false,
				TargetDir:               "/usr/local/bin",
				TargetFileName:          "kubelet",
			},
		},
	}
	tasks = append(tasks, kubeletTask)

	// === Task for kubectl ===
	kubectlTask := &spec.TaskSpec{
		Name: fmt.Sprintf("Fetch kubectl %s (%s)", kubeVersion, arch),
		Steps: []spec.StepSpec{
			&component_downloads.DownloadKubectlStepSpec{
				Version: kubeVersion, Arch: arch, Zone: zone,
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: component_downloads.KubectlDownloadedPathKey,
				SourceIsDirectory:       false,
				TargetDir:               "/usr/local/bin",
				TargetFileName:          "kubectl",
			},
		},
	}
	tasks = append(tasks, kubectlTask)

	// === Task for containerd ===
	// For containerd, DownloadContainerdStepSpec outputs to component_downloads.ContainerdDownloadedPathKey
	// ExtractArchiveStepSpec needs this as input.
	// InstallBinaryStepSpec needs ExtractArchiveStepSpec's output.
	containerdExtractedDirKey := "ContainerdExtractedDir" // Custom key for clarity

	containerdTask := &spec.TaskSpec{
		Name: fmt.Sprintf("Fetch containerd %s (%s)", containerdVersion, arch),
		Steps: []spec.StepSpec{
			&component_downloads.DownloadContainerdStepSpec{
				Version: containerdVersion, Arch: arch, Zone: zone,
				// OutputFilePathKey defaults to component_downloads.ContainerdDownloadedPathKey
			},
			&common.ExtractArchiveStepSpec{
				ArchivePathSharedDataKey: component_downloads.ContainerdDownloadedPathKey,
				// ExtractionDir: Default unique dir
				ExtractedDirSharedDataKey: containerdExtractedDirKey, // Specific output key for containerd's extracted content
			},
			// Binaries are typically in a 'bin' subdirectory of the extracted archive.
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: containerdExtractedDirKey,
				SourceIsDirectory:       true,
				SourceFileName:          "bin/containerd", // Path relative to the root of extracted archive
				TargetDir:               "/usr/local/bin",
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: containerdExtractedDirKey,
				SourceIsDirectory:       true,
				SourceFileName:          "bin/ctr",
				TargetDir:               "/usr/local/bin",
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: containerdExtractedDirKey,
				SourceIsDirectory:       true,
				SourceFileName:          "bin/containerd-shim",
				TargetDir:               "/usr/local/bin",
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: containerdExtractedDirKey,
				SourceIsDirectory:       true,
				SourceFileName:          "bin/containerd-shim-runc-v1", // Name might vary with version
				TargetDir:               "/usr/local/bin",
			},
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: containerdExtractedDirKey,
				SourceIsDirectory:       true,
				SourceFileName:          "bin/containerd-shim-runc-v2",
				TargetDir:               "/usr/local/bin",
			},
			// runc is often also in containerd bundles, but could be a separate download.
			// If runc is present in the bundle at bin/runc:
			&common.InstallBinaryStepSpec{
				SourcePathSharedDataKey: containerdExtractedDirKey,
				SourceIsDirectory:       true,
				SourceFileName:          "bin/runc",
				TargetDir:               "/usr/local/sbin", // Or /usr/local/bin, ensure consistency
				TargetFileName:          "runc",
			},
		},
	}
	tasks = append(tasks, containerdTask)

	return &spec.ModuleSpec{
		Name: "Kubernetes Components Management",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Example: Enable if Kubernetes installation is specified.
			// This is a placeholder; actual enabling logic depends on config structure.
			// return clusterCfg != nil && clusterCfg.Spec.Kubernetes != nil && clusterCfg.Spec.Kubernetes.Enable
			return true // For now, enable by default for testing.
		},
		Tasks: tasks,
	}
}
