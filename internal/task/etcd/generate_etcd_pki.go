package etcd

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	etcdcertsstep "github.com/mensylisir/kubexm/internal/step/etcd"
	"github.com/mensylisir/kubexm/internal/task"
)

type GenerateEtcdPKITask struct {
	task.Base
}

func NewGenerateEtcdPKITask() task.Task {
	return &GenerateEtcdPKITask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateEtcdPKI",
				Description: "Generate etcd CA and certificates for all etcd nodes",
			},
		},
	}
}

func (t *GenerateEtcdPKITask) Name() string                                     { return t.Meta.Name }
func (t *GenerateEtcdPKITask) Description() string                              { return t.Meta.Description }
func (t *GenerateEtcdPKITask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Only generate etcd PKI when kubexm is deploying etcd (binary mode)
	// For kubeadm type, kubeadm generates its own etcd certs
	// For external type, user provides their own etcd certs
	etcdSpec := ctx.GetClusterConfig().Spec.Etcd
	if etcdSpec == nil {
		return false, nil
	}
	return etcdSpec.Type == string(common.EtcdDeploymentTypeKubexm), nil
}

func (t *GenerateEtcdPKITask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHost := []remotefw.Host{controlNode}

	generateEtcdCA, err := etcdcertsstep.NewGenerateEtcdCAStepBuilder(runtimeCtx, "GenerateEtcdCA").Build()
	if err != nil {
		return nil, err
	}
	generateEtcdCerts, err := etcdcertsstep.NewGenerateEtcdCertsStepBuilder(runtimeCtx, "GenerateEtcdCerts").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateEtcdCA", Step: generateEtcdCA, Hosts: executionHost})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateEtcdCerts", Step: generateEtcdCerts, Hosts: executionHost})

	fragment.AddDependency("GenerateEtcdCA", "GenerateEtcdCerts")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
