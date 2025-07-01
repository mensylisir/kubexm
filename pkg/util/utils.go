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
