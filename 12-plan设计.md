### 核心思想:
节点 (Node): 执行计划的基本单元是一个“节点”，每个节点代表一个 Action（即一个 Step 在一组 Host 上执行）。
边 (Edge): 节点之间的“边”代表依赖关系。一个节点必须等待其所有依赖的父节点都成功执行后才能开始。
无环 (Acyclic): 整个图必须是“有向无环图 (DAG)”，以避免循环依赖和死锁。
pkg/plan/graph_plan.go - 执行图计划定义
这个文件定义了要做什么 (What to do)，但以图的形式来组织。
##### pkg/plan/graph_plan.go
```aiignore
package plan

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/step"
)

// NodeID is the unique identifier for a node within the execution graph.
// It can be the same as the Action's name if names are guaranteed to be unique.
type NodeID string

// ExecutionGraph represents the entire set of operations and their dependencies.
// It is the primary input for a DAG-aware execution engine.
type ExecutionGraph struct {
	// A descriptive name for the overall plan.
	Name string `json:"name"`

	// Nodes is a map of all execution nodes in the graph, keyed by their unique ID.
	Nodes map[NodeID]*ExecutionNode `json:"nodes"`
}

// ExecutionNode represents a single, schedulable unit of work in the graph.
// It corresponds to what was previously an 'Action'.
type ExecutionNode struct {
	// A descriptive name for the node, e.g., "Upload etcd binary".
	Name string `json:"name"`

	// The Step to be executed. This contains the logic and configuration for the operation.
	Step step.Step `json:"-"`

	// The target hosts on which the Step will be executed.
	Hosts []connector.Host `json:"-"`

	// Dependencies lists the IDs of all nodes that must complete successfully
	// before this node can be scheduled for execution.
	Dependencies []NodeID `json:"dependencies"`

	// StepName is for marshalling/logging purposes, so we can see what step was used.
	StepName string `json:"stepName"`
}

// NewExecutionGraph creates an empty execution graph.
func NewExecutionGraph(name string) *ExecutionGraph {
	return &ExecutionGraph{
		Name:  name,
		Nodes: make(map[NodeID]*ExecutionNode),
	}
}

// AddNode adds a new execution node to the graph.
// It returns an error if a node with the same ID already exists.
func (g *ExecutionGraph) AddNode(id NodeID, node *ExecutionNode) error {
	if _, exists := g.Nodes[id]; exists {
		return fmt.Errorf("node with ID '%s' already exists in the execution graph", id)
	}
	g.Nodes[id] = node
	return nil
}

// Validate checks the graph for structural integrity, such as cyclic dependencies.
// This is a crucial step before execution.
func (g *ExecutionGraph) Validate() error {
	// Implementation would involve a cycle detection algorithm (e.g., using Depth First Search).
	// For each node, perform a DFS to see if it can reach itself.
	// This is a non-trivial but standard graph algorithm.
	// If a cycle is detected, return a descriptive error.
	return nil // Placeholder for actual validation logic
}
```
### 设计优化点:
从 Phase 到 Graph: ExecutionPlan 被 ExecutionGraph 替代，Phase 被完全移除。Action 的概念被 ExecutionNode 取代。
显式依赖: ExecutionNode 包含一个 Dependencies 字段，这是一个 NodeID 的切片。这明确地定义了执行顺序，取代了隐式的 Phase 顺序。
唯一标识符 (NodeID): 每个节点都有一个唯一的 NodeID，用于在图中引用它。这通常可以就是节点的 Name，但使用独立的类型 NodeID 提供了更大的灵活性。
图操作辅助函数: 提供了 NewExecutionGraph 和 AddNode 这样的辅助函数，使得 Task 层构建图更加方便和安全。
验证 (Validate): 图结构必须在执行前进行验证，最重要的是循环检测。Validate 方法为这个关键步骤提供了一个入口点。Engine 在执行前必须调用它。

