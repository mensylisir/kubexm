package resource

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step/command" // For command.NewCommandStep
	"github.com/mensylisir/kubexm/pkg/task"
)

// RemoteImageHandle manages the pulling of a container image to specified hosts.
type RemoteImageHandle struct {
	// BaseImageName is the original name of the image, e.g., "kube-apiserver", "coredns/coredns".
	BaseImageName string
	// Version (tag) of the image, e.g., "v1.23.5", "1.8.6".
	Version string
	// Arch of the image. If empty, might not be used in the final image tag if the registry handles multi-arch tags.
	Arch string
	// RegistryOverride, if specified, replaces the original registry part of the image name.
	// e.g., "docker.io" might be replaced with "my.private.registry".
	RegistryOverride string
	// NamespaceOverride, if specified, replaces the original namespace part of the image name.
	// e.g., "library" or "google_containers" might be replaced with "mycustomnamespace".
	NamespaceOverride string
	// TargetRoles is a list of host roles where this image should be pulled.
	// If empty, it might default to all hosts or specific roles based on context.
	TargetRoles []string
}

// NewRemoteImageHandle creates a new RemoteImageHandle.
func NewRemoteImageHandle(baseImageName, version, arch string, registryOverride, namespaceOverride string, targetRoles []string) Handle {
	return &RemoteImageHandle{
		BaseImageName:     baseImageName,
		Version:           version,
		Arch:              arch, // Arch might be part of the tag or image name itself for some registries
		RegistryOverride:  registryOverride,
		NamespaceOverride: namespaceOverride,
		TargetRoles:       targetRoles,
	}
}

// getFullImageName constructs the final image name to be pulled, considering overrides.
func (h *RemoteImageHandle) getFullImageName(cfg *v1alpha1.RegistryConfig) string {
	imageRef := h.BaseImageName
	if !strings.Contains(imageRef, ":") && h.Version != "" {
		imageRef = fmt.Sprintf("%s:%s", imageRef, h.Version)
	}

	// TODO: More sophisticated image name parsing and manipulation might be needed.
	// This is a simplified approach. Libraries like Docker's distribution/reference
	// or containerd/containerd/reference/spec.go are more robust.

	parts := strings.SplitN(imageRef, "/", 3)
	registry := ""
	repoAndTag := imageRef

	if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) { // Heuristic for registry
		registry = parts[0]
		repoAndTag = strings.Join(parts[1:], "/")
	}

	namespace := ""
	imageNameAndTag := repoAndTag
	if strings.Contains(repoAndTag, "/") {
		nsParts := strings.SplitN(repoAndTag, "/", 2)
		namespace = nsParts[0]
		imageNameAndTag = nsParts[1]
	}

	finalRegistry := registry
	if h.RegistryOverride != "" {
		finalRegistry = h.RegistryOverride
	} else if cfg != nil && cfg.PrivateRegistry != "" { // Use global private registry if no specific override
		finalRegistry = cfg.PrivateRegistry
	}

	finalNamespace := namespace
	if h.NamespaceOverride != "" {
		finalNamespace = h.NamespaceOverride
	} else if cfg != nil && cfg.NamespaceOverride != "" { // Use global namespace override
		finalNamespace = cfg.NamespaceOverride
	}

	// Reconstruct the image name
	var finalImageName strings.Builder
	if finalRegistry != "" {
		finalImageName.WriteString(finalRegistry)
		finalImageName.WriteString("/")
	}
	if finalNamespace != "" {
		finalImageName.WriteString(finalNamespace)
		finalImageName.WriteString("/")
	}
	finalImageName.WriteString(imageNameAndTag)

	return finalImageName.String()
}

func (h *RemoteImageHandle) ID() string {
	// ID should be unique based on the final image name and target scope (though scope isn't part of ID here)
	// For simplicity, using BaseImageName and Version. A more robust ID might use getFullImageName.
	return fmt.Sprintf("image-%s-%s-%s", strings.ReplaceAll(h.BaseImageName, "/", "-"), h.Version, h.Arch)
}

// Path for an image handle might not be a filesystem path, but the full image reference string.
func (h *RemoteImageHandle) Path(ctx runtime.TaskContext) (string, error) {
	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg == nil {
		return "", fmt.Errorf("cluster config is nil in TaskContext for image %s", h.ID())
	}
	return h.getFullImageName(clusterCfg.Spec.Registry), nil
}

func (h *RemoteImageHandle) Type() string {
	return "remote-image"
}

