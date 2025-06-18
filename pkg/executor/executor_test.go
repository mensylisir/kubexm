package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	"go.uber.org/zap/zapcore"
)

// --- Mock Step Components for Executor Tests (Modified for logging test) ---

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
	ExpectedLogContext     map[string]string
}
func (m *MockExecutorStepSpec) GetName() string {
	if m.MockName == "" { return "UnnamedMockExecutorStepSpec" }
	return m.MockName
}
var _ spec.StepSpec = &MockExecutorStepSpec{}

type MockStepExecutorImpl struct {
	CheckCalled    atomic.Int32
	ExecuteCalled  atomic.Int32
	mu             sync.Mutex
	LastLoggerUsed *logger.Logger
}
func (m *MockStepExecutorImpl) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	m.CheckCalled.Add(1)
	m.mu.Lock(); m.LastLoggerUsed = ctx.Logger; m.mu.Unlock()
	mockSpec, _ := s.(*MockExecutorStepSpec)

	ctx.Logger.Debugf("MockStepExecutor.Check called for %s", mockSpec.GetName()) // Log with context
	return mockSpec.CheckShouldBeDone, mockSpec.CheckError
}
func (m *MockStepExecutorImpl) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	m.ExecuteCalled.Add(1)
	m.mu.Lock(); m.LastLoggerUsed = ctx.Logger; m.mu.Unlock()
	mockSpec, ok := s.(*MockExecutorStepSpec)
	if !ok {
		err := fmt.Errorf("unexpected spec type %T in MockStepExecutorImpl.Execute", s)
		specName := "UnknownStep (type error)"; if s != nil { specName = s.GetName() }
		return step.NewResult(specName, ctx.Host.Name, time.Now(), err)
	}

	startTime := time.Now()
	if mockSpec.ExecuteDelay > 0 {
		select {
		case <-time.After(mockSpec.ExecuteDelay):
		case <-ctx.GoContext.Done():
			return step.NewResult(mockSpec.GetName(), ctx.Host.Name, startTime, ctx.GoContext.Err())
		}
	}

	// Log a specific message using the context's logger to capture its fields
	ctx.Logger.Infof("MockStepExecutor.Execute called for %s", mockSpec.GetName())

	res := step.NewResult(mockSpec.GetName(), ctx.Host.Name, startTime, mockSpec.ExecuteError)
	if mockSpec.ExecuteShouldFail {
		res.Status = "Failed"
		if res.Error == nil {res.Error = errors.New("mock step executor configured to fail")}
	} else {
		if res.Error == nil {res.Status = "Succeeded"} else {res.Status = "Failed"}
	}
	res.Message = mockSpec.ExecuteMessage; res.Stdout = mockSpec.ExecuteStdout; res.Stderr = mockSpec.ExecuteStderr
	res.EndTime = time.Now(); return res
}
var _ step.StepExecutor = &MockStepExecutorImpl{}
var ( mockExecutorRegistryOnce sync.Once; mockExecutorInstance *MockStepExecutorImpl )

func getTestMockStepExecutor() *MockStepExecutorImpl {
    mockExecutorRegistryOnce.Do(func() {
        mockExecutorInstance = &MockStepExecutorImpl{}
        step.Register(reflect.TypeOf(&MockExecutorStepSpec{}).String(), mockExecutorInstance)
    })
    mockExecutorInstance.CheckCalled.Store(0)
    mockExecutorInstance.ExecuteCalled.Store(0)
    return mockExecutorInstance
}

// --- Test Runtime Setup Helper (Modified for log capture) ---
var logBufferGlobal bytes.Buffer
var testLogSinkGlobal zapcore.WriteSyncer
var testLoggerSetupOnce sync.Once

