###pkg/step - 原子执行单元
#### step.Step 接口: 定义所有 Step 必须实现的行为。
##### step的interface.go
```aiignore
package step

import (
    "github.com/mensylisir/kubexm/pkg/connector"
    "github.com/mensylisir/kubexm/pkg/runtime"
    "github.com/mensylisir/kubexm/pkg/spec"
)

// Step defines an atomic, idempotent execution unit.
type Step interface {
    // Meta returns the step's metadata.
    Meta() *spec.StepMeta

    // Precheck determines if the step's desired state is already met.
    // If it returns true, Run will be skipped.
    Precheck(ctx runtime.StepContext, host connector.Host) (isDone bool, err error)

    // Run executes the main logic of the step.
    Run(ctx runtime.StepContext, host connector.Host) error

    // Rollback attempts to revert the changes made by Run.
    // It's called only if Run fails.
    Rollback(ctx runtime.StepContext, host connector.Host) error
}
```
示例 Step 规格 - command.CommandStepSpec: 这是您提供的完美示例，它展示了一个 Step 规格应该如何设计：包含 StepMeta、所有配置字段，并实现 step.Step 接口。其他 Step 如 UploadFileStepSpec, InstallPackageStepSpec 等都将遵循此模式。