// EnsurePlan generates an ExecutionFragment to pull the image on target hosts.
func (h *RemoteImageHandle) EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("resource_id", h.ID(), "image_base_name", h.BaseImageName)
	logger.Info("Planning resource assurance for remote image...")

	clusterCfg := ctx.GetClusterConfig()
	if clusterCfg == nil {
		return nil, fmt.Errorf("cluster config is nil, cannot plan image pull for %s", h.ID())
	}

	fullImageName, err := h.Path(ctx) // Path returns the full image name
	if err != nil {
		return nil, fmt.Errorf("failed to determine full image name for %s: %w", h.ID(), err)
	}

	var targetHosts []connector.Host
	if len(h.TargetRoles) == 0 { // If no specific roles, pull on all nodes
		for _, hr := range ctx.GetClusterConfig().Spec.Hosts {
			// Need to convert v1alpha1.HostSpec to connector.Host
			// This requires the runtime.Context or builder to have already done this conversion
			// and made them available, e.g., via ctx.GetHostsByRole("all") or similar.
			// For now, let's assume TaskContext can provide all hosts.
			// This highlights a dependency on how hosts are accessed.
			// Let's assume a method ctx.GetAllHosts() exists for this scenario.
			allHosts, err := ctx.GetHostsByRole(common.AllHostsRole) // Assuming "all" is a way to get all hosts
			if err != nil {
				return nil, fmt.Errorf("failed to get all hosts for image pull %s: %w", fullImageName, err)
			}
			targetHosts = allHosts
			if len(targetHosts) == 0 {
                 logger.Warn("No hosts found for pulling image (TargetRoles empty, GetAllHosts returned empty).", "image", fullImageName)
                 // Return empty fragment if no hosts
                 return &task.ExecutionFragment{Nodes: make(map[plan.NodeID]*plan.ExecutionNode)}, nil
            }
		}
	} else {
		for _, role := range h.TargetRoles {
			hostsInRole, err := ctx.GetHostsByRole(role)
			if err != nil {
				return nil, fmt.Errorf("failed to get hosts for role '%s' for image pull %s: %w", role, fullImageName, err)
			}
			targetHosts = append(targetHosts, hostsInRole...)
		}
		if len(targetHosts) == 0 {
			logger.Warn("No hosts found for specified TargetRoles for pulling image.", "image", fullImageName, "roles", h.TargetRoles)
			return &task.ExecutionFragment{Nodes: make(map[plan.NodeID]*plan.ExecutionNode)}, nil
		}
		// Deduplicate hosts if multiple roles map to the same host
		targetHosts = DeduplicateHosts(targetHosts)
	}


	nodes := make(map[plan.NodeID]*plan.ExecutionNode)
	var entryNodes, exitNodes []plan.NodeID // All pull nodes can be entry and exit if parallel

	// TODO: Handle registry authentication (e.g., from clusterCfg.Spec.Registry.Auths)
	// This might involve creating temporary credential files or using specific crictl flags.
	// For now, assuming public images or pre-configured node authentication.

	for _, host := range targetHosts {
		nodeID := plan.NodeID(fmt.Sprintf("pull-image-%s-on-%s", strings.ReplaceAll(fullImageName, "/", "-"), host.GetName()))

		// Determine container runtime from host facts or global config
		// For simplicity, assume crictl is available and works with the configured runtime.
		// A more robust solution might check facts.ContainerRuntime.Type
		pullCmd := fmt.Sprintf("crictl pull %s", fullImageName)

		// Precheck: crictl images --quiet --name <imageName> (or similar)
		// If image already exists, skip. This makes the operation idempotent.
		// crictl inspecti <imageName> > /dev/null 2>&1
		checkCmd := fmt.Sprintf("crictl inspecti %s > /dev/null 2>&1", fullImageName)

		pullImageStep := command.NewCommandStep(pullCmd, checkCmd, "", true, "", 0, nil)

		nodes[nodeID] = &plan.ExecutionNode{
			Name:     fmt.Sprintf("Pull image %s on host %s", fullImageName, host.GetName()),
			Step:     pullImageStep,
			Hosts:    []connector.Host{host}, // Each node targets one host
			StepName: pullImageStep.Meta().Name,
			// No dependencies between these image pull operations on different hosts, they can run in parallel.
		}
		entryNodes = append(entryNodes, nodeID)
		exitNodes = append(exitNodes, nodeID)
	}

	logger.Info("Remote image resource assurance plan created.", "image", fullImageName, "target_host_count", len(targetHosts))
	return &task.ExecutionFragment{
		Nodes:      nodes,
		EntryNodes: entryNodes, // All are entry points
		ExitNodes:  exitNodes,  // All are exit points
	}, nil
}

// DeduplicateHosts removes duplicate hosts from a slice of connector.Host.
func DeduplicateHosts(hosts []connector.Host) []connector.Host {
	seen := make(map[string]bool)
	result := []connector.Host{}
	for _, host := range hosts {
		if _, ok := seen[host.GetName()]; !ok {
			seen[host.GetName()] = true
			result = append(result, host)
		}
	}
	return result
}


var _ Handle = (*RemoteImageHandle)(nil)
