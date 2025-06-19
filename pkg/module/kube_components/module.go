package kube_components

import (
	"fmt"
	"path/filepath"
	goruntime "runtime" // Alias to avoid conflict with kubexms/pkg/runtime
	"os" // For os.Getenv

	"github.com/kubexms/kubexms/pkg/config" // Assumed to have necessary fields
	"github.com/kubexms/kubexms/pkg/spec"
	// Task constructors will be imported
	taskKubeComponents "github.com/kubexms/kubexms/pkg/task/kube_components"
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
	// TODO: Re-evaluate architecture detection. cfg.Spec.Arch removed.
	// Consider deriving from host list or a new global config if diverse archs are supported.
	arch := goruntime.GOARCH
	arch = normalizeArchFunc(arch)

	// TODO: Re-evaluate zone determination. v1alpha1.GlobalSpec does not have Zone.
	// Consider using a new global config field or deriving from hosts if needed.
	zone := os.Getenv("KKZONE") // Fallback to environment variable or empty string

	programBaseDir := "/opt/kubexms/default_run_dir" // Fallback
	if cfg.Spec.Global != nil && cfg.Spec.Global.WorkDir != "" {
		programBaseDir = cfg.Spec.Global.WorkDir
	}
	// appFSBaseDir is the root for KubeXMS specific persistent data, like artifacts: <executable_dir>/.kubexm
	appFSBaseDir := filepath.Join(programBaseDir, ".kubexm")

	// --- Kubernetes Components ---
	kubeVersion := ""
	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "" { // Nil check for Kubernetes
		kubeVersion = cfg.Spec.Kubernetes.Version
	}

	if kubeVersion != "" {
		if task := taskKubeComponents.NewFetchKubeadmTask(cfg, kubeVersion, arch, zone, appFSBaseDir); task != nil {
			tasks = append(tasks, task)
		}
		if task := taskKubeComponents.NewFetchKubeletTask(cfg, kubeVersion, arch, zone, appFSBaseDir); task != nil {
			tasks = append(tasks, task)
		}
		if task := taskKubeComponents.NewFetchKubectlTask(cfg, kubeVersion, arch, zone, appFSBaseDir); task != nil {
			tasks = append(tasks, task)
		}
	}

	// --- Containerd ---
	// Assuming ContainerRuntime.Version holds the version for Containerd if Type is containerd.
	// This logic might need to be more robust based on ContainerRuntimeConfig structure.
	containerdVersion := ""
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Type == "containerd" && cfg.Spec.ContainerRuntime.Version != "" { // Nil check
		containerdVersion = cfg.Spec.ContainerRuntime.Version
	}

	if containerdVersion != "" {
		// TODO: Ensure NewFetchContainerdTask correctly uses the version from ContainerRuntime.Version.
		// If ContainerdConfig has its own version field, that should be preferred if it exists.
		if task := taskKubeComponents.NewFetchContainerdTask(cfg, containerdVersion, arch, zone, appFSBaseDir); task != nil {
			tasks = append(tasks, task)
		}
	}

	return &spec.ModuleSpec{
		Name: "Kubernetes Components Download & Install",
		IsEnabled: func(currentCfg *config.Cluster) bool {
			k8sEnabled := currentCfg != nil && currentCfg.Spec.Kubernetes != nil && currentCfg.Spec.Kubernetes.Version != ""
			containerRuntimeEnabled := currentCfg != nil && currentCfg.Spec.ContainerRuntime != nil && currentCfg.Spec.ContainerRuntime.Version != ""
			return k8sEnabled || containerRuntimeEnabled
		},
		Tasks: tasks,
	}
}
