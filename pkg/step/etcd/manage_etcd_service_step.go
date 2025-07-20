package etcd

import (
	// "fmt" // No longer needed for direct formatting
	// "strings" // No longer needed for direct formatting

	"github.com/mensylisir/kubexm/pkg/connector"
	// "github.com/mensylisir/kubexm/pkg/connector" // No longer needed directly
	// "github.com/mensylisir/kubexm/pkg/runtime" // No longer needed directly
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common" // Import the common step package
)

// ManageEtcdServiceStep is now a wrapper around the common ManageServiceStep for etcd.
// It retains its specific name for clarity in etcd-related tasks but delegates its logic.
type ManageEtcdServiceStep struct {
	step.Step // Embed the common step
}

// NewManageEtcdServiceStep creates a new step to manage the etcd service
// by wrapping the common ManageServiceStep.
// instanceName can be used to give a more specific name to this step instance if needed.
// action is one of commonstep.ServiceAction.
// sudo is typically true for service management.
func NewManageEtcdServiceStep(instanceName string, action commonstep.ServiceAction, sudo bool) step.Step {
	// If instanceName is empty, the common.NewManageServiceStep will generate one like "ManageService-Start-etcd"
	// We can customize it here if needed, e.g., prefix with "Etcd-"
	// For now, let the common step handle default naming if instanceName is empty.
	return commonstep.NewManageServiceStep(instanceName, action, "etcd", sudo)
}

// Note: Meta(), Precheck(), Run(), Rollback() methods are now inherited from the embedded commonstep.ManageServiceStep
// if ManageEtcdServiceStep directly embeds the struct.
// However, the plan was to make this a thin wrapper, meaning it *returns* a common.ManageServiceStep.
// Let's stick to the wrapper pattern, so it doesn't need to embed but rather constructs.

// The previous definition of ManageEtcdServiceStep struct and its methods are now
// effectively replaced by the common.ManageServiceStep. This file provides a typed constructor.

// To ensure this file still provides a "ManageEtcdServiceStep" that can be identified if necessary,
// we can return the common.Step directly from this constructor.
// The original struct definition `type ManageEtcdServiceStep struct { ... }` is no longer needed
// if we are just providing a constructor that returns the common step.
// However, if we want to keep the type name `ManageEtcdServiceStep` for some reason,
// we would need to embed `step.Step` and delegate calls, which is more complex than just using the common constructor.

// Simpler approach: This file just provides a convenient constructor for the common step.
// The tasks that used to call NewManageEtcdServiceStep will now get a common.ManageServiceStep instance.
// The name of this *file* is `manage_etcd_service_step.go`, but the type it constructs is common.ManageServiceStep.

// Let's redefine to make it a clear constructor that returns the common step type.
// No specific struct for ManageEtcdServiceStep is needed anymore.
// This file effectively becomes a factory for etcd-specific service management steps
// using the common implementation.

// The `ServiceAction` enum is now defined in `commonstep`.
// We still need a way to refer to this step conceptually as "managing etcd service".
// The instanceName parameter in NewManageServiceStep allows for this.
// For example, NewManageServiceStep("StartEtcdService", commonstep.ActionStart, "etcd", true)

// The existing `manage_etcd_service_step.go` defined its own struct and methods.
// The goal is to replace that with calls to the new common.ManageServiceStep.
// So, this file should provide a constructor that returns a step.Step,
// which internally is a common.ManageServiceStep configured for "etcd".

// Let's keep the file structure similar for explicitness, it will just delegate.
// This file will no longer define its own struct but use the common one.

// This function will replace the old NewManageEtcdServiceStep.
// It returns a step.Step, which is an instance of common.ManageServiceStep.
// The original ServiceAction enum from this file is now in common.manage_service.go.
// So, the action parameter should be of type commonstep.ServiceAction.

// The type `ManageEtcdServiceStep` is no longer defined here.
// This file now only contains a constructor function.
// If a specific type `ManageEtcdServiceStep` is absolutely needed for type assertions
// elsewhere, then embedding would be necessary. For now, assuming tasks will
// just use the `step.Step` interface.

