pkg/engine - 执行引擎 (设计变更)
Engine 的接口现在明确表示它操作的是图 (Graph)。
engine.Engine 接口 (变更):
```aiignore
package engine

import (
    "github.com/mensylisir/kubexm/pkg/plan"
    "github.com/mensylisir/kubexm/pkg/runtime"
)

// Engine is responsible for executing a given ExecutionGraph.
// Its implementation must be a DAG scheduler.
type Engine interface {
    // Execute takes an ExecutionGraph and returns a GraphExecutionResult.
    Execute(ctx *runtime.Context, g *plan.ExecutionGraph, dryRun bool) (*plan.GraphExecutionResult, error)
}
```
设计解读:
接口的改变清晰地定义了 Engine 的新职责：它不再是一个简单的线性阶段执行器，而是一个能够处理复杂依赖关系的图调度器。
其实现将涉及拓扑排序、并发工作池、依赖跟踪等高级调度算法。总结：系统性的演进
层级	原模型 (线性)	新模型 (DAG)	核心职责变化
plan	ExecutionPlan ([]Phase)	ExecutionGraph (map[NodeID])	数据结构根本改变：从列表到图。
task	返回 *ExecutionPlan	返回 *ExecutionFragment	成为子图构建器，定义内部依赖和连接点。
module	返回 *ExecutionPlan	返回 *ExecutionFragment	成为图链接器，负责将任务子图拼接起来。
pipeline	返回 *ExecutionPlan	返回 *ExecutionGraph	成为最终图组装者，生成完整的、可执行的图。
engine	输入 *ExecutionPlan	输入 *ExecutionGraph	成为图调度器，负责按拓扑顺序并发执行节点。
这个全新的、基于DAG的设计，虽然在 Task 和 Module 的规划实现上更具挑战性，但它提供了一个高度灵活和高效的执行模型，是构建复杂、高性能自动化系统的坚实基础。