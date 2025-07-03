package util

import (
	"testing"
	"time" // Added import

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// "gopkg.in/yaml.v3" // Not directly needed for test assertions if using interface{}
)

const sampleYamlData = `
title: YAML Example
owner:
  name: Tom Preston-Werner
  dob: 1979-05-27T07:32:00-08:00
database:
  enabled: true
  ports:
    - 8000
    - 8001
    - 8002
  data:
    - ["delta", "phi"]
    - [3.14]
  temp_targets:
    cpu: 79.5
    case: 72.0
servers:
  - host: alpha
    ip: 10.0.0.1
  - host: beta
    ip: 10.0.0.2
`

const simpleYaml = `
key1: value1
key2:
  subkey1: subvalue1
  subkey2: subvalue2
`

func TestYamlToMap(t *testing.T) {
	yamlBytes := []byte(sampleYamlData)

	t.Run("ValidYaml", func(t *testing.T) {
		m, err := YamlToMap(yamlBytes)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.Equal(t, "YAML Example", m["title"])

		owner, ok := m["owner"].(map[string]interface{})
		require.True(t, ok, "owner should be a map")
		assert.Equal(t, "Tom Preston-Werner", owner["name"])
		// yaml.v3 unmarshals ISO 8601 date strings to time.Time by default with interface{}
		expectedTime, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00-08:00")
		assert.Equal(t, expectedTime, owner["dob"])


		db, ok := m["database"].(map[string]interface{})
		require.True(t, ok, "database should be a map")
		assert.Equal(t, true, db["enabled"])

		ports, ok := db["ports"].([]interface{})
		require.True(t, ok, "database.ports should be an array")
		// YAML unmarshals numbers as int or float64 into interface{}
		assert.Equal(t, []interface{}{8000, 8001, 8002}, ports)


		servers, ok := m["servers"].([]interface{})
		require.True(t, ok, "servers should be an array of maps")
		require.Len(t, servers, 2)
		firstServer, ok := servers[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "alpha", firstServer["host"])
	})

	t.Run("InvalidYaml", func(t *testing.T) {
		invalidYaml := []byte("this: is : not: valid")
		_, err := YamlToMap(invalidYaml)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal YAML to map")
	})

	t.Run("EmptyYaml", func(t *testing.T) {
		emptyYaml := []byte("")
		m, err := YamlToMap(emptyYaml)
		// Unmarshaling empty string into map[string]interface{} results in nil map, no error
		require.NoError(t, err)
		assert.Nil(t, m, "Map should be nil for empty YAML")

		emptyYamlExplicitNull := []byte("null")
		m2, err2 := YamlToMap(emptyYamlExplicitNull)
		require.NoError(t, err2)
		assert.Nil(t, m2, "Map should be nil for 'null' YAML")

		emptyYamlExplicitMap := []byte("{}")
		m3, err3 := YamlToMap(emptyYamlExplicitMap)
		require.NoError(t, err3)
		assert.NotNil(t, m3)
		assert.Empty(t, m3, "Map should be empty for '{}' YAML")
	})
}

