package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleTomlDataForTest = `
title = "TOML Example"

[owner]
name = "Tom Preston-Werner"
dob = 1979-05-27T07:32:00-08:00

[database]
enabled = true
ports = [ 8000, 8001, 8002 ]
data = [ ["delta", "phi"], [3.14] ]
temp_targets = { cpu = 79.5, case = 72.0 }

[[servers]]
host = "alpha"
ip = "10.0.0.1"

[[servers]]
host = "beta"
ip = "10.0.0.2"
`
// Note: TOML unmarshals dates to time.Time, numbers to int64 or float64 by default into map[string]interface{}

func TestGetTomlValue_MapNavigation(t *testing.T) {
	tomlBytes := []byte(sampleTomlDataForTest)

	t.Run("GetString", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "title")
		require.NoError(t, err)
		assert.Equal(t, "TOML Example", val)
	})

	t.Run("GetFromTable", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "owner.name")
		require.NoError(t, err)
		assert.Equal(t, "Tom Preston-Werner", val)
	})

	t.Run("GetTime", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "owner.dob")
		require.NoError(t, err)
		// go-toml/v2 unmarshals TOML datetime to time.Time
		expectedTime, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00-08:00")
		assert.Equal(t, expectedTime, val)
	})

	t.Run("GetBool", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "database.enabled")
		require.NoError(t, err)
		assert.Equal(t, true, val)
	})

	t.Run("GetInlineTableAsMap", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "database.temp_targets")
		require.NoError(t, err)
		// The value will be map[string]interface{}
		expected := map[string]interface{}{"cpu": 79.5, "case": 72.0}
		assert.Equal(t, expected, val)
	})

	t.Run("GetNestedFromInlineTable", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "database.temp_targets.cpu")
		require.NoError(t, err)
		assert.Equal(t, 79.5, val)
	})

	t.Run("GetEntireArray", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "database.ports")
		require.NoError(t, err)
		// go-toml by default decodes integers as int64 when unmarshaling to interface{}
		assert.Equal(t, []interface{}{int64(8000), int64(8001), int64(8002)}, val)
	})

	t.Run("GetEntireArrayOfTables", func(t *testing.T) {
		val, err := GetTomlValue(tomlBytes, "servers") // This gets the array
		require.NoError(t, err)
		serversArray, ok := val.([]interface{})
		require.True(t, ok)
		require.Len(t, serversArray, 2)

		server0, ok := serversArray[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "alpha", server0["host"])

		// Path like "servers[0].host" IS NOW SUPPORTED by GetTomlValue
		val, err = GetTomlValue(tomlBytes, "servers[0].host")
		require.NoError(t, err, "Expected no error for array path indexing servers[0].host")
		assert.Equal(t, "alpha", val)

		val, err = GetTomlValue(tomlBytes, "servers[1].ip")
		require.NoError(t, err, "Expected no error for array path indexing servers[1].ip")
		assert.Equal(t, "10.0.0.2", val)

		// Test invalid index
		_, err = GetTomlValue(tomlBytes, "servers[2].host")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "index 2 out of bounds for array 'servers'")

		// Test path into non-map element
		_, err = GetTomlValue(tomlBytes, "database.ports[0].sub")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is not a map")


	})


	t.Run("PathNotFound", func(t *testing.T) {
		_, err := GetTomlValue(tomlBytes, "nonexistent.key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key 'nonexistent' not found at path 'nonexistent'")
	})

	t.Run("PathNotMap", func(t *testing.T) {
		_, err := GetTomlValue(tomlBytes, "title.subpart") // title is a string
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path part 'subpart' (from 'title.subpart') is not a map")
	})


	t.Run("InvalidTomlData", func(t *testing.T) {
		invalidToml := []byte("this is not toml")
		_, err := GetTomlValue(invalidToml, "title")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal TOML to map")
	})
}

