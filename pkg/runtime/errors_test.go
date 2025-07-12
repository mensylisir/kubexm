package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestInitializationError_Error(t *testing.T) {
	tests := []struct {
		name      string
		subErrors []error
		want      string
	}{
		{
			name:      "no errors",
			subErrors: []error{},
			want:      "no initialization errors",
		},
		{
			name:      "single error",
			subErrors: []error{errors.New("connection failed")},
			want:      "runtime initialization failed: connection failed",
		},
		{
			name: "multiple errors",
			subErrors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
				errors.New("error 3"),
			},
			want: "runtime initialization failed with 3 errors:\n  [1] error 1\n  [2] error 2\n  [3] error 3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &InitializationError{
				SubErrors: tt.subErrors,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("InitializationError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitializationError_Add(t *testing.T) {
	e := &InitializationError{}

	// Test adding nil error
	e.Add(nil)
	if len(e.SubErrors) != 0 {
		t.Errorf("Adding nil error should not increase SubErrors count, got %d", len(e.SubErrors))
	}

	// Test adding valid error
	err1 := errors.New("first error")
	e.Add(err1)
	if len(e.SubErrors) != 1 {
		t.Errorf("Adding valid error should increase SubErrors count to 1, got %d", len(e.SubErrors))
	}
	if e.SubErrors[0] != err1 {
		t.Errorf("Added error should be the same as provided, got %v, want %v", e.SubErrors[0], err1)
	}

	// Test adding another valid error
	err2 := errors.New("second error")
	e.Add(err2)
	if len(e.SubErrors) != 2 {
		t.Errorf("Adding second error should increase SubErrors count to 2, got %d", len(e.SubErrors))
	}
	if e.SubErrors[1] != err2 {
		t.Errorf("Second added error should be the same as provided, got %v, want %v", e.SubErrors[1], err2)
	}
}

func TestInitializationError_IsEmpty(t *testing.T) {
	tests := []struct {
		name      string
		subErrors []error
		want      bool
	}{
		{
			name:      "empty errors",
			subErrors: []error{},
			want:      true,
		},
		{
			name:      "nil errors",
			subErrors: nil,
			want:      true,
		},
		{
			name:      "with errors",
			subErrors: []error{errors.New("some error")},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &InitializationError{
				SubErrors: tt.subErrors,
			}
			if got := e.IsEmpty(); got != tt.want {
				t.Errorf("InitializationError.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitializationError_Unwrap(t *testing.T) {
	tests := []struct {
		name      string
		subErrors []error
		want      error
	}{
		{
			name:      "empty errors",
			subErrors: []error{},
			want:      nil,
		},
		{
			name:      "nil errors",
			subErrors: nil,
			want:      nil,
		},
		{
			name:      "single error",
			subErrors: []error{errors.New("first error")},
			want:      errors.New("first error"),
		},
		{
			name: "multiple errors",
			subErrors: []error{
				errors.New("first error"),
				errors.New("second error"),
			},
			want: errors.New("first error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &InitializationError{
				SubErrors: tt.subErrors,
			}
			got := e.Unwrap()
			if tt.want == nil {
				if got != nil {
					t.Errorf("InitializationError.Unwrap() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("InitializationError.Unwrap() = nil, want %v", tt.want)
				} else if got.Error() != tt.want.Error() {
					t.Errorf("InitializationError.Unwrap() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestInitializationError_ErrorsIntegration(t *testing.T) {
	baseErr := errors.New("base error")
	initErr := &InitializationError{}
	initErr.Add(baseErr)

	// Test that errors.Is works with the unwrapped error
	if !errors.Is(initErr, baseErr) {
		t.Error("errors.Is should return true for base error")
	}

	// Test that errors.As works
	var target *InitializationError
	if !errors.As(initErr, &target) {
		t.Error("errors.As should successfully extract InitializationError")
	}
	if target != initErr {
		t.Error("errors.As should extract the same InitializationError instance")
	}
}

func TestInitializationError_ConcurrentAdd(t *testing.T) {
	// Test concurrent adding of errors to ensure basic safety
	e := &InitializationError{}
	
	// Add errors concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			e.Add(errors.New("error " + string(rune('0'+index))))
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	if len(e.SubErrors) != 10 {
		t.Errorf("Expected 10 errors after concurrent addition, got %d", len(e.SubErrors))
	}
}

func TestInitializationError_LargeErrorList(t *testing.T) {
	e := &InitializationError{}
	
	// Add many errors
	for i := 0; i < 1000; i++ {
		e.Add(errors.New("error"))
	}

	// Test that Error() method can handle large number of errors
	errorStr := e.Error()
	if !strings.Contains(errorStr, "1000 errors") {
		t.Error("Error string should mention 1000 errors")
	}
	if !strings.Contains(errorStr, "[1000]") {
		t.Error("Error string should include the last error index [1000]")
	}
}