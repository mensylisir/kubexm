package executor

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/connector"
)

// --- Mock Step Components for Executor Tests ---

type MockExecutorStepSpec struct {
	MockName               string
	CheckShouldBeDone      bool
	CheckError             error
	ExecuteError           error
	ExecuteShouldFail      bool
	ExecuteMessage         string
	ExecuteDelay           time.Duration
	ExecuteStdout          string
	ExecuteStderr          string
}
func (m *MockExecutorStepSpec) GetName() string {
	if m.MockName == "" { return "UnnamedMockExecutorStepSpec" }
	return m.MockName
}
var _ spec.StepSpec = &MockExecutorStepSpec{}


type MockStepExecutorImpl struct {
	CheckCalled   atomic.Int32
	ExecuteCalled atomic.Int32
	mu            sync.Mutex
}
func (m *MockStepExecutorImpl) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	m.CheckCalled.Add(1)
	mockSpec, ok := s.(*MockExecutorStepSpec)
	if !ok { return false, fmt.Errorf("unexpected spec type %T in MockStepExecutorImpl.Check", s)}
	return mockSpec.CheckShouldBeDone, mockSpec.CheckError
}
func (m *MockStepExecutorImpl) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	m.ExecuteCalled.Add(1)
	mockSpec, ok := s.(*MockExecutorStepSpec)
	if !ok {
		err := fmt.Errorf("unexpected spec type %T in MockStepExecutorImpl.Execute", s)
		specName := "UnknownStep (type error)"; if s != nil { specName = s.GetName() }
		return step.NewResult(specName, ctx.Host.Name, time.Now(), err)
	}

	startTime := time.Now() // Capture start time for result accurately
	if mockSpec.ExecuteDelay > 0 {
		select {
		case <-time.After(mockSpec.ExecuteDelay):
		case <-ctx.GoContext.Done():
			return step.NewResult(mockSpec.GetName(), ctx.Host.Name, startTime, ctx.GoContext.Err())
		}
	}

	res := step.NewResult(mockSpec.GetName(), ctx.Host.Name, startTime, mockSpec.ExecuteError)
	if mockSpec.ExecuteShouldFail {
		res.Status = "Failed"
		if res.Error == nil { // If mockSpec.ExecuteError was nil but it should fail
			res.Error = errors.New("mock step executor configured to fail")
		}
	} else {
		if res.Error == nil { // Only Succeeded if no explicit error and not configured to fail
			res.Status = "Succeeded"
		} else {
			res.Status = "Failed" // NewResult sets this if error is non-nil
		}
	}
	res.Message = mockSpec.ExecuteMessage; res.Stdout = mockSpec.ExecuteStdout; res.Stderr = mockSpec.ExecuteStderr
	res.EndTime = time.Now();
	return res
}
var _ step.StepExecutor = &MockStepExecutorImpl{}

var (
    mockExecutorRegistryOnce sync.Once
    mockExecutorInstance     *MockStepExecutorImpl
)

func getTestMockStepExecutor() *MockStepExecutorImpl {
    mockExecutorRegistryOnce.Do(func() {
        mockExecutorInstance = &MockStepExecutorImpl{}
        typeName := reflect.TypeOf(&MockExecutorStepSpec{}).String()
        step.Register(typeName, mockExecutorInstance)
    })
    mockExecutorInstance.CheckCalled.Store(0)
    mockExecutorInstance.ExecuteCalled.Store(0)
    return mockExecutorInstance
}


