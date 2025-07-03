package util

import (
	"encoding/json"
	"fmt"
	"strings"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// GetJsonValue retrieves a value from JSON data using a gjson path.
// Returns a gjson.Result, which should be checked for existence via result.Exists().
// Example path: "name", "friends[0].first", "company.name"
func GetJsonValue(jsonData []byte, path string) gjson.Result {
	// gjson.GetBytes handles invalid JSON by returning a result where Exists() is false.
	// No explicit error is returned by gjson.GetBytes itself for malformed JSON.
	return gjson.GetBytes(jsonData, path)
}

// SetJsonValue sets a value in JSON data using an sjson path.
// Path syntax is similar to gjson.
// value can be any standard Go type that can be marshalled to JSON.
// Returns the modified JSON data as []byte.
func SetJsonValue(jsonData []byte, path string, value interface{}) ([]byte, error) {
	isValid := gjson.ValidBytes(jsonData)
	// log.Printf("[DEBUG_SJSON] SetJsonValue: input valid? %v. Input: %s\n", isValid, string(jsonData)) // Debug line removed
	if !isValid {
		// Check if it's an empty array/object which sjson might handle by creating them
		// sjson can operate on empty/non-existent JSON to build it up.
		// However, if it's truly malformed beyond simple emptiness, we should error.
		// A simple heuristic: if it's not empty and not valid, it's likely malformed.
		trimmedData := strings.TrimSpace(string(jsonData))
		if trimmedData != "" && trimmedData != "null" && trimmedData != "{}" && trimmedData != "[]" {
			return nil, fmt.Errorf("input JSON data is invalid")
		}
		// Allow sjson to attempt to work with empty or placeholder valid JSON structures
	}

	res, err := sjson.SetBytes(jsonData, path, value)
	if err != nil {
		return nil, fmt.Errorf("failed to set JSON value at path '%s': %w", path, err)
	}
	return res, nil
}

// JsonToMap converts JSON data to a map[string]interface{}.
// This uses the standard library's json.Unmarshal.
func JsonToMap(jsonData []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := json.Unmarshal(jsonData, &m)
	if err != nil {
		// Check if the error is due to empty input, which might not be an "error" for some use cases
		// but json.Unmarshal will error if input is empty or just "null".
		if len(jsonData) == 0 || string(jsonData) == "null" {
			return make(map[string]interface{}), nil // Return empty map for empty/null JSON
		}
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}
	// If jsonData was "null", Unmarshal stores nil in m, but no error.
	// Handle this by returning an empty map instead of nil map.
	if m == nil && string(jsonData) == "null" { // cater for "null" string
		return make(map[string]interface{}), nil
	}
	return m, nil
}

// JsonToInterface converts JSON data to a generic interface{}.
// This can be useful if the top level of the JSON is an array or a scalar, not an object.
func JsonToInterface(jsonData []byte) (interface{}, error) {
	var i interface{}
	err := json.Unmarshal(jsonData, &i)
	if err != nil {
		if len(jsonData) == 0 { // Empty input is not valid JSON for Unmarshal to interface{}
			return nil, fmt.Errorf("cannot unmarshal empty JSON to interface: %w", err)
		}
		return nil, fmt.Errorf("failed to unmarshal JSON to interface: %w", err)
	}
	return i, nil
}
