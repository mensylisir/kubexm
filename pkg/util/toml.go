package util

import (
	"github.com/mensylisir/kubexm/pkg/tool"
)

func TomlToMap(tomlData []byte) (map[string]interface{}, error) {
	return tool.TomlToMap(tomlData)
}

func GetTomlValue(tomlData []byte, path string) (interface{}, error) {
	return tool.GetTomlValue(tomlData, path)
}

func SetTomlValue(tomlData []byte, path string, value interface{}) ([]byte, error) {
	return tool.SetTomlValue(tomlData, path, value)
}
