package helpers

func StrPtr(s string) *string {
	return &s
}

func BoolPtr(b bool) *bool {
	return &b
}

func IntPtr(i int) *int {
	return &i
}

func Int64Ptr(i int64) *int64 {
	return &i
}

func Float64Ptr(f float64) *float64 {
	return &f
}

func UintPtr(u uint) *uint {
	return &u
}

func Uint32Ptr(u uint32) *uint32 {
	return &u
}

// Uint64Ptr returns a pointer to the uint64 value u.
func Uint64Ptr(u uint64) *uint64 {
	return &u
}

// Int32Ptr returns a pointer to the int32 value i.
func Int32Ptr(i int32) *int32 {
	return &i
}
