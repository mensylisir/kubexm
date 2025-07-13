package util

import "sort"

// IsInStringSlice checks if a given string exists in a slice of strings.
// It performs a case-sensitive comparison.
//
// Parameters:
//
//	slice: The slice of strings to search within.
//	str: The string to search for.
//
// Returns:
//
//	true if the string is found in the slice, false otherwise.
//
// Example:
//
//	haystack := []string{"apple", "banana", "cherry"}
//	needle := "banana"
//	if util.IsInStringSlice(haystack, needle) {
//	    // ... it's there
//	}
func IsInStringSlice(slice []string, str string) bool {
	// Iterate through each element in the slice.
	for _, item := range slice {
		// If the current item matches the target string, return true immediately.
		if item == str {
			return true
		}
	}
	// If the loop completes without finding a match, return false.
	return false
}

// IsInStringSliceWithMap pre-processes the slice into a map for faster lookups.
// This is more efficient if you need to perform many checks against the same large slice.
// Note: This function has a higher initial memory and time cost to build the map.
func IsInStringSliceWithMap(slice []string, str string) bool {
	// Create a map for efficient lookups. The value `struct{}` uses zero memory.
	lookupMap := make(map[string]struct{}, len(slice))
	for _, item := range slice {
		lookupMap[item] = struct{}{}
	}

	// Check for existence in the map. This is an O(1) operation on average.
	_, ok := lookupMap[str]
	return ok
}

// IsInSortedStringSlice checks if a string exists in a *sorted* slice of strings.
// It uses binary search for efficiency.
// IMPORTANT: The input slice MUST be sorted alphabetically.
func IsInSortedStringSlice(sortedSlice []string, str string) bool {
	// sort.SearchStrings performs a binary search.
	// It returns the index where the string would be inserted to maintain order.
	index := sort.SearchStrings(sortedSlice, str)

	// If the index is within the slice's bounds and the element at that index
	// actually matches the string, then the string exists.
	return index < len(sortedSlice) && sortedSlice[index] == str
}
