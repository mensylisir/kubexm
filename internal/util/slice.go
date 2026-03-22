package util

import (
	"cmp"
	"sort"
)

func IsInStringSlice(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func IsInStringSliceWithMap(slice []string, str string) bool {
	lookupMap := make(map[string]struct{}, len(slice))
	for _, item := range slice {
		lookupMap[item] = struct{}{}
	}
	_, ok := lookupMap[str]
	return ok
}

func IsInSortedStringSlice(sortedSlice []string, str string) bool {
	index := sort.SearchStrings(sortedSlice, str)
	return index < len(sortedSlice) && sortedSlice[index] == str
}

func Contains[S ~[]E, E comparable](slice S, v E) bool {
	for i := range slice {
		if slice[i] == v {
			return true
		}
	}
	return false
}

func BinarySearch[S ~[]E, E cmp.Ordered](sortedSlice S, v E) bool {
	idx, found := sort.Find(len(sortedSlice), func(i int) int {
		return cmp.Compare(sortedSlice[i], v)
	})
	return found && sortedSlice[idx] == v
}

func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func Map[S ~[]E, E any, T any](s S, f func(E) T) []T {
	result := make([]T, len(s))
	for i, v := range s {
		result[i] = f(v)
	}
	return result
}

func Filter[S ~[]E, E any](s S, f func(E) bool) S {
	result := make(S, 0)
	for _, v := range s {
		if f(v) {
			result = append(result, v)
		}
	}
	return result
}

func Unique[S ~[]E, E comparable](s S) S {
	if len(s) < 2 {
		return s
	}
	seen := make(map[E]struct{}, len(s))
	result := make(S, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
