package tool

import (
	"fmt"
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
		if err == nil && idx >= 0 {
			return name, idx, true
		}
	}
	return part, 0, false
}

func SetArrayValue(m map[string]interface{}, key string, index int, value interface{}) error {
	arrInterface, ok := m[key]
	if !ok {
		if index != 0 {
			return fmt.Errorf("cannot set index %d on new array '%s'; only index 0 is allowed for creation", index, key)
		}
		m[key] = []interface{}{value}
		return nil
	}

	arr, ok := arrInterface.([]interface{})
	if !ok {
		return fmt.Errorf("path conflict: field '%s' is not an array (type: %T)", key, arrInterface)
	}
	if index < len(arr) {
		arr[index] = value
	} else if index == len(arr) {
		m[key] = append(arr, value)
	} else {
		return fmt.Errorf("index %d out of bounds for array '%s' (len %d), cannot create sparse elements", index, key, len(arr))
	}
	return nil
}

func DescendIntoArray(m map[string]interface{}, key string, index int) (interface{}, error) {
	arrInterface, ok := m[key]
	if !ok {
		newMap := make(map[string]interface{})
		if index != 0 {
			return nil, fmt.Errorf("cannot create nested structure at index %d of new array '%s'; intermediate paths must be created sequentially", index, key)
		}
		m[key] = []interface{}{newMap}
		return newMap, nil
	}

	arr, ok := arrInterface.([]interface{})
	if !ok {
		return nil, fmt.Errorf("path conflict: field '%s' is not an array (type: %T)", key, arrInterface)
	}

	if index < len(arr) {
		return arr[index], nil
	} else if index == len(arr) {
		newMap := make(map[string]interface{})
		m[key] = append(arr, newMap)
		return newMap, nil
	} else {
		return nil, fmt.Errorf("index %d out of bounds for array '%s' (len %d), cannot create sparse elements", index, key, len(arr))
	}
}

func DescendIntoMap(m map[string]interface{}, key string) (interface{}, error) {
	next, ok := m[key]
	if !ok {
		newMap := make(map[string]interface{})
		m[key] = newMap
		return newMap, nil
	}
	return next, nil
}
