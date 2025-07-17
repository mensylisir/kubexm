package plan

func NonEmptyNodeIDs(ids ...NodeID) []NodeID {
	result := make([]NodeID, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			result = append(result, id)
		}
	}
	return result
}