// --- Test Runtime Setup Helper ---
func newTestRuntimeForExecutor(t *testing.T, numHosts int, hostRoles map[string][]string) (*runtime.ClusterRuntime, []*runtime.Host) {
	t.Helper();
	logOpts := logger.DefaultOptions(); logOpts.ConsoleLevel = logger.DebugLevel; logOpts.ConsoleOutput = false; logOpts.FileOutput = false;
	logger.Init(logOpts); log := logger.Get()

	hosts := make([]*runtime.Host, numHosts); inventory := make(map[string]*runtime.Host); roleInventory := make(map[string][]*runtime.Host)
	for i := 0; i < numHosts; i++ {
		hostName := fmt.Sprintf("host%d", i+1)
		dummyRunner := &runner.Runner{ Facts: &runner.Facts{OS: &connector.OS{ID: "linux-test", Arch: "amd64"}, Hostname: hostName} }
		h := &runtime.Host{ Name: hostName, Address: fmt.Sprintf("192.168.1.%d", i+1), Roles: make(map[string]bool), Runner:  dummyRunner }
		if hostRoles != nil { for role, names := range hostRoles { for _, name := range names { if name == hostName { h.Roles[role] = true } } } }
		hosts[i] = h; inventory[hostName] = h
		if hostRoles != nil {
			for roleName := range h.Roles { // Iterate over roles assigned to THIS host
				roleInventory[roleName] = append(roleInventory[roleName], h)
			}
		}
	}
	clusterConf := &config.Cluster{Metadata: config.Metadata{Name:"test-pipeline"}, Spec: config.ClusterSpec{Global: config.GlobalSpec{}}}
	return &runtime.ClusterRuntime{
		Logger: log, Hosts: hosts, Inventory: inventory, RoleInventory: roleInventory,
		ClusterConfig: clusterConf,
	}, hosts
}
func selectedHostNamesForTest(hosts []*runtime.Host) []string { names := make([]string, len(hosts)); for i, h := range hosts { names[i] = h.Name }; return names }

