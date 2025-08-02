package kubernetes

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/task"
)

// PullImagesTask pre-pulls necessary container images on target nodes.
type PullImagesTask struct {
	task.BaseTask
}

// NewPullImagesTask creates a new PullImagesTask.
func NewPullImagesTask() task.Task {
	return &PullImagesTask{
		BaseTask: task.NewBaseTask(
			"PrePullKubernetesImages",
			"Pre-pulls core Kubernetes container images.",
			[]string{common.RoleMaster, common.RoleWorker},
			nil,
			false,
		),
	}
}

func (t *PullImagesTask) Plan(ctx task.TaskContext) (*task.ExecutionFragment, error) {
	logger := ctx.GetLogger().With("task", t.Name())
	fragment := task.NewExecutionFragment(t.Name())

	clusterCfg := ctx.GetClusterConfig()
	k8sVersion := clusterCfg.Spec.Kubernetes.Version
	if k8sVersion == "" {
		return nil, fmt.Errorf("kubernetes version is not specified in the cluster configuration")
	}

	// This list should be managed by the BOM in a real scenario.
	coreImages := []string{
		fmt.Sprintf("kube-apiserver:%s", k8sVersion),
		fmt.Sprintf("kube-controller-manager:%s", k8sVersion),
		fmt.Sprintf("kube-scheduler:%s", k8sVersion),
		fmt.Sprintf("kube-proxy:%s", k8sVersion),
		"pause:3.6", // This version is often fixed for a range of k8s versions.
		fmt.Sprintf("etcd:%s", "3.5.9-0"), // Example version
		"coredns/coredns:v1.9.3", // Example version
	}

	// The image registry (e.g., registry.k8s.io) should be prepended.
	imageRepo := common.DefaultImageRegistry
	if clusterCfg.Spec.Registry != nil && clusterCfg.Spec.Registry.Mirrors != nil {
		// A more complex logic would be needed to handle mirrors, rewrites, etc.
		// For now, we assume a single override.
		if len(clusterCfg.Spec.Registry.Mirrors) > 0 {
			imageRepo = clusterCfg.Spec.Registry.Mirrors[0]
		}
	}

	targetHosts, err := ctx.GetHostsByRole(t.GetRunOnRoles()...)
	if err != nil {
		return nil, err
	}

	var allPullNodes []plan.NodeID
	for _, host := range targetHosts {
		for _, imageName := range coreImages {
			fullImageName := fmt.Sprintf("%s/%s", imageRepo, imageName)

			pullStep := commonstep.NewPullImageStep(
				fmt.Sprintf("Pull-%s-on-%s", strings.ReplaceAll(imageName, ":", "-"), host.GetName()),
				fullImageName,
				5, // retries
				true,
			)
			pullNodeID, _ := fragment.AddNode(&plan.ExecutionNode{
				Name:  pullStep.Meta().Name,
				Step:  pullStep,
				Hosts: []connector.Host{host},
			})
			allPullNodes = append(allPullNodes, pullNodeID)
		}
	}

	// All pulls can happen in parallel, so they are all both entry and exit nodes.
	fragment.EntryNodes = allPullNodes
	fragment.ExitNodes = allPullNodes

	logger.Info("Kubernetes core images pull task planning complete.")
	return fragment, nil
}

var _ task.Task = (*PullImagesTask)(nil)
