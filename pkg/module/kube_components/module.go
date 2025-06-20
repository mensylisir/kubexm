package kube_components

import (
	// "fmt"
	"path/filepath"
	goruntime "runtime" // Alias to avoid conflict with kubexms/pkg/runtime
	"os"                // For os.Getenv

	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster
	"github.com/mensylisir/kubexm/pkg/spec"
	taskKubeComponentsFactory "github.com/mensylisir/kubexm/pkg/task/kube_components" // Alias for task spec factories
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

// NewKubernetesComponentsModuleSpec creates a module specification for fetching Kubernetes components.
func NewKubernetesComponentsModuleSpec(cfg *config.Cluster) *spec.ModuleSpec {
	if cfg == nil {
		return &spec.ModuleSpec{
			Name:        "Kubernetes Components Download & Install",
			Description: "Fetches Kubernetes core components and container runtime binaries (Error: Missing Configuration)",
			IsEnabled:   "false",
			Tasks:       []*spec.TaskSpec{},
		}
	}
	tasks := []*spec.TaskSpec{}

	// Determine global parameters from cfg
	// TODO: Re-evaluate architecture detection. cfg.Spec.Arch removed.
	// Use cfg.Spec.Kubernetes.Arch or a more robust detection mechanism.
	arch := cfg.Spec.Kubernetes.Arch
	if arch == "" {
		arch = goruntime.GOARCH // Fallback
	}
	arch = normalizeArchFunc(arch)

	// TODO: Re-evaluate zone determination. cfg.Spec.Global.Zone does not exist.
	// Use cfg.Spec.ImageStore.Zone or similar.
	zone := cfg.Spec.ImageStore.Zone // Assuming this is the intended source for zone
	if zone == "" {
		zone = os.Getenv("KKZONE") // Fallback
	}

	programBaseDir := "/opt/kubexms/default_run_dir" // Fallback
	if cfg.Spec.Global.WorkDir != "" { // Assuming Global is not nil
		programBaseDir = cfg.Spec.Global.WorkDir
	}
	appFSBaseDir := filepath.Join(programBaseDir, ".kubexm")

	// Kubernetes Components
	kubeVersion := ""
	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "" {
		kubeVersion = cfg.Spec.Kubernetes.Version
	}

	if kubeVersion != "" {
		if taskSpec := taskKubeComponentsFactory.NewFetchKubeadmTask(cfg, kubeVersion, arch, zone, appFSBaseDir); taskSpec != nil {
			tasks = append(tasks, taskSpec)
		}
		if taskSpec := taskKubeComponentsFactory.NewFetchKubeletTask(cfg, kubeVersion, arch, zone, appFSBaseDir); taskSpec != nil {
			tasks = append(tasks, taskSpec)
		}
		if taskSpec := taskKubeComponentsFactory.NewFetchKubectlTask(cfg, kubeVersion, arch, zone, appFSBaseDir); taskSpec != nil {
			tasks = append(tasks, taskSpec)
		}
	}

	// Containerd
	containerdVersion := ""
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Type == "containerd" && cfg.Spec.ContainerRuntime.Version != "" {
		containerdVersion = cfg.Spec.ContainerRuntime.Version
	}

	if containerdVersion != "" {
		if taskSpec := taskKubeComponentsFactory.NewFetchContainerdTask(cfg, containerdVersion, arch, zone, appFSBaseDir); taskSpec != nil {
			tasks = append(tasks, taskSpec)
		}
	}

	// Condition string for IsEnabled. This logic mirrors the original function.
	// It assumes 'cfg' will be the context for evaluating this expression by the executor.
	isEnabledCondition := `(cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "") ||
						 (cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Type == "containerd" && cfg.Spec.ContainerRuntime.Version != "")`

	return &spec.ModuleSpec{
		Name:        "Kubernetes Components Download & Install",
		Description: "Fetches Kubernetes core components (kubeadm, kubelet, kubectl) and container runtime (containerd) binaries.",
		IsEnabled:   isEnabledCondition,
		Tasks:       tasks,
		PreRunHook:  "",
		PostRunHook: "",
	}
}
