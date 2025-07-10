package common

import (
	"github.com/mensylisir/kubexm/pkg/plan"
)

// NonEmptyNodeIDs filters a list of NodeIDs, returning only those that are not empty strings.
// This is useful for constructing dependency lists where some dependencies might be conditional.
func NonEmptyNodeIDs(ids ...plan.NodeID) []plan.NodeID {
	result := make([]plan.NodeID, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			result = append(result, id)
		}
	}
	return result
}

// ContainsString checks if a string is present in a slice of strings.
// This is a common utility.
func ContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TODO: Add other common utility functions here as needed,
// for example, pointer helpers (StrPtr, BoolPtr, IntPtr) if not already in pkg/util.
// However, pkg/util is a more appropriate place for generic Go helpers.
// This file should focus on common utilities very specific to the Kubexm domain logic
// that don't fit well into more specialized packages.
```
