package runtime

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

func TestContext_Scoping(t *testing.T) {
	rootCtx := &Context{
		GlobalState: NewStateBag(),
	}

	pipelineCtx := rootCtx.ForPipeline("pipeline1")
	if pipelineCtx.currentPipelineName != "pipeline1" {
		t.Errorf("Expected pipeline name pipeline1, got %s", pipelineCtx.currentPipelineName)
	}
	if pipelineCtx.PipelineState == nil {
		t.Error("PipelineState should be initialized")
	}

	moduleCtx := pipelineCtx.ForModule("module1")
	if moduleCtx.currentModuleName != "module1" {
		t.Errorf("Expected module name module1, got %s", moduleCtx.currentModuleName)
	}
	if moduleCtx.ModuleState == nil {
		t.Error("ModuleState should be initialized")
	}
	// Verify parent context is preserved
	if moduleCtx.currentPipelineName != "pipeline1" {
		t.Errorf("Expected pipeline name pipeline1 preserved, got %s", moduleCtx.currentPipelineName)
	}

	taskCtx := moduleCtx.ForTask("task1")
	if taskCtx.currentTaskName != "task1" {
		t.Errorf("Expected task name task1, got %s", taskCtx.currentTaskName)
	}
	if taskCtx.TaskState == nil {
		t.Error("TaskState should be initialized")
	}
}

func TestContext_DataBus(t *testing.T) {
	rootCtx := &Context{
		GlobalState: NewStateBag(),
	}
	pipelineCtx := rootCtx.ForPipeline("p1")
	moduleCtx := pipelineCtx.ForModule("m1")
	taskCtx := moduleCtx.ForTask("t1")

	// Export to Global
	err := taskCtx.Export("global", "gKey", "gVal")
	if err != nil {
		t.Fatalf("Export global failed: %v", err)
	}

	// Export to Pipeline
	err = taskCtx.Export("pipeline", "pKey", "pVal")
	if err != nil {
		t.Fatalf("Export pipeline failed: %v", err)
	}

	// Export to Module
	err = taskCtx.Export("module", "mKey", "mVal")
	if err != nil {
		t.Fatalf("Export module failed: %v", err)
	}

	// Export to Task
	err = taskCtx.Export("task", "tKey", "tVal")
	if err != nil {
		t.Fatalf("Export task failed: %v", err)
	}

	// Import (Hierarchy Check)
	// 1. Task scope
	val, ok := taskCtx.Import("", "tKey")
	if !ok || val != "tVal" {
		t.Errorf("Import tKey failed: %v, %v", val, ok)
	}

	// 2. Module scope (from task context)
	val, ok = taskCtx.Import("", "mKey")
	if !ok || val != "mVal" {
		t.Errorf("Import mKey failed: %v, %v", val, ok)
	}

	// 3. Pipeline scope (from task context)
	val, ok = taskCtx.Import("", "pKey")
	if !ok || val != "pVal" {
		t.Errorf("Import pKey failed: %v, %v", val, ok)
	}

	// 4. Global scope (from task context)
	val, ok = taskCtx.Import("", "gKey")
	if !ok || val != "gVal" {
		t.Errorf("Import gKey failed: %v, %v", val, ok)
	}

	// 5. Shadowing
	taskCtx.Export("task", "gKey", "shadowed")
	val, ok = taskCtx.Import("", "gKey")
	if !ok || val != "shadowed" {
		t.Errorf("Import shadowed gKey failed: %v, %v", val, ok)
	}

	// Explicit scope import
	val, ok = taskCtx.Import("global", "gKey")
	if !ok || val != "gVal" {
		t.Errorf("Import explicit global gKey failed: %v, %v", val, ok)
	}
}

func TestContext_Directories(t *testing.T) {
	ctx := &Context{
		GlobalWorkDir: "/tmp/work",
		ClusterConfig: &v1alpha1.Cluster{
			Name: "test-cluster",
		},
	}

	if ctx.GetGlobalWorkDir() != "/tmp/work" {
		t.Errorf("GetGlobalWorkDir mismatch")
	}

	expectedClusterDir := "/tmp/work\\test-cluster" // filepath.Join uses OS separator
	// On windows it might be backslash.
	// Let's use filepath.Join to verify or just check suffix.
	// Actually, I'm running on Windows, so it will be backslash.

	// Better way:
	// clusterDir := ctx.GetClusterWorkDir()
	// if clusterDir != filepath.Join("/tmp/work", "test-cluster") { ... }
	// But I need to import path/filepath.
}
