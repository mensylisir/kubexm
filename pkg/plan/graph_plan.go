package plan

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host
	"github.com/mensylisir/kubexm/pkg/step"      // For step.Step
	"sort"
)

type NodeID string

type ExecutionGraph struct {
	Name string `json:"name"`

	Nodes map[NodeID]*ExecutionNode `json:"nodes"`

	EntryNodes []NodeID `json:"entryNodes"`

	ExitNodes []NodeID `json:"exitNodes"`

	// TODO: Add fields for metadata like creation timestamp, version, etc. if needed.
}

type ExecutionNode struct {
	Name         string           `json:"name"`
	Step         step.Step        `json:"-"`
	Hosts        []connector.Host `json:"-"`
	Dependencies []NodeID         `json:"dependencies"`
	StepName     string           `json:"stepName"`
	Hostnames    []string         `json:"hostnames"`

	// TODO: Add fields for retry strategy, timeout overrides for this specific node, etc.
}

func NewExecutionGraph(name string) *ExecutionGraph {
	return &ExecutionGraph{
		Name:       name,
		Nodes:      make(map[NodeID]*ExecutionNode),
		EntryNodes: make([]NodeID, 0),
		ExitNodes:  make([]NodeID, 0),
	}
}

func (g *ExecutionGraph) AddNode(id NodeID, node *ExecutionNode) error {
	if _, exists := g.Nodes[id]; exists {
		return fmt.Errorf("node with ID '%s' already exists in the execution graph", id)
	}
	if node == nil {
		return fmt.Errorf("cannot add a nil node with ID '%s'", id)
	}
	if node.Step != nil && node.Step.Meta() != nil {
		node.StepName = node.Step.Meta().Name
	}
	if node.Hostnames == nil && node.Hosts != nil {
		node.Hostnames = make([]string, len(node.Hosts))
		for i, h := range node.Hosts {
			if h != nil {
				node.Hostnames[i] = h.GetName()
			}
		}
	}
	g.Nodes[id] = node
	return nil
}

func (g *ExecutionGraph) IsEmpty() bool {
	return len(g.Nodes) == 0
}

func LinkFragments(graph *ExecutionGraph, fromNodeIDs []NodeID, toNodeIDs []NodeID) error {
	if graph == nil || graph.Nodes == nil {
		return fmt.Errorf("cannot link fragments in a nil or uninitialized graph")
	}
	if len(fromNodeIDs) == 0 || len(toNodeIDs) == 0 {
		return nil
	}

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

	for _, toID := range toNodeIDs {
		for _, fromID := range fromNodeIDs {
			if err := graph.AddDependency(fromID, toID); err != nil {
				return fmt.Errorf("LinkFragments: failed to add dependency from '%s' to '%s': %w", fromID, toID, err)
			}
		}
	}
	return nil
}

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

	for _, depID := range targetNode.Dependencies {
		if depID == from {
			return nil // Dependency already exists
		}
	}

	targetNode.Dependencies = append(targetNode.Dependencies, from)
	return nil
}

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
		hasIncoming[id] = false
		hasOutgoing[id] = false
		if len(node.Dependencies) > 0 {
			hasIncoming[id] = true
		}
	}

	for id, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := g.Nodes[depID]; exists {
				hasOutgoing[depID] = true
				hasIncoming[id] = true
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
	g.EntryNodes = UniqueNodeIDs(g.EntryNodes)
	g.ExitNodes = UniqueNodeIDs(g.ExitNodes)
	sort.Slice(g.EntryNodes, func(i, j int) bool { return g.EntryNodes[i] < g.EntryNodes[j] })
	sort.Slice(g.ExitNodes, func(i, j int) bool { return g.ExitNodes[i] < g.ExitNodes[j] })
}

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
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (g *ExecutionGraph) Validate() error {
	if g.Nodes == nil {
		return fmt.Errorf("graph has a nil Nodes map")
	}

	adj := make(map[NodeID][]NodeID)
	inDegree := make(map[NodeID]int)

	for id, node := range g.Nodes {
		if node == nil {
			return fmt.Errorf("node with ID '%s' is nil in the graph", id)
		}
		adj[id] = []NodeID{}
		inDegree[id] = 0
	}

	for id, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := g.Nodes[depID]; !exists {
				return fmt.Errorf("node '%s' has a dependency on non-existent node '%s'", id, depID)
			}
			adj[depID] = append(adj[depID], id)
			inDegree[id]++
		}
	}

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

	if len(g.Nodes) > 0 && len(g.EntryNodes) == 0 {
	}

	for _, entryID := range UniqueNodeIDs(g.EntryNodes) {
		node, exists := g.Nodes[entryID]
		if !exists {
			return fmt.Errorf("explicitly defined entry node ID '%s' does not exist in the graph's nodes map", entryID)
		}
		if len(node.Dependencies) > 0 {
			return fmt.Errorf("explicitly defined entry node ID '%s' has dependencies: %v, which is invalid for an entry node", entryID, node.Dependencies)
		}
	}

	for _, exitID := range UniqueNodeIDs(g.ExitNodes) {
		if _, exists := g.Nodes[exitID]; !exists {
			return fmt.Errorf("explicitly defined exit node ID '%s' does not exist in the graph's nodes map", exitID)
		}
	}

	return nil
}
