package util

// DifferenceBy returns the difference between two slices based on key function
func DifferenceBy[T any, K comparable](a, b []T, keyFunc func(T) K) []T {
	bMap := make(map[K]struct{}, len(b))
	for _, item := range b {
		bMap[keyFunc(item)] = struct{}{}
	}

	var result []T
	for _, item := range a {
		if _, found := bMap[keyFunc(item)]; !found {
			result = append(result, item)
		}
	}
	return result
}

// UnionBy returns the union of two slices based on key function
func UnionBy[T any, K comparable](a, b []T, keyFunc func(T) K) []T {
	seen := make(map[K]T, len(a)+len(b))
	for _, item := range a {
		key := keyFunc(item)
		seen[key] = item
	}
	for _, item := range b {
		key := keyFunc(item)
		if _, ok := seen[key]; !ok {
			seen[key] = item
		}
	}

	result := make([]T, 0, len(seen))

	for _, item := range seen {
		result = append(result, item)
	}

	return result
}

// RemoveDuplicates removes duplicate strings from a slice
func RemoveDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}
	for v := range elements {
		if encountered[elements[v]] == true {
		} else {
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}
	return result
}