// --- Tests (NewExecutor, SelectHostsForTaskSpec, executeTaskSpec, executeHookSteps - previous tests are concise below) ---
func TestNewExecutor(t *testing.T) {
	exec, err := NewExecutor(ExecutorOptions{}); if err != nil {t.Fatalf("NewExecutor empty opts: %v", err)}; if exec.Logger == nil {t.Error("Logger nil")}; if exec.DefaultTaskConcurrency != DefaultTaskConcurrency {t.Errorf("Conc = %d, want %d", exec.DefaultTaskConcurrency, DefaultTaskConcurrency)}
	customLogger, _ := logger.NewLogger(logger.Options{ConsoleOutput:false}); execCustom, err := NewExecutor(ExecutorOptions{Logger: customLogger, DefaultTaskConcurrency: 5}); if err != nil {t.Fatalf("NewExecutor custom opts: %v",err)}; if execCustom.Logger != customLogger {t.Error("Custom logger not set")}; if execCustom.DefaultTaskConcurrency != 5 {t.Error("Custom concurrency not set")}
}
func TestExecutor_SelectHostsForTaskSpec(t *testing.T) {
	clusterRt, _ := newTestRuntimeForExecutor(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	taskSpec := &spec.TaskSpec{Name:"Test", RunOnRoles:[]string{}, Filter: func(h *runtime.Host)bool{return true}}
	if len(exec.selectHostsForTaskSpec(taskSpec, clusterRt)) != 1 {t.Error("Select all failed")}
}
func TestExecutor_ExecuteTaskSpec_SimpleSuccess(t *testing.T) {
	_ = getTestMockStepExecutor()
	clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	mockStep1Spec := &MockExecutorStepSpec{MockName: "MockStep1"}
	taskSpec := &spec.TaskSpec{ Name:  "TestSimpleTask", Steps: []spec.StepSpec{mockStep1Spec}}
	results, err := exec.executeTaskSpec(context.Background(), taskSpec, targetHosts, clusterRt)
	if err != nil {t.Fatalf("executeTaskSpec failed: %v", err)}
	if len(results) != 1 {t.Errorf("Expected 1 result, got %d", len(results))}
	if results[0].Status != "Succeeded" {t.Errorf("Step status %s, Message: %s", results[0].Status, results[0].Message)}
}
func TestExecutor_ExecuteTaskSpec_StepSkipped(t *testing.T) {
	mockExec := getTestMockStepExecutor(); clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	skippedStepSpec := &MockExecutorStepSpec{MockName: "SkippedStep", CheckShouldBeDone: true}; taskSpec := &spec.TaskSpec{Name: "TestSkip", Steps: []spec.StepSpec{skippedStepSpec}}
	results, err := exec.executeTaskSpec(context.Background(), taskSpec, targetHosts, clusterRt); if err != nil {t.Fatalf("executeTaskSpec failed for skipped step: %v", err)}; if len(results) != 1 {t.Fatalf("Expected 1 result, got %d", len(results))}; if results[0].Status != "Skipped" {t.Errorf("Status = %s, want Skipped", results[0].Status)}; if mockExec.ExecuteCalled.Load() != 0 {t.Error("Execute() called")}; if mockExec.CheckCalled.Load() != 1 {t.Errorf("Check() count = %d, want 1", mockExec.CheckCalled.Load())}
}
func TestExecutor_ExecuteTaskSpec_StepExecuteFails(t *testing.T) {
	mockExec := getTestMockStepExecutor(); clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	failingStepSpec := &MockExecutorStepSpec{MockName: "FailingStep", ExecuteShouldFail: true, ExecuteError: errors.New("deliberate exec failure")}; taskSpec := &spec.TaskSpec{Name: "TestExecuteFail", Steps: []spec.StepSpec{failingStepSpec}}
	results, err := exec.executeTaskSpec(context.Background(), taskSpec, targetHosts, clusterRt); if err == nil {t.Fatal("expected error")}; if !strings.Contains(err.Error(), "deliberate exec failure") {t.Errorf("err msg mismatch: %v", err)}; if len(results)!=1{t.Fatalf("results = %d",len(results))}; if results[0].Status != "Failed" {t.Errorf("status = %s", results[0].Status)}; if !errors.Is(results[0].Error, failingStepSpec.ExecuteError){t.Errorf("error mismatch got %v want %v", results[0].Error, failingStepSpec.ExecuteError)}; if mockExec.ExecuteCalled.Load() != 1 {t.Error("Execute not called")}
}
func TestExecutor_ExecuteTaskSpec_StepCheckFails(t *testing.T) {
	mockExec := getTestMockStepExecutor(); clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	checkFailError := errors.New("deliberate check failure"); checkFailSpec := &MockExecutorStepSpec{MockName: "CheckFailStep", CheckError: checkFailError}; taskSpec := &spec.TaskSpec{Name: "TestCheckFail", Steps: []spec.StepSpec{checkFailSpec}}
	results, err := exec.executeTaskSpec(context.Background(), taskSpec, targetHosts, clusterRt); if err == nil {t.Fatal("expected error")}; if !strings.Contains(err.Error(), "deliberate check failure") {t.Errorf("err msg mismatch: %v", err)}; if len(results)!=1{t.Fatalf("results = %d", len(results))}; if results[0].Status != "Failed" {t.Errorf("status = %s", results[0].Status)}; if !strings.Contains(results[0].StepName, "[CheckPhase]") {t.Errorf("StepName missing [CheckPhase]: %s", results[0].StepName)}; if mockExec.CheckCalled.Load()!=1{t.Error("Check not called")}; if mockExec.ExecuteCalled.Load()!=0{t.Error("Execute called after check fail")}
}
func TestExecutor_ExecuteTaskSpec_Concurrency(t *testing.T) {
	mockExec := getTestMockStepExecutor(); numHosts := 3; stepDelay := 50*time.Millisecond; clusterRt, targetHosts := newTestRuntimeForExecutor(t, numHosts, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger, DefaultTaskConcurrency: 2})
	stepSpec := &MockExecutorStepSpec{MockName: "ConcurrentStep", ExecuteDelay: stepDelay}; taskSpec := &spec.TaskSpec{Name: "TestConcurrencyTask", Steps: []spec.StepSpec{stepSpec}, Concurrency: 2}
	startTime := time.Now(); results, err := exec.executeTaskSpec(context.Background(), taskSpec, targetHosts, clusterRt); duration := time.Since(startTime)
	if err != nil {t.Fatalf("concurrency failed: %v", err)}; if len(results) != numHosts {t.Errorf("results = %d, want %d", len(results), numHosts)}; if mockExec.ExecuteCalled.Load() != int32(numHosts) {t.Errorf("ExecuteCalled = %d, want %d", mockExec.ExecuteCalled.Load(), numHosts)}
	expectedBatches := (numHosts + taskSpec.Concurrency - 1) / taskSpec.Concurrency; minExpectedDuration := time.Duration(expectedBatches) * stepDelay; maxExpectedDuration := minExpectedDuration + stepDelay + (50 * time.Millisecond)
	if duration < minExpectedDuration || duration > maxExpectedDuration {t.Errorf("Exec time %v, expected between %v and %v", duration, minExpectedDuration, maxExpectedDuration)}
}
func TestExecutor_ExecuteHookSteps_SimpleSuccess(t *testing.T) {
	mockExec := getTestMockStepExecutor(); clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	hookSpec := &MockExecutorStepSpec{MockName: "MyHookStep"}
	results, err := exec.executeHookSteps(context.Background(), hookSpec, "TestHook", targetHosts, clusterRt); if err != nil {t.Fatalf("failed: %v", err)}; if len(results) != 1 {t.Errorf("results = %d", len(results))}; if results[0].Status != "Succeeded" {t.Errorf("status = %s", results[0].Status)}; if results[0].StepName != "MyHookStep" {t.Errorf("name = %s", results[0].StepName)}; if mockExec.ExecuteCalled.Load() != 1 {t.Error("Execute not called")}
}
func TestExecutor_ExecuteHookSteps_ExecuteFailsOnMultipleHosts(t *testing.T) {
	mockExec := getTestMockStepExecutor(); clusterRt, targetHosts := newTestRuntimeForExecutor(t, 2, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	hookExecuteError := errors.New("hook exec deliberate failure"); hookSpec := &MockExecutorStepSpec{MockName: "FailingHook", ExecuteShouldFail: true, ExecuteError: hookExecuteError}
	results, err := exec.executeHookSteps(context.Background(), hookSpec, "TestFailingHook", targetHosts, clusterRt); if err == nil {t.Fatal("expected error")}; if !strings.Contains(err.Error(), "hook exec deliberate failure") {t.Errorf("err msg = %q", err.Error())}; if len(results) != len(targetHosts) {t.Errorf("results = %d", len(results))}
	for i, r := range results { if r.Status != "Failed" {t.Errorf("Host %d status = %s",i,r.Status)}; if !errors.Is(r.Error, hookExecuteError){t.Errorf("Host %d err mismatch",i)} }
	if mockExec.ExecuteCalled.Load() != int32(len(targetHosts)) {t.Errorf("ExecuteCalled = %d", mockExec.ExecuteCalled.Load())}
}
func TestExecutor_ExecuteHookSteps_NilOrNoHosts(t *testing.T) {
	clusterRt, _ := newTestRuntimeForExecutor(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	results, err := exec.executeHookSteps(context.Background(), nil, "NilHook", clusterRt.Hosts, clusterRt); if err != nil || results != nil {t.Errorf("nil spec: res=%v, err=%v", results, err)}
	hookSpec := &MockExecutorStepSpec{MockName: "ValidHook"}; results, err = exec.executeHookSteps(context.Background(), hookSpec, "NoHostsHook", []*runtime.Host{}, clusterRt); if err != nil || results != nil {t.Errorf("no hosts: res=%v, err=%v", results, err)}
}

// --- Tests for selectHostsForModule ---
func TestExecutor_SelectHostsForModule(t *testing.T) {
	hostRoles := map[string][]string{ "master": {"host1", "host2"}, "worker": {"host2", "host3"}, "etcd":   {"host1"}, }
	clusterRt, _ := newTestRuntimeForExecutor(t, 3, hostRoles)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	taskSpecMaster := &spec.TaskSpec{Name: "MasterTask", RunOnRoles: []string{"master"}}
	taskSpecWorker := &spec.TaskSpec{Name: "WorkerTask", RunOnRoles: []string{"worker"}}
	taskSpecAll := &spec.TaskSpec{Name: "AllTask"}
	taskSpecHost3OnlyFilter := &spec.TaskSpec{Name: "Host3Filter", Filter: func(h *runtime.Host) bool { return h.Name == "host3"}}

	moduleSpec1 := &spec.ModuleSpec{ Name:  "Module1_MasterWorker", Tasks: []*spec.TaskSpec{taskSpecMaster, taskSpecWorker}}
	moduleSpec2 := &spec.ModuleSpec{ Name:  "Module2_AllAndFilter", Tasks: []*spec.TaskSpec{taskSpecAll, taskSpecHost3OnlyFilter}}
	moduleSpec3_NoMatchingTasks := &spec.ModuleSpec{ Name: "Module3_NoMatch", Tasks: []*spec.TaskSpec{{Name: "DBTask", RunOnRoles: []string{"db"}}}}
	moduleSpec_NoTasks := &spec.ModuleSpec{Name: "Module_NoTasks", Tasks: []*spec.TaskSpec{}}


	tests := []struct { name string; moduleSpec *spec.ModuleSpec; expectedCount int; expectedNames []string }{
		{"Module with master and worker tasks", moduleSpec1, 3, []string{"host1", "host2", "host3"}},
		{"Module with all and specific filter", moduleSpec2, 3, []string{"host1", "host2", "host3"}},
		{"Module with no matching tasks", moduleSpec3_NoMatchingTasks, 0, []string{}},
		{"Module with no tasks", moduleSpec_NoTasks, 0, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := exec.selectHostsForModule(tt.moduleSpec, clusterRt)
			if len(selected) != tt.expectedCount {
				t.Errorf("selectHostsForModule for '%s' expected %d hosts, got %d (%v)",
					tt.moduleSpec.Name, tt.expectedCount, len(selected), selectedHostNamesForTest(selected))
			}
			if tt.expectedNames != nil {
				names := selectedHostNamesForTest(selected); sort.Strings(names); sort.Strings(tt.expectedNames)
				if !reflect.DeepEqual(names, tt.expectedNames) {
					t.Errorf("selectHostsForModule for '%s' expected names %v, got %v", tt.moduleSpec.Name, tt.expectedNames, names)
				}
			}
		})
	}
}


// --- Tests for executeModule ---
func TestExecutor_ExecuteModule_SimpleSuccess(t *testing.T) {
	mockExec := getTestMockStepExecutor()
	clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	// TaskSpec that targets the single host (host1)
	taskSpec1 := &spec.TaskSpec{Name: "Task1", Steps: []spec.StepSpec{&MockExecutorStepSpec{MockName: "T1Step1"}}}
	moduleSpec := &spec.ModuleSpec{Name: "SimpleModule", Tasks: []*spec.TaskSpec{taskSpec1}}

	// Mock selectHostsForModule to return our target host for hooks
	// And selectHostsForTaskSpec to return targetHost for the task
	originalSelectMod := exec.selectHostsForModule
	originalSelectTask := exec.selectHostsForTaskSpec
	defer func() {
		exec.selectHostsForModule = originalSelectMod
		exec.selectHostsForTaskSpec = originalSelectTask
	}()
	exec.selectHostsForModule = func(ms *spec.ModuleSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }
	exec.selectHostsForTaskSpec = func(ts *spec.TaskSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }


	results, err := exec.executeModule(context.Background(), moduleSpec, clusterRt)
	if err != nil {t.Fatalf("executeModule failed: %v", err)}
	if len(results) != 1 {t.Errorf("Expected 1 step result, got %d", len(results))}
	if results[0].Status != "Succeeded" {t.Errorf("Step status = %s", results[0].Status)}
	if mockExec.ExecuteCalled.Load() != 1 {t.Errorf("ExecuteCalled = %d, want 1", mockExec.ExecuteCalled.Load())}
}

func TestExecutor_ExecuteModule_TaskFails_ModuleFails(t *testing.T) {
	mockExec := getTestMockStepExecutor()
	clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	failingStep := &MockExecutorStepSpec{MockName: "FailingStepInTask", ExecuteShouldFail: true, ExecuteError: errors.New("task step failed")}
	taskSpec1 := &spec.TaskSpec{Name: "TaskWithFailure", Steps: []spec.StepSpec{failingStep}, IgnoreError: false}
	taskSpec2 := &spec.TaskSpec{Name: "TaskShouldNotRun", Steps: []spec.StepSpec{&MockExecutorStepSpec{MockName: "T2Step1"}}}
	moduleSpec := &spec.ModuleSpec{Name: "ModuleTaskFail", Tasks: []*spec.TaskSpec{taskSpec1, taskSpec2}}

	exec.selectHostsForModule = func(ms *spec.ModuleSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }
	exec.selectHostsForTaskSpec = func(ts *spec.TaskSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }

	results, err := exec.executeModule(context.Background(), moduleSpec, clusterRt)
	if err == nil {t.Fatal("executeModule expected error, got nil")}
	if !strings.Contains(err.Error(), "TaskWithFailure") || !strings.Contains(err.Error(), "task step failed") {t.Errorf("Error message mismatch: %v", err)}
	if len(results) != 1 {t.Errorf("Expected 1 result (failing step), got %d", len(results))}
	if results[0].Status != "Failed" {t.Errorf("Result status = %s", results[0].Status)}
	if mockExec.ExecuteCalled.Load() != 1 {t.Errorf("ExecuteCalled = %d, want 1", mockExec.ExecuteCalled.Load())}
}

func TestExecutor_ExecuteModule_PreRunHookFails(t *testing.T) {
	mockExec := getTestMockStepExecutor()
	clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	preRunHookSpec := &MockExecutorStepSpec{MockName: "ModulePreRunFail", ExecuteShouldFail: true, ExecuteError: errors.New("pre-run hook failed")}
	taskSpec1 := &spec.TaskSpec{Name: "TaskShouldNotRunDueToPreRun", Steps: []spec.StepSpec{&MockExecutorStepSpec{MockName: "T1Step1"}}}
	moduleSpec := &spec.ModuleSpec{Name: "ModulePreRunFail", PreRun: preRunHookSpec, Tasks: []*spec.TaskSpec{taskSpec1}}

	exec.selectHostsForModule = func(ms *spec.ModuleSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts } // Hook runs on these hosts

	results, err := exec.executeModule(context.Background(), moduleSpec, clusterRt)
	if err == nil {t.Fatal("executeModule with failing PreRun expected error, got nil")}
	if !strings.Contains(err.Error(), "ModulePreRunFail") || !strings.Contains(err.Error(), "pre-run hook failed") {t.Errorf("Error message mismatch: %v", err)}
	if len(results) != 1 {t.Errorf("Expected 1 result (failing hook step), got %d", len(results))}
	if results[0].Status != "Failed" {t.Errorf("Hook result status = %s", results[0].Status)}
	if mockExec.ExecuteCalled.Load() != 1 { t.Errorf("PreRun ExecuteCalled = %d, want 1", mockExec.ExecuteCalled.Load()) }
}


// --- Tests for ExecutePipeline ---
func TestExecutor_ExecutePipeline_SimpleSuccess(t *testing.T) {
	mockExec := getTestMockStepExecutor()
	clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	taskSpec := &spec.TaskSpec{Name: "PipeTask1", Steps: []spec.StepSpec{&MockExecutorStepSpec{MockName: "P_T1_S1"}}}
	moduleSpec := &spec.ModuleSpec{Name: "PipeModule1", Tasks: []*spec.TaskSpec{taskSpec}}
	pipelineSpec := &spec.PipelineSpec{Name: "TestPipe", Modules: []*spec.ModuleSpec{moduleSpec}}

	exec.selectHostsForModule = func(ms *spec.ModuleSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }
	exec.selectHostsForTaskSpec = func(ts *spec.TaskSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }

	results, err := exec.ExecutePipeline(context.Background(), pipelineSpec, clusterRt)
	if err != nil {t.Fatalf("ExecutePipeline failed: %v", err)}
	if len(results) != 1 {t.Errorf("Expected 1 step result, got %d", len(results))}
	if results[0].Status != "Succeeded" {t.Errorf("Step status = %s", results[0].Status)}
	if mockExec.ExecuteCalled.Load() != 1 {t.Errorf("ExecuteCalled = %d, want 1", mockExec.ExecuteCalled.Load())}
}

func TestExecutor_ExecutePipeline_ModuleFails_PipelineFails(t *testing.T) {
	mockExec := getTestMockStepExecutor()
	clusterRt, targetHosts := newTestRuntimeForExecutor(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	failingStep := &MockExecutorStepSpec{MockName: "PipeFailingStep", ExecuteShouldFail: true, ExecuteError: errors.New("pipe mod task step failed")}
	taskSpec1 := &spec.TaskSpec{Name: "PipeTaskWithFailure", Steps: []spec.StepSpec{failingStep}, IgnoreError: false}
	moduleSpec1 := &spec.ModuleSpec{Name: "PipeModuleFail", Tasks: []*spec.TaskSpec{taskSpec1}}
	moduleSpec2 := &spec.ModuleSpec{Name: "PipeModuleShouldNotRun", Tasks: []*spec.TaskSpec{&spec.TaskSpec{Name:"T2"}}} // This module won't run
	pipelineSpec := &spec.PipelineSpec{Name: "TestPipeFail", Modules: []*spec.ModuleSpec{moduleSpec1, moduleSpec2}}

	exec.selectHostsForModule = func(ms *spec.ModuleSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }
	exec.selectHostsForTaskSpec = func(ts *spec.TaskSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }

	results, err := exec.ExecutePipeline(context.Background(), pipelineSpec, clusterRt)
	if err == nil {t.Fatal("ExecutePipeline expected error, got nil")}
	if !strings.Contains(err.Error(), "PipeModuleFail") || !strings.Contains(err.Error(), "PipeTaskWithFailure") || !strings.Contains(err.Error(), "pipe mod task step failed"){
		t.Errorf("Error message mismatch: %v", err)
	}
	if len(results) != 1 {t.Errorf("Expected 1 result (failing step), got %d", len(results))}
	if results[0].Status != "Failed" {t.Errorf("Result status = %s", results[0].Status)}
	if mockExec.ExecuteCalled.Load() != 1 {
		t.Errorf("ExecuteCalled = %d, want 1", mockExec.ExecuteCalled.Load())
	}
}

// TODO: Add more tests for ExecutePipeline:
// - Pipeline PreRun/PostRun hooks (success and failure)
// - Module IsEnabled false
// - Module PreRun/PostRun hooks (success and failure)
// - Task IgnoreError true within a module, pipeline continues
// - Context cancellation propagation
```

Key changes in this iteration:
-   **Mock Executor Registration**: `getTestMockStepExecutor` now uses `atomic.Int32` for `CheckCalled` and `ExecuteCalled` and resets them using `Store(0)` each time it's called. This ensures that tests for different methods (like `executeTaskSpec` vs `executeHookSteps` vs `executeModule`) get a clean state for call count verification.
-   **Refined `MockStepExecutorImpl.Execute`**: More accurately sets `res.Status` based on `mockSpec.ExecuteError` and `mockSpec.ExecuteShouldFail`.
-   **`newTestRuntimeForExecutor`**:
    -   Ensures `ClusterConfig.Spec.Global` is initialized to prevent nil panics if any code (like `IsEnabled` funcs in specs) tries to access global config settings that might not be set in a minimal dummy config.
    -   Corrected `RoleInventory` population.
-   **`TestExecutor_SelectHostsForModule`**: Added a test case for a module with no tasks, expecting zero hosts for hooks.
-   **Tests for `executeModule`**:
    -   `TestExecutor_ExecuteModule_SimpleSuccess`: Basic success path.
    -   `TestExecutor_ExecuteModule_TaskFails_ModuleFails`: Verifies module halts if a critical task fails.
    -   `TestExecutor_ExecuteModule_PreRunHookFails`: Verifies module halts if PreRun hook fails. This test assumes `selectHostsForModule` (which is still a placeholder) would correctly identify hosts for the module. To make this test fully independent now, the test temporarily mocks `exec.selectHostsForModule` and `exec.selectHostsForTaskSpec` for the scope of the `executeModule` tests. This is a common pattern for testing methods that call other (untested or complex) methods on the same struct.
-   **Tests for `ExecutePipeline`**:
    -   `TestExecutor_ExecutePipeline_SimpleSuccess`: Basic success path.
    -   `TestExecutor_ExecutePipeline_ModuleFails_PipelineFails`: Verifies pipeline halts if a critical module fails.
    -   Similar to `executeModule` tests, these also temporarily mock out `selectHostsForModule` and `selectHostsForTaskSpec` to focus on `ExecutePipeline`'s orchestration logic.
-   Removed some redundant `atomic.LoadInt32` calls in assertions where direct access after all operations is fine for single-goroutine tests. Used `Load()` for clarity where it matters for reading the final atomic value.
-   Concise test functions for previously tested parts (`NewExecutor`, etc.) using single lines for assertions where appropriate.
-   The `reflect.DeepEqual` is used for comparing sorted host name slices for more robust list comparison.

This provides a more comprehensive set of tests for the executor's core orchestration logic. The remaining TODOs for more nuanced hook behaviors, `IsEnabled`, `IgnoreError` at task level within module, and context cancellation are still important for full coverage.
