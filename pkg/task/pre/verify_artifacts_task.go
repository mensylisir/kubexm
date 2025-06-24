package pre

import (
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
	// commonsteps "github.com/mensylisir/kubexm/pkg/step/common"
)

// VerifyArtifactsTask checks the integrity of downloaded offline artifacts.
type VerifyArtifactsTask struct {
	task.BaseTask
	// ExpectedArtifacts map[string]string // map[filePath]expectedChecksum
	// ArtifactsDir string // Directory where artifacts are stored
}

// NewVerifyArtifactsTask creates a new VerifyArtifactsTask.
func NewVerifyArtifactsTask( /*expectedArtifacts map[string]string, artifactsDir string*/) task.Task {
	return &VerifyArtifactsTask{
		BaseTask: task.BaseTask{
			TaskName: "VerifyOfflineArtifacts",
			TaskDesc: "Verifies the checksums of offline installation artifacts.",
			// This task likely runs on the control node.
			// RunOnRoles: []string{common.ControlNodeRole},
		},
		// ExpectedArtifacts: expectedArtifacts,
		// ArtifactsDir: artifactsDir,
	}
}

func (t *VerifyArtifactsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	// Required if offline installation mode is specified in ClusterConfig.
	// Example: return ctx.GetClusterConfig().Spec.OfflineInstall, nil
	return true, nil // Placeholder
}

func (t *VerifyArtifactsTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
	// fragment := task.NewEmptyFragment()
	// controlHost, _ := ctx.GetControlNode()

	// For each artifact in t.ExpectedArtifacts:
	//   checksumStep := commonsteps.NewFileChecksumStep(
	//	   "Verify-"+filepath.Base(filePath),
	//	   filepath.Join(t.ArtifactsDir, filePath),
	//	   expectedChecksum,
	//	   "sha256", // Or configurable
	//   )
	//   nodeID := plan.NodeID("verify-artifact-" + filepath.Base(filePath))
	//   fragment.Nodes[nodeID] = &plan.ExecutionNode{
	//	   Name: "Verify " + filepath.Base(filePath),
	//	   Step: checksumStep,
	//	   Hosts: []connector.Host{controlHost},
	//   }
	//   fragment.EntryNodes = append(fragment.EntryNodes, nodeID)
	//   fragment.ExitNodes = append(fragment.ExitNodes, nodeID) // Can be parallel

	// For now, returning empty as specific artifact list/paths are not yet defined.
	return task.NewEmptyFragment(), nil
}

var _ task.Task = (*VerifyArtifactsTask)(nil)
