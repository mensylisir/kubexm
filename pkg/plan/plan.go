package plan

// This file is intentionally left sparse.
// Core graph types (NodeID, ExecutionGraph, ExecutionNode) and their methods
// are defined in graph_plan.go.
// Result types (Status, GraphExecutionResult, NodeResult, HostResult)
// are defined in graph_result.go.

// It can be used for future high-level planning interfaces or concepts
// not directly tied to graph structure or results.
// For now, it serves to ensure the package 'plan' exists and can be imported.

// Example of a non-conflicting definition that could live here:
// type Planner interface {
//    GeneratePlan(config interface{}) (*ExecutionGraph, error)
// }
