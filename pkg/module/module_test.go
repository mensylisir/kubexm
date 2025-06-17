package module

import (
	"context"
	"errors"
	"fmt"
	"sort" // For sorting string slices in helper
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step" // For step.Result
	"github.com/kubexms/kubexms/pkg/task"

	// Import module factories to test them
	module_containerd "github.com/kubexms/kubexms/pkg/module/containerd" // Aliased
	module_preflight "github.com/kubexms/kubexms/pkg/module/preflight"   // Aliased
)

// MockTask is a mock implementation of task.Task for testing Module.Run.
type MockTask struct {
	task.Task                 // Embed real Task to get Name, IgnoreError, etc.
	RunFunc   func(goCtx context.Context, hosts []*runtime.Host, cluster *runtime.ClusterRuntime) ([]*step.Result, error)
	RunCount  int32
}

func (m *MockTask) Run(goCtx context.Context, hosts []*runtime.Host, cluster *runtime.ClusterRuntime) ([]*step.Result, error) {
	atomic.AddInt32(&m.RunCount, 1)
	if m.RunFunc != nil {
		return m.RunFunc(goCtx, hosts, cluster)
	}
	// Default behavior: success, one dummy result per host
	results := make([]*step.Result, len(hosts))
	taskNameForStep := m.Name
	if taskNameForStep == "" { taskNameForStep = "UnnamedMockTask" }

	for i, h := range hosts {
		// Using a more descriptive step name for the dummy result
		results[i] = step.NewResult(taskNameForStep+"_mockStep_on_"+h.Name, h.Name, time.Now(), nil)
	}
	return results, nil
}

// newTestRuntimeForModule creates a minimal runtime.ClusterRuntime with mock hosts.
func newTestRuntimeForModule(t *testing.T, numHosts int, hostRolesSetup map[string][]string) (*runtime.ClusterRuntime, []*runtime.Host) {
	t.Helper()
	testLogOpts := logger.DefaultOptions()
	testLogOpts.ConsoleOutput = false
	testLogOpts.FileOutput = false
	logInst, _ := logger.NewLogger(testLogOpts)

	hosts := make([]*runtime.Host, numHosts)
	inventory := make(map[string]*runtime.Host)
	roleInventory := make(map[string][]*runtime.Host)

	for i := 0; i < numHosts; i++ {
		hostName := fmt.Sprintf("host%d", i+1)
		h := &runtime.Host{
			Name:    hostName,
			Address: fmt.Sprintf("192.168.1.%d", i+1),
			Roles:   make(map[string]bool),
		}
		// Assign roles
		for role, namesInRole := range hostRolesSetup {
			for _, name := range namesInRole {
				if name == hostName {
					h.Roles[role] = true
				}
			}
		}
		hosts[i] = h
		inventory[hostName] = h
		for roleName := range h.Roles {
			roleInventory[roleName] = append(roleInventory[roleName], h)
		}
	}

	return &runtime.ClusterRuntime{
		Logger:        logInst,
		Hosts:         hosts,
		Inventory:     inventory,
		RoleInventory: roleInventory,
		ClusterConfig: &config.Cluster{ Spec: config.ClusterSpec{ Global: config.GlobalSpec{}}},
	}, hosts
}


func TestModule_Run_Success_AllTasks(t *testing.T) {
	clusterRt, targetHosts := newTestRuntimeForModule(t, 1, nil) // 1 host, no specific roles needed for this test

	task1 := &MockTask{Task: task.Task{Name: "Task1"}}
	task2 := &MockTask{Task: task.Task{Name: "Task2"}}

	mod := &Module{
		Name:  "TestModule_AllSuccess",
		Tasks: []*task.Task{&task1.Task, &task2.Task},
	}

	results, err := mod.Run(context.Background(), clusterRt)
	if err != nil {
		t.Fatalf("Module.Run() error = %v, wantErr nil", err)
	}
	if atomic.LoadInt32(&task1.RunCount) != 1 {
		t.Errorf("Task1 RunCount = %d, want 1", task1.RunCount)
	}
	if atomic.LoadInt32(&task2.RunCount) != 1 {
		t.Errorf("Task2 RunCount = %d, want 1", task2.RunCount)
	}
	// Each MockTask (by default) returns one result per host. 1 host, 2 tasks.
	if len(results) != 2*len(targetHosts) {
		t.Errorf("Expected %d step results (1 host * 1 step/task * 2 tasks), got %d", 2*len(targetHosts), len(results))
	}
}

