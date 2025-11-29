package plan

import (
	"fmt"
)

type ExecutionFragment struct {
	Nodes      map[NodeID]*ExecutionNode
	EntryNodes []NodeID
	ExitNodes  []NodeID
	Name       string
}

func NewExecutionFragment(name string) *ExecutionFragment {
	return &ExecutionFragment{
		Name:       name,
		Nodes:      make(map[NodeID]*ExecutionNode),
		EntryNodes: make([]NodeID, 0),
		ExitNodes:  make([]NodeID, 0),
	}
}

func (ef *ExecutionFragment) AddNode(node *ExecutionNode, id ...NodeID) (NodeID, error) {
	var nodeID NodeID
	if len(id) > 0 {
		nodeID = id[0]
	} else {
		if node.Name == "" {
			return "", fmt.Errorf("cannot add node to fragment %s: node name is empty and no ID provided", ef.Name)
		}
		nodeID = NodeID(node.Name)
	}

	if _, exists := ef.Nodes[nodeID]; exists {
		return "", fmt.Errorf("cannot add node to fragment %s: node with ID '%s' already exists", ef.Name, nodeID)
	}
	if node == nil {
		return "", fmt.Errorf("cannot add nil node with ID '%s' to fragment %s", nodeID, ef.Name)
	}
	ef.Nodes[nodeID] = node
	return nodeID, nil
}

func NewEmptyFragment(name string) *ExecutionFragment {
	return &ExecutionFragment{
		Name:       name,
		Nodes:      make(map[NodeID]*ExecutionNode),
		EntryNodes: []NodeID{},
		ExitNodes:  []NodeID{},
	}
}

func (ef *ExecutionFragment) AddDependency(fromNodeID NodeID, toNodeID NodeID) error {
	if fromNodeID == toNodeID {
		return fmt.Errorf("cannot add self-dependency for node ID '%s' in fragment %s", fromNodeID, ef.Name)
	}

	_, ok := ef.Nodes[fromNodeID]
	if !ok {
		return fmt.Errorf("source node '%s' not found in fragment '%s' when adding dependency", fromNodeID, ef.Name)
	}

	targetNode, ok := ef.Nodes[toNodeID]
	if !ok {
		return fmt.Errorf("target node '%s' not found in fragment '%s' when adding dependency", toNodeID, ef.Name)
	}
	for _, depID := range targetNode.Dependencies {
		if depID == fromNodeID {
			return nil // Dependency already exists
		}
	}
	targetNode.Dependencies = append(targetNode.Dependencies, fromNodeID)
	return nil
}

func (ef *ExecutionFragment) CalculateEntryAndExitNodes() {
	ef.EntryNodes = make([]NodeID, 0)
	ef.ExitNodes = make([]NodeID, 0)

	if len(ef.Nodes) == 0 {
		return
	}

	allNodeIDsInFragment := make(map[NodeID]bool)
	hasIncomingDepInFragment := make(map[NodeID]bool)
	hasOutgoingDepInFragment := make(map[NodeID]bool)

	for id := range ef.Nodes {
		allNodeIDsInFragment[id] = true
		hasIncomingDepInFragment[id] = false
		hasOutgoingDepInFragment[id] = false
	}

	for id, node := range ef.Nodes {
		if len(node.Dependencies) > 0 {
			hasIncomingDepInFragment[id] = true
		}
		for _, depID := range node.Dependencies {
			if _, existsInFragment := ef.Nodes[depID]; existsInFragment {
				hasOutgoingDepInFragment[depID] = true
			}
		}
	}

	for id := range hasOutgoingDepInFragment {
		hasOutgoingDepInFragment[id] = false
	}

	for _, node := range ef.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := ef.Nodes[depID]; exists {
				hasOutgoingDepInFragment[depID] = true
			}
		}
	}

	for id := range allNodeIDsInFragment {
		isEntry := true
		for _, depID := range ef.Nodes[id].Dependencies {
			if _, existsInFragment := ef.Nodes[depID]; existsInFragment {
				isEntry = false
				break
			}
		}
		if isEntry {
			ef.EntryNodes = append(ef.EntryNodes, id)
		}

		if !hasOutgoingDepInFragment[id] {
			ef.ExitNodes = append(ef.ExitNodes, id)
		}
	}

	ef.EntryNodes = UniqueNodeIDs(ef.EntryNodes)
	ef.ExitNodes = UniqueNodeIDs(ef.ExitNodes)
}

func (ef *ExecutionFragment) GetNode(id NodeID) *ExecutionNode {
	return ef.Nodes[id]
}

func (ef *ExecutionFragment) HasNode(id NodeID) bool {
	_, exists := ef.Nodes[id]
	return exists
}

func (ef *ExecutionFragment) MergeFragment(other *ExecutionFragment) error {
	if other == nil {
		return nil // Nothing to merge
	}
	for id, node := range other.Nodes {
		if _, exists := ef.Nodes[id]; exists {
		}
		ef.Nodes[id] = node
	}
	return nil
}

func (ef *ExecutionFragment) IsEmpty() bool {
	return len(ef.Nodes) == 0
}
