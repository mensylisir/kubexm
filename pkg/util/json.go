package util

import (
	"github.com/mensylisir/kubexm/pkg/tool"
)

func GetJsonValue(jsonData []byte, path string) (interface{}, error) {
	return tool.GetJsonValue(jsonData, path)
}

func SetJsonValue(jsonData []byte, path string, value interface{}) ([]byte, error) {
	return tool.SetJsonValue(jsonData, path, value)
}

func JsonToMap(jsonData []byte) (map[string]interface{}, error) {
	return tool.JsonToMap(jsonData)
}

func JsonToInterface(jsonData []byte) (interface{}, error) {
	return tool.JsonToInterface(jsonData)
}
