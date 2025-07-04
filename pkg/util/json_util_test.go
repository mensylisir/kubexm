package util

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const sampleJsonData = `
{
  "name": "Jules",
  "age": 30,
  "isEngineer": true,
  "address": {
    "street": "123 Main St",
    "city": "Anytown"
  },
  "friends": [
    {"first": "Alex", "last": "Smith"},
    {"first": "Beth", "last": "Jones"}
  ],
  "scores": [100, 95, 88],
  "metadata": null
}
`

func TestGetJsonValue(t *testing.T) {
	jsonBytes := []byte(sampleJsonData)

	t.Run("GetString", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "name")
		require.True(t, result.Exists(), "Expected 'name' to exist")
		assert.Equal(t, "Jules", result.String())
		assert.Equal(t, gjson.String, result.Type)
	})

	t.Run("GetNumberAsInt", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "age")
		require.True(t, result.Exists())
		assert.Equal(t, int64(30), result.Int()) // gjson.Result.Int() returns int64
		assert.Equal(t, gjson.Number, result.Type)
	})

	t.Run("GetBool", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "isEngineer")
		require.True(t, result.Exists())
		assert.Equal(t, true, result.Bool())
		assert.Equal(t, gjson.True, result.Type) // or gjson.False
	})

	t.Run("GetNestedString", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "address.city")
		require.True(t, result.Exists())
		assert.Equal(t, "Anytown", result.String())
	})

	t.Run("GetFromArrayByIndex", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "friends.1.first") // Corrected path
		require.True(t, result.Exists(), "Expected 'friends.1.first' to exist")
		assert.Equal(t, "Beth", result.String())
	})

	t.Run("GetEntireObject", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "address")
		require.True(t, result.Exists())
		assert.True(t, result.IsObject(), "Expected address to be an object")
		// Can further assert by parsing result.Raw or result.Value().(map[string]interface{})
		var addrMap map[string]interface{}
		err := json.Unmarshal([]byte(result.Raw), &addrMap)
		require.NoError(t, err)
		assert.Equal(t, "123 Main St", addrMap["street"])
	})

	t.Run("GetEntireArray", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "scores")
		require.True(t, result.Exists())
		assert.True(t, result.IsArray(), "Expected scores to be an array")
		expectedScores := []interface{}{float64(100), float64(95), float64(88)} // Numbers in JSON arrays become float64 when unmarshalled to []interface{}
		assert.Equal(t, expectedScores, result.Value())
	})

	t.Run("GetNullValue", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "metadata")
		require.True(t, result.Exists(), "Expected 'metadata' to exist even if null")
		assert.Equal(t, gjson.Null, result.Type, "Expected metadata type to be Null")
		assert.Nil(t, result.Value(), "Expected value of null metadata to be nil via .Value()")
	})


	t.Run("PathNotFound", func(t *testing.T) {
		result := GetJsonValue(jsonBytes, "nonexistent.key")
		assert.False(t, result.Exists(), "Expected 'nonexistent.key' to not exist")
	})

	t.Run("InvalidJsonData", func(t *testing.T) {
		invalidJson := []byte("{name: Jules") // Missing quote and closing brace
		result := GetJsonValue(invalidJson, "name")
		assert.False(t, result.Exists(), "Expected result.Exists() to be false for invalid JSON")
		// gjson doesn't return an error for malformed JSON, it just results in a non-existent value.
	})

	t.Run("EmptyJsonData", func(t *testing.T) {
		emptyJson := []byte("")
		result := GetJsonValue(emptyJson, "name")
		assert.False(t, result.Exists(), "Expected result.Exists() to be false for empty JSON")
	})
}

