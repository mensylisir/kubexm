package util

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"strings"
)

func YamlToMap(yamlData []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	if len(strings.TrimSpace(string(yamlData))) == 0 {
		return make(map[string]interface{}), nil
	}
	err := yaml.Unmarshal(yamlData, &m)
	if err != nil {
		if strings.Contains(err.Error(), "cannot unmarshal") {
			return nil, fmt.Errorf("cannot convert to map: root of YAML is not a map or the data is invalid")
		}
		return nil, fmt.Errorf("failed to unmarshal YAML to map: %w", err)
	}
	if m == nil {
		return make(map[string]interface{}), nil
	}
	return m, nil
}

func GetYamlValue(yamlData []byte, path string) (interface{}, error) {
	var dataNode yaml.Node
	err := yaml.Unmarshal(yamlData, &dataNode)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML data: %w", err)
	}
	if dataNode.Kind == 0 || len(dataNode.Content) == 0 {
		return nil, fmt.Errorf("path '%s' not found in empty or comment-only YAML document", path)
	}

	return findNodeByPath(dataNode.Content[0], strings.Split(path, "."))
}

func SetYamlValue(yamlData []byte, path string, value interface{}) ([]byte, error) {
	data, err := YamlToMap(yamlData)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(path, ".")
	var current interface{} = data

	for i, part := range parts {
		isLastPart := (i == len(parts)-1)
		arrayName, index, isArrayAccess := parseArrayPathPart(part)
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("path conflict: cannot access '%s', preceding path is not a map (it's a %T)", part, current)
		}
		if isArrayAccess {
			if isLastPart {
				if err := setArrayValue(currentMap, arrayName, index, value); err != nil {
					return nil, err
				}
				break
			}
			next, err := descendIntoArray(currentMap, arrayName, index)
			if err != nil {
				return nil, err
			}
			current = next
		} else {
			if isLastPart {
				currentMap[part] = value
				break
			}
			next, err := descendIntoMap(currentMap, part)
			if err != nil {
				return nil, err
			}
			current = next
		}
	}

	updatedYamlData, err := yaml.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated YAML data: %w", err)
	}
	return updatedYamlData, nil
}

func findNodeByPath(node *yaml.Node, pathParts []string) (interface{}, error) {
	if len(pathParts) == 0 {
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
		return findNodeByPath(arrayNode.Content[index], remainingPath)

	} else {
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

func convertYamlNodeToInterface(node *yaml.Node) (interface{}, error) {
	var v interface{}
	err := node.Decode(&v)
	if err != nil {
		return nil, fmt.Errorf("failed to decode yaml node: %w", err)
	}
	return v, nil
}

func setArrayValue(m map[string]interface{}, key string, index int, value interface{}) error {
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

func descendIntoArray(m map[string]interface{}, key string, index int) (interface{}, error) {
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

func descendIntoMap(m map[string]interface{}, key string) (interface{}, error) {
	next, ok := m[key]
	if !ok {
		newMap := make(map[string]interface{})
		m[key] = newMap
		return newMap, nil
	}
	return next, nil
}
