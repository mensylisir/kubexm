package util

import (
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// TomlToMap converts TOML data to a map[string]interface{}.
func TomlToMap(tomlData []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := toml.Unmarshal(tomlData, &m)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal TOML to map: %w", err)
	}
	if m == nil { // Ensure a non-nil map is returned for empty valid TOML
		return make(map[string]interface{}), nil
	}
	return m, nil
}

// GetTomlValue retrieves a value from TOML data using a dot-separated path.
// This implementation unmarshals the TOML into a map[string]interface{}
// and then navigates the map structure.
// Array access by index in path (e.g., "array[0].key") is not supported by this simple helper.
func GetTomlValue(tomlData []byte, path string) (interface{}, error) {
	m, err := TomlToMap(tomlData)
	if err != nil {
		return nil, err // Error already wrapped by TomlToMap
	}

	parts := strings.Split(path, ".")
	var current interface{} = m

	for i, part := range parts {
		// Check for array indexing, e.g., "array[0]"
		arrayName, index, isArrayAccess := parseArrayPathPart(part)

		if isArrayAccess {
			// Current must be a map to access the array by arrayName
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
			current = array[index] // Value from array becomes the new current
		} else { // Simple map key
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

// SetTomlValue sets a value in TOML data using a dot-separated path.
// It unmarshals the TOML into a map, sets the value, and then marshals it back.
// This process may lose comments and original formatting from the TOML.
// Array access by index in path (e.g., "array[0].key") is not supported by this simple helper.
func SetTomlValue(tomlData []byte, path string, value interface{}) ([]byte, error) {
	var m map[string]interface{}
	// Handle empty TOML data by starting with an empty map
	if len(tomlData) == 0 {
		m = make(map[string]interface{})
	} else {
		var err error
		m, err = TomlToMap(tomlData)
		if err != nil {
			return nil, err // Error already wrapped
		}
	}

	parts := strings.Split(path, ".")
	currentMap := m

	for i, part := range parts {
		arrayName, index, isArrayAccess := parseArrayPathPart(part)
		isLastPart := (i == len(parts)-1)

		if isArrayAccess {
			// Ensure currentMap[arrayName] exists and is a slice
			arrayInterface, ok := currentMap[arrayName]
			if !ok { // Array doesn't exist, create it
				if !isLastPart { // If not last part, next part needs a map inside array
					return nil, fmt.Errorf("cannot create nested structure within a new array element ('%s') in this simple SetTomlValue; create array first", part)
				}
				newArray := make([]interface{}, index+1)
				newArray[index] = value // Set the value at the specified index
				currentMap[arrayName] = newArray
				currentMap = nil // Mark currentMap as consumed or no longer a map for next iteration
				break           // Value set, path consumed for this branch
			}

			array, ok := arrayInterface.([]interface{})
			if !ok {
				return nil, fmt.Errorf("field '%s' at path '%s' is not an array (type: %T)", arrayName, strings.Join(parts[:i+1], "."), arrayInterface)
			}

			// Handle index: update, append, or error for out of bounds for non-last part
			if index < len(array) {
				if isLastPart {
					array[index] = value
					currentMap = nil; break // Value set
				}
				// Navigate into this element for next part
				if nextMap, mapOk := array[index].(map[string]interface{}); mapOk {
					currentMap = nextMap
				} else {
					// Trying to navigate into an array element that is not a map
					return nil, fmt.Errorf("element at index %d of array '%s' is not a map (type: %T), cannot set nested key '%s'", index, arrayName, array[index], strings.Join(parts[i+1:], "."))
				}
			} else if index == len(array) { // Append
				if !isLastPart {
					return nil, fmt.Errorf("cannot create nested structure when appending to array '%s' in this simple SetTomlValue; set the map/value directly", arrayName)
				}
				currentMap[arrayName] = append(array, value)
				currentMap = nil; break // Value set
			} else { // Index out of bounds
				return nil, fmt.Errorf("index %d out of bounds for array '%s' (len %d) when setting value", index, arrayName, len(array))
			}
		} else { // Simple map key (part is not an array access)
			if isLastPart {
				currentMap[part] = value
				currentMap = nil; break // Value set
			}

			next, ok := currentMap[part]
			if !ok { // Key doesn't exist, create new map for it
				newMap := make(map[string]interface{})
				currentMap[part] = newMap
				currentMap = newMap
			} else if nextMap, isMap := next.(map[string]interface{}); isMap {
				currentMap = nextMap // Navigate to existing map
			} else {
				return nil, fmt.Errorf("path conflict: part '%s' in path '%s' is not a map (type: %T)", part, path, next)
			}
		}
		if currentMap == nil && !isLastPart { // Should not happen if logic is correct
			return nil, fmt.Errorf("internal error: currentMap became nil before path ended")
		}
	}

	updatedTomlData, err := toml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated TOML data: %w", err)
	}
	return updatedTomlData, nil
}
