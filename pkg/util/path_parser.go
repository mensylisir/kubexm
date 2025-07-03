package util

import (
	"strconv"
	"strings"
)

// parseArrayPathPart checks if a path part string is an array access (e.g., "name[index]").
// If it is, it returns the array name, the index, and true.
// Otherwise, it returns the original part as name, 0, and false.
func parseArrayPathPart(part string) (name string, index int, isArray bool) {
	openBracket := strings.Index(part, "[")
	closeBracket := strings.LastIndex(part, "]")

	if openBracket != -1 && closeBracket == len(part)-1 && openBracket < closeBracket {
		name = part[:openBracket]
		indexStr := part[openBracket+1 : closeBracket]
		idx, err := strconv.Atoi(indexStr)
		if err == nil && idx >= 0 { // Ensure index is a non-negative integer
			return name, idx, true
		}
	}
	return part, 0, false // Not a valid array access pattern or error in parsing index
}
