package kubexm

import (
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	etcdstep "github.com/mensylisir/kubexm/pkg/step/etcd"
	apiserverstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	controllerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	proxystep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	schedulerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	kubectlstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubectl"
	kubeletstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DownloadBinariesTask struct {
	task.Base
}

func NewDownloadBinariesTask() task.Task {
	return &DownloadBinariesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DownloadBinaries",
				Description: "Download all required binaries for Kubernetes components and Etcd",
			},
		},
	}
}

func (t *DownloadBinariesTask) Name() string {
	return t.Meta.Name
}

func (t *DownloadBinariesTask) Description() string {
	return t.Meta.Description
}

func (t *DownloadBinariesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return !ctx.IsOfflineMode(), nil
}

func (t *DownloadBinariesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHost := []connector.Host{controlNode}

	downloadSteps := map[string]step.Step{
		"DownloadEtcd":                  etcdstep.NewDownloadEtcdStepBuilder(*runtimeCtx, "DownloadEtcd").Build(),
		"DownloadKubeApiServer":         apiserverstep.NewDownloadKubeApiServerStepBuilder(*runtimeCtx, "DownloadKubeApiServer").Build(),
		"DownloadKubeControllerManager": controllerstep.NewDownloadKubeControllerManagerStepBuilder(*runtimeCtx, "DownloadKubeControllerManager").Build(),
		"DownloadKubeScheduler":         schedulerstep.NewDownloadKubeSchedulerStepBuilder(*runtimeCtx, "DownloadKubeScheduler").Build(),
		"DownloadKubelet":               kubeletstep.NewDownloadKubeletStepBuilder(*runtimeCtx, "DownloadKubelet").Build(),
		"DownloadKubeProxy":             proxystep.NewDownloadKubeProxyStepBuilder(*runtimeCtx, "DownloadKubeProxy").Build(),
		"DownloadKubectl":               kubectlstep.NewDownloadKubectlStepBuilder(*runtimeCtx, "DownloadKubectl").Build(),
	}

	for name, s := range downloadSteps {
		if s != nil {
			fragment.AddNode(&plan.ExecutionNode{Name: name, Step: s, Hosts: executionHost})
		}
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