func TestSetTomlValue_MapNavigation(t *testing.T) {
	tomlBytes := []byte(sampleTomlDataForTest)

	t.Run("SetExistingString", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "title", "New TOML Example")
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "title")
		require.NoError(t, getErr)
		assert.Equal(t, "New TOML Example", val)
	})

	t.Run("SetExistingInTable", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "owner.name", "John Doe")
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "owner.name")
		require.NoError(t, getErr)
		assert.Equal(t, "John Doe", val)
	})

	t.Run("SetNewKeyInExistingTable", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "owner.city", "San Francisco")
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "owner.city")
		require.NoError(t, getErr)
		assert.Equal(t, "San Francisco", val)
	})

	t.Run("SetNewTopLevelKey", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "new_key", "new_value")
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "new_key")
		require.NoError(t, getErr)
		assert.Equal(t, "new_value", val)
	})

	t.Run("SetNewTableAndKey", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "new_table.new_key", "value_in_new_table")
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "new_table.new_key")
		require.NoError(t, getErr)
		assert.Equal(t, "value_in_new_table", val)

		tableVal, tableGetErr := GetTomlValue(updatedToml, "new_table")
		require.NoError(t, tableGetErr)
		expectedTable := map[string]interface{}{"new_key": "value_in_new_table"}
		assert.Equal(t, expectedTable, tableVal)
	})

	t.Run("SetNewKeyInInlineTable", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "database.temp_targets.disk", 90.0)
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "database.temp_targets.disk")
		require.NoError(t, getErr)
		assert.Equal(t, 90.0, val)
	})

	t.Run("SetEntireArray", func(t *testing.T) {
		newArray := []interface{}{"x", "y", int64(100)} // Use int64 for numbers to match TOML default
		updatedToml, err := SetTomlValue(tomlBytes, "database.ports", newArray)
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "database.ports")
		require.NoError(t, getErr)
		assert.Equal(t, newArray, val)
	})

	t.Run("SetPathConflictsWithNonMap", func(t *testing.T) {
		_, err := SetTomlValue(tomlBytes, "title.newKey", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path conflict: part 'title' in path 'title.newKey' is not a map")
	})

	t.Run("SetOnEmptyToml", func(t *testing.T) {
		emptyToml := []byte("")
		updatedToml, err := SetTomlValue(emptyToml, "a.b.c", "value")
		require.NoError(t, err)

		val, getErr := GetTomlValue(updatedToml, "a.b.c")
		require.NoError(t, getErr)
		assert.Equal(t, "value", val)

		// Check structure
		m, mapErr := TomlToMap(updatedToml)
		require.NoError(t, mapErr)
		a, okA := m["a"].(map[string]interface{})
		require.True(t, okA)
		b, okB := a["b"].(map[string]interface{})
		require.True(t, okB)
		assert.Equal(t, "value", b["c"])
	})

	t.Run("InvalidTomlDataForSet", func(t *testing.T) {
		invalidToml := []byte("this = not = toml")
		_, err := SetTomlValue(invalidToml, "title", "new title")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal TOML to map")
	})

	// Array path indexing is now partially supported by SetTomlValue
	t.Run("SetInExistingArrayElement", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "database.ports[0]", int64(9999))
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "database.ports[0]")
		require.NoError(t, getErr)
		assert.Equal(t, int64(9999), val)
	})

	t.Run("AppendToArray", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "database.ports[3]", int64(8003)) // Appending
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "database.ports[3]")
		require.NoError(t, getErr)
		assert.Equal(t, int64(8003), val)

		arrVal, _ := GetTomlValue(updatedToml, "database.ports")
		assert.Len(t, arrVal.([]interface{}), 4)
	})

	t.Run("SetInArrayOfTables", func(t *testing.T) {
		updatedToml, err := SetTomlValue(tomlBytes, "servers[0].ip", "10.8.8.8")
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "servers[0].ip")
		require.NoError(t, getErr)
		assert.Equal(t, "10.8.8.8", val)
	})

	t.Run("SetNewArrayAtIndexWhenArrayIsNew", func(t *testing.T) { // Renamed for clarity
		// Path "newly_created_array[0]", value "value"
		// SetTomlValue should create "newly_created_array" as an array and set its 0th element.
		initialTomlBytes := []byte(sampleTomlDataForTest) // Use fresh bytes

		updatedToml, err := SetTomlValue(initialTomlBytes, "newly_created_array[0]", "value")
		require.NoError(t, err, "Setting element in a new array should succeed")

		// Verify the specific element
		val, getErr := GetTomlValue(updatedToml, "newly_created_array[0]")
		require.NoError(t, getErr)
		assert.Equal(t, "value", val)

		// Verify the whole array
		arrValInterface, getArrErr := GetTomlValue(updatedToml, "newly_created_array")
		require.NoError(t, getArrErr)
		expectedArr := []interface{}{"value"} // Array should have one element: "value"
		assert.Equal(t, expectedArr, arrValInterface)
	})

	t.Run("SetNewArrayDirectly", func(t *testing.T) {
		newArr := []interface{}{"one", int64(2)}
		updatedToml, err := SetTomlValue(tomlBytes, "brand_new_array", newArr)
		require.NoError(t, err)
		val, getErr := GetTomlValue(updatedToml, "brand_new_array")
		require.NoError(t, getErr)
		assert.Equal(t, newArr, val)
	})


}

func TestTomlToMap(t *testing.T) {
	tomlBytes := []byte(sampleTomlDataForTest)

	t.Run("ValidToml", func(t *testing.T) {
		m, err := TomlToMap(tomlBytes)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.Equal(t, "TOML Example", m["title"])

		owner, ok := m["owner"].(map[string]interface{})
		require.True(t, ok, "owner should be a map")
		assert.Equal(t, "Tom Preston-Werner", owner["name"])

		db, ok := m["database"].(map[string]interface{})
		require.True(t, ok, "database should be a map")
		assert.Equal(t, true, db["enabled"])

		ports, ok := db["ports"].([]interface{})
		require.True(t, ok, "database.ports should be an array")
		assert.Equal(t, []interface{}{int64(8000), int64(8001), int64(8002)}, ports) // TOML int -> int64

		servers, ok := m["servers"].([]interface{})
		require.True(t, ok, "servers should be an array of maps")
		require.Len(t, servers, 2)
		firstServer, ok := servers[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "alpha", firstServer["host"])
	})

	t.Run("InvalidToml", func(t *testing.T) {
		invalidToml := []byte("this = not = valid")
		_, err := TomlToMap(invalidToml)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal TOML to map")
	})

	t.Run("EmptyToml", func(t *testing.T) {
		emptyToml := []byte("")
		m, err := TomlToMap(emptyToml)
		require.NoError(t, err) // go-toml/v2 unmarshals empty string to empty map without error
		assert.NotNil(t, m)
		assert.Empty(t, m, "Map should be empty for empty TOML")
	})
}
