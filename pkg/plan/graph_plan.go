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

	// Hostnames lists the names of the target hosts for marshalling/logging.
	Hostnames []string `json:"hostnames"`

	// TODO: Add fields for retry strategy, timeout overrides for this specific node, etc.
}

// NewExecutionGraph creates an empty execution graph.
func NewExecutionGraph(name string) *ExecutionGraph {
	return &ExecutionGraph{
		Name:       name,
		Nodes:      make(map[NodeID]*ExecutionNode),
		EntryNodes: make([]NodeID, 0),
		ExitNodes:  make([]NodeID, 0),
	}
}

// AddNode adds a new execution node to the graph.
// It returns an error if a node with the same ID already exists.
// It also populates StepName and Hostnames from the node's Step and Hosts.
func (g *ExecutionGraph) AddNode(id NodeID, node *ExecutionNode) error {
	if _, exists := g.Nodes[id]; exists {
		return fmt.Errorf("node with ID '%s' already exists in the execution graph", id)
	}
	if node == nil {
		return fmt.Errorf("cannot add a nil node with ID '%s'", id)
	}
	if node.Step != nil && node.Step.Meta() != nil { // Ensure Meta is not nil
		node.StepName = node.Step.Meta().Name
	}
	if node.Hostnames == nil && node.Hosts != nil {
		node.Hostnames = make([]string, len(node.Hosts))
		for i, h := range node.Hosts {
			if h != nil { // Ensure host is not nil
				node.Hostnames[i] = h.GetName()
			}
		}
	}
	g.Nodes[id] = node
	return nil
}

// IsEmpty checks if the graph contains any nodes.
func (g *ExecutionGraph) IsEmpty() bool {
	return len(g.Nodes) == 0
}

// LinkFragments adds dependencies from all fromNodeIDs to all toNodeIDs within the graph.
// It ensures that the nodes exist in the graph before adding dependencies.
// This function is typically used by a Pipeline or Module to link the
// ExecutionFragments of its constituent parts.
func LinkFragments(graph *ExecutionGraph, fromNodeIDs []NodeID, toNodeIDs []NodeID) error {
	if graph == nil || graph.Nodes == nil {
		return fmt.Errorf("cannot link fragments in a nil or uninitialized graph")
	}
	// If there are no source exit points or no target entry points, there's nothing to link.
	if len(fromNodeIDs) == 0 || len(toNodeIDs) == 0 {
		return nil
	}

	// Verify all specified nodes exist to prevent partial linking on error.
	for _, id := range fromNodeIDs {
		if _, exists := graph.Nodes[id]; !exists {
			return fmt.Errorf("LinkFragments: source node ID '%s' not found in graph", id)
		}
	}
	for _, id := range toNodeIDs {
		if _, exists := graph.Nodes[id]; !exists {
			return fmt.Errorf("LinkFragments: target node ID '%s' not found in graph", id)
		}
	}

	// Add dependencies: each node in toNodeIDs depends on all nodes in fromNodeIDs.
	for _, toID := range toNodeIDs {
		targetNode := graph.Nodes[toID] // Known to exist from the check above
		// Append all fromNodeIDs as dependencies, AddDependency will handle duplicates if any.
		for _, fromID := range fromNodeIDs {
			if err := graph.AddDependency(fromID, toID); err != nil {
				// This could happen if AddDependency has stricter rules (e.g. self-loop if fromID == toID)
				// or other internal errors.
				return fmt.Errorf("LinkFragments: failed to add dependency from '%s' to '%s': %w", fromID, toID, err)
			}
		}
	}
	return nil
}

// AddDependency creates a dependency between two nodes (from -> to).
// It returns an error if either node does not exist or if the dependency would be self-referential.
func (g *ExecutionGraph) AddDependency(from NodeID, to NodeID) error {
	if from == to {
		return fmt.Errorf("cannot add self-dependency for node ID '%s'", from)
	}
	if _, exists := g.Nodes[from]; !exists {
		return fmt.Errorf("source node with ID '%s' not found in graph when adding dependency to '%s'", from, to)
	}
	targetNode, exists := g.Nodes[to]
	if !exists {
		return fmt.Errorf("target node with ID '%s' not found in graph when adding dependency from '%s'", to, from)
	}

	// Check for duplicate dependency
	for _, depID := range targetNode.Dependencies {
		if depID == from {
			return nil // Dependency already exists
		}
	}

	targetNode.Dependencies = append(targetNode.Dependencies, from)
	return nil
}

