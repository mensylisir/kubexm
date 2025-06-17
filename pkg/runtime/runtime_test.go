package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	// "sync" // No longer needed directly in tests with current approach
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/connector" // For type assertion
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner" // For runner.Runner type and runner.Facts
)


func TestNewRuntime_Success(t *testing.T) {
	cfg := &config.Cluster{
		Spec: config.ClusterSpec{
			Global: config.GlobalSpec{ConnectionTimeout: 10 * time.Second, WorkDir: "/global/work"},
			Hosts: []config.HostSpec{
				{Name: "host1", Address: "192.168.1.1", Port: 22, User: "user1", Roles: []string{"master", "etcd"}, Type: "ssh", WorkDir: "/host1/work"},
				{Name: "host2", Address: "192.168.1.2", Port: 22, User: "user2", Roles: []string{"worker"}, Type: ""}, // Default to SSH
				{Name: "local1", Type: "local", Roles: []string{"local"}},
			},
		},
	}
	// Use a throwaway logger for this test, or initialize global logger if preferred for general test setup
	testLoggerOpts := logger.DefaultOptions()
	testLoggerOpts.ConsoleOutput = false // Disable console output during tests unless debugging
	testLoggerOpts.FileOutput = false
	log, _ := logger.NewLogger(testLoggerOpts)


	// Store original runner.NewRunner and defer restore
	origNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origNewRunner }()

	var newRunnerCallCount int
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		newRunnerCallCount++
		// The mock connector passed to NewRuntime will be used by the actual NewRunner,
		// so its GetOS and Exec methods will be called for fact gathering.
		// We just need to return a valid runner struct.
		// The facts will be populated by runner.NewRunner using the provided mockConnectorForRuntime.
		// We can use the actual runner.NewRunner's logic but with our mock connector.
		// For this test, we'll call the original runner.NewRunner but it will use the mock connector.
		// This ensures runner.NewRunner's fact-gathering is indirectly tested with the mock.
		// This is more of an integration test for NewRuntime + (NewRunner with mock connector).
		// If NewRunner was very complex, we might return a simpler mock runner here.
		// For now, let the actual NewRunner run with the mockConnectorForRuntime.
		// The mockConnectorForRuntime is instantiated by NewRuntime itself when it creates connectors.
		// This test setup means we are not directly passing a mock connector to runnerNewRunner here.
		// runnerNewRunner is called with the connector that NewRuntime created (which is SSH or Local).
		// To test with mockConnectorForRuntime, NewRuntime itself would need to use it.

		// Let's simplify: Assume runner.NewRunner succeeds and returns a basic runner.
		// The actual fact gathering is tested in runner's own tests.
		// Here, we care that NewRuntime wires things up.
		mockOS := &connector.OS{ID: "linux-from-mock-runner", Arch: "amd64"}
		if c, ok := conn.(*mockConnectorForRuntime); ok { // If NewRuntime was changed to use this mock type
			mockOS, _ = c.GetOS(ctx)
		}
		return &runner.Runner{
			Conn: conn, // The connector NewRuntime created
			Facts: &runner.Facts{OS: mockOS, Hostname: "fakehostname"},
		}, nil
	}

	// For this test, to ensure SSHConnector and LocalConnector are correctly chosen by NewRuntime,
	// we don't mock the connector creation part of NewRuntime.
	// Instead, we'll rely on the default behavior of mockConnectorForRuntime if those were used,
	// or the actual connectors. The current NewRuntime creates actual SSH/Local connectors.
	// The runnerNewRunner mock above is for the *runner* creation step.


	rt, err := NewRuntime(cfg, log)
	if err != nil {
		t.Fatalf("NewRuntime() error = %v, wantErr nil", err)
	}

	if rt == nil {
		t.Fatal("NewRuntime() returned nil runtime, want non-nil")
	}
	if len(rt.Hosts) != 3 {
		t.Errorf("len(rt.Hosts) = %d, want 3", len(rt.Hosts))
	}
	if newRunnerCallCount != 3 {
		t.Errorf("runner.NewRunner expected to be called 3 times, got %d", newRunnerCallCount)
	}

	host1 := rt.GetHost("host1")
	if host1 == nil {
		t.Fatal("rt.GetHost(\"host1\") returned nil")
	}
	if !host1.HasRole("master") || !host1.HasRole("etcd") {
		t.Errorf("host1 roles = %v, want master and etcd", host1.Roles)
	}
	if _, ok := host1.Connector.(*connector.SSHConnector); !ok {
		t.Errorf("host1 connector is not SSHConnector, got %T", host1.Connector)
	}
	if host1.WorkDir != "/host1/work" {
		t.Errorf("host1.WorkDir = %s, want /host1/work", host1.WorkDir)
	}


	host2 := rt.GetHost("host2")
	if host2 == nil {
		t.Fatal("rt.GetHost(\"host2\") returned nil")
	}
	if _, ok := host2.Connector.(*connector.SSHConnector); !ok { // Default type is SSH
		t.Errorf("host2 connector is not SSHConnector, got %T", host2.Connector)
	}
	if host2.WorkDir != "/global/work" { // Should inherit global
		t.Errorf("host2.WorkDir = %s, want /global/work", host2.WorkDir)
	}


	localHost := rt.GetHost("local1")
	if localHost == nil {
		t.Fatal("rt.GetHost(\"local1\") returned nil")
	}
    if _, ok := localHost.Connector.(*connector.LocalConnector); !ok {
		t.Errorf("local1 connector is not LocalConnector, got %T", localHost.Connector)
	}


	if len(rt.GetHostsByRole("master")) != 1 {
		t.Errorf("len(rt.GetHostsByRole(\"master\")) = %d, want 1", len(rt.GetHostsByRole("master")))
	}
	if len(rt.GetHostsByRole("worker")) != 1 {
		t.Errorf("len(rt.GetHostsByRole(\"worker\")) = %d, want 1", len(rt.GetHostsByRole("worker")))
	}
	if rt.Logger == nil {
		t.Error("ClusterRuntime.Logger is nil")
	}
}

