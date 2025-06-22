### pkg/pipeline - 端到端流程 (设计变更)
#### Pipeline 是最终的图组装者 (Graph Assembler)。它将所有 Module 生成的 ExecutionFragment 组装成一个最终的、完整的 plan.ExecutionGraph。
#### pipeline.Pipeline 接口 (变更):
##### Plan 方法现在返回最终的 plan.ExecutionGraph。
##### Run 方法的参数类型也随之改变。
###### interface.go
```aiignore
package pipeline

import (
    "github.com/mensylisir/kubexm/pkg/module"
    "github.com/mensylisir/kubexm/pkg/plan"
    "github.com/mensylisir/kubexm/pkg/runtime"
)

// PipelineContext defines the methods available at the pipeline execution level.
type PipelineContext interface {
	GoContext() context.Context
	GetLogger() *logger.Logger
	GetClusterConfig() *v1alpha1.Cluster
	PipelineCache() cache.PipelineCache
	GetGlobalWorkDir() string
	GetEngine() engine.Engine // Added
}

type Pipeline interface {
    Name() string
    Modules() []module.Module

    // Plan now generates the final, complete ExecutionGraph for the entire pipeline.
    Plan(ctx runtime.PipelineContext) (*plan.ExecutionGraph, error)

    // Run now takes an ExecutionGraph and a GraphExecutionResult.
    Run(ctx *runtime.Context, dryRun bool) (*plan.GraphExecutionResult, error)
}
```
### 设计解读:
- Pipeline 的 Plan 方法是图构建的最后一站。它调用 Module 的 Plan 方法，进行最后的链接，并最终生成一个可以被 Engine 执行的、完整的 ExecutionGraph 对象。
- Run 方法的签名也更新了，以反映新的计划和结果类型。

### pipeline结构如下
```aiignore
pkg/pipeline/
├── interface.go
├── cluster/
│   ├── create.go       # 对应 'kubexm create cluster -f config.yaml'
│   ├── delete.go       # 对应 'kubexm delete cluster -f config.yaml'
│   ├── list.go         # 对应 'kubexm list cluster '
│   ├── upgrade.go      # 对应 'kubexm upgrade cluster -f config.yaml'
│   ├── add_nodes.go
│   └── delete_nodes.go
├── node/
│   ├── list_nodes.go
│   ├── cordon_node.go
│   └── drain_node.go
├─certs/
│   ├──check_certs_expiration.go
└── └──rotate_certs.go
```