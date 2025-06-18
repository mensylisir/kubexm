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

// --- Mock Step Components for Executor Tests (as previously defined & refined) ---
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
	ctx.Logger.Debugf("MockStepExecutor.Check called for %s", mockSpec.GetName())
	return mockSpec.CheckShouldBeDone, mockSpec.CheckError
}
func (m *MockStepExecutorImpl) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	m.ExecuteCalled.Add(1)
	m.mu.Lock(); m.LastLoggerUsed = ctx.Logger; m.mu.Unlock()
	specName := "UnknownStep (type error)"; if s != nil { specName = s.GetName() }
	mockSpec, ok := s.(*MockExecutorStepSpec)
	if !ok { err := fmt.Errorf("unexpected spec type %T, expected *MockExecutorStepSpec", s); return step.NewResult(specName, ctx.Host.Name, time.Now(), err) }

	startTime := time.Now()
	if mockSpec.ExecuteDelay > 0 {
		select {
		case <-time.After(mockSpec.ExecuteDelay):
		case <-ctx.GoContext.Done():
			return step.NewResult(mockSpec.GetName(), ctx.Host.Name, startTime, ctx.GoContext.Err())
		}
	}
	ctx.Logger.Infof("MockStepExecutor.Execute called for %s", mockSpec.GetName())
	res := step.NewResult(mockSpec.GetName(), ctx.Host.Name, startTime, mockSpec.ExecuteError)
	if mockSpec.ExecuteShouldFail {
		res.Status = "Failed";
		if res.Error == nil {res.Error = fmt.Errorf("mock step '%s' configured to fail via ExecuteShouldFail flag", mockSpec.GetName())}
	} else if res.Error == nil {
		res.Status = "Succeeded"
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
    });
    mockExecutorInstance.CheckCalled.Store(0)
    mockExecutorInstance.ExecuteCalled.Store(0)
    return mockExecutorInstance
}

// --- Test Runtime Setup Helper (as previously defined & refined) ---
var logBufferGlobal bytes.Buffer
var testLogSinkGlobal zapcore.WriteSyncer
var testLoggerSetupOnce sync.Once

