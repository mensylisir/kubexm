package etcd

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	etcdstep "github.com/mensylisir/kubexm/pkg/step/pki/etcd"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateNewCertificatesTask struct {
	task.Base
}

func NewGenerateNewCertificatesTask() task.Task {
	return &GenerateNewCertificatesTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateNewCertificates",
				Description: "Generates new ETCD CA and/or leaf certificates on the control node",
			},
		},
	}
}

func (t *GenerateNewCertificatesTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateNewCertificatesTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateNewCertificatesTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	runtimeCtx := ctx.(*runtime.Context)
	caCacheKey := fmt.Sprintf(common.CacheKubexmEtcdCACertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	caRenewVal, _ := ctx.GetModuleCache().Get(caCacheKey)
	caRenew, _ := caRenewVal.(bool)
	leafCacheKey := fmt.Sprintf(common.CacheKubexmEtcdLeafCertRenew, runtimeCtx.GetRunID(), runtimeCtx.GetPipelineName(), runtimeCtx.GetModuleName(), t.Name())
	leafRenewVal, _ := ctx.GetModuleCache().Get(leafCacheKey)
	leafRenew, _ := leafRenewVal.(bool)

	return caRenew || leafRenew, nil
}

func (t *GenerateNewCertificatesTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())
	fragment := plan.NewExecutionFragment(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node for task %s: %w", t.Name(), err)
	}

	prepareAssetsStep := etcdstep.NewPrepareAssetsStepBuilder(*runtimeCtx, "PrepareEtcdCARenewalAssets").Build()
	resignCaStep := etcdstep.NewResignCAStepBuilder(*runtimeCtx, "ResignEtcdCA").Build()
	resignCertStep := etcdstep.NewGenerateNewLeafCertsStepBuilder(*runtimeCtx, "ResignEtcdCerts").Build()
	createBundleStep := etcdstep.NewPrepareCATransitionStepBuilder(*runtimeCtx, "CreateCABundle").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "PrepareAssets", Step: prepareAssetsStep, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "ResignEtcdCA", Step: resignCaStep, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateNewEtcdLeafCerts", Step: resignCertStep, Hosts: []connector.Host{controlNode}})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateCABundle", Step: createBundleStep, Hosts: []connector.Host{controlNode}})

	fragment.AddDependency("PrepareAssets", "ResignEtcdCA")
	fragment.AddDependency("PrepareAssets", "CreateCABundle")
	fragment.AddDependency("ResignEtcdCA", "GenerateNewEtcdLeafCerts")
	fragment.AddDependency("ResignEtcdCA", "CreateCABundle")
	fragment.CalculateEntryAndExitNodes()

	return fragment, nil
}

var _ task.Task = (*GenerateNewCertificatesTask)(nil)