func TestNewRuntime_ConnectionFailure(t *testing.T) {
	cfg := &config.Cluster{
		Spec: config.ClusterSpec{
			Hosts: []config.HostSpec{
				{Name: "host1", Address: "1.1.1.1", Type: "ssh", PrivateKeyPath: "valid/path"}, // Assume this one is fine for the mock
				{Name: "host2", Address: "2.2.2.2", Type: "ssh", PrivateKeyPath: "/path/to/nonexistent/key/for/host2"},
			},
			Global: config.GlobalSpec{ConnectionTimeout: 1 * time.Millisecond}, // Very short timeout
		},
	}
	log, _ := logger.NewLogger(logger.Options{ConsoleOutput:false, FileOutput:false})

	// Mock ReadFile to simulate failure for host2's key
	originalOsReadFile := osReadFile
	osReadFile = func(name string) ([]byte, error) {
		if name == "/path/to/nonexistent/key/for/host2" {
			return nil, os.ErrNotExist
		}
		if name == "valid/path" { // For host1
			return []byte("fake_key_data"), nil
		}
		return originalOsReadFile(name) // Call original for other cases if any
	}
	defer func() { osReadFile = originalOsReadFile }()


	// Mock runner.NewRunner to succeed if connection part succeeds
	origNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origNewRunner }()
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		return &runner.Runner{Facts: &runner.Facts{OS: &connector.OS{ID:"linux"}, Hostname: "mock"}}, nil
	}


	rt, err := NewRuntime(cfg, log)

	if err == nil {
		t.Fatal("NewRuntime() with a failing host expected error, got nil")
	}
	if rt != nil { // Expect rt to be nil because one host failed, and errgroup aborts.
		t.Errorf("NewRuntime() returned non-nil runtime on error: %+v", rt)
	}

	initErr, ok := err.(*InitializationError)
	if !ok {
		t.Fatalf("Expected InitializationError, got %T: %v", err, err)
	}
	if len(initErr.SubErrors) == 0 { // Should be at least 1 for the ReadFile error
		t.Errorf("Expected at least 1 sub-error, got 0")
	} else {
		if !strings.Contains(initErr.SubErrors[0].Error(), "host2") ||
		   !strings.Contains(initErr.SubErrors[0].Error(), "failed to read private key") {
			t.Errorf("Sub-error message mismatch: %v", initErr.SubErrors[0])
		}
	}
}


