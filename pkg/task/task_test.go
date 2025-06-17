package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/config" // For creating dummy config for task factories
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"

	// Import task factories to test them
	task_containerd "github.com/kubexms/kubexms/pkg/task/containerd" // Aliased to avoid collision
	task_preflight "github.com/kubexms/kubexms/pkg/task/preflight"   // Aliased

	// Import step definitions to potentially use in mock task creation
	// stepPreflight "github.com/kubexms/kubexms/pkg/step/preflight"
	step_containerd "github.com/kubexms/kubexms/pkg/step/containerd" // Aliased for type assertion
)

// MockStep is a simple mock implementation of step.Step for testing Task.Run.
type MockStep struct {
	NameFunc      func() string
	CheckFunc     func(ctx *runtime.Context) (isDone bool, err error)
	RunFunc       func(ctx *runtime.Context) *step.Result
	StepName      string
	CheckIsDone   bool
	CheckError    error
	RunResult     *step.Result // Predefined result to return by RunFunc if set
	RunError      error        // If RunFunc itself should populate Result.Error
	RunShouldFail bool         // If true, RunResult.Status will be "Failed"
	RunDelay      time.Duration // To simulate step taking time for concurrency tests
}

func (m *MockStep) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	if m.StepName == "" {
		return "MockStepUnnamed"
	}
	return m.StepName
}
func (m *MockStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if m.CheckFunc != nil {
		return m.CheckFunc(ctx)
	}
	return m.CheckIsDone, m.CheckError
}
func (m *MockStep) Run(ctx *runtime.Context) *step.Result {
	if m.RunDelay > 0 {
		// Simulate work by sleeping.
		// Ensure GoContext cancellation is respected if the step were real.
		select {
		case <-time.After(m.RunDelay):
		case <-ctx.GoContext.Done():
			// If context is cancelled during sleep, return a result indicating this.
			return step.NewResult(m.Name(), ctx.Host.Name, time.Now().Add(-m.RunDelay), ctx.GoContext.Err())
		}
	}
	if m.RunFunc != nil {
		return m.RunFunc(ctx)
	}
	if m.RunResult != nil { // If a full result is predefined, use it
		return m.RunResult
	}
	// Otherwise, construct a result based on RunError and RunShouldFail
	res := step.NewResult(m.Name(), ctx.Host.Name, time.Now().Add(-m.RunDelay), m.RunError)
	if m.RunShouldFail {
		res.Status = "Failed"
		if m.RunError == nil && res.Error == nil { // Ensure there's an error object if status is Failed
			res.Error = errors.New("mock step configured to fail without specific error")
		}
	} else if res.Error == nil { // Only set to Succeeded if no error was passed via m.RunError
		res.Status = "Succeeded"
	}
	return res
}
var _ step.Step = &MockStep{}


// newTestRuntimeForTask creates a minimal runtime.ClusterRuntime with a specified number of mock hosts.
func newTestRuntimeForTask(t *testing.T, numHosts int) (*runtime.ClusterRuntime, []*runtime.Host) {
	t.Helper()

	// Ensure global logger is initialized for tests, or use a specific test logger.
	// For tests, using a controlled instance is better.
	testLogOpts := logger.DefaultOptions()
	testLogOpts.ConsoleOutput = false // Disable console output during tests unless specifically debugging a test.
	testLogOpts.FileOutput = false
	logInst, _ := logger.NewLogger(testLogOpts)

	hosts := make([]*runtime.Host, numHosts)
	inventory := make(map[string]*runtime.Host)

	for i := 0; i < numHosts; i++ {
		hostName := fmt.Sprintf("host%d", i+1)
		// For task tests, the Host objects don't need real Connectors/Runners
		// as the Steps themselves are mocked.
		h := &runtime.Host{
			Name:    hostName,
			Address: fmt.Sprintf("192.168.1.%d", i+1),
			// Runner and Connector can be nil here if MockStep doesn't use them.
			// If MockStep needs ctx.Host.Runner, then a minimal mock runner would be needed.
			// The runtime.NewHostContext will create a logger.
		}
		hosts[i] = h
		inventory[hostName] = h
	}

	return &runtime.ClusterRuntime{
		Logger:    logInst,
		Hosts:     hosts,
		Inventory: inventory,
		ClusterConfig: &config.Cluster{ Spec: config.ClusterSpec{} }, // Minimal config
		RoleInventory: make(map[string][]*runtime.Host), // Initialize to avoid nil issues
	}, hosts
}


