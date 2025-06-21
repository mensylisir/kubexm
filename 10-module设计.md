### pkg/module - 战略组合单元 (设计变更)
#### Module 的核心职责变成了图的链接器 (Graph Linker)。它获取其下所有 Task 生成的 ExecutionFragment，并将它们按逻辑顺序（或依赖关系）拼接成一个更大的 ExecutionFragment。
#### module.Module 接口 (变更):
##### Plan 方法的返回值也变成了 ExecutionFragment。
###### interface.go
```aiignore
package module

import (
    "github.com/mensylisir/kubexm/pkg/runtime"
    "github.com/mensylisir/kubexm/pkg/task"
)

// ModuleContext defines the methods available at the module execution level.
type ModuleContext interface {
	// Methods previously from pipeline.PipelineContext that modules need:
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	PipelineCache() cache.PipelineCache
	GetGlobalWorkDir() string
	GetEngine() engine.Engine // Consider if modules really need direct engine access

	// Module-specific methods:
	ModuleCache() cache.ModuleCache
}

type Module interface {
    Name() string
    Tasks() []task.Task
    
    // Plan now aggregates fragments from its tasks into a larger fragment.
    // It is responsible for linking the exit nodes of one task to the
    // entry nodes of the next, creating dependencies.
    Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error)
}
```
### 设计解读:
- Module 的 Plan 方法实现会比较复杂。它需要：
  - 遍历所有 Task 并调用它们的 Plan 方法，收集所有的 ExecutionFragment。
  - 将所有 fragment.Nodes 合并到一个大的 map 中。
  - 核心逻辑: 创建依赖。例如，对于一个线性的模块，它会将 task1.ExitNodes 作为 task2.EntryNodes 的依赖。具体做法是，遍历 task2.EntryNodes，为每个入口节点的 Dependencies 字段追加上 task1.ExitNodes。
  - 最后，返回一个合并后的、新的 ExecutionFragment，其 EntryNodes 是第一个 Task 的入口，ExitNodes 是最后一个 Task 的出口。