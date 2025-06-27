package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoolPtr(t *testing.T) {
	bTrue := true
	ptrTrue := boolPtr(bTrue)
	assert.NotNil(t, ptrTrue)
	assert.Equal(t, bTrue, *ptrTrue)

	bFalse := false
	ptrFalse := boolPtr(bFalse)
	assert.NotNil(t, ptrFalse)
	assert.Equal(t, bFalse, *ptrFalse)
}

func TestInt32Ptr(t *testing.T) {
	val := int32(123)
	ptr := int32Ptr(val)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)

	valZero := int32(0)
	ptrZero := int32Ptr(valZero)
	assert.NotNil(t, ptrZero)
	assert.Equal(t, valZero, *ptrZero)
}

func TestStringPtr(t *testing.T) {
	val := "hello"
	ptr := stringPtr(val)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)

	valEmpty := ""
	ptrEmpty := stringPtr(valEmpty)
	assert.NotNil(t, ptrEmpty)
	assert.Equal(t, valEmpty, *ptrEmpty)
}

func TestIntPtr(t *testing.T) {
	val := 456
	ptr := intPtr(val)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)

	valZero := 0
	ptrZero := intPtr(valZero)
	assert.NotNil(t, ptrZero)
	assert.Equal(t, valZero, *ptrZero)
}

func TestUintPtr(t *testing.T) {
	val := uint(789)
	ptr := uintPtr(val)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)

	valZero := uint(0)
	ptrZero := uintPtr(valZero)
	assert.NotNil(t, ptrZero)
	assert.Equal(t, valZero, *ptrZero)
}

func TestInt64Ptr(t *testing.T) {
	val := int64(1234567890)
	ptr := int64Ptr(val)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)

	valZero := int64(0)
	ptrZero := int64Ptr(valZero)
	assert.NotNil(t, ptrZero)
	assert.Equal(t, valZero, *ptrZero)
}

func TestUint64Ptr(t *testing.T) {
	val := uint64(9876543210)
	ptr := uint64Ptr(val)
	assert.NotNil(t, ptr)
	assert.Equal(t, val, *ptr)

	valZero := uint64(0)
	ptrZero := uint64Ptr(valZero)
	assert.NotNil(t, ptrZero)
	assert.Equal(t, valZero, *ptrZero)
}

func TestContainsString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	assert.True(t, containsString(slice, "banana"))
	assert.False(t, containsString(slice, "grape"))
	assert.False(t, containsString([]string{}, "apple"))
	assert.False(t, containsString(nil, "apple"))
	assert.True(t, containsString([]string{"", "a"}, ""))
}