func TestModule_Run_TaskFails_StopsModule(t *testing.T) {
	clusterRt, _ := newTestRuntimeForModule(t, 1, nil)
	expectedErr := errors.New("task1 deliberate failure")

	task1 := &MockTask{
		Task: task.Task{Name: "Task1_Fails", IgnoreError: false},
		RunFunc: func(goCtx context.Context, hosts []*runtime.Host, cluster *runtime.ClusterRuntime) ([]*step.Result, error) {
			// Return some dummy results even on failure, as Task.Run collects them
			dummyResults := []*step.Result{step.NewResult("dummyStep", hosts[0].Name, time.Now(), expectedErr)}
			dummyResults[0].Status = "Failed"
			return dummyResults, expectedErr
		},
	}
	task2_should_not_run := &MockTask{Task: task.Task{Name: "Task2_ShouldNotRun"}}

	mod := &Module{
		Name:  "TestModule_TaskFails",
		Tasks: []*task.Task{&task1.Task, &task2_should_not_run.Task},
	}

	results, err := mod.Run(context.Background(), clusterRt)
	if err == nil {
		t.Fatal("Module.Run() with a failing task expected an error, got nil")
	}
	// The error from Module.Run should wrap the original task error.
	if !strings.Contains(err.Error(), "Task1_Fails failed") || !errors.Is(err, expectedErr) {
		t.Errorf("Error message = %q, expected to contain failure details and wrap original error", err.Error())
	}
	if atomic.LoadInt32(&task1.RunCount) != 1 {
		t.Error("Failing Task1 was not run")
	}
	if atomic.LoadInt32(&task2_should_not_run.RunCount) != 0 {
		t.Error("Task2 after failing task was run unexpectedly")
	}
	if len(results) != 1 { // Should have results from the failing task
		t.Errorf("Expected 1 result from the failing task, got %d", len(results))
	}
}

func TestModule_Run_TaskFails_IgnoreError_Continues(t *testing.T) {
	clusterRt, _ := newTestRuntimeForModule(t, 1, nil)
	task1Err := errors.New("task1 soft failure")

	task1 := &MockTask{
		Task: task.Task{Name: "Task1_SoftFails", IgnoreError: true},
		RunFunc: func(goCtx context.Context, hosts []*runtime.Host, cluster *runtime.ClusterRuntime) ([]*step.Result, error) {
			return []*step.Result{step.NewResult("ignoredStep", hosts[0].Name, time.Now(), task1Err)}, task1Err
		},
	}
	task2_should_run := &MockTask{Task: task.Task{Name: "Task2_ShouldRunAfterIgnoredError"}}

	mod := &Module{
		Name:  "TestModule_IgnoreTaskError",
		Tasks: []*task.Task{&task1.Task, &task2_should_run.Task},
	}

	_, err := mod.Run(context.Background(), clusterRt)
	if err != nil { // Module's Run should return nil error overall
		t.Fatalf("Module.Run() expected nil error due to IgnoreError, got %v", err)
	}
	if atomic.LoadInt32(&task1.RunCount) != 1 {t.Error("Task1 (ignored fail) was not run")}
	if atomic.LoadInt32(&task2_should_run.RunCount) != 1 {t.Error("Task2 after ignored fail was not run")}
}

