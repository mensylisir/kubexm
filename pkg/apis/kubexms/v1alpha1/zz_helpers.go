package v1alpha1

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool {
	return &b
}

// int32Ptr returns a pointer to an int32.
func int32Ptr(i int32) *int32 {
	return &i
}

// stringPtr returns a pointer to a string.
func stringPtr(s string) *string {
	return &s
}

// intPtr returns a pointer to an int.
func intPtr(i int) *int {
    return &i
}

// uintPtr returns a pointer to a uint.
func uintPtr(u uint) *uint {
	return &u
}

// int64Ptr returns a pointer to an int64.
func int64Ptr(i int64) *int64 {
	return &i
}

// uint64Ptr returns a pointer to a uint64.
func uint64Ptr(u uint64) *uint64 {
	return &u
}

// containsString checks if a string slice contains a specific string.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