func TestSetJsonValue(t *testing.T) {
	jsonBytes := []byte(sampleJsonData)

	t.Run("SetExistingString", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "name", "Julian")
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "name")
		assert.Equal(t, "Julian", result.String())
	})

	t.Run("SetExistingNumber", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "age", 35)
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "age")
		assert.Equal(t, int64(35), result.Int())
	})

	t.Run("SetNewTopLevelKey", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "occupation", "Developer")
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "occupation")
		assert.Equal(t, "Developer", result.String())
	})

	t.Run("SetNewNestedKey", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "address.zip", "90210")
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "address.zip")
		assert.Equal(t, "90210", result.String()) // sjson converts numbers to string if path expects string, or use int
	})

	t.Run("SetNewNestedKeyAsInt", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "address.zipcode", 90210)
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "address.zipcode")
		assert.Equal(t, int64(90210), result.Int())
	})


	t.Run("SetInArray", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "friends[0].first", "Alexander")
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "friends[0].first")
		assert.Equal(t, "Alexander", result.String())
	})

	t.Run("SetNewObject", func(t *testing.T) {
		newPet := map[string]interface{}{"type": "dog", "name": "Buddy"}
		updatedJson, err := SetJsonValue(jsonBytes, "pet", newPet)
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "pet.name")
		assert.Equal(t, "Buddy", result.String())
	})

	t.Run("SetNewArray", func(t *testing.T) {
		newHobbies := []string{"coding", "reading"}
		updatedJson, err := SetJsonValue(jsonBytes, "hobbies", newHobbies)
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "hobbies.1") // Corrected path
		assert.Equal(t, "reading", result.String())
	})

	t.Run("SetReplacingWholeArray", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "scores", []int{10,20})
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "scores")
		assert.True(t, result.IsArray())
		assert.Len(t, result.Array(), 2)
		assert.Equal(t, int64(10), result.Array()[0].Int())
	})

	t.Run("SetAppendingToArray", func(t *testing.T) {
		// sjson path to append: "scores.-1"
		updatedJson, err := SetJsonValue(jsonBytes, "scores.-1", 77)
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "scores")
		assert.True(t, result.IsArray())
		scores := result.Array()
		require.Len(t, scores, 4) // Original 3 + 1 appended
		assert.Equal(t, int64(77), scores[3].Int())
	})

	t.Run("SetNullValue", func(t *testing.T) {
		updatedJson, err := SetJsonValue(jsonBytes, "address", nil)
		require.NoError(t, err)
		result := GetJsonValue(updatedJson, "address")
		require.True(t, result.Exists())
		assert.Equal(t, gjson.Null, result.Type)
	})


	t.Run("InvalidJsonForSet", func(t *testing.T) {
		invalidJson := []byte("{\"name\": \"test\",") // Incomplete JSON
		_, err := SetJsonValue(invalidJson, "name", "newname")
		// sjson might not error on invalid input JSON for all cases,
		// it might try to work with it or produce unexpected output.
		// The behavior depends on how sjson handles partially valid JSON.
		// For completely unparseable, it should error.
		// If it's just "not a valid json object or array", sjson.Set might error.
		if err == nil { // If sjson didn't error, check if output is still bad
			// This case is tricky as sjson might "fix" some minor issues or just work on a fragment.
			// Typically, operating on invalid JSON is undefined.
			// For this test, we expect an error if the JSON is fundamentally broken for parsing by sjson.
			// sjson.SetBytes itself can return an error if the input JSON is invalid.
			t.Logf("sjson.SetBytes did not return an error for apparently invalid JSON, this might be specific to sjson's robustness or the nature of the invalidity.")
		}
		// A more robust test would be to use a clearly invalid JSON that sjson cannot process.
		// e.g. completely non-JSON string
		trulyInvalidJson := []byte("not json at all")
		_, errTrulyInvalid := SetJsonValue(trulyInvalidJson, "name", "newname")
		require.Error(t, errTrulyInvalid, "Expected error when setting value in truly invalid JSON")

	})
}

func TestJsonToMap(t *testing.T) {
	jsonBytes := []byte(sampleJsonData)

	t.Run("ValidJson", func(t *testing.T) {
		m, err := JsonToMap(jsonBytes)
		require.NoError(t, err)
		require.NotNil(t, m)

		assert.Equal(t, "Jules", m["name"])
		assert.Equal(t, float64(30), m["age"]) // Numbers unmarshal to float64 in map[string]interface{}

		addr, ok := m["address"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Anytown", addr["city"])

		friends, ok := m["friends"].([]interface{})
		require.True(t, ok)
		require.Len(t, friends, 2)
		firstFriend, ok := friends[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Alex", firstFriend["first"])

		metadata := m["metadata"]
		assert.Nil(t, metadata, "Expected metadata to be nil in map")
	})

	t.Run("InvalidJson", func(t *testing.T) {
		invalidJson := []byte(`{"name": "test", "age": }`) // Invalid: no value for age
		_, err := JsonToMap(invalidJson)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON to map")
	})

	t.Run("EmptyJsonString", func(t *testing.T) {
		emptyJson := []byte("")
		m, err := JsonToMap(emptyJson)
		require.NoError(t, err, "JsonToMap with empty string should not error")
		assert.NotNil(t, m, "JsonToMap with empty string should return an empty map")
		assert.Empty(t, m, "JsonToMap with empty string should result in an empty map")
	})

	t.Run("NullJsonString", func(t *testing.T) {
		nullJson := []byte("null")
		m, err := JsonToMap(nullJson)
		require.NoError(t, err, "JsonToMap with 'null' string should not error")
		assert.NotNil(t, m, "JsonToMap with 'null' string should return an empty map")
		assert.Empty(t, m, "JsonToMap with 'null' string should result in an empty map")

	})

	t.Run("JsonArrayNotObject", func(t *testing.T) {
		jsonArray := []byte(`["a", "b"]`)
		_, err := JsonToMap(jsonArray) // JsonToMap expects top-level object
		require.Error(t, err)
		assert.Contains(t, err.Error(), "json: cannot unmarshal array into Go value of type map[string]interface {}")
	})
}

func TestJsonToInterface(t *testing.T) {
	t.Run("ValidJsonObject", func(t *testing.T) {
		jsonBytes := []byte(sampleJsonData)
		data, err := JsonToInterface(jsonBytes)
		require.NoError(t, err)
		m, ok := data.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "Jules", m["name"])
	})

	t.Run("ValidJsonArray", func(t *testing.T) {
		jsonArray := []byte(`["a", "b", {"c": 1}]`)
		data, err := JsonToInterface(jsonArray)
		require.NoError(t, err)
		arr, ok := data.([]interface{})
		require.True(t, ok)
		assert.Len(t, arr, 3)
		assert.Equal(t, "a", arr[0])
		mapVal, ok := arr[2].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(1), mapVal["c"])
	})

	t.Run("ValidJsonString", func(t *testing.T) {
		jsonStr := []byte(`"hello"`)
		data, err := JsonToInterface(jsonStr)
		require.NoError(t, err)
		str, ok := data.(string)
		require.True(t, ok)
		assert.Equal(t, "hello", str)
	})

	t.Run("ValidJsonNumber", func(t *testing.T) {
		jsonNum := []byte(`123.45`)
		data, err := JsonToInterface(jsonNum)
		require.NoError(t, err)
		num, ok := data.(float64)
		require.True(t, ok)
		assert.Equal(t, 123.45, num)
	})

	t.Run("ValidJsonNull", func(t *testing.T) {
		jsonNull := []byte(`null`)
		data, err := JsonToInterface(jsonNull)
		require.NoError(t, err)
		assert.Nil(t, data)
	})

	t.Run("InvalidJson", func(t *testing.T) {
		invalidJson := []byte(`{name: "test"`)
		_, err := JsonToInterface(invalidJson)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON to interface")
	})

	t.Run("EmptyJson", func(t *testing.T) {
		emptyJson := []byte("")
		_, err := JsonToInterface(emptyJson)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot unmarshal empty JSON to interface")
	})
}