func TestModule_Run_PreRun_Hook_Fails(t *testing.T) {
	clusterRt, _ := newTestRuntimeForModule(t, 1, nil)
	expectedPreRunErr := errors.New("prerun deliberate failure")

	preRunCalled := false
	postRunCalledAfterPreFail := false

	task1_should_not_run := &MockTask{Task: task.Task{Name: "Task1_ShouldNotRunAfterPreRunFail"}}
	mod := &Module{
		Name:  "TestModule_PreRunFails",
		Tasks: []*task.Task{&task1_should_not_run.Task},
		PreRun: func(cluster *runtime.ClusterRuntime) error {
			preRunCalled = true
			return expectedPreRunErr
		},
		PostRun: func(cluster *runtime.ClusterRuntime, moduleExecError error) error {
			postRunCalledAfterPreFail = true
			if !errors.Is(moduleExecError, expectedPreRunErr) { // Check if PostRun received the PreRun error
				t.Errorf("PostRun error = %v, want wrapped preRunErr %v", moduleExecError, expectedPreRunErr)
			}
			return nil // PostRun itself doesn't fail here
		},
	}

	_, err := mod.Run(context.Background(), clusterRt)
	if err == nil {
		t.Fatal("Module.Run() with failing PreRun hook expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "PreRun hook failed") || !errors.Is(err, expectedPreRunErr) {
		t.Errorf("Error message = %q, expected PreRun failure details and wrapped error", err.Error())
	}
	if !preRunCalled {t.Error("PreRun hook was not called")}
	if atomic.LoadInt32(&task1_should_not_run.RunCount) != 0 {
		t.Error("Task was run after PreRun hook failed")
	}
	if !postRunCalledAfterPreFail {t.Error("PostRun hook was not called after PreRun failure")}
}

func TestModule_Run_PostRun_Hook_Error(t *testing.T) {
	clusterRt, _ := newTestRuntimeForModule(t, 1, nil)
	expectedPostRunErr := errors.New("postrun deliberate failure")
	postRunCalled := false

	mod := &Module{
		Name: "TestModule_PostRunError",
		Tasks: []*task.Task{},
		PostRun: func(cluster *runtime.ClusterRuntime, moduleExecError error) error {
			postRunCalled = true
			return expectedPostRunErr
		},
	}
	_, err := mod.Run(context.Background(), clusterRt)
	if err == nil {
		t.Fatal("Module.Run() with failing PostRun hook expected an error, got nil")
	}
	if !errors.Is(err, expectedPostRunErr) || !strings.Contains(err.Error(), "PostRun hook failed") {
		t.Errorf("Error = %v, want wrapped PostRun error %v", err, expectedPostRunErr)
	}
	if !postRunCalled {t.Error("PostRun hook was not called")}
}


func TestModule_Run_IsEnabled_False(t *testing.T) {
	clusterRt, _ := newTestRuntimeForModule(t, 1, nil)
	task1 := &MockTask{Task: task.Task{Name: "Task1_ShouldNotRunIfModuleDisabled"}}
	mod := &Module{
		Name:  "TestModule_Disabled",
		Tasks: []*task.Task{&task1.Task},
		IsEnabled: func(clusterCfg *config.Cluster) bool { return false },
	}

	results, err := mod.Run(context.Background(), clusterRt)
	if err != nil {
		t.Fatalf("Module.Run() for disabled module error = %v, want nil", err)
	}
	if results != nil {
		t.Errorf("Expected nil results for disabled module, got %d results", len(results))
	}
	if atomic.LoadInt32(&task1.RunCount) != 0 {
		t.Error("Task was run for a disabled module")
	}
}

func TestSelectHostsForTask(t *testing.T) {
	hostRolesSetup := map[string][]string{
		"master": {"host1", "host2"},
		"worker": {"host2", "host3"},
		"etcd":   {"host1"},
	}
	clusterRt, _ := newTestRuntimeForModule(t, 3, hostRolesSetup)

	taskAll := &task.Task{Name: "OnAll", RunOnRoles: []string{}}
	taskMaster := &task.Task{Name: "OnMaster", RunOnRoles: []string{"master"}}
	taskWorkerEtcd := &task.Task{Name: "OnWorkerOrEtcd", RunOnRoles: []string{"worker", "etcd"}}
	taskMasterWithFilter := &task.Task{
		Name: "OnMasterWithFilter",
		RunOnRoles: []string{"master"},
		Filter: func(h *runtime.Host) bool { return h.Name == "host1" },
	}
	taskNonExistentRole := &task.Task{Name: "NonExistentRole", RunOnRoles: []string{"db"}}


	tests := []struct {
		name         string
		task         *task.Task
		expectedHostCount int
		expectedHostNames []string
	}{
		{"all hosts (no roles specified)", taskAll, 3, []string{"host1", "host2", "host3"}},
		{"master role", taskMaster, 2, []string{"host1", "host2"}},
		{"worker or etcd role", taskWorkerEtcd, 3, []string{"host1", "host2", "host3"}},
		{"master role with filter for host1", taskMasterWithFilter, 1, []string{"host1"}},
		{"non-existent role", taskNonExistentRole, 0, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected := selectHostsForTask(clusterRt, tt.task)
			if len(selected) != tt.expectedHostCount {
				t.Errorf("selectHostsForTask for '%s' expected %d hosts, got %d (%v)",
					tt.task.Name, tt.expectedHostCount, len(selected), selectedHostNames(selected))
			}
			if tt.expectedHostNames != nil { // If specific names are expected, check them
				selectedNames := selectedHostNames(selected)
				// Sort slices before comparing if order doesn't matter
				sort.Strings(selectedNames)
				sort.Strings(tt.expectedHostNames)
				if !equalStringSlicesIgnoreOrder(selectedNames, tt.expectedHostNames) { // Use order-agnostic compare
					t.Errorf("selectHostsForTask for '%s' expected hosts %v, got %v",
						tt.task.Name, tt.expectedHostNames, selectedNames)
				}
			}
		})
	}
}

