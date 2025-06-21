package plan

import (
	"fmt"
	"time" // Added for result timestamps

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

	// EntryNodes are the IDs of nodes in this graph that have no dependencies.
	// These are the starting points for execution.
	// While derivable, storing them can be a convenience for the engine.
	EntryNodes []NodeID `json:"entryNodes,omitempty"`
}

// ExecutionNode represents a single, schedulable unit of work in the graph.
// It corresponds to what was previously an 'Action'.
type ExecutionNode struct {
	// A descriptive name for the node, e.g., "Upload etcd binary".
	Name string `json:"name"`

	// The Step to be executed. This contains the logic and configuration for the operation.
	// Marked as json:"-" because Step interface cannot be directly marshalled.
	// StepName is used for marshalling/logging.
	Step step.Step `json:"-"`

	// The target hosts on which the Step will be executed.
	// Marked as json:"-" because Host interface cannot be directly marshalled.
	// HostNames are used for marshalling/logging if needed, or derived from Host objects.
	Hosts []connector.Host `json:"-"`

	// Dependencies lists the IDs of all nodes that must complete successfully
	// before this node can be scheduled for execution.
	Dependencies []NodeID `json:"dependencies"`

	// StepName is for marshalling/logging purposes, so we can see what step was used.
	StepName string `json:"stepName"`

	// HostNames is for marshalling/logging, storing the names of the target hosts.
	HostNames []string `json:"hostNames"`
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
	if node.HostNames == nil && node.Hosts != nil {
		node.HostNames = make([]string, len(node.Hosts))
		for i, h := range node.Hosts {
			node.HostNames[i] = h.GetName()
		}
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

	// Placeholder for actual validation logic:
	// 1. Check for missing node dependencies.
	// 2. Detect cycles.
	for id, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := g.Nodes[depID]; !exists {
				return fmt.Errorf("node '%s' has an undefined dependency '%s'", id, depID)
			}
		}
	}
	// Basic cycle detection (can be enhanced)
	visited := make(map[NodeID]bool)
	recursionStack := make(map[NodeID]bool)

	for id := range g.Nodes {
		if !visited[id] {
			if g.hasCycle(id, visited, recursionStack) {
				return fmt.Errorf("cycle detected in execution graph involving node '%s'", id)
			}
		}
	}
	return nil
}

// hasCycle is a helper for DFS-based cycle detection
func (g *ExecutionGraph) hasCycle(nodeID NodeID, visited map[NodeID]bool, recursionStack map[NodeID]bool) bool {
	visited[nodeID] = true
	recursionStack[nodeID] = true

	node := g.Nodes[nodeID]
	for _, depID := range node.Dependencies {
		// In a typical DAG for execution, dependencies point from child to parent (child depends on parent).
		// Or, if dependencies are "tasks that must run before this one", then edges are parent->child.
		// The current `Dependencies` field means "nodes that must complete BEFORE this one".
		// So, an edge exists from each Dependency TO this node.
		// For cycle detection, we need to traverse along these "depends on" relationships.
		// The way `Dependencies` is defined, it lists prerequisites.
		// A cycle exists if by following prerequisites, we reach the starting node.

		// Let's adjust the interpretation for standard DFS cycle detection:
		// Consider the graph where an edge A -> B means A must complete before B.
		// So, node.Dependencies lists incoming edges.
		// To detect cycles, we should look for paths from a node to itself.
		// The current `Dependencies` are parents. We need to traverse to children.
		// This requires building an adjacency list of children for each node.

		// Let's redefine: For cycle detection, we are looking for a path A -> B -> C -> A.
		// If node D depends on C (D.Dependencies contains C), C depends on B, B depends on A.
		// If A depends on D, then we have a cycle.
		// The DFS should follow the "depends on" links.

		// If the current node's dependency (prerequisite) is already in the recursion stack, it's a cycle.
		// This seems wrong. `Dependencies` are prerequisites.
		// We need to build the graph's adjacency list (outgoing edges) to do standard cycle detection.

		// Let's assume the definition: `Dependencies` are nodes that this node depends on (parents).
		// To detect a cycle A -> B -> A (B depends on A, A depends on B):
		// When visiting A: A is on stack. Check A's dependencies. Say B.
		// When visiting B: B is on stack. Check B's dependencies. Say A. A is on stack -> cycle.

		// This seems correct:
		if recursionStack[depID] {
			return true // Found a back edge to a node in the current recursion path
		}
		if !visited[depID] {
			if g.hasCycle(depID, visited, recursionStack) {
				return true
			}
		}
	}

	recursionStack[nodeID] = false
	return false
}


// Status defines the execution status of a plan, node, or host operation.
type Status string

const (
	StatusPending Status = "Pending"
	StatusRunning Status = "Running"
	StatusSuccess Status = "Success"
	StatusFailed  Status = "Failed"
	StatusSkipped Status = "Skipped" // A node can be skipped if its dependencies fail or precheck is true.
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
type NodeResult struct {
	NodeName    string                 `json:"nodeName"`
	StepName    string                 `json:"stepName"` // From ExecutionNode.StepName
	Status      Status                 `json:"status"`
	StartTime   time.Time              `json:"startTime"`
	EndTime     time.Time              `json:"endTime"`
	Message     string                 `json:"message,omitempty"` // e.g., "Skipped due to failed dependency 'node-X'" or error message if node failed
	HostResults map[string]*HostResult `json:"hostResults"`     // Keyed by HostName.
}

// HostResult captures the outcome of a single step on a single host.
type HostResult struct {
	HostName  string    `json:"hostName"`
	Status    Status    `json:"status"`
	Message   string    `json:"message,omitempty"` // Error message if any
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Skipped   bool      `json:"skipped"` // True if this host's operation was skipped due to Precheck.
}

func NewGraphExecutionResult(graphName string) *GraphExecutionResult {
	return &GraphExecutionResult{
		GraphName:   graphName,
		StartTime:   time.Now(),
		Status:      StatusPending,
		NodeResults: make(map[NodeID]*NodeResult),
	}
}

func NewNodeResult(nodeName, stepName string) *NodeResult {
	return &NodeResult{
		NodeName:    nodeName,
		StepName:    stepName,
		Status:      StatusPending,
		HostResults: make(map[string]*HostResult),
		StartTime:   time.Now(), // Node starts when its first host operation attempts
	}
}
func NewHostResult(hostName string) *HostResult {
	return &HostResult{
		HostName: hostName,
		Status:   StatusPending,
		StartTime: time.Now(),
	}
}
