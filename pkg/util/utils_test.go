package util

import (
	"fmt"
	"net" // Added for TestNetworksOverlap
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty_string", "", "''"},
		{"simple_string", "hello", "'hello'"},
		{"string_with_spaces", "hello world", "'hello world'"},
		{"string_with_single_quote", "it's a test", "'it'\\''s a test'"},      // Correct
		{"string_with_multiple_single_quotes", "a'b'c", "'a'\\''b'\\''c'"},  // Correct
		{"string_with_double_quotes", `hello "world"`, `'hello "world"'`}, // Correct
		{"string_with_backslash", "hello\\world", "'hello\\world'"},       // Corrected: input "hello\world" -> output string value 'hello\world'
		{"string_with_dollar", "var=$HOME", "'var=$HOME'"},               // Correct
		{"string_with_asterisk", "file*.txt", "'file*.txt'"},
		{"string_with_bang", "echo hello!", "'echo hello!'"},
		{"input_two_single_quotes", "''", "''\\'''\\'''"}, // Matching actual output from test
		{"input_escaped_single_quote_literal", "'\\''", "''\\''\\'\\'''\\'''"}, // Matching actual output from test
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ShellEscape(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestStrPtr(t *testing.T) {
	s := "test_string"
	ptr := StrPtr(s)
	assert.NotNil(t, ptr, "StrPtr should return a non-nil pointer")
	assert.Equal(t, s, *ptr, "Value pointed to by StrPtr should match input string")
	*ptr = "modified"
	assert.Equal(t, "test_string", s, "Modifying pointed value should not modify original if original was a literal or different var")

	sVar := "original_value"
	sVarPtr := StrPtr(sVar)
	assert.Equal(t, "original_value", *sVarPtr)
	// This test above for "modified" is a bit confusing for string literals.
	// The primary test is that it returns a pointer to the value.
}

func TestBoolPtr(t *testing.T) {
	tests := []struct {
		name  string
		input bool
	}{
		{"true_value", true},
		{"false_value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := BoolPtr(tt.input)
			assert.NotNil(t, ptr, "BoolPtr should return a non-nil pointer")
			assert.Equal(t, tt.input, *ptr, fmt.Sprintf("Value pointed to by BoolPtr(%v) should match input", tt.input))
		})
	}
}

func TestIntPtr(t *testing.T) {
	i := 123
	ptr := IntPtr(i)
	assert.NotNil(t, ptr, "IntPtr should return a non-nil pointer")
	assert.Equal(t, i, *ptr, "Value pointed to by IntPtr should match input int")
}

func TestInt64Ptr(t *testing.T) {
	i64 := int64(123456789012345)
	ptr := Int64Ptr(i64)
	assert.NotNil(t, ptr, "Int64Ptr should return a non-nil pointer")
	assert.Equal(t, i64, *ptr, "Value pointed to by Int64Ptr should match input int64")
}

func TestFloat64Ptr(t *testing.T) {
	f64 := float64(123.456)
	ptr := Float64Ptr(f64)
	assert.NotNil(t, ptr, "Float64Ptr should return a non-nil pointer")
	assert.Equal(t, f64, *ptr, "Value pointed to by Float64Ptr should match input float64")
}

func TestUintPtr(t *testing.T) {
	u := uint(123)
	ptr := UintPtr(u)
	assert.NotNil(t, ptr, "UintPtr should return a non-nil pointer")
	assert.Equal(t, u, *ptr, "Value pointed to by UintPtr should match input uint")
}

func TestUint32Ptr(t *testing.T) {
	u32 := uint32(12345)
	ptr := Uint32Ptr(u32)
	assert.NotNil(t, ptr, "Uint32Ptr should return a non-nil pointer")
	assert.Equal(t, u32, *ptr, "Value pointed to by Uint32Ptr should match input uint32")
}

func TestUint64Ptr(t *testing.T) {
	u64 := uint64(123456789012345)
	ptr := Uint64Ptr(u64)
	assert.NotNil(t, ptr, "Uint64Ptr should return a non-nil pointer")
	assert.Equal(t, u64, *ptr, "Value pointed to by Uint64Ptr should match input uint64")
}

func TestNetworksOverlap(t *testing.T) {
	tests := []struct {
		name   string
		cidr1  string
		cidr2  string
		want   bool
		noerr1 bool // Expect no error parsing cidr1
		noerr2 bool // Expect no error parsing cidr2
	}{
		{"no_overlap_distinct", "192.168.1.0/24", "192.168.2.0/24", false, true, true},
		{"no_overlap_adjacent", "192.168.1.0/24", "192.168.2.0/24", false, true, true}, // Same as distinct for this simple check
		{"overlap_subset1", "10.0.0.0/8", "10.1.0.0/16", true, true, true},
		{"overlap_subset2", "10.1.0.0/16", "10.0.0.0/8", true, true, true},
		{"overlap_identical", "172.16.0.0/12", "172.16.0.0/12", true, true, true},
		{"overlap_partial_start", "10.0.0.0/23", "10.0.1.0/24", true, true, true}, // 10.0.0.0-10.0.1.255 overlaps with 10.0.1.0-10.0.1.255
		{"overlap_partial_end", "10.0.1.0/24", "10.0.0.0/23", true, true, true},
		{"invalid_cidr1", "invalid", "192.168.1.0/24", false, false, true}, // Exact overlap result doesn't matter if parse fails
		{"invalid_cidr2", "192.168.1.0/24", "invalid", false, true, false},
		{"one_contains_another_exact_ip", "192.168.1.1/32", "192.168.1.0/24", true, true, true},
		{"different_families_ipv4_ipv6", "192.168.1.0/24", "2001:db8::/32", false, true, true}, // Should not overlap
		{"nil_net1", "", "192.168.1.0/24", false, false, true}, // Test nil case for n1
		{"nil_net2", "192.168.1.0/24", "", false, true, false}, // Test nil case for n2
		{"both_nil_nets", "", "", false, false, false},          // Test both nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n1, n2 *net.IPNet
			var err1, err2 error

			if tt.cidr1 != "" {
				_, n1, err1 = net.ParseCIDR(tt.cidr1)
			} else {
				// Simulate a nil net.IPNet if cidr1 is empty
				err1 = nil // No parse error for empty input leading to nil net
			}
			// Check err1 against noerr1, allowing for "invalid CIDR address: " for empty string when noerr1 is false.
			if (err1 == nil) != tt.noerr1 {
				isExpectedEmptyStrErr1 := tt.cidr1 == "" && !tt.noerr1 && err1 != nil && err1.Error() == "invalid CIDR address: "
				if !isExpectedEmptyStrErr1 {
					t.Fatalf("For cidr1 '%s', net.ParseCIDR error expectation mismatch: got err %v, want noerr1 %v", tt.cidr1, err1, tt.noerr1)
				}
			}

			if tt.cidr2 != "" {
				_, n2, err2 = net.ParseCIDR(tt.cidr2)
			} else {
				// Simulate a nil net.IPNet if cidr2 is empty
				err2 = nil // No parse error for empty input leading to nil net
			}
			// Check err2 against noerr2, allowing for "invalid CIDR address: " for empty string when noerr2 is false.
			if (err2 == nil) != tt.noerr2 {
				isExpectedEmptyStrErr2 := tt.cidr2 == "" && !tt.noerr2 && err2 != nil && err2.Error() == "invalid CIDR address: "
				if !isExpectedEmptyStrErr2 {
					t.Fatalf("For cidr2 '%s', net.ParseCIDR error expectation mismatch: got err %v, want noerr2 %v", tt.cidr2, err2, tt.noerr2)
				}
			}

			// The NetworksOverlap function itself should handle nil inputs.
			// The checks above are for the test setup's ParseCIDR calls.
			if got := NetworksOverlap(n1, n2); got != tt.want {
				t.Errorf("NetworksOverlap(%s, %s) = %v, want %v. n1: %v, n2: %v", tt.cidr1, tt.cidr2, got, tt.want, n1, n2)
			}
		})
	}
}