func TestTask_Run_Success_AllStepsDone(t *testing.T) {
	clusterRt, targetHosts := newTestRuntimeForTask(t, 1)
	taskToRun := &Task{
		Name: "TestTask_AllDone",
		Steps: []step.Step{
			&MockStep{StepName: "Step1_Done", CheckIsDone: true},
			&MockStep{StepName: "Step2_Done", CheckIsDone: true},
		},
		Concurrency: 1,
	}

	results, err := taskToRun.Run(context.Background(), targetHosts, clusterRt)
	if err != nil {
		t.Fatalf("Task.Run() error = %v, wantErr nil", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 results (for 2 steps), got %d", len(results))
	}
	for i, res := range results {
		if res.Status != "Skipped" {
			t.Errorf("Result %d status = %s, want Skipped", i, res.Status)
		}
	}
}

func TestTask_Run_Success_StepsRun(t *testing.T) {
	clusterRt, targetHosts := newTestRuntimeForTask(t, 1)
	taskToRun := &Task{
		Name: "TestTask_StepsRun",
		Steps: []step.Step{
			&MockStep{StepName: "Step1_Run", CheckIsDone: false, RunShouldFail: false},
			&MockStep{StepName: "Step2_Run", CheckIsDone: false, RunShouldFail: false},
		},
	}

	results, err := taskToRun.Run(context.Background(), targetHosts, clusterRt)
	if err != nil {
		t.Fatalf("Task.Run() error = %v, wantErr nil", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	for i, res := range results {
		if res.Status != "Succeeded" {
			t.Errorf("Result %d for step %s status = %s, want Succeeded", i, res.StepName, res.Status)
		}
	}
}

func TestTask_Run_StepFails_StopsHostExecution(t *testing.T) {
	clusterRt, targetHosts := newTestRuntimeForTask(t, 1)

	failingStepError := errors.New("step1 deliberate failure")
	failingStep := &MockStep{StepName: "Step1_Fails", CheckIsDone: false, RunShouldFail: true, RunError: failingStepError}
	successStepAfterFail := &MockStep{StepName: "Step2_ShouldNotRun", CheckIsDone: false}

	taskToRun := &Task{
		Name:  "TestTask_StepFails",
		Steps: []step.Step{failingStep, successStepAfterFail},
	}

	results, err := taskToRun.Run(context.Background(), targetHosts, clusterRt)
	if err == nil {
		t.Fatal("Task.Run() with a failing step expected an error, got nil")
	}
	// The error returned by Task.Run wraps the step's error
	if !strings.Contains(err.Error(), "Step1_Fails") || !strings.Contains(err.Error(), "failed on host") || !errors.Is(err, failingStepError) {
		t.Errorf("Error message = %q, expected to contain failure details and wrap original error", err.Error())
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result (the failing step), got %d. Results: %+v", len(results), results)
	}
	if results[0].StepName != "Step1_Fails" || results[0].Status != "Failed" {
		t.Errorf("Result for failing step is incorrect: Name=%s, Status=%s", results[0].StepName, results[0].Status)
	}
}

func TestTask_Run_CheckFails_StopsHostExecution(t *testing.T) {
	clusterRt, targetHosts := newTestRuntimeForTask(t, 1)
	expectedCheckErr := errors.New("check phase deliberate failure")

	checkFailingStep := &MockStep{StepName: "Step1_CheckFails", CheckError: expectedCheckErr}
	stepAfterCheckFail := &MockStep{StepName: "Step2_ShouldNotRunAfterCheckFail"}

	taskToRun := &Task{
		Name: "TestTask_CheckFails",
		Steps: []step.Step{checkFailingStep, stepAfterCheckFail},
	}

	results, err := taskToRun.Run(context.Background(), targetHosts, clusterRt)
	if err == nil {
		t.Fatal("Task.Run() with a failing step check expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "Step1_CheckFails") || !strings.Contains(err.Error(), "pre-check failed") || !errors.Is(err, expectedCheckErr) {
		t.Errorf("Error message = %q, expected to contain check failure details and wrap original error", err.Error())
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result (for the failing check), got %d. Results: %+v", len(results), results)
	}
	if results[0].StepName != "Step1_CheckFails [CheckPhase]" || results[0].Status != "Failed" {
		t.Errorf("Result for failing check is incorrect: Name=%s, Status=%s", results[0].StepName, results[0].Status)
	}
}


func TestTask_Run_Concurrency(t *testing.T) {
	numHosts := 5
	clusterRt, targetHosts := newTestRuntimeForTask(t, numHosts)

	var stepsRunHostCount int32

	taskToRun := &Task{
		Name: "TestTask_Concurrency",
		Steps: []step.Step{
			&MockStep{
				StepName: "Step_Concurrent",
				CheckIsDone: false,
				RunDelay: 100 * time.Millisecond,
				RunFunc: func(ctx *runtime.Context) *step.Result {
					atomic.AddInt32(&stepsRunHostCount, 1)
					// Must respect context cancellation during delay for accurate test
					select {
					case <-time.After(100 * time.Millisecond): // Simulate work
					case <-ctx.GoContext.Done():
						return step.NewResult("Step_Concurrent", ctx.Host.Name, time.Now(), ctx.GoContext.Err())
					}
					return step.NewResult("Step_Concurrent", ctx.Host.Name, time.Now(), nil)
				},
			},
		},
		Concurrency: 2,
	}

	startTime := time.Now()
	results, err := taskToRun.Run(context.Background(), targetHosts, clusterRt)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Task.Run() with concurrency error = %v", err)
	}
	if len(results) != numHosts {
		t.Fatalf("Expected %d results, got %d", numHosts, len(results))
	}
	if atomic.LoadInt32(&stepsRunHostCount) != int32(numHosts) {
		t.Errorf("Step was run on %d hosts, want %d", stepsRunHostCount, numHosts)
	}

	expectedBatches := (numHosts + taskToRun.Concurrency - 1) / taskToRun.Concurrency
	minExpectedDuration := time.Duration(expectedBatches) * (100 * time.Millisecond)
	maxExpectedDuration := minExpectedDuration + (150 * time.Millisecond) // Allow generous overhead for scheduling

	if duration < minExpectedDuration || duration > maxExpectedDuration {
		t.Errorf("Task execution time = %v, expected to be roughly between %v and %v for concurrency %d on %d hosts over %d batches",
			duration, minExpectedDuration, maxExpectedDuration, taskToRun.Concurrency, numHosts, expectedBatches)
	}
	t.Logf("Concurrency test duration: %v (min_expected: %v, batches: %d)", duration, minExpectedDuration, expectedBatches)
}

func TestTask_Run_NoHosts(t *testing.T) {
	clusterRt, _ := newTestRuntimeForTask(t, 0)
	taskToRun := &Task{Name: "TestTask_NoHosts", Steps: []step.Step{&MockStep{StepName: "Step1"}}}

	results, err := taskToRun.Run(context.Background(), []*runtime.Host{}, clusterRt)
	if err != nil {
		t.Fatalf("Task.Run() with no hosts error = %v", err)
	}
	if results != nil {
		t.Errorf("Expected nil results for no hosts, got %d results", len(results))
	}
}


func TestNewSystemChecksTask_Assembly(t *testing.T) {
	dummyCfg := &config.Cluster{}
	task := task_preflight.NewSystemChecksTask(dummyCfg)

	if task.Name != "Run System Preflight Checks" {
		t.Errorf("NewSystemChecksTask name = %s, want 'Run System Preflight Checks'", task.Name)
	}
	if len(task.Steps) < 3 {
		t.Errorf("NewSystemChecksTask expected at least 3 steps, got %d", len(task.Steps))
	}
	var foundCPU, foundMem, foundSwap bool
	for _, s := range task.Steps {
		if strings.Contains(s.Name(), "Check CPU Cores") { foundCPU = true }
		if strings.Contains(s.Name(), "Check Memory") { foundMem = true }
		if s.Name() == "Disable Swap" { foundSwap = true }
	}
	if !foundCPU || !foundMem || !foundSwap {
		t.Errorf("NewSystemChecksTask missing one of expected steps (CPU:%v, Mem:%v, Swap:%v)", foundCPU, foundMem, foundSwap)
	}
}

func TestNewInstallContainerdTask_Assembly(t *testing.T) {
	// Example config structure for Containerd - this would live in config package
	type ContainerdSpecForTest struct {
		Version string
	}
	type ClusterSpecForTest struct {
		Containerd *ContainerdSpecForTest
	}
	dummyCfg := &config.Cluster{
		Spec: config.ClusterSpec{
			// Containerd: &ContainerdSpecForTest{Version: "1.2.3"}, // This needs actual field in config.ClusterSpec
			// For now, we test the factory with nil or default config values
		},
	}
	// Test with nil config, should use defaults
	taskNilCfg := task_containerd.NewInstallContainerdTask(nil)
	if len(taskNilCfg.Steps) < 3 {
		t.Errorf("NewInstallContainerdTask (nil cfg) expected at least 3 steps, got %d", len(taskNilCfg.Steps))
	}

	// Test with config (even if fields are not deeply used yet by factory)
	taskWithCfg := task_containerd.NewInstallContainerdTask(dummyCfg)


	if taskWithCfg.Name != "Install and Configure Containerd" {
		t.Errorf("NewInstallContainerdTask name = %s", taskWithCfg.Name)
	}
	if len(taskWithCfg.Steps) < 3 { // Install, Configure, EnableAndStart
		t.Errorf("NewInstallContainerdTask expected at least 3 steps, got %d", len(taskWithCfg.Steps))
	}

	var installStepFound bool
	var configuredVersion string
	for _, s_raw := range taskWithCfg.Steps {
		if s_typed, ok := s_raw.(*step_containerd.InstallContainerdStep); ok {
			installStepFound = true
			configuredVersion = s_typed.Version // Check what version it defaulted to or got from (mocked) config
			break
		}
	}
	if !installStepFound {
		t.Error("InstallContainerdStep not found in NewInstallContainerdTask")
	}
	// If config parsing was actually implemented in factory:
	// if configuredVersion != "1.2.3" {
	// 	t.Errorf("InstallContainerdStep version = %s, want 1.2.3 (from dummyCfg)", configuredVersion)
	// }
	// For now, it will be empty or the factory's internal default.
	if configuredVersion != "" { // Assuming factory default is "" for latest
		t.Logf("InstallContainerdStep version defaulted to: %s", configuredVersion)
	}
}
