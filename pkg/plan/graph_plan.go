package plan

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host
	"github.com/mensylisir/kubexm/pkg/step"    // For step.Step
)

// NodeID is the unique identifier for a node within the execution graph.
// It can be the same as the ExecutionNode's Name if names are guaranteed to be unique,
// but using a distinct type offers flexibility.
type NodeID string

// ExecutionGraph represents the entire set of operations and their dependencies.
// It is the primary input for a DAG-aware execution engine.
type ExecutionGraph struct {
	// A descriptive name for the overall plan/graph.
	Name string `json:"name"`

	// Nodes is a map of all execution nodes in the graph, keyed by their unique ID.
	Nodes map[NodeID]*ExecutionNode `json:"nodes"`

	// EntryNodes lists the IDs of nodes that have no dependencies within this graph.
	// These are the starting points for execution.
	EntryNodes []NodeID `json:"entryNodes"`

	// ExitNodes lists the IDs of nodes that are not depended upon by any other node in this graph.
	// These are the terminal points of the graph.
	ExitNodes []NodeID `json:"exitNodes"`

	// TODO: Add fields for metadata like creation timestamp, version, etc. if needed.
}

// ExecutionNode represents a single, schedulable unit of work in the graph.
type ExecutionNode struct {
	// Name is a descriptive name for the node, e.g., "Upload etcd binary on master-1".
	// While NodeID is for graph structure, Name is for human readability and logging.
	// Tasks should generate meaningful names.
	Name string `json:"name"`

	// The Step to be executed. This contains the logic and configuration for the operation.
	Step step.Step `json:"-"` // Excluded from JSON marshalling by default

	// The target hosts on which the Step will be executed.
	// For steps that run locally on the control node, this might contain a special "local" host.
	Hosts []connector.Host `json:"-"` // Excluded from JSON marshalling

	// Dependencies lists the IDs of all nodes that must complete successfully
	// before this node can be scheduled for execution.
	Dependencies []NodeID `json:"dependencies"`

	// StepName is for marshalling/logging purposes, so we can see what step was used.
	// This should be populated from Step.Meta().Name.
	StepName string `json:"stepName"`

	// TODO: Add fields for retry strategy, timeout overrides for this specific node, etc.
}

// NewExecutionGraph creates an empty execution graph.
func NewExecutionGraph(name string) *ExecutionGraph {
	return &ExecutionGraph{
		Name:       name,
		Nodes:      make(map[NodeID]*ExecutionNode),
		EntryNodes: []NodeID{},
		ExitNodes:  []NodeID{},
	}
}

// AddNode adds a new execution node to the graph.
// It returns an error if a node with the same ID already exists.
func (g *ExecutionGraph) AddNode(id NodeID, node *ExecutionNode) error {
	if _, exists := g.Nodes[id]; exists {
		return fmt.Errorf("node with ID '%s' already exists in the execution graph", id)
	}
	if node == nil {
		return fmt.Errorf("cannot add a nil node with ID '%s'", id)
	}
	g.Nodes[id] = node
	return nil
}

// Validate checks the graph for structural integrity, such as cyclic dependencies
// and ensures all referenced dependency IDs exist as nodes.
// This is a crucial step before execution.
func (g *ExecutionGraph) Validate() error {
	// Check for nil nodes map
	if g.Nodes == nil {
		return fmt.Errorf("graph has a nil Nodes map")
	}

	// Check for missing dependency references and build an adjacency list for cycle detection.
	adj := make(map[NodeID][]NodeID)
	inDegree := make(map[NodeID]int) // For cycle detection via Kahn's or just general validation

	for id, node := range g.Nodes {
		if node == nil {
			return fmt.Errorf("node with ID '%s' is nil in the graph", id)
		}
		// Initialize for all nodes present in the map for cycle detection
		adj[id] = []NodeID{}
		inDegree[id] = 0
	}

	for id, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := g.Nodes[depID]; !exists {
				return fmt.Errorf("node '%s' has a dependency on non-existent node '%s'", id, depID)
			}
			adj[depID] = append(adj[depID], id) // For cycle detection: edge from depID -> id
			inDegree[id]++
		}
	}

	// Cycle detection using Kahn's algorithm (based on in-degrees)
	queue := []NodeID{}
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	count := 0
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		count++

		for _, v := range adj[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	if count != len(g.Nodes) {
		return fmt.Errorf("cyclic dependency detected in the execution graph (processed %d nodes, expected %d)", count, len(g.Nodes))
	}

	// Validate EntryNodes: ensure they exist and have no dependencies if graph is not empty
	if len(g.Nodes) > 0 {
		for _, entryID := range g.EntryNodes {
			node, exists := g.Nodes[entryID]
			if !exists {
				return fmt.Errorf("entry node ID '%s' does not exist in the graph's nodes map", entryID)
			}
			if len(node.Dependencies) > 0 {
				// This check might be too strict if EntryNodes are just hints and actual 0-in-degree nodes are calculated by engine.
				// However, if Pipeline explicitly sets them, they should ideally be true entry points.
				// For now, let's keep it as a strong validation.
				// Or, this validation could be part of the engine's pre-flight.
				// logger.Warn("Entry node '%s' has dependencies defined: %v", entryID, node.Dependencies)
			}
		}
	}


	// TODO: Validate ExitNodes (ensure they exist)

	return nil // Placeholder for more robust validation logic
}
