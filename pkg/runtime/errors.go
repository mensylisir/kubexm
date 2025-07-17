package runtime

import (
	"fmt"
	"strings"
)

type InitializationError struct {
	SubErrors []error
}

func (e *InitializationError) Error() string {
	if len(e.SubErrors) == 0 {
		return "no initialization errors"
	}
	if len(e.SubErrors) == 1 {
		return fmt.Sprintf("runtime initialization failed: %v", e.SubErrors[0])
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("runtime initialization failed with %d errors:\n", len(e.SubErrors)))
	for i, err := range e.SubErrors {
		sb.WriteString(fmt.Sprintf("  [%d] %v\n", i+1, err))
	}
	return sb.String()
}
func (e *InitializationError) Add(err error) {
	if err != nil {
		e.SubErrors = append(e.SubErrors, err)
	}
}

func (e *InitializationError) IsEmpty() bool {
	return len(e.SubErrors) == 0
}
func (e *InitializationError) Unwrap() error {
	if len(e.SubErrors) == 0 {
		return nil
	}
	return e.SubErrors[0]
}
