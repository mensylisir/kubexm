package util

import (
	"strconv"
	"strings"
)

type PathPart struct {
	Key     string
	Index   int
	IsArray bool
}

func ParsePath(path string) []PathPart {
	if path == "" {
		return nil
	}
	rawParts := strings.Split(path, ".")
	parsedParts := make([]PathPart, len(rawParts))

	for i, partStr := range rawParts {
		name, index, isArray := parseArrayPathPart(partStr)
		if isArray {
			parsedParts[i] = PathPart{Key: name, Index: index, IsArray: true}
		} else {
			parsedParts[i] = PathPart{Key: partStr, Index: -1, IsArray: false}
		}
	}
	return parsedParts
}

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
	return part, 0, false
}
