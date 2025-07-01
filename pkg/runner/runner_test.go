package runner

import (
	"testing"
	// Note: Imports like "context", "errors", "fmt", "strings", "time"
	// and "github.com/mensylisir/kubexm/pkg/connector" may no longer be needed
	// if the removed tests were the only ones using them.
	// Go's tools (like goimports or editor plugins) will typically clean these up.
)

// TestNewRunner tests the NewRunner constructor.
func TestNewRunner(t *testing.T) {
	r := NewRunner()
	if r == nil {
		t.Fatal("NewRunner() returned nil")
	}
	if _, ok := r.(*defaultRunner); !ok {
		t.Errorf("NewRunner() did not return a *defaultRunner, got %T", r)
	}
}

// Other test functions (TestDefaultRunner_GatherFacts, TestRunner_DeployAndEnableService, TestRunner_Reboot)
// and any associated helper types (like a custom MockConnector if it was defined in this file)
// have been removed as their functionality is now tested in dedicated files:
// - facts_test.go for GatherFacts
// - deploy_test.go for DeployAndEnableService
// - reboot_test.go for Reboot
// These new tests use testify/mock for mocking dependencies.
