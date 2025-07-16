package util

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func TomlToMap(tomlData []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := toml.Unmarshal(tomlData, &m)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal TOML to map: %w", err)
	}
	if m == nil {
		return make(map[string]interface{}), nil
	}
	return m, nil
}

func GetTomlValue(tomlData []byte, path string) (interface{}, error) {
	m, err := TomlToMap(tomlData)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(path, ".")
	var current interface{} = m

	for i, part := range parts {
		arrayName, index, isArrayAccess := parseArrayPathPart(part)

		if isArrayAccess {
			currentMap, ok := current.(map[string]interface{})
			if !ok {
				prevPath := strings.Join(parts[:i], ".")
				return nil, fmt.Errorf("path part '%s' (for array '%s') is not within a map at '%s'", part, arrayName, prevPath)
			}
			arrayInterface, found := currentMap[arrayName]
			if !found {
				return nil, fmt.Errorf("array '%s' not found at path '%s'", arrayName, strings.Join(parts[:i+1], "."))
			}
			array, ok := arrayInterface.([]interface{})
			if !ok {
				return nil, fmt.Errorf("field '%s' at path '%s' is not an array (type: %T)", arrayName, strings.Join(parts[:i+1], "."), arrayInterface)
			}
			if index < 0 || index >= len(array) {
				return nil, fmt.Errorf("index %d out of bounds for array '%s' (len %d) at path '%s'", index, arrayName, len(array), strings.Join(parts[:i+1], "."))
			}
			current = array[index]
		} else {
			currentMap, ok := current.(map[string]interface{})
			if !ok {
				prevPath := strings.Join(parts[:i], ".")
				return nil, fmt.Errorf("path part '%s' (from '%s') is not a map at '%s' (actual type: %T)", part, path, prevPath, current)
			}
			value, found := currentMap[part]
			if !found {
				return nil, fmt.Errorf("key '%s' not found at path '%s'", part, strings.Join(parts[:i+1], "."))
			}
			current = value
		}
	}
	return current, nil
}

func SetTomlValue(tomlData []byte, path string, value interface{}) ([]byte, error) {
	m, err := TomlToMap(tomlData)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(path, ".")
	var current interface{} = m

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

	updatedTomlData, err := toml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated TOML data: %w", err)
	}
	return updatedTomlData, nil
}
