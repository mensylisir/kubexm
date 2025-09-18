package pre

import (
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	prestep "github.com/mensylisir/kubexm/pkg/step/pre"
	"github.com/mensylisir/kubexm/pkg/task"
)

type VerifyArtifactsTask struct {
	task.Base
}

func NewVerifyArtifactsTask() task.Task {
	return &VerifyArtifactsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "VerifyArtifacts",
				Description: "Verify the integrity of all downloaded artifacts using checksums",
			},
		},
	}
}

func (t *VerifyArtifactsTask) Name() string {
	return t.Meta.Name
}

func (t *VerifyArtifactsTask) Description() string {
	return t.Meta.Description
}

func (t *VerifyArtifactsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// This task should run if there are any artifacts defined with checksums.
	// For now, let's assume it's always required if not explicitly skipped.
	// A more advanced implementation would inspect the config.
	return true, nil
}

func (t *VerifyArtifactsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	// This step runs on the control node, where the artifacts are downloaded.
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	// In a real implementation, this list would be dynamically generated
	// from the Bill of Materials (BOM) or cluster spec.
	// For now, we create a placeholder list.
	// TODO: Replace this with dynamic artifact collection.
	filesToVerify := []prestep.FileChecksum{
		// Example:
		// {
		// 	Path:     "/kubexm/artifacts/etcd-v3.5.4-linux-amd64.tar.gz",
		// 	Checksum: "expected_checksum_here",
		// 	Algo:     "sha256",
		// },
	}

	// If there are no files to verify, we can return an empty fragment.
	if len(filesToVerify) == 0 {
		ctx.GetLogger().Info("No artifacts with checksums found to verify. Skipping task.")
		return fragment, nil
	}

	verifyStep := prestep.NewVerifyChecksumsStepBuilder(*runtimeCtx, "VerifyArtifactChecksums").
		WithFiles(filesToVerify).
		Build()

	node := &plan.ExecutionNode{
		Name:  "VerifyArtifactChecksumsNode",
		Step:  verifyStep,
		Hosts: []connector.Host{controlNode},
	}

	fragment.AddNode(node)
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