func TestNewRuntime_RunnerInitializationFailure(t *testing.T) {
	cfg := &config.Cluster{
		Spec: config.ClusterSpec{
			Hosts: []config.HostSpec{{Name: "host1", Address: "1.1.1.1", Type: "ssh"}},
			Global: config.GlobalSpec{ConnectionTimeout: 1 * time.Second},
		},
	}
	log, _ := logger.NewLogger(logger.Options{ConsoleOutput: false, FileOutput: false})


	origNewRunner := runnerNewRunner
	defer func() { runnerNewRunner = origNewRunner }()

	expectedRunnerErr := errors.New("runner init deliberately failed")
	runnerNewRunner = func(ctx context.Context, conn connector.Connector) (*runner.Runner, error) {
		return nil, expectedRunnerErr // Fail runner creation
	}

	_, err := NewRuntime(cfg, log)
	if err == nil {
		t.Fatal("NewRuntime() with failing runner init expected error, got nil")
	}
	initErr, ok := err.(*InitializationError)
	if !ok {
		t.Fatalf("Expected InitializationError, got %T: %v", err, err)
	}
	if len(initErr.SubErrors) != 1 {
		t.Errorf("Expected 1 sub-error for runner failure, got %d", len(initErr.SubErrors))
	} else {
		if !strings.Contains(initErr.SubErrors[0].Error(), "host1") ||
		!strings.Contains(initErr.SubErrors[0].Error(), "runner initialization failed") ||
		!errors.Is(initErr.SubErrors[0], expectedRunnerErr) {
			t.Errorf("Runner init sub-error message/type mismatch: %v", initErr.SubErrors[0])
		}
	}
}


func TestNewHostContext(t *testing.T) {
	host := &Host{Name: "test-host", Address: "1.2.3.4"}
	logOpts := logger.DefaultOptions()
	logOpts.ConsoleOutput = false; logOpts.FileOutput = false;
	clusterLogger, _ := logger.NewLogger(logOpts)

	clusterRt := &ClusterRuntime{
		Logger: clusterLogger,
		ClusterConfig: &config.Cluster{}, // Avoid nil panic in logger if it tries to access this
	}
	goCtx := context.WithValue(context.Background(), "testkey", "testvalue")

	hc := NewHostContext(goCtx, host, clusterRt)

	if hc.GoContext == nil {
		t.Error("NewHostContext().GoContext is nil")
	}
	if val := hc.GoContext.Value("testkey"); val != "testvalue" {
		t.Errorf("GoContext value mismatch, got %v, want testvalue", val)
	}
	if hc.Host != host {
		t.Error("NewHostContext().Host mismatch")
	}
	if hc.Cluster != clusterRt {
		t.Error("NewHostContext().Cluster mismatch")
	}
	if hc.Logger == nil {
		t.Error("NewHostContext().Logger is nil")
	}
	if hc.SharedData == nil {
		t.Error("NewHostContext().SharedData is nil")
	}

	// Test if logger has host fields. This requires capturing output or a more introspectable logger.
	// For now, we assume the .With() call in NewHostContext correctly adds fields.
	// A more complex test could involve a custom zapcore.Core to inspect fields.
}

func TestInitializationError_Methods(t *testing.T) {
	err1 := errors.New("first error")
	err2 := errors.New("second error")

	initErr := &InitializationError{}
	if !initErr.IsEmpty() {
		t.Error("IsEmpty() should be true for new InitializationError")
	}
	if initErr.Error() != "no initialization errors" {
		t.Errorf("Error() for empty = %q, want 'no initialization errors'", initErr.Error())
	}

	initErr.Add(err1)
	if initErr.IsEmpty() {
		t.Error("IsEmpty() should be false after adding one error")
	}
	if !strings.Contains(initErr.Error(), "runtime initialization failed: first error") {
		t.Errorf("Error() for one error = %q, expected to contain 'runtime initialization failed: first error'", initErr.Error())
	}
	if unwrapped := initErr.Unwrap(); unwrapped != err1 {
		t.Errorf("Unwrap() for one error = %v, want %v", unwrapped, err1)
	}


	initErr.Add(err2)
	// Normalize newline for consistent comparison, as fmt.Sprintf might behave differently or OS might affect it.
	expectedMultiErrorStringPart1 := "runtime initialization failed with 2 errors:"
	expectedMultiErrorStringPart2 := "[1] first error"
	expectedMultiErrorStringPart3 := "[2] second error"

	actualErrorString := initErr.Error()
	if !strings.Contains(actualErrorString, expectedMultiErrorStringPart1) ||
	   !strings.Contains(actualErrorString, expectedMultiErrorStringPart2) ||
	   !strings.Contains(actualErrorString, expectedMultiErrorStringPart3) {
		t.Errorf("Error() for multiple errors = %q, expected to contain key parts ('%s', '%s', '%s')",
			actualErrorString, expectedMultiErrorStringPart1, expectedMultiErrorStringPart2, expectedMultiErrorStringPart3)
	}

	if unwrapped := initErr.Unwrap(); unwrapped != err1 { // Unwrap still returns the first
		t.Errorf("Unwrap() for multiple errors = %v, want first error %v", unwrapped, err1)
	}

	initErr.Add(nil) // Should not add nil
	if len(initErr.SubErrors) != 2 {
		t.Error("Add(nil) should not increase sub-error count")
	}
}

// To allow mocking os.ReadFile for tests
var osReadFile = os.ReadFile