func TestGetYamlValue(t *testing.T) {
	yamlBytes := []byte(simpleYaml)

	t.Run("GetTopLevel", func(t *testing.T) {
		val, err := GetYamlValue(yamlBytes, "key1")
		require.NoError(t, err)
		assert.Equal(t, "value1", val)
	})

	t.Run("GetNestedValue", func(t *testing.T) {
		val, err := GetYamlValue(yamlBytes, "key2.subkey1")
		require.NoError(t, err)
		assert.Equal(t, "subvalue1", val)
	})

	t.Run("GetEntireSubMap", func(t *testing.T) {
		val, err := GetYamlValue(yamlBytes, "key2")
		require.NoError(t, err)
		expectedMap := map[string]interface{}{"subkey1": "subvalue1", "subkey2": "subvalue2"}
		assert.Equal(t, expectedMap, val)
	})

	t.Run("PathNotFound", func(t *testing.T) { // Tries to access subkey of a scalar
		_, err := GetYamlValue(yamlBytes, "key1.nonexistent")
		require.Error(t, err)
		// key1 is "value1" (ScalarNode). findNodeByPath will try to apply "nonexistent" to it.
		assert.Contains(t, err.Error(), "cannot access key 'nonexistent' in a non-map node")
	})

	t.Run("PathNotFoundIntermediate", func(t *testing.T) { // Intermediate key "nonexistent" does not exist
		_, err := GetYamlValue(yamlBytes, "nonexistent.subkey")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key 'nonexistent' not found in map")
	})

	t.Run("InvalidYamlData_SyntaxError", func(t *testing.T) {
		// This YAML has a syntax error that yaml.Unmarshal to Node should catch
		invalidYaml := []byte("key: [val1, val2") // Unclosed array
		_, err := GetYamlValue(invalidYaml, "key")
		require.Error(t, err, "Expected error for syntactically invalid YAML")
		if err != nil { // Check only if error is not nil, to avoid panic on err.Error()
			assert.Contains(t, err.Error(), "failed to unmarshal YAML data", "Error message mismatch for syntax error")
		}
	})

	t.Run("InvalidYamlData_BadIndentInterpretedAsString", func(t *testing.T) {
		// This specific bad indent might parse as a valid node structure initially,
		// and then the scalar value includes the badly indented part.
		yamlWithBadIndent := []byte("key: value\n  bad_indent")
		val, err := GetYamlValue(yamlWithBadIndent, "key")
		require.NoError(t, err, "Expected no error for this type of 'bad indent' when getting as interface{}")
		// YAML scalar folding rules will typically convert this to "value bad_indent"
		assert.Equal(t, "value bad_indent", val, "Value after YAML scalar folding")
	})

	t.Run("EmptyActualYamlDocument", func(t *testing.T) {
		emptyDoc := []byte("# Only comments")
		_, err := GetYamlValue(emptyDoc, "key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path 'key' not found in empty or comment-only YAML document")
	})


	// Tests for more complex paths (arrays, etc.)
	complexYamlBytes := []byte(sampleYamlData)
	t.Run("GetScalarFromComplex", func(t *testing.T) {
		val, err := GetYamlValue(complexYamlBytes, "owner.name")
		require.NoError(t, err)
		assert.Equal(t, "Tom Preston-Werner", val)
	})

	// Current GetYamlValue now supports array indexing in path.
	t.Run("GetFromArrayInComplex_SupportedByPath", func(t *testing.T) {
		val, err := GetYamlValue(complexYamlBytes, "database.ports[0]")
		require.NoError(t, err)
		assert.Equal(t, 8000, val) // yaml.v3 decodes numbers to int or float64

		val, err = GetYamlValue(complexYamlBytes, "servers[1].host")
		require.NoError(t, err)
		assert.Equal(t, "beta", val)

		_, err = GetYamlValue(complexYamlBytes, "servers[2].host") // Out of bounds
		require.Error(t, err)
		assert.Contains(t, err.Error(), "index 2 out of bounds for array 'servers'")

		// Test getting an element from a nested array by first getting the outer array element
		valOuterArrayEl, errOuter := GetYamlValue(complexYamlBytes, "database.data[0]")
		require.NoError(t, errOuter, "Error getting database.data[0]")
		innerArray, ok := valOuterArrayEl.([]interface{})
		require.True(t, ok, "database.data[0] should be an array")
		require.Len(t, innerArray, 2, "Inner array length mismatch")
		assert.Equal(t, "phi", innerArray[1], "Value from nested array mismatch")

		// Accessing nested array element directly with path "database.data[0][1]" is not supported
		// by the current simplified findNodeByPath. It would try to find a key "data[0][1]" in "database".
		_, err = GetYamlValue(complexYamlBytes, "database.data[0][1]")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key 'data[0][1]' not found in map")


	})

	t.Run("GetArrayDirectlyInComplex", func(t *testing.T) {
		val, err := GetYamlValue(complexYamlBytes, "database.ports")
		require.NoError(t, err)
		assert.Equal(t, []interface{}{8000, 8001, 8002}, val)
	})
}

func TestSetYamlValue(t *testing.T) {
	yamlBytes := []byte(simpleYaml)

	t.Run("SetExistingTopLevel", func(t *testing.T) {
		updatedYaml, err := SetYamlValue(yamlBytes, "key1", "new_value1")
		require.NoError(t, err)

		m, _ := YamlToMap(updatedYaml)
		assert.Equal(t, "new_value1", m["key1"])
	})

	t.Run("SetExistingNested", func(t *testing.T) {
		updatedYaml, err := SetYamlValue(yamlBytes, "key2.subkey1", "new_subvalue1")
		require.NoError(t, err)

		m, _ := YamlToMap(updatedYaml)
		key2Map, _ := m["key2"].(map[string]interface{})
		assert.Equal(t, "new_subvalue1", key2Map["subkey1"])
	})

	t.Run("SetNewTopLevel", func(t *testing.T) {
		updatedYaml, err := SetYamlValue(yamlBytes, "key3", "value3")
		require.NoError(t, err)

		m, _ := YamlToMap(updatedYaml)
		assert.Equal(t, "value3", m["key3"])
	})

	t.Run("SetNewNestedCreatesMaps", func(t *testing.T) {
		updatedYaml, err := SetYamlValue(yamlBytes, "key4.subkey3.deepkey", "deep_value")
		require.NoError(t, err)

		m, _ := YamlToMap(updatedYaml)
		key4Map, ok := m["key4"].(map[string]interface{})
		require.True(t, ok)
		subkey3Map, ok := key4Map["subkey3"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "deep_value", subkey3Map["deepkey"])
	})

	t.Run("SetReplacingNonMapWithPath", func(t *testing.T) {
		_, err := SetYamlValue(yamlBytes, "key1.cannotCreate", "value")
		require.Error(t, err)
		// Error from SetYamlValue: "path conflict: part '%s' in path '%s' is not a map (type: %T)"
		assert.Contains(t, err.Error(), "path conflict: part 'key1' in path 'key1.cannotCreate' is not a map")
	})

	t.Run("SetArrayValue", func(t *testing.T) {
		newArr := []string{"a", "b"}
		updatedYaml, err := SetYamlValue(yamlBytes, "key2.newArray", newArr)
		require.NoError(t, err)

		m, _ := YamlToMap(updatedYaml)
		key2Map, _ := m["key2"].(map[string]interface{})

		// YAML unmarshals to []interface{} for generic map values
		expectedArr := []interface{}{"a", "b"}
		assert.Equal(t, expectedArr, key2Map["newArray"])
	})

	t.Run("InvalidYamlForSet", func(t *testing.T) {
		invalidYaml := []byte("key: val\n  bad:")
		_, err := SetYamlValue(invalidYaml, "key", "newval")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal YAML for setting value")
	})
}
