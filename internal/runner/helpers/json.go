package helpers

import (
	"encoding/json"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"strings"
)

func GetJsonValue(jsonData []byte, path string) (interface{}, error) {
	result := gjson.GetBytes(jsonData, path)
	if !result.Exists() {
		return nil, fmt.Errorf("path '%s' not found in JSON", path)
	}
	return result.Value(), nil
}
func SetJsonValue(jsonData []byte, path string, value interface{}) ([]byte, error) {
	res, err := sjson.SetBytes(jsonData, path, value)
	if err != nil {
		return nil, fmt.Errorf("failed to set JSON value at path '%s': %w", path, err)
	}
	return res, nil
}

func JsonToMap(jsonData []byte) (map[string]interface{}, error) {
	var m map[string]interface{}

	trimmedData := strings.TrimSpace(string(jsonData))
	if trimmedData == "" || trimmedData == "null" {
		return make(map[string]interface{}), nil
	}

	if !strings.HasPrefix(trimmedData, "{") {
		return nil, fmt.Errorf("cannot unmarshal JSON to map: root is not a JSON object")
	}

	err := json.Unmarshal([]byte(trimmedData), &m)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return m, nil
}

func JsonToInterface(jsonData []byte) (interface{}, error) {
	var i interface{}

	trimmedData := strings.TrimSpace(string(jsonData))
	if trimmedData == "" {
		return nil, nil
	}

	err := json.Unmarshal([]byte(trimmedData), &i)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to interface: %w", err)
	}
	return i, nil
}
