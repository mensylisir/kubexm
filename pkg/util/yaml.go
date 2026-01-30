package util

import (
	"github.com/mensylisir/kubexm/pkg/tool"
)

func YamlToMap(yamlData []byte) (map[string]interface{}, error) {
	return tool.YamlToMap(yamlData)
}

func GetYamlValue(yamlData []byte, path string) (interface{}, error) {
	return tool.GetYamlValue(yamlData, path)
}

func SetYamlValue(yamlData []byte, path string, value interface{}) ([]byte, error) {
	return tool.SetYamlValue(yamlData, path, value)
}
