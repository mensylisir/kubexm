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

	programBaseDir := cfg.WorkDir
	if programBaseDir == "" {
		programBaseDir = "/opt/kubexms/default_workdir" // Fallback
	}
	// appFSBaseDir is the root for KubeXMS specific persistent data, like artifacts.
	appFSBaseDir := filepath.Join(programBaseDir, ".kubexm")

	// --- Kubernetes Components ---
	kubeVersion := ""
	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "" {
		kubeVersion = cfg.Spec.Kubernetes.Version
	}

	if kubeVersion != "" {
		tasks = append(tasks, taskKubeComponents.NewFetchKubeadmTask(cfg, kubeVersion, arch, zone, appFSBaseDir))
		tasks = append(tasks, taskKubeComponents.NewFetchKubeletTask(cfg, kubeVersion, arch, zone, appFSBaseDir))
		tasks = append(tasks, taskKubeComponents.NewFetchKubectlTask(cfg, kubeVersion, arch, zone, appFSBaseDir))
	}

	// --- Containerd ---
	containerdVersion := ""
	if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Version != "" {
		// Assuming ContainerRuntime.Type could be checked here if multiple runtimes were supported.
		// For now, if version is set, assume it's for containerd.
		containerdVersion = cfg.Spec.ContainerRuntime.Version
	}

	if containerdVersion != "" {
		tasks = append(tasks, taskKubeComponents.NewFetchContainerdTask(cfg, containerdVersion, arch, zone, appFSBaseDir))
	}

	return &spec.ModuleSpec{
		Name: "Kubernetes Components Download & Install",
		IsEnabled: func(currentCfg *config.Cluster) bool {
			// Enable if Kubernetes version is specified, implying it's a K8s cluster setup,
			// OR if a ContainerRuntime version is specified (as this module handles containerd).
			k8sEnabled := currentCfg != nil && currentCfg.Spec.Kubernetes != nil && currentCfg.Spec.Kubernetes.Version != ""
			containerRuntimeEnabled := currentCfg != nil && currentCfg.Spec.ContainerRuntime != nil && currentCfg.Spec.ContainerRuntime.Version != ""
			// Consider enabling if EITHER Kubernetes components OR a container runtime managed by this module is specified.
			// This module might be responsible for just containerd even in a non-K8s scenario or a custom K8s where only CR is needed from here.
			return k8sEnabled || containerRuntimeEnabled
		},
		Tasks: tasks,
	}
}