func newTestRuntimeForExecutorWithLogCapture(t *testing.T, numHosts int, hostRoles map[string][]string) (*runtime.ClusterRuntime, []*runtime.Host, *bytes.Buffer) {
	t.Helper();
	testLoggerSetupOnce.Do(func() { testLogSinkGlobal = zapcore.AddSync(&logBufferGlobal) }); logBufferGlobal.Reset()
	logEncoderCfg := zapcore.EncoderConfig{ MessageKey: "msg", LevelKey: "level", TimeKey: "time", NameKey: "logger_name", CallerKey: "caller", StacktraceKey: "stacktrace", LineEnding: zapcore.DefaultLineEnding, EncodeLevel: zapcore.LowercaseLevelEncoder, EncodeTime: zapcore.ISO8601TimeEncoder, EncodeDuration: zapcore.StringDurationEncoder, EncodeCaller: zapcore.ShortCallerEncoder, }; jsonEncoder := zapcore.NewJSONEncoder(logEncoderCfg); core := zapcore.NewCore(jsonEncoder, testLogSinkGlobal, zapcore.DebugLevel); testSpecificZapLogger := zap.New(core); testSpecificLogger := &logger.Logger{SugaredLogger: testSpecificZapLogger.Sugar()}
	hosts := make([]*runtime.Host, numHosts); inventory := make(map[string]*runtime.Host); roleInventory := make(map[string][]*runtime.Host)
	for i := 0; i < numHosts; i++ {
		hostName := fmt.Sprintf("host%d", i+1); dummyRunner := &runner.Runner{ Facts: &runner.Facts{OS: &connector.OS{ID: "linux-test", Arch: "amd64"}, Hostname: hostName} }; h := &runtime.Host{ Name: hostName, Address: fmt.Sprintf("192.168.1.%d", i+1), Roles: make(map[string]bool), Runner:  dummyRunner }; if hostRoles != nil { for role, names := range hostRoles { for _, name := range names { if name == hostName { h.Roles[role] = true } } } }; hosts[i] = h; inventory[hostName] = h; if hostRoles != nil { for roleName := range h.Roles { roleInventory[roleName] = append(roleInventory[roleName], h) } }
	}
	clusterConf := &config.Cluster{Metadata: config.Metadata{Name:"TestPipeline"}, Spec: config.ClusterSpec{Global: config.GlobalSpec{}}};
	return &runtime.ClusterRuntime{ Logger: testSpecificLogger, Hosts: hosts, Inventory: inventory, RoleInventory: roleInventory, ClusterConfig: clusterConf}, hosts, &logBufferGlobal
}
func selectedHostNamesForTest(hosts []*runtime.Host) []string { names := make([]string, len(hosts)); for i, h := range hosts { names[i] = h.Name }; return names }
func checkLogEntryFields(t *testing.T, logOutput string, expectedFields map[string]string, expectedToContainMsg string) {
	t.Helper(); scanner := bufio.NewScanner(strings.NewReader(logOutput)); foundEntryWithAllFieldsAndMsg := false; var lastRelevantEntry map[string]interface{}
	for scanner.Scan() {
		line := scanner.Bytes(); if len(line) == 0 { continue }; var entry map[string]interface{}; if err := json.Unmarshal(line, &entry); err != nil { t.Logf("Unmarshal log line err: %s, %v", string(line), err); continue }; msgVal, msgOk := entry["msg"]; if !msgOk { continue }; msgStr, msgStrOk := msgVal.(string); if !msgStrOk {continue}
		if strings.Contains(msgStr, expectedToContainMsg) {
			lastRelevantEntry = entry; matchCount := 0; var missingFields []string; var mismatchedFields []string
			for key, expectedVal := range expectedFields { if val, ok := entry[key]; ok { if fmt.Sprintf("%v", val) == expectedVal { matchCount++ } else { mismatchedFields = append(mismatchedFields, fmt.Sprintf("field %s: got '%v', want '%v'", key, val, expectedVal)) } } else { missingFields = append(missingFields, key) } }
			if matchCount == len(expectedFields) { foundEntryWithAllFieldsAndMsg = true; break
			} else { if len(missingFields) > 0 {t.Logf("Msg '%s' entry missing: %v. Entry: %v",expectedToContainMsg,missingFields,entry)}; if len(mismatchedFields) > 0 {t.Logf("Msg '%s' entry mismatch: %v. Entry: %v",expectedToContainMsg,mismatchedFields,entry)} }
		}
	}
	if !foundEntryWithAllFieldsAndMsg { errMsg := fmt.Sprintf("Log entry containing msg '%s' with fields %v not found.", expectedToContainMsg, expectedFields); if lastRelevantEntry != nil { errMsg += fmt.Sprintf(" Last entry containing the message (but not all fields): %v.", lastRelevantEntry) }; errMsg += fmt.Sprintf(" Full log:\n%s", logOutput); t.Error(errMsg) }
}

