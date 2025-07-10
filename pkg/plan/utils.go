package plan

// NonEmptyNodeIDs filters a list of NodeIDs, returning only those that are not empty strings.
// This is useful for constructing dependency lists where some dependencies might be conditional.
func NonEmptyNodeIDs(ids ...NodeID) []NodeID {
	result := make([]NodeID, 0, len(ids))
	for _, id := range ids {
		if id != "" { // NodeID is `type NodeID string` defined in graph_plan.go
			result = append(result, id)
		}
	}
	return result
}