func newTestRuntimeForExecutorWithLogCapture(t *testing.T, numHosts int, hostRoles map[string][]string) (*runtime.ClusterRuntime, []*runtime.Host, *bytes.Buffer) {
	t.Helper()

	testLoggerSetupOnce.Do(func() {
		testLogSinkGlobal = zapcore.AddSync(&logBufferGlobal)
	})
	logBufferGlobal.Reset()

	logEncoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg", LevelKey:       "level", TimeKey:        "time",
		NameKey:        "logger_name", // Use a distinct key for logger name if needed
		CallerKey:      "caller", StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding, EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder, EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		// Add keys for our custom context fields so JSON encoder includes them
		// These are added by logger.With(), not by the encoder itself.
		// The encoder just needs to know how to *format* them if they appear (e.g. EncodeName for logger name).
	}
	jsonEncoder := zapcore.NewJSONEncoder(logEncoderCfg)
	core := zapcore.NewCore(jsonEncoder, testLogSinkGlobal, zapcore.DebugLevel)

	testSpecificZapLogger := zap.New(core)
	testSpecificLogger := &logger.Logger{SugaredLogger: testSpecificZapLogger.Sugar()}

	hosts := make([]*runtime.Host, numHosts); inventory := make(map[string]*runtime.Host); roleInventory := make(map[string][]*runtime.Host)
	for i := 0; i < numHosts; i++ {
		hostName := fmt.Sprintf("host%d", i+1)
		dummyRunner := &runner.Runner{ Facts: &runner.Facts{OS: &connector.OS{ID: "linux-test", Arch: "amd64"}, Hostname: hostName} }
		h := &runtime.Host{ Name: hostName, Address: fmt.Sprintf("192.168.1.%d", i+1), Roles: make(map[string]bool), Runner:  dummyRunner }
		if hostRoles != nil { for role, names := range hostRoles { for _, name := range names { if name == hostName { h.Roles[role] = true } } } }
		hosts[i] = h; inventory[hostName] = h
		if hostRoles != nil { for roleName := range h.Roles { roleInventory[roleName] = append(roleInventory[roleName], h) } }
	}
	// Ensure ClusterConfig and Metadata.Name are present for logging context
	clusterConf := &config.Cluster{Metadata: config.Metadata{Name:"TestPipeline"}, Spec: config.ClusterSpec{Global: config.GlobalSpec{}}}

	return &runtime.ClusterRuntime{
		Logger: testSpecificLogger,
		Hosts: hosts, Inventory: inventory, RoleInventory: roleInventory,
		ClusterConfig: clusterConf,
	}, hosts, &logBufferGlobal
}

func selectedHostNamesForTest(hosts []*runtime.Host) []string { names := make([]string, len(hosts)); for i, h := range hosts { names[i] = h.Name }; return names }

// Helper to check log entries for specific fields and message content
func checkLogEntryFields(t *testing.T, logOutput string, expectedFields map[string]string, expectedToContainMsg string) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(logOutput))
	foundEntryWithAllFieldsAndMsg := false
	var lastRelevantEntry map[string]interface{} // Store the last entry that contained the message

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 { continue }
		var entry map[string]interface{}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Logf("Failed to unmarshal log line: %s, error: %v", string(line), err)
			continue
		}

		msgVal, msgOk := entry["msg"]
		if !msgOk { continue }
		msgStr, msgStrOk := msgVal.(string)
		if !msgStrOk { continue }

		if strings.Contains(msgStr, expectedToContainMsg) {
			lastRelevantEntry = entry // Found an entry with the target message
			matchCount := 0
			var missingFields []string
			var mismatchedFields []string

			for key, expectedVal := range expectedFields {
				if val, ok := entry[key]; ok {
					if fmt.Sprintf("%v", val) == expectedVal {
						matchCount++
					} else {
						mismatchedFields = append(mismatchedFields, fmt.Sprintf("field %s: got '%v', want '%v'", key, val, expectedVal))
					}
				} else {
					missingFields = append(missingFields, key)
				}
			}
			if matchCount == len(expectedFields) {
				foundEntryWithAllFieldsAndMsg = true
				break
			} else {
				// Log details if message matched but fields didn't, for better debugging
				if len(missingFields) > 0 {
					t.Logf("Entry with msg '%s' was missing fields: %v. Entry: %v", expectedToContainMsg, missingFields, entry)
				}
				if len(mismatchedFields) > 0 {
					t.Logf("Entry with msg '%s' had mismatched fields: %v. Entry: %v", expectedToContainMsg, mismatchedFields, entry)
				}
			}
		}
	}
	if !foundEntryWithAllFieldsAndMsg {
		errMsg := fmt.Sprintf("Expected log entry containing msg '%s' with fields %v not found.", expectedToContainMsg, expectedFields)
		if lastRelevantEntry != nil {
			errMsg += fmt.Sprintf(" Last entry containing the message (but not all fields): %v.", lastRelevantEntry)
		}
		errMsg += fmt.Sprintf(" Full log:\n%s", logOutput)
		t.Error(errMsg)
	}
}