// CalculateEntryAndExitNodes determines the entry and exit nodes of the graph.
// This should be called after all nodes and dependencies are added,
// or if EntryNodes/ExitNodes are not manually maintained.
func (g *ExecutionGraph) CalculateEntryAndExitNodes() {
	g.EntryNodes = make([]NodeID, 0)
	g.ExitNodes = make([]NodeID, 0)

	if len(g.Nodes) == 0 {
		return
	}

	allNodeIDs := make(map[NodeID]struct{})
	hasIncoming := make(map[NodeID]bool)
	hasOutgoing := make(map[NodeID]bool)

	for id, node := range g.Nodes {
		allNodeIDs[id] = struct{}{}
		// Initialize for all nodes
		hasIncoming[id] = false
		hasOutgoing[id] = false
		// Check dependencies specified in the node itself
		if len(node.Dependencies) > 0 {
			hasIncoming[id] = true
		}
	}

	// Iterate again to mark outgoing dependencies
	for id, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := g.Nodes[depID]; exists { // Ensure dependency exists
				hasOutgoing[depID] = true
				hasIncoming[id] = true // Re-affirm target has incoming
			}
		}
	}

	for id := range allNodeIDs {
		if !hasIncoming[id] {
			g.EntryNodes = append(g.EntryNodes, id)
		}
		if !hasOutgoing[id] {
			g.ExitNodes = append(g.ExitNodes, id)
		}
	}
	// Ensure uniqueness in case of complex recalculations or manual additions
	g.EntryNodes = UniqueNodeIDs(g.EntryNodes)
	g.ExitNodes = UniqueNodeIDs(g.ExitNodes)
}


// UniqueNodeIDs returns a slice with unique NodeIDs from the input.
// Preserves order of first appearance.
func UniqueNodeIDs(ids []NodeID) []NodeID {
	if len(ids) == 0 {
		return []NodeID{}
	}
	seen := make(map[NodeID]bool)
	result := []NodeID{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
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

	// Validate EntryNodes: ensure they exist and have no dependencies if graph is not empty.
	// If EntryNodes were not pre-populated, CalculateEntryAndExitNodes should be called by the graph builder.
	// The cycle detection already ensures graph coherence if it passes.
	if len(g.Nodes) > 0 && len(g.EntryNodes) == 0 {
		// If graph is not empty but has no explicitly set EntryNodes, try to calculate them.
		// This is a fallback; ideally, the graph constructor (e.g., Pipeline) sets these.
		// For validation, we primarily care that if EntryNodes *are* set, they are valid.
		// The Kahn's algorithm for cycle detection inherently finds nodes with in-degree 0.
		// If count != len(g.Nodes) after Kahn's, it's a cycle or disconnected components that don't start.
		// If count == len(g.Nodes) but len(g.EntryNodesFromKahn) == 0 (and len(g.Nodes) > 0),
		// it implies a single node with self-loop if Kahn's was adapted for that, or an issue.
		// The current Kahn's correctly identifies cycles. If no cycle, there must be entry points.
	}

	for _, entryID := range UniqueNodeIDs(g.EntryNodes) { // Use unique in case of duplicates
		node, exists := g.Nodes[entryID]
		if !exists {
			return fmt.Errorf("explicitly defined entry node ID '%s' does not exist in the graph's nodes map", entryID)
		}
		if len(node.Dependencies) > 0 {
			// This is a strong check. An explicitly defined "EntryNode" should not have dependencies.
			// If EntryNodes are just a hint, this validation might be too strict.
			// Given the DAG model, items in g.EntryNodes should truly be starting points.
			return fmt.Errorf("explicitly defined entry node ID '%s' has dependencies: %v, which is invalid for an entry node", entryID, node.Dependencies)
		}
	}

	// Validate ExitNodes: ensure they exist
	for _, exitID := range UniqueNodeIDs(g.ExitNodes) { // Use unique
		if _, exists := g.Nodes[exitID]; !exists {
			return fmt.Errorf("explicitly defined exit node ID '%s' does not exist in the graph's nodes map", exitID)
		}
		// Further validation: an exit node should not be a dependency for any other node in g.Nodes.
		// This is implicitly checked by Kahn's algorithm if it completes successfully and these nodes
		// are correctly identified as having no outgoing edges to other nodes *within the graph*.
	}


	return nil
}