func selectedHostNames(hosts []*runtime.Host) []string {
	names := make([]string, len(hosts))
	for i, h := range hosts {
		names[i] = h.Name
	}
	return names
}

func equalStringSlicesIgnoreOrder(a, b []string) bool {
	if len(a) != len(b) { return false }
	aCopy := make([]string, len(a)); copy(aCopy, a)
	bCopy := make([]string, len(b)); copy(bCopy, b)
	sort.Strings(aCopy)
	sort.Strings(bCopy)
	for i := range aCopy {
		if aCopy[i] != bCopy[i] {
			return false
		}
	}
	return true
}


func TestNewPreflightModule_Assembly(t *testing.T) {
	dummyCfg := &config.Cluster{}
	mod := module_preflight.NewPreflightModule(dummyCfg)

	if mod.Name != "Preflight Checks and Setup" {
		t.Errorf("Name = %s", mod.Name)
	}
	if mod.IsEnabled == nil || !mod.IsEnabled(dummyCfg) {
		t.Error("IsEnabled func not set or returns false unexpectedly")
	}
	if len(mod.Tasks) < 2 {
		t.Errorf("Expected at least 2 tasks (SystemChecks, SetupKernel), got %d", len(mod.Tasks))
	}
	if mod.PreRun == nil { t.Error("PreRun hook is nil") }
	if mod.PostRun == nil { t.Error("PostRun hook is nil") }
}

func TestNewContainerdModule_Assembly(t *testing.T) {
	// Dummy config for testing IsEnabled logic if it depends on config fields
	dummyCfgWithContainerd := &config.Cluster{
		Spec: config.ClusterSpec{
			ContainerRuntime: &config.ContainerRuntimeSpec{Type: "containerd"},
		},
	}
	dummyCfgNoContainerdSection := &config.Cluster{
		Spec: config.ClusterSpec{ /* No ContainerRuntime specified, IsEnabled might default to true */ },
	}
	dummyCfgOtherRuntime := &config.Cluster{
		Spec: config.ClusterSpec{
			ContainerRuntime: &config.ContainerRuntimeSpec{Type: "docker"},
		},
	}

	mod := module_containerd.NewContainerdModule(dummyCfgWithContainerd)
	if mod.Name != "Containerd Runtime" { t.Errorf("Name = %s", mod.Name) }
	if mod.IsEnabled == nil { t.Fatal("IsEnabled func is nil") }
	if !mod.IsEnabled(dummyCfgWithContainerd) {
		t.Error("IsEnabled func returns false unexpectedly for 'containerd' type config")
	}
	if !mod.IsEnabled(dummyCfgNoContainerdSection) { // Current factory defaults to true if section missing
		t.Error("IsEnabled func returns false unexpectedly for nil ContainerRuntime config (expected default true)")
	}
	if mod.IsEnabled(dummyCfgOtherRuntime) { // Should be false if type is not containerd
		t.Error("IsEnabled func returns true unexpectedly for 'docker' type config")
	}


	if len(mod.Tasks) < 1 { // At least NewInstallContainerdTask
		t.Errorf("Expected at least 1 task, got %d", len(mod.Tasks))
	}
}
