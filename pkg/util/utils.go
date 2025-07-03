package util

import "strings"

// ShellEscape provides basic shell escaping for a string.
// WARNING: This is a simplified version and may not cover all edge cases.
// For production use, a more robust library or approach might be needed if paths can be arbitrary.
func ShellEscape(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// StrPtr returns a pointer to the string value s.
func StrPtr(s string) *string {
	return &s
}

// BoolPtr returns a pointer to the bool value b.
func BoolPtr(b bool) *bool {
	return &b
}

// IntPtr returns a pointer to the int value i.
func IntPtr(i int) *int {
	return &i
}

// Int64Ptr returns a pointer to the int64 value i.
func Int64Ptr(i int64) *int64 {
	return &i
}

// Float64Ptr returns a pointer to the float64 value f.
func Float64Ptr(f float64) *float64 {
	return &f
}

// UintPtr returns a pointer to the uint value u.
func UintPtr(u uint) *uint {
	return &u
}

// Uint32Ptr returns a pointer to the uint32 value u.
func Uint32Ptr(u uint32) *uint32 {
	return &u
}

// Uint64Ptr returns a pointer to the uint64 value u.
func Uint64Ptr(u uint64) *uint64 {
	return &u
}