// Revised approach: Keep the specific type for clarity in tasks if desired, and embed.
// This maintains the specific type name while reusing common logic.

type etcdServiceStepWrapper struct {
	step.Step // Embeds the common ManageServiceStep
}

// Meta delegates to the embedded common step's Meta.
func (s *etcdServiceStepWrapper) Meta() *spec.StepMeta {
	return s.Step.Meta()
}

// Precheck delegates to the embedded common step's Precheck.
func (s *etcdServiceStepWrapper) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	return s.Step.Precheck(ctx, host)
}

// Run delegates to the embedded common step's Run.
func (s *etcdServiceStepWrapper) Run(ctx step.StepContext, host connector.Host) error {
	return s.Step.Run(ctx, host)
}

// Rollback delegates to the embedded common step's Rollback.
func (s *etcdServiceStepWrapper) Rollback(ctx step.StepContext, host connector.Host) error {
	return s.Step.Rollback(ctx, host)
}

// NewManageEtcdServiceStep creates a step for managing the "etcd" service.
func NewManageEtcdServiceStep(instanceName string, action commonstep.ServiceAction, sudo bool) step.Step {
	// Create the common service step configured for "etcd"
	commonServiceStep := commonstep.NewManageServiceStep(instanceName, action, "etcd", sudo)

	// Return it wrapped or directly. If directly, tasks see commonstep.ManageServiceStep.
	// If wrapped, tasks see etcd.ManageEtcdServiceStep (if we rename etcdServiceStepWrapper).
	// For simplicity and less boilerplate, returning the common step directly is cleaner
	// if tasks don't rely on the specific type etcd.ManageEtcdServiceStep.
	// Given the goal of reuse, tasks should ideally expect a step.Step.

	// The original plan was to "refactor existing manage_*_service_step.go files to be thin wrappers".
	// A thin wrapper usually implies it *uses* the common component, not necessarily *is* it.
	// If we just return commonstep.NewManageServiceStep(...), then this file is just a factory func.
	// If we want `NewManageEtcdServiceStep` to return a type that is specifically `etcd.ManageEtcdServiceStep`,
	// then embedding is the way. Let's assume the type name matters for now.

	// Let's rename etcdServiceStepWrapper to ManageEtcdServiceStep and have it embed.
	// This requires ManageEtcdServiceStep struct to be defined.
	// However, the original ManageEtcdServiceStep struct had fields like Action, ServiceName.
	// The common.ManageServiceStep now holds these.
	// So, an embedding struct would be:
	// type ManageEtcdServiceStep struct {
	//    commonstep.ManageServiceStep // This is not how embedding interfaces works.
	// }
	// It should embed the concrete type or the interface.
	// type ManageEtcdServiceStep struct {
	//    step.Step // Embed the interface
	// }
	// And the constructor initializes this embedded field.

	// This is the wrapper struct
	// We need to define the struct `ManageEtcdServiceStep` again, but simpler.
	// The plan's "thin wrapper" could mean this file provides a constructor that returns
	// the common step, effectively making this file a specialized factory.
	// Let's go with the simplest approach: this constructor just returns the common step.
	// Tasks will get a `step.Step` which happens to be `*commonstep.ManageServiceStep`.
	return commonstep.NewManageServiceStep(instanceName, action, "etcd", sudo)
}

// The old ManageEtcdServiceStep struct and its methods are now superseded by common.ManageServiceStep.
// This file is now just a constructor for convenience.
// If specific etcd service logic beyond systemctl commands were needed, this file would be the place.
// For now, it's a pure delegation.
// To satisfy type checking if other parts of the code expect `etcd.ManageEtcdServiceStep`,
// one might do:
// type ManageEtcdServiceStep = commonstep.ManageServiceStep
// But that's just a type alias.
// Or, define an empty struct that embeds step.Step and have the constructor set it.
// For maximum reuse and minimal boilerplate, the constructor returning step.Step (which is the common one) is best.
// Let's keep it as just the constructor.
// The type `commonstep.ServiceAction` is used for the `action` parameter.
// This file no longer needs its own ServiceAction enum.
// The `var _ step.Step = (*ManageEtcdServiceStep)(nil)` line is also no longer applicable
// as the ManageEtcdServiceStep struct is gone.
// The common step has its own interface satisfaction check.