// --- Minimal Existing Tests (from previous steps for brevity) ---
func TestNewExecutor(t *testing.T) {
	exec, err := NewExecutor(ExecutorOptions{}); if err != nil {t.Fatalf("NewExecutor empty opts: %v", err)}; if exec.Logger == nil {t.Error("Logger nil")}; if exec.DefaultTaskConcurrency != DefaultTaskConcurrency {t.Errorf("Conc = %d, want %d", exec.DefaultTaskConcurrency, DefaultTaskConcurrency)}
}
func TestExecutor_SelectHostsForTaskSpec(t *testing.T) {
	clusterRt, _, _ := newTestRuntimeForExecutorWithLogCapture(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	taskSpec := &spec.TaskSpec{Name:"Test", RunOnRoles:[]string{}, Filter: func(h *runtime.Host)bool{return true}}
	if len(exec.selectHostsForTaskSpec(taskSpec, clusterRt)) != 1 {t.Error("Select all failed")}
}
func TestExecutor_SelectHostsForModule(t *testing.T) {
    clusterRt, _, _ := newTestRuntimeForExecutorWithLogCapture(t, 3, map[string][]string{"master": {"host1"}}); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
    taskSpecMaster := &spec.TaskSpec{Name: "MasterTask", RunOnRoles: []string{"master"}}
    moduleSpec1 := &spec.ModuleSpec{ Name:  "Module1_Master", Tasks: []*spec.TaskSpec{taskSpecMaster}}
    selected := exec.selectHostsForModule(moduleSpec1, clusterRt); if len(selected) != 1 || selected[0].Name != "host1" {t.Errorf("Select for module failed, got %v", selectedHostNamesForTest(selected))}
}
func TestExecutor_ExecuteTaskSpec_SimpleSuccess(t *testing.T) {
	_ = getTestMockStepExecutor(); clusterRt, targetHosts, _ := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger}); mockStep1Spec := &MockExecutorStepSpec{MockName: "MockStep1"}
	taskSpec := &spec.TaskSpec{ Name:  "TestSimpleTask", Steps: []spec.StepSpec{mockStep1Spec}}
	results, err := exec.executeTaskSpec(context.Background(), taskSpec, targetHosts, clusterRt, clusterRt.Logger)
	if err != nil {t.Fatalf("executeTaskSpec failed: %v", err)}; if len(results) != 1 {t.Errorf("Expected 1 result, got %d", len(results))}; if results[0].Status != "Succeeded" {t.Errorf("Step status %s", results[0].Status)}
}
func TestExecutor_ExecuteHookSteps_SimpleSuccess(t *testing.T) {
	_ = getTestMockStepExecutor(); clusterRt, targetHosts, _ := newTestRuntimeForExecutorWithLogCapture(t, 1, nil); exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})
	hookSpec := &MockExecutorStepSpec{MockName: "MyHookStep"}
	results, err := exec.executeHookSteps(context.Background(), hookSpec, "TestHook", targetHosts, clusterRt, clusterRt.Logger); if err != nil {t.Fatalf("failed: %v", err)}; if len(results) != 1 {t.Errorf("results = %d", len(results))}; if results[0].Status != "Succeeded" {t.Errorf("status = %s", results[0].Status)}
}