pkg/plan/graph_result.go - 执行图结果定义
这个文件定义了图执行后发生了什么 (What happened)。结果的结构也需要相应地调整，以反映图的结构。
##### pkg/plan/graph_result.go
```aiignore
package plan

import (
	"time"
)

// Status remains the same as it's a universal concept.
type Status string

const (
	StatusPending Status = "Pending"
	StatusRunning Status = "Running"
	StatusSuccess Status = "Success"
	StatusFailed  Status = "Failed"
	StatusSkipped Status = "Skipped" // A node can be skipped if its dependencies fail.
)

// GraphExecutionResult is the top-level report for a graph-based execution.
type GraphExecutionResult struct {
	GraphName    string                    `json:"graphName"`
	StartTime    time.Time                 `json:"startTime"`
	EndTime      time.Time                 `json:"endTime"`
	Status       Status                    `json:"status"`
	NodeResults  map[NodeID]*NodeResult    `json:"nodeResults"`
}

// NodeResult captures the outcome of a single ExecutionNode's execution.
// It's equivalent to the old ActionResult.
type NodeResult struct {
	NodeName    string                 `json:"nodeName"`
	StepName    string                 `json:"stepName"`
	Status      Status                 `json:"status"`
	StartTime   time.Time              `json:"startTime"`
	EndTime     time.Time              `json:"endTime"`
	Message     string                 `json:"message,omitempty"` // e.g., "Skipped due to failed dependency 'node-X'"
	HostResults map[string]*HostResult `json:"hostResults"`     // Keyed by HostName. This structure remains the same.
}

// HostResult captures the outcome of a single step on a single host.
// This structure is fundamental and does not need to change.
type HostResult struct {
	HostName  string    `json:"hostName"`
	Status    Status    `json:"status"`
	Message   string    `json:"message"`
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Skipped   bool      `json:"skipped"` // Skipped due to precheck, not dependency failure.
}
```
#### 设计优化点:
从 PhaseResult 到 NodeResult: 结果的顶层结构现在是一个 map[NodeID]*NodeResult，直接反映了图的节点集合，而不是线性的阶段列表。
更丰富的 Skipped 语义: StatusSkipped 现在可以有两种含义：
- 在 HostResult 中，Skipped: true 意味着 Precheck 成功，该主机上的操作被跳过。
- 在 NodeResult 中，Status: StatusSkipped 意味着该节点因为其依赖的父节点失败而根本没有被调度执行。NodeResult.Message 字段可以用来解释这一点。
- 结果扁平化: 结果结构是扁平的 map，这使得查询特定节点的结果变得非常高效（O(1) 时间复杂度）。
对其他层的影响
- 这个改变对 pkg/plan 之外的层级有明确的影响：
  - pkg/task (决策层):
  - Task 的 Plan 方法现在需要返回 *plan.ExecutionGraph 而不是 *plan.ExecutionPlan。
  - Task 的核心职责变为：创建节点 (ExecutionNode) 并定义它们之间的依赖关系 (Dependencies)。例如，"Upload-Binary" 节点的 Dependencies 必须包含 "Download-Binary" 节点的ID。
  - pkg/engine (执行引擎):
    - 引擎的实现需要从一个简单的循环迭代器升级为一个图调度器 (Graph Scheduler)。
    - 核心算法:
      - 调用 graph.Validate() 确保无环。
      - 计算每个节点的入度（即依赖数量）。
      - 将所有入度为 0 的节点放入一个“可执行队列”。
      - 当有计算资源时，从队列中取出一个节点来执行。
      - 节点执行并发进行（可以使用 worker pool 或 errgroup）。
      - 当一个节点成功执行完毕后，遍历图中所有以它为依赖的下游节点，将它们的入度减 1。
      - 如果某个下游节点的入度变为 0，则将其放入“可执行队列”。
      - 如果一个节点执行失败，所有直接或间接依赖于它的下游节点的状态都将被标记为 Skipped，并且不会被执行。
      - 当队列为空且没有正在运行的节点时，整个图执行完毕。
      - 这个基于执行图的设计是业界领先的部署和编排工具（如 a lot of CI/CD systems, Airflow, etc.）所采用的核心模型，它提供了无与伦比的灵活性和执行效率。