package step

import (
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
)

// RetryPolicy defines retry behavior for steps
type RetryPolicy struct {
	MaxRetries        int
	RetryDelay        time.Duration
	BackoffMultiplier float64
	RetryableErrors   []string
}

// ExtendedStep extends the basic Step interface with additional lifecycle methods
type ExtendedStep interface {
	// Basic Step interface
	Meta() *spec.StepMeta
	Precheck(ctx runtime.ExecutionContext) (bool, error)
	Run(ctx runtime.ExecutionContext) error
	Rollback(ctx runtime.ExecutionContext) error
	GetBase() *Base

	// Extended lifecycle methods
	Validate(ctx runtime.ExecutionContext) error                // Validate configuration before execution
	Cleanup(ctx runtime.ExecutionContext) error                 // Cleanup resources after execution (success or failure)
	Retry(ctx runtime.ExecutionContext) error                   // Execute retry logic
	GetStatus(ctx runtime.ExecutionContext) (StepStatus, error) // Get current execution status
}

// StepWithRetry wraps a Step with retry capability
type StepWithRetry struct {
	Step        Step
	RetryPolicy *RetryPolicy
}

func NewStepWithRetry(step Step, policy *RetryPolicy) *StepWithRetry {
	if policy == nil {
		policy = &RetryPolicy{
			MaxRetries:        3,
			RetryDelay:        time.Second * 5,
			BackoffMultiplier: 2.0,
		}
	}
	return &StepWithRetry{
		Step:        step,
		RetryPolicy: policy,
	}
}

// Forward basic Step interface methods
func (s *StepWithRetry) Meta() *spec.StepMeta { return s.Step.Meta() }
func (s *StepWithRetry) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return s.Step.Precheck(ctx)
}
func (s *StepWithRetry) Rollback(ctx runtime.ExecutionContext) error { return s.Step.Rollback(ctx) }
func (s *StepWithRetry) GetBase() *Base                              { return s.Step.GetBase() }

// Forward ExtendedStep methods if the underlying step implements them
func (s *StepWithRetry) Validate(ctx runtime.ExecutionContext) error {
	if validateStep, ok := s.Step.(interface {
		Validate(ctx runtime.ExecutionContext) error
	}); ok {
		return validateStep.Validate(ctx)
	}
	return nil
}

func (s *StepWithRetry) Cleanup(ctx runtime.ExecutionContext) error {
	if cleanupStep, ok := s.Step.(interface {
		Cleanup(ctx runtime.ExecutionContext) error
	}); ok {
		return cleanupStep.Cleanup(ctx)
	}
	return nil
}

func (s *StepWithRetry) Retry(ctx runtime.ExecutionContext) error {
	return s.Run(ctx)
}

func (s *StepWithRetry) GetStatus(ctx runtime.ExecutionContext) (StepStatus, error) {
	if statusStep, ok := s.Step.(interface {
		GetStatus(ctx runtime.ExecutionContext) (StepStatus, error)
	}); ok {
		return statusStep.GetStatus(ctx)
	}
	return StepStatusPending, nil
}

// Run implements retry logic
func (s *StepWithRetry) Run(ctx runtime.ExecutionContext) error {
	var lastErr error
	delay := s.RetryPolicy.RetryDelay

	for attempt := 0; attempt <= s.RetryPolicy.MaxRetries; attempt++ {
		err := s.Step.Run(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		if !s.isRetryableError(err) {
			return err
		}

		if attempt < s.RetryPolicy.MaxRetries {
			ctx.GetLogger().Warnf("Step %s failed (attempt %d/%d), retrying in %v: %v",
				s.Step.Meta().Name, attempt+1, s.RetryPolicy.MaxRetries, delay, err)
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * s.RetryPolicy.BackoffMultiplier)
		}
	}

	return lastErr
}

func (s *StepWithRetry) isRetryableError(err error) bool {
	for _, pattern := range s.RetryPolicy.RetryableErrors {
		if s.contains(err.Error(), pattern) {
			return true
		}
	}
	return false
}

func (s *StepWithRetry) contains(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
