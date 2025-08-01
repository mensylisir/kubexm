package helpers

import "golang.org/x/exp/constraints"

// FirstNonEmpty returns the first non-empty string from a list of strings.
func FirstNonEmpty(strings ...string) string {
	for _, s := range strings {
		if s != "" {
			return s
		}
	}
	return ""
}

// FirstNonZero returns the first non-zero value from a list of values.
func FirstNonZero[T comparable](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}

// UniqueStringSlice returns a new slice with unique strings from the input slice.
func UniqueStringSlice(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// Generic helper for integer types
type Integer interface {
	constraints.Integer
}

// FirstNonZeroInteger returns the first non-zero integer from a list of integers.
func FirstNonZeroInteger[T Integer](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}
