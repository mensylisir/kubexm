package util

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
)

// YamlToMap converts YAML data to a map[string]interface{}.
func YamlToMap(yamlData []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := yaml.Unmarshal(yamlData, &m)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML to map: %w", err)
	}
	return m, nil
}

// GetYamlValue retrieves a value from YAML data using a dot-separated path.
// This implementation unmarshals the entire YAML into a map[string]interface{}
// and then navigates the map structure.
// It supports nested maps. For arrays, use integer indices like "array[0].key".
// This approach is simpler but less efficient for very large YAMLs or frequent gets,
// as it unmarshals everything each time. It also doesn't preserve all YAML features
// if one were to marshal it back (comments, style).
func GetYamlValue(yamlData []byte, path string) (interface{}, error) {
	var dataNode yaml.Node
	err := yaml.Unmarshal(yamlData, &dataNode)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML data: %w", err) // Changed message for consistency
	}

	// If yamlData is empty or only comments, dataNode.Kind might be 0 or dataNode.Content empty.
	// If Unmarshal succeeded but content is not usable (e.g. no actual document root)
	if dataNode.Kind == 0 || len(dataNode.Content) == 0 {
		// This case means valid YAML syntax but no actual data to traverse (e.g. empty file, or just comments)
		// For path traversal, this is effectively path not found from the root.
		return nil, fmt.Errorf("path '%s' not found in empty or comment-only YAML document", path)
	}

	// yaml.Node typically has one document node at Content[0] if it's a valid single YAML doc
	return findNodeByPath(dataNode.Content[0], strings.Split(path, "."))
}

// findNodeByPath recursively navigates a yaml.Node structure.
// This is a helper for GetYamlValue.
func findNodeByPath(node *yaml.Node, pathParts []string) (interface{}, error) {
	if len(pathParts) == 0 { // Path exhausted, return current node's value
		return convertYamlNodeToInterface(node)
	}

	part := pathParts[0]
	remainingPath := pathParts[1:]
	arrayName, index, isArrayAccess := parseArrayPathPart(part)

	if isArrayAccess {
		if node.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("cannot access array '%s' in a non-map node (kind %v)", arrayName, node.Kind)
		}
		var arrayNode *yaml.Node
		foundKey := false
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			if keyNode.Value == arrayName {
				arrayNode = node.Content[i+1]
				foundKey = true
				break
			}
		}
		if !foundKey {
			return nil, fmt.Errorf("array key '%s' not found", arrayName)
		}
		if arrayNode.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("field '%s' is not an array (kind %v)", arrayName, arrayNode.Kind)
		}
		if index < 0 || index >= len(arrayNode.Content) {
			return nil, fmt.Errorf("index %d out of bounds for array '%s' (len %d)", index, arrayName, len(arrayNode.Content))
		}
		// If there are more path parts, the element arrayNode.Content[index] must be a MappingNode (for sub-key access)
		// or if it's the last part, we just return its value.
		return findNodeByPath(arrayNode.Content[index], remainingPath)

	} else { // Simple map key
		if node.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("cannot access key '%s' in a non-map node (kind %v)", part, node.Kind)
		}
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			if keyNode.Value == part {
				return findNodeByPath(valueNode, remainingPath)
			}
		}
		return nil, fmt.Errorf("key '%s' not found in map", part)
	}
}

// convertYamlNodeToInterface converts a yaml.Node to a Go interface{} value.
// This is a simplified conversion. For complex structures, especially sequences (arrays)
// and nested mappings, this needs to be more robust.
func convertYamlNodeToInterface(node *yaml.Node) (interface{}, error) {
	var v interface{}
	err := node.Decode(&v) // Decode the specific node into an interface{}
	if err != nil {
		return nil, fmt.Errorf("failed to decode yaml node: %w", err)
	}
	return v, nil
}


// SetYamlValue sets a value in YAML data using a dot-separated path.
// This implementation unmarshals to map[string]interface{}, modifies the map,
// and then marshals back. This will lose comments, styling, and ordering.
// It's suitable for simple structural modifications.
// For complex operations preserving structure/comments, direct yaml.Node manipulation is needed.
func SetYamlValue(yamlData []byte, path string, value interface{}) ([]byte, error) {
	var data map[string]interface{}
	err := yaml.Unmarshal(yamlData, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML for setting value: %w", err)
	}

	parts := strings.Split(path, ".")
	currentMap := data

	for i, part := range parts {
		arrayName, index, isArrayAccess := parseArrayPathPart(part)
		isLastPart := (i == len(parts)-1)

		if isArrayAccess {
			// Ensure currentMap[arrayName] exists and is a slice
			arrayInterface, ok := currentMap[arrayName]
			if !ok { // Array doesn't exist, create it
				if !isLastPart {
					return nil, fmt.Errorf("cannot create nested structure within new array '%s'; create array and set element in separate steps or set a full structure", arrayName)
				}
				newArray := make([]interface{}, index+1)
				newArray[index] = value
				currentMap[arrayName] = newArray
				currentMap = nil; break
			}

			array, ok := arrayInterface.([]interface{})
			if !ok {
				return nil, fmt.Errorf("field '%s' is not an array (type: %T)", arrayName, arrayInterface)
			}

			if index < len(array) { // Existing element
				if isLastPart {
					array[index] = value
					currentMap = nil; break
				}
				if nextMap, mapOk := array[index].(map[string]interface{}); mapOk {
					currentMap = nextMap
				} else {
					return nil, fmt.Errorf("element at index %d of array '%s' is not a map (type: %T)", index, arrayName, array[index])
				}
			} else if index == len(array) { // Append
				if !isLastPart { // Need to append a map to navigate further
					newMap := make(map[string]interface{})
					currentMap[arrayName] = append(array, newMap)
					currentMap = newMap
				} else { // Append the direct value
					currentMap[arrayName] = append(array, value)
					currentMap = nil; break
				}
			} else { // Index out of bounds for append
				return nil, fmt.Errorf("index %d out of bounds for array '%s' (len %d), cannot create sparse array elements", index, arrayName, len(array))
			}
		} else { // Simple map key
			if isLastPart {
				currentMap[part] = value
				currentMap = nil; break
			}

			next, ok := currentMap[part]
			if !ok {
				newMap := make(map[string]interface{})
				currentMap[part] = newMap
				currentMap = newMap
			} else if nextMap, isMap := next.(map[string]interface{}); isMap {
				currentMap = nextMap
			} else {
				return nil, fmt.Errorf("path conflict: part '%s' in path '%s' is not a map (type: %T)", part, path, next)
			}
		}
		if currentMap == nil && !isLastPart {
             return nil, fmt.Errorf("internal error: currentMap became nil before path ended for path '%s'", path)
        }
	}

	updatedYamlData, err := yaml.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated YAML data: %w", err)
	}
	return updatedYamlData, nil
}