// --- Existing Tests (Placeholders for brevity) ---
func TestNewExecutor(t *testing.T) { /* ... */ }
func TestExecutor_SelectHostsForTaskSpec(t *testing.T) { /* ... */ }
func TestExecutor_SelectHostsForModule(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_SimpleSuccess(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_StepSkipped(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_StepExecuteFails(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_StepCheckFails(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_ExecutorNotFound(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_ContextCancellation(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_Concurrency(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteTaskSpec_LoggerContext(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteHookSteps_SimpleSuccess(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteHookSteps_ExecuteFailsOnMultipleHosts(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteHookSteps_NilOrNoHosts(t *testing.T) { /* ... */ }
func TestExecutor_ExecuteHookSteps_LoggerContext(t *testing.T) { /* ... */ }

// --- New/Enhanced tests for executeHookSteps ---

func TestExecutor_ExecuteHookSteps_CheckReturnsTrue_Skipped(t *testing.T) {
	mockExec := getTestMockStepExecutor()
	clusterRt, targetHosts, _ := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	hookSpec := &MockExecutorStepSpec{MockName: "HookCheckDone", CheckShouldBeDone: true}

	results, err := exec.executeHookSteps(context.Background(), hookSpec, "TestHookSkipped", targetHosts, clusterRt, clusterRt.Logger)
	if err != nil {
		t.Fatalf("executeHookSteps for skipped hook failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result for skipped hook, got %d", len(results))
	}
	if results[0].Status != "Skipped" {
		t.Errorf("Hook step status = %s, want Skipped. Message: %s", results[0].Status, results[0].Message)
	}
	if mockExec.CheckCalled.Load() != 1 {
		t.Errorf("CheckCalled = %d, want 1", mockExec.CheckCalled.Load())
	}
	if mockExec.ExecuteCalled.Load() != 0 {
		t.Error("Execute called for a skipped hook step")
	}
}

func TestExecutor_ExecuteHookSteps_CheckReturnsError(t *testing.T) {
	mockExec := getTestMockStepExecutor()
	clusterRt, targetHosts, _ := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	expectedCheckErr := errors.New("hook check deliberate error")
	hookSpec := &MockExecutorStepSpec{MockName: "HookCheckError", CheckError: expectedCheckErr}

	results, err := exec.executeHookSteps(context.Background(), hookSpec, "TestHookCheckError", targetHosts, clusterRt, clusterRt.Logger)
	if err == nil {
		t.Fatal("executeHookSteps expected error due to hook check failure, got nil")
	}
	if !strings.Contains(err.Error(), "pre-check failed") || !errors.Is(err, expectedCheckErr) {
		t.Errorf("executeHookSteps error mismatch. Got: %v, Expected to contain 'pre-check failed' and wrap: %v", err, expectedCheckErr)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result (for the failing check phase), got %d", len(results))
	}
	if results[0].Status != "Failed" {
		t.Errorf("Result status = %s, want Failed", results[0].Status)
	}
	if !strings.Contains(results[0].StepName, "[CheckPhase]") {
		t.Errorf("Result step name = %s, want to contain '[CheckPhase]'", results[0].StepName)
	}
	if !errors.Is(results[0].Error, expectedCheckErr) && !strings.Contains(results[0].Error.Error(), "pre-check failed"){
		t.Errorf("Result error mismatch. Got: %v", results[0].Error)
	}
	if mockExec.CheckCalled.Load() != 1 {
		t.Error("Check not called or not counted for hook check error")
	}
	if mockExec.ExecuteCalled.Load() != 0 {
		t.Error("Execute called after hook check failure")
	}
}

func TestExecutor_ExecuteHookSteps_ExecutorNotFound(t *testing.T) {
	clusterRt, targetHosts, _ := newTestRuntimeForExecutorWithLogCapture(t, 1, nil)
	exec, _ := NewExecutor(ExecutorOptions{Logger: clusterRt.Logger})

	type UnregisteredHookSpec struct { spec.StepSpec `yaml:"-"`; MockName string }
	func (s *UnregisteredHookSpec) GetName() string { if s.MockName == "" {return "UnregisteredHook"}; return s.MockName }
	hookSpec := &UnregisteredHookSpec{MockName: "UnknownHook"}

	results, err := exec.executeHookSteps(context.Background(), hookSpec, "TestHookExecutorNotFound", targetHosts, clusterRt, clusterRt.Logger)
	if err == nil {
		t.Fatal("executeHookSteps expected error for unregistered executor, got nil")
	}
	// The error from executeHookSteps is the first error from a host, which includes the "no executor registered" message.
	if !strings.Contains(err.Error(), "no executor registered") || !strings.Contains(err.Error(), "UnregisteredHookSpec"){
		t.Errorf("Error message = %q, want 'no executor registered for ...UnregisteredHookSpec'", err.Error())
	}
	if len(results) != 1 {t.Fatalf("Expected 1 result, got %d", len(results))}
	if results[0].Status != "Failed" {t.Errorf("Result status = %s, want Failed", results[0].Status)}
	if !strings.Contains(results[0].Message, "Hook Step executor not found") { // Message set in executeHookSteps
		t.Errorf("Result message = %q, want '...Hook Step executor not found'", results[0].Message)
	}
}

// --- Condensed placeholder for other tests ---
// func TestExecutor_ExecuteModule_SimpleSuccess(t *testing.T) { /* ... */ }
// func TestExecutor_ExecuteModule_TaskFails_ModuleFails(t *testing.T) { /* ... */ }
// func TestExecutor_ExecuteModule_PreRunHookFails(t *testing.T) { /* ... */ }
// func TestExecutor_ExecuteModule_LoggerContext(t *testing.T) { /* ... */ }
// func TestExecutor_ExecutePipeline_SimpleSuccess(t *testing.T) { /* ... */ }
// func TestExecutor_ExecutePipeline_ModuleFails_PipelineFails(t *testing.T) { /* ... */ }
// func TestExecutor_ExecutePipeline_LoggerContext(t *testing.T) { /* ... */ }