// --- New/Updated tests focusing on logger context propagation ---

func TestExecutor_ExecuteTaskSpec_LoggerContext(t *testing.T) {
	mockExecutor := getTestMockStepExecutor()
	clusterRt, targetHosts, logBuf := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	stepSpec1 := &MockExecutorStepSpec{MockName: "StepAlpha"}
	taskSpec := &spec.TaskSpec{ Name:  "LoggingTask", Steps: []spec.StepSpec{stepSpec1} }

	_, err := exec.executeTaskSpec(context.Background(), taskSpec, targetHosts, clusterRt, clusterRt.Logger)
	if err != nil {t.Fatalf("executeTaskSpec failed: %v", err)}

	loggedOutput := logBuf.String()
	expectedMsgFromStep := "MockStepExecutor.Execute called for StepAlpha"
	// Fields added by executor's logger hierarchy before step's own logger
	expectedFields := map[string]string{
		"pipeline_name": "TestPipeline",
		"task_name":     "LoggingTask",
		"host_name":     "host1",
		// "step_name": "StepAlpha" // This is added by the step's own logger, check message content instead
	}
	checkLogEntryFields(t, loggedOutput, expectedFields, expectedMsgFromStep)
	if mockExecutor.ExecuteCalled.Load() != 1 {t.Error("Execute not called")}
}

func TestExecutor_ExecuteHookSteps_LoggerContext(t *testing.T) {
	mockExecutor := getTestMockStepExecutor()
	clusterRt, targetHosts, logBuf := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	hookSpec := &MockExecutorStepSpec{MockName: "MyPipelinePreRunHook"}

	_, err := exec.executeHookSteps(context.Background(), hookSpec, "PipelinePreRunEvent", targetHosts, clusterRt, clusterRt.Logger)
	if err != nil {t.Fatalf("executeHookSteps failed: %v", err)}

	loggedOutput := logBuf.String()
	expectedMsgFromStep := "MockStepExecutor.Execute called for MyPipelinePreRunHook"
	expectedFields := map[string]string{
		"pipeline_name":  "TestPipeline",
		"hook_event":     "PipelinePreRunEvent",
		"hook_step_name": "MyPipelinePreRunHook",
		"host_name":      "host1",
	}
	checkLogEntryFields(t, loggedOutput, expectedFields, expectedMsgFromStep)
	if mockExecutor.ExecuteCalled.Load() != 1 {t.Error("Hook Execute not called")}
}

func TestExecutor_ExecuteModule_LoggerContext(t *testing.T) {
	mockExecutor := getTestMockStepExecutor()
	clusterRt, targetHosts, logBuf := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	stepInTask := &MockExecutorStepSpec{MockName: "ModTaskStep"}
	taskSpec := &spec.TaskSpec{Name: "ModuleTask1", Steps: []spec.StepSpec{stepInTask}}
	preHookSpec := &MockExecutorStepSpec{MockName: "ModulePreHook"}
	moduleSpec := &spec.ModuleSpec{ Name:   "LoggingModule", Tasks:  []*spec.TaskSpec{taskSpec}, PreRun: preHookSpec }

	// Assume selectHostsForModule and selectHostsForTaskSpec correctly return targetHosts for this test.
	// We are testing logger propagation through executeModule.
	originalSelectModuleHosts := exec.selectHostsForModule; originalSelectTaskHosts := exec.selectHostsForTaskSpec
	defer func() { exec.selectHostsForModule = originalSelectModuleHosts; exec.selectHostsForTaskSpec = originalSelectTaskHosts }()
	exec.selectHostsForModule = func(ms *spec.ModuleSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }
	exec.selectHostsForTaskSpec = func(ts *spec.TaskSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }

	_, err := exec.executeModule(context.Background(), moduleSpec, clusterRt, clusterRt.Logger)
	if err != nil {t.Fatalf("executeModule failed: %v", err)}
	loggedOutput := logBuf.String()

	expectedHookMsg := "MockStepExecutor.Execute called for ModulePreHook"
	expectedHookFields_Hook := map[string]string{
		"pipeline_name":  "TestPipeline", "module_name":    "LoggingModule",
		"hook_event":     "ModulePreRun[LoggingModule]", "hook_step_name": "ModulePreHook",
		"host_name":      "host1",
	}
	checkLogEntryFields(t, loggedOutput, expectedHookFields_Hook, expectedHookMsg)

	expectedTaskStepMsg := "MockStepExecutor.Execute called for ModTaskStep"
	expectedTaskStepFields_Task := map[string]string{
		"pipeline_name": "TestPipeline", "module_name":   "LoggingModule",
		"task_name":     "ModuleTask1",  "host_name":     "host1",
		// "step_name":     "ModTaskStep", // step_name is part of the step's own logger context
	}
	checkLogEntryFields(t, loggedOutput, expectedTaskStepFields_Task, expectedTaskStepMsg)
	if mockExecutor.ExecuteCalled.Load() != 2 { t.Errorf("ExecuteCalled = %d, want 2 (PreRun hook + one step in task)", mockExecutor.ExecuteCalled.Load()) }
}

