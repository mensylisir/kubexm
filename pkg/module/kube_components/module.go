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
	arch := cfg.Spec.Arch
	if arch == "" {
		arch = goruntime.GOARCH
	}
	arch = normalizeArchFunc(arch)

	zone := ""
	if cfg.Spec.Global != nil && cfg.Spec.Global.Zone != "" {
		zone = cfg.Spec.Global.Zone
	}
	if zone == "" {
		zone = os.Getenv("KKZONE")
	}

	programBaseDir := cfg.WorkDir // Assumed to be <executable_dir>
	if programBaseDir == "" {
		programBaseDir = "/opt/kubexms/default_run_dir" // Fallback
	}
	// appFSBaseDir is the root for KubeXMS specific persistent data, like artifacts: <executable_dir>/.kubexm
	appFSBaseDir := filepath.Join(programBaseDir, ".kubexm")

	// --- Kubernetes Components ---
	kubeVersion := ""
	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "" {
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
	containerdVersion := ""
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Version != "" {
		containerdVersion = cfg.Spec.ContainerRuntime.Version
	}

	if containerdVersion != "" {
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