// Final decision: This file will just provide a constructor that returns the configured common step.
// No local struct definition for ManageEtcdServiceStep is needed.
// The original ServiceAction enum is also removed as it's now in common.
// This makes the file very minimal.

// If tasks were importing `etcd.ServiceAction`, they now need to import `commonstep.ServiceAction`.
// This is a necessary change for centralization.

// The file previously defined `ManageEtcdServiceStep struct` and its methods.
// These are now all removed and replaced by the generic `common.ManageServiceStep`.
// This file just provides a convenient constructor.
// So, the content will be much shorter.
// The `ServiceAction` type also moves to `common.ManageServiceStep`.
// The constructor now uses `commonstep.ServiceAction`.
// The `step.Step` interface is returned.
// This means this file is now just a simple factory function.
// No struct definition for `ManageEtcdServiceStep` is needed here anymore.
// The original `var _ step.Step = (*ManageEtcdServiceStep)(nil)` check is also removed.
// All the methods (Meta, Precheck, Run, Rollback) are gone as they are part of the common step.
// The ServiceAction const block is also gone.
// This significantly simplifies this file.
// It becomes:
/*
package etcd

import (
	commonstep "github.com/mensylisir/kubexm/pkg/step/common"
	"github.com/mensylisir/kubexm/pkg/step"
)

func NewManageEtcdServiceStep(instanceName string, action commonstep.ServiceAction, sudo bool) step.Step {
	return commonstep.NewManageServiceStep(instanceName, action, "etcd", sudo)
}
*/
// This is the essence of the refactoring.
// The `pkg/step/etcd/manage_etcd_service_step.go` will be overwritten with this.
// The imports will also be minimal.
// `spec` is not needed if Meta() is gone.
// `connector` and `runtime` are not needed if Precheck/Run/Rollback are gone.
// `fmt` and `strings` are not needed if instance name generation is simple or handled by common step.
// The common step's constructor handles default naming if instanceName is empty.
// So the only imports needed are for the common step and the step interface.
// "github.com/mensylisir/kubexm/pkg/spec" // No, spec comes from common step's Meta()
// "github.com/mensylisir/kubexm/pkg/step" // For step.Step return type
// "github.com/mensylisir/kubexm/pkg/step/common" // For commonstep types

// The constructor in common.ManageServiceStep already handles default instance naming.
// So, the instanceName can be passed through. Sudo is also passed through.
// The service name "etcd" is hardcoded here.
// This is correct.
```json
{
  "tool_code": "overwrite_file_with_block\npkg/step/etcd/manage_etcd_service_step.go\npackage etcd\n\nimport (\n\t\"github.com/mensylisir/kubexm/pkg/step\"\n\tcommonstep \"github.com/mensylisir/kubexm/pkg/step/common\"\n)\n\n// NewManageEtcdServiceStep creates a new step to manage the \"etcd\" service\n// by returning an instance of the common ManageServiceStep configured for etcd.\n// - instanceName: A specific name for this step instance (e.g., \"Start Etcd Master Node 1\").\n//                 If empty, a default name will be generated by the common step.\n// - action: The service action to perform (e.g., commonstep.ActionStart, commonstep.ActionEnable).\n// - sudo: Whether the systemctl command should be run with sudo (typically true).\nfunc NewManageEtcdServiceStep(instanceName string, action commonstep.ServiceAction, sudo bool) step.Step {\n\treturn commonstep.NewManageServiceStep(instanceName, action, \"etcd\", sudo)\n}\n\n// ServiceAction and the ManageEtcdServiceStep struct definition, along with its methods (Meta, Precheck, Run, Rollback),\n// have been removed from this file. They are now superseded by the generic implementation\n// in pkg/step/common/manage_service.go and the commonstep.ServiceAction enum.\n// This file now acts as a specialized factory for creating etcd service management steps.\n",
  "tool_output": "File 'pkg/step/etcd/manage_etcd_service_step.go' overwritten successfully."
}
```