func TestExecutor_ExecutePipeline_LoggerContext(t *testing.T) {
    mockExecutor := getTestMockStepExecutor()
    clusterRt, targetHosts, logBuf := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
    exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

    pipelinePreHookSpec := &MockExecutorStepSpec{MockName: "PipelineLevelPreHook"}
    stepInTaskSpec := &MockExecutorStepSpec{MockName: "DeepStepInPipeline"}
    taskInModuleSpec := &spec.TaskSpec{Name: "PipelineTask", Steps: []spec.StepSpec{stepInTaskSpec}}
    moduleInPipelineSpec := &spec.ModuleSpec{Name: "PipelineModule", Tasks: []*spec.TaskSpec{taskInModuleSpec}}

    pipelineTestSpec := &spec.PipelineSpec{ Name: "FullContextPipelineForLog", PreRun:  pipelinePreHookSpec, Modules: []*spec.ModuleSpec{moduleInPipelineSpec} }

    originalSelectModuleHosts := exec.selectHostsForModule; originalSelectTaskHosts := exec.selectHostsForTaskSpec
	defer func() { exec.selectHostsForModule = originalSelectModuleHosts; exec.selectHostsForTaskSpec = originalSelectTaskHosts }()
    exec.selectHostsForModule = func(ms *spec.ModuleSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }
    exec.selectHostsForTaskSpec = func(ts *spec.TaskSpec, cr *runtime.ClusterRuntime) []*runtime.Host { return targetHosts }

    _, err := exec.ExecutePipeline(context.Background(), pipelineTestSpec, clusterRt)
    if err != nil {t.Fatalf("ExecutePipeline failed: %v", err)}
    loggedOutput := logBuf.String()

    expectedPipeHookMsg := "MockStepExecutor.Execute called for PipelineLevelPreHook"
    expectedPipeHookFields := map[string]string{ "pipeline_name":  "FullContextPipelineForLog", "hook_event": "PipelinePreRun", "hook_step_name": "PipelineLevelPreHook", "host_name": "host1" }
    checkLogEntryFields(t, loggedOutput, expectedPipeHookFields, expectedPipeHookMsg)

    expectedDeepStepMsg := "MockStepExecutor.Execute called for DeepStepInPipeline"
    expectedDeepStepFields := map[string]string{ "pipeline_name": "FullContextPipelineForLog", "module_name": "PipelineModule", "task_name": "PipelineTask", "host_name": "host1" }
    checkLogEntryFields(t, loggedOutput, expectedDeepStepFields, expectedDeepStepMsg)
	if mockExecutor.ExecuteCalled.Load() != 2 { t.Errorf("ExecuteCalled = %d, want 2 (Pipeline PreHook + one step in task)", mockExecutor.ExecuteCalled.Load()) }
}
