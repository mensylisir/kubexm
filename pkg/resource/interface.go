package resource

import (
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task" // For task.ExecutionFragment
)

// Handle is an abstraction for a resource that might need to be acquired (e.g., downloaded).
// It decouples the "need" for a resource from "how" it's obtained.
type Handle interface {
	// ID returns a unique identifier for this resource instance (e.g., including version, arch).
	// This can be used for caching and de-duplication of resource acquisition plans.
	ID() string

	// Path returns the expected final local path of the primary file/artifact of this resource
	// on the control node after it has been successfully acquired and prepared by EnsurePlan.
	// Tasks use this path as the source for subsequent steps like UploadFileStep.
	// Returns an error if the path cannot be determined (e.g. missing configuration).
	Path(ctx runtime.TaskContext) (string, error)

	// EnsurePlan generates an ExecutionFragment containing the necessary steps to
	// acquire and prepare the resource locally on the control node.
	// This method should be idempotent:
	//   - If the resource (identified by Path) already exists and is valid (e.g., checksum matches),
	//     it might return an empty fragment or a fragment indicating no action is needed.
	//   - Otherwise, it returns a fragment with steps like Download, Extract, etc.
	// The generated steps are intended to run on the control-node.
	EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error)
}
