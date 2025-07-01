package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks"
)

func TestDefaultRunner_VMExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	vmName := "test-vm"

	// Test case 1: VM exists
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.VMExists(ctx, mockConn, vmName)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test case 2: VM does not exist (command fails with non-zero exit, typical for "not found")
	// In the actual implementation, non-zero is treated as "does not exist" for simplicity.
	// A more robust check would inspect stderr for specific "not found" messages.
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("error: failed to get domain 'test-vm'"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.VMExists(ctx, mockConn, vmName)
	assert.NoError(t, err) // The function VMExists is designed to return (false, nil) if not found based on exit code
	assert.False(t, exists)

	// Test case 3: virsh command execution error (not related to VM existence)
	execErr := fmt.Errorf("virsh command failed")
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("some other error"), execErr).Times(1)
	exists, err = runner.VMExists(ctx, mockConn, vmName)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "virsh command failed")

	// Test case 4: Empty VM name
	exists, err = runner.VMExists(ctx, mockConn, "")
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "vmName cannot be empty")

	// Test case 5: Nil connector
	exists, err = runner.VMExists(ctx, nil, vmName)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "connector cannot be nil")
}

func TestDefaultRunner_GetVMState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	vmName := "test-vm-state"

	// Test case 1: VM is running
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
		Return([]byte("running\n"), []byte{}, nil).Times(1)
	state, err := runner.GetVMState(ctx, mockConn, vmName)
	assert.NoError(t, err)
	assert.Equal(t, "running", state)

	// Test case 2: VM is shut off
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
		Return([]byte("shut off\n"), []byte{}, nil).Times(1)
	state, err = runner.GetVMState(ctx, mockConn, vmName)
	assert.NoError(t, err)
	assert.Equal(t, "shut off", state)

	// Test case 3: VM not found (virsh domstate errors)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("error: failed to get domain 'test-vm-state'"), &connector.CommandError{ExitCode: 1}).Times(1)
	state, err = runner.GetVMState(ctx, mockConn, vmName)
	assert.Error(t, err)
	assert.Empty(t, state)
	assert.Contains(t, err.Error(), "failed to get state")

	// Test case 4: Empty VM name
	_, err = runner.GetVMState(ctx, mockConn, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vmName cannot be empty")

	// Test case 5: Nil connector
	_, err = runner.GetVMState(ctx, nil, vmName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connector cannot be nil")
}

func TestDefaultRunner_StartVM(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	vmName := "test-start-vm"

	// Test case 1: VM is shut off, successfully starts
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("shut off\n"), []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh start %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.StartVM(ctx, mockConn, vmName)
	assert.NoError(t, err)

	// Test case 2: VM is already running
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("running\n"), []byte{}, nil).Times(1)
	// virsh start should not be called if already running
	err = runner.StartVM(ctx, mockConn, vmName)
	assert.NoError(t, err)

	// Test case 3: Failed to get VM state before starting
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte("state error"), fmt.Errorf("state exec error")).Times(1)
	err = runner.StartVM(ctx, mockConn, vmName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get current state")

	// Test case 4: VM is shut off, but virsh start command fails
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("shut off\n"), []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh start %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte("start error"), fmt.Errorf("start exec error")).Times(1)
	err = runner.StartVM(ctx, mockConn, vmName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start VM")
}

func TestDefaultRunner_DestroyVM(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    vmName := "test-destroy-vm"

    // Test case 1: VM is running, successfully destroys
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("running\n"), []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh destroy %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.DestroyVM(ctx, mockConn, vmName)
    assert.NoError(t, err)

    // Test case 2: VM is already shut off
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("shut off\n"), []byte{}, nil).Times(1)
    // virsh destroy should not be called if already shut off, or if it is, it should be idempotent
    // The current implementation calls destroy but expects it to succeed or indicate already off.
    err = runner.DestroyVM(ctx, mockConn, vmName) // destroy is not called if already shut off based on current code
    assert.NoError(t, err)

    // Test case 3: VM does not exist (domstate fails, destroy should be idempotent)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
        Return(nil, []byte("error: failed to get domain 'test-destroy-vm'"), &connector.CommandError{ExitCode: 1}).Times(1)
    // virsh destroy might or might not be called depending on error parsing. Current code path for "Domain not found" returns nil for DestroyVM.
    err = runner.DestroyVM(ctx, mockConn, vmName)
    assert.NoError(t, err)

    // Test case 4: virsh destroy command fails
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("running\n"), []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh destroy %s", shellEscape(vmName)), gomock.Any()).
        Return(nil, []byte("destroy error"), fmt.Errorf("destroy exec error")).Times(1)
    err = runner.DestroyVM(ctx, mockConn, vmName)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to destroy VM")

	// Test case 5: virsh destroy command fails but stderr indicates domain not running (idempotent)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("running\n"), []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh destroy %s", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("error: domain is not running"), &connector.CommandError{ExitCode:1}).Times(1)
	err = runner.DestroyVM(ctx, mockConn, vmName)
	assert.NoError(t, err)

}


func TestDefaultRunner_ListVMs(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner() // Assuming NewDefaultRunner() initializes your runner
    ctx := context.Background()

    vm1Name := "vm1"
    vm2Name := "vm2"

    // Mock for `virsh list --all --name`
    mockConn.EXPECT().Exec(ctx, "virsh list --all --name", gomock.Any()).
        Return([]byte(fmt.Sprintf("%s\n%s\n", vm1Name, vm2Name)), nil, nil).Times(1)

    // Mocks for GetVMState for vm1 and vm2
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vm1Name)), gomock.Any()).
        Return([]byte("running\n"), nil, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vm2Name)), gomock.Any()).
        Return([]byte("shut off\n"), nil, nil).Times(1)

    // Mocks for `virsh dominfo vm1`
    vm1DominfoOutput := `
Id:             1
Name:           vm1
UUID:           1111-1111-1111-1111
OS Type:        hvm
State:          running
CPU(s):         2
Max memory:     2048 KiB
Used memory:    1024 KiB
`
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s", shellEscape(vm1Name)), gomock.Any()).
        Return([]byte(vm1DominfoOutput), nil, nil).Times(1)

    // Mocks for `virsh dominfo vm2`
    vm2DominfoOutput := `
Id:             -
Name:           vm2
UUID:           2222-2222-2222-2222
OS Type:        hvm
State:          shut off
CPU(s):         4
Max memory:     4096000 KiB
Used memory:    0 KiB
`
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s", shellEscape(vm2Name)), gomock.Any()).
        Return([]byte(vm2DominfoOutput), nil, nil).Times(1)

    vms, err := runner.ListVMs(ctx, mockConn, true)
    assert.NoError(t, err)
    assert.Len(t, vms, 2)

    // VM1 assertions
    assert.Equal(t, "vm1", vms[0].Name)
    assert.Equal(t, "running", vms[0].State)
    assert.Equal(t, "1111-1111-1111-1111", vms[0].UUID)
    assert.Equal(t, 2, vms[0].CPUs)
    assert.Equal(t, uint(2), vms[0].Memory) // 2048 KiB / 1024 = 2 MiB

    // VM2 assertions
    assert.Equal(t, "vm2", vms[1].Name)
    assert.Equal(t, "shut off", vms[1].State)
    assert.Equal(t, "2222-2222-2222-2222", vms[1].UUID)
    assert.Equal(t, 4, vms[1].CPUs)
    assert.Equal(t, uint(4000), vms[1].Memory) // 4096000 KiB / 1024 = 4000 MiB
}
// Further tests for ShutdownVM, UndefineVM, CreateVMTemplate (basic disk part) can be added here following similar patterns.
// For CreateVMTemplate, testing the qemu-img call would be the main part for the current implementation.
// ShutdownVM test would be more complex due to its polling logic and conditional DestroyVM call.
// UndefineVM test would check different flags like --snapshots-metadata and --remove-all-storage.

// Example for testing the disk creation part of CreateVMTemplate
func TestDefaultRunner_CreateVMTemplate_DiskCreation(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    vmName := "template-vm"
    osVariant := "ubuntu22.04"
    diskPath := "/var/lib/libvirt/images/template-vm.qcow2"
    diskDir := "/var/lib/libvirt/images"
    diskSizeGB := uint(20)

    // Disk does not exist, mkdir and qemu-img create are called
    mockConn.EXPECT().Exists(ctx, mockConn, diskPath).Return(false, nil).Times(1)
    mockConn.EXPECT().Mkdirp(ctx, mockConn, diskDir, "0755", true).Return(nil).Times(1)
    expectedQemuCmd := fmt.Sprintf("qemu-img create -f qcow2 %s %dG", shellEscape(diskPath), diskSizeGB)
    mockConn.EXPECT().Exec(ctx, expectedQemuCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)

    // The function is expected to return an error because the virsh define part is not implemented
    err := runner.CreateVMTemplate(ctx, mockConn, vmName, osVariant, 2048, 2, diskPath, diskSizeGB, "default", "vnc", "")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not fully implemented")

    // Disk exists, no mkdir or qemu-img
    mockConn.EXPECT().Exists(ctx, mockConn, diskPath).Return(true, nil).Times(1)
    err = runner.CreateVMTemplate(ctx, mockConn, vmName, osVariant, 2048, 2, diskPath, diskSizeGB, "default", "vnc", "")
    assert.Error(t, err) // Still errors due to unimplemented define part
    assert.Contains(t, err.Error(), "not fully implemented")
}


func TestDefaultRunner_StoragePoolExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	poolName := "test-pool"

	// Test case 1: Pool exists
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-info %s > /dev/null 2>&1", shellEscape(poolName)), gomock.Any()).
		Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.StoragePoolExists(ctx, mockConn, poolName)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test case 2: Pool does not exist (command fails with non-zero exit)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-info %s > /dev/null 2>&1", shellEscape(poolName)), gomock.Any()).
		Return(nil, []byte("error: failed to get pool 'test-pool'"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.StoragePoolExists(ctx, mockConn, poolName)
	assert.NoError(t, err) // Expects (false, nil) for not found
	assert.False(t, exists)

	// Test case 3: virsh command execution error
	execErr := fmt.Errorf("virsh pool-info command failed")
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-info %s > /dev/null 2>&1", shellEscape(poolName)), gomock.Any()).
		Return(nil, []byte("some other error"), execErr).Times(1)
	exists, err = runner.StoragePoolExists(ctx, mockConn, poolName)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "virsh pool-info command failed")
}

func TestDefaultRunner_VolumeExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	poolName := "test-pool"
	volName := "test-vol"

	// Test case 1: Volume exists
	cmd := fmt.Sprintf("virsh vol-info --pool %s %s > /dev/null 2>&1", shellEscape(poolName), shellEscape(volName))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.VolumeExists(ctx, mockConn, poolName, volName)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test case 2: Volume does not exist
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("error: Failed to get volume 'test-vol'"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.VolumeExists(ctx, mockConn, poolName, volName)
	assert.NoError(t, err) // Expects (false, nil)
	assert.False(t, exists)

	// Test case 3: virsh command execution error
	execErr := fmt.Errorf("virsh vol-info command failed")
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("some other error"), execErr).Times(1)
	exists, err = runner.VolumeExists(ctx, mockConn, poolName, volName)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "virsh vol-info command failed")
}


func TestDefaultRunner_CreateStoragePool_DirType(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    poolName := "my-dir-pool"
    targetPath := "/data/vms/my-dir-pool"
    poolType := "dir"

    // Mock Mkdirp
    mockConn.EXPECT().Mkdirp(ctx, mockConn, targetPath, "0755", true).Return(nil).Times(1)

    // Mock pool-define-as
    defineCmd := fmt.Sprintf("virsh pool-define-as %s dir --target %s", shellEscape(poolName), shellEscape(targetPath))
    mockConn.EXPECT().Exec(ctx, defineCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)

    // Mock pool-build
    buildCmd := fmt.Sprintf("virsh pool-build %s", shellEscape(poolName))
    mockConn.EXPECT().Exec(ctx, buildCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)

    // Mock pool-start
    startCmd := fmt.Sprintf("virsh pool-start %s", shellEscape(poolName))
    mockConn.EXPECT().Exec(ctx, startCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)

    // Mock pool-autostart
    autostartCmd := fmt.Sprintf("virsh pool-autostart %s", shellEscape(poolName))
    mockConn.EXPECT().Exec(ctx, autostartCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)

    err := runner.CreateStoragePool(ctx, mockConn, poolName, poolType, targetPath)
    assert.NoError(t, err)
}


func TestDefaultRunner_DeleteStoragePool(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    poolName := "pool-to-delete"

    // Test Case 1: Successful deletion (pool active, then destroyed and undefined)
    destroyCmd := fmt.Sprintf("virsh pool-destroy %s", shellEscape(poolName))
    undefineCmd := fmt.Sprintf("virsh pool-undefine %s", shellEscape(poolName))

    mockConn.EXPECT().Exec(ctx, destroyCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, undefineCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)

    err := runner.DeleteStoragePool(ctx, mockConn, poolName)
    assert.NoError(t, err)

    // Test Case 2: Pool already not active (destroy reports "not active", undefine succeeds)
    mockConn.EXPECT().Exec(ctx, destroyCmd, gomock.Any()).
        Return(nil, []byte("error: Pool by this name is not active"), &connector.CommandError{ExitCode: 1}).Times(1) // Assuming error for not active
    mockConn.EXPECT().Exec(ctx, undefineCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err = runner.DeleteStoragePool(ctx, mockConn, poolName)
    assert.NoError(t, err)


    // Test Case 3: Pool not found (destroy reports "not found", undefine also reports "not found")
     mockConn.EXPECT().Exec(ctx, destroyCmd, gomock.Any()).
        Return(nil, []byte("error: failed to get pool 'pool-to-delete'"), &connector.CommandError{ExitCode: 1}).Times(1)
    // Undefine might not be called if destroy indicates not found, or it might also report not found.
    // Depending on implementation, if destroy says "not found", undefine might be skipped or also say "not found".
    // For this test, let's assume undefine is still called and also says not found.
    mockConn.EXPECT().Exec(ctx, undefineCmd, gomock.Any()).
        Return(nil, []byte("error: failed to get pool 'pool-to-delete'"), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.DeleteStoragePool(ctx, mockConn, poolName)
    assert.NoError(t, err) // Idempotent: already deleted or never existed.
}

func TestDefaultRunner_RefreshStoragePool(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    poolName := "pool-to-refresh"

    // Test Case 1: Successful refresh
    refreshCmd := fmt.Sprintf("virsh pool-refresh %s", shellEscape(poolName))
    mockConn.EXPECT().Exec(ctx, refreshCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.RefreshStoragePool(ctx, mockConn, poolName)
    assert.NoError(t, err)

    // Test Case 2: Refresh command fails
    mockConn.EXPECT().Exec(ctx, refreshCmd, gomock.Any()).
        Return(nil, []byte("error refreshing pool"), fmt.Errorf("exec error")).Times(1)
    err = runner.RefreshStoragePool(ctx, mockConn, poolName)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to refresh storage pool")
}

func TestDefaultRunner_ImportVMTemplate(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()
    vmName := "imported-vm"
    filePath := "/tmp/vm_template.xml"

    // Test Case 1: Successful import
    defineCmd := fmt.Sprintf("virsh define %s", shellEscape(filePath))
    mockConn.EXPECT().Exec(ctx, defineCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.ImportVMTemplate(ctx, mockConn, vmName, filePath)
    assert.NoError(t, err)

    // Test Case 2: Define command fails
    mockConn.EXPECT().Exec(ctx, defineCmd, gomock.Any()).
        Return(nil, []byte("error defining VM"), fmt.Errorf("exec error")).Times(1)
    err = runner.ImportVMTemplate(ctx, mockConn, vmName, filePath)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to define VM")
}

func TestDefaultRunner_CreateVM_Basic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	vmName := "test-vm-create"
	memoryMB := uint(1024)
	vcpus := uint(2)
	diskPaths := []string{"/var/lib/libvirt/images/test-vm-create.qcow2"}
	networkInterfaces := []VMNetworkInterface{{Type: "network", Source: "default", Model: "virtio"}}

	// Mock VMExists -> false
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("error: failed to get domain 'test-vm-create'"), &connector.CommandError{ExitCode: 1}).Times(1)

	// Mock WriteFile for the temporary XML
	var capturedXMLBytes []byte
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), gomock.Contains(fmt.Sprintf("/tmp/kubexm-vmdef-%s-", vmName)), "0600", true).
		DoAndReturn(func(_ context.Context, _ connector.Connector, content []byte, path string, _ string, _ bool) error {
			capturedXMLBytes = content // Capture the XML
			// fmt.Printf("Captured XML for CreateVM test:\n%s\n", string(content)) // For debugging test
			assert.Contains(t, string(content), fmt.Sprintf("<name>%s</name>", vmName))
			assert.Contains(t, string(content), fmt.Sprintf("<memory unit='KiB'>%d</memory>", memoryMB*1024))
			assert.Contains(t, string(content), fmt.Sprintf("<vcpu placement='static'>%d</vcpu>", vcpus))
			assert.Contains(t, string(content), fmt.Sprintf("<source file='%s'/>", diskPaths[0]))
			assert.Contains(t, string(content), fmt.Sprintf("<source network='%s'/>", networkInterfaces[0].Source))
			return nil
		}).Times(1)

	// Mock virsh define
	defineCmdMatcher := gomock. दट(func(x interface{}) bool {
		cmdStr, ok := x.(string)
		if !ok { return false }
		return strings.HasPrefix(cmdStr, "virsh define /tmp/kubexm-vmdef-")
	})
	mockConn.EXPECT().Exec(ctx, defineCmdMatcher, gomock.Any()).Return(nil, []byte{}, nil).Times(1)

	// Mock Remove (for temp XML file)
	removeCmdMatcher := gomock. दट(func(x interface{}) bool {
		cmdStr, ok := x.(string)
		if !ok { return false }
		// This is a bit tricky as Remove uses a direct command.
		// We can check if it's trying to remove the temp file path.
		return strings.HasPrefix(cmdStr, "rm -rf /tmp/kubexm-vmdef-") || strings.HasPrefix(cmdStr, "rm -f /tmp/kubexm-vmdef-")
	})
	mockConn.EXPECT().Run(ctx, mockConn, removeCmdMatcher, true).Return("", nil).Times(1)


	// Mock StartVM sequence (domstate then start)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("shut off\n"), []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh start %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)


	err := runner.CreateVM(ctx, mockConn, vmName, memoryMB, vcpus, "rhel8.6", diskPaths, networkInterfaces, "vnc", "", nil, nil)
	assert.NoError(t, err)
}

func TestDefaultRunner_CloneVolume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	poolName := "default-pool"
	origVolName := "base-image.qcow2"
	newVolName := "cloned-vm-disk.qcow2"

	// Test Case 1: Successful clone, no resize
	cloneCmd := fmt.Sprintf("virsh vol-clone --pool %s %s %s", shellEscape(poolName), shellEscape(origVolName), shellEscape(newVolName))
	mockConn.EXPECT().Exec(ctx, cloneCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CloneVolume(ctx, mockConn, poolName, origVolName, newVolName, 0, "qcow2") // newSizeGB = 0 means no resize
	assert.NoError(t, err)

	// Test Case 2: Successful clone, with resize
	newSizeGB := uint(30)
	resizeCmd := fmt.Sprintf("virsh vol-resize --pool %s %s %sG", shellEscape(poolName), shellEscape(newVolName), fmt.Sprint(newSizeGB))
	gomock.InOrder(
		mockConn.EXPECT().Exec(ctx, cloneCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1),
		mockConn.EXPECT().Exec(ctx, resizeCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1),
	)
	err = runner.CloneVolume(ctx, mockConn, poolName, origVolName, newVolName, newSizeGB, "qcow2")
	assert.NoError(t, err)

	// Test Case 3: Clone fails
	mockConn.EXPECT().Exec(ctx, cloneCmd, gomock.Any()).Return(nil, []byte("clone error"), fmt.Errorf("exec clone error")).Times(1)
	err = runner.CloneVolume(ctx, mockConn, poolName, origVolName, newVolName, 0, "qcow2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clone volume")

	// Test Case 4: Clone succeeds, but resize fails
	mockConn.EXPECT().Exec(ctx, cloneCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, resizeCmd, gomock.Any()).Return(nil, []byte("resize error"), fmt.Errorf("exec resize error")).Times(1)
	err = runner.CloneVolume(ctx, mockConn, poolName, origVolName, newVolName, newSizeGB, "qcow2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cloned successfully, but failed to resize")
}

func TestDefaultRunner_ResizeVolume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	poolName := "images"
	volName := "vm-disk1.img"
	newSizeGB := uint(50)

	// Test Case 1: Successful resize
	expectedCmd := fmt.Sprintf("virsh vol-resize --pool %s %s %sG", shellEscape(poolName), shellEscape(volName), fmt.Sprint(newSizeGB))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.ResizeVolume(ctx, mockConn, poolName, volName, newSizeGB)
	assert.NoError(t, err)

	// Test Case 2: Resize command fails
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte("resize error"), fmt.Errorf("exec resize error")).Times(1)
	err = runner.ResizeVolume(ctx, mockConn, poolName, volName, newSizeGB)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resize volume")

	// Test Case 3: Invalid arguments (e.g. size 0)
	err = runner.ResizeVolume(ctx, mockConn, poolName, volName, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-zero newSizeGB")
}

func TestDefaultRunner_DeleteVolume(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()
	poolName := "default"
	volName := "test-vol-to-delete.qcow2"

	// Test Case 1: Successful delete
	cmd := fmt.Sprintf("virsh vol-delete --pool %s %s", shellEscape(poolName), shellEscape(volName))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.DeleteVolume(ctx, mockConn, poolName, volName)
	assert.NoError(t, err)

	// Test Case 2: Volume not found (idempotency)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("error: Failed to get volume 'test-vol-to-delete.qcow2'"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.DeleteVolume(ctx, mockConn, poolName, volName)
	assert.NoError(t, err)

	// Test Case 3: Other command execution error
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).
		Return(nil, []byte("some other delete error"), fmt.Errorf("exec delete error")).Times(1)
	err = runner.DeleteVolume(ctx, mockConn, poolName, volName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete volume")
}

func TestDefaultRunner_CreateVolume(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    poolName := "vms"
    volName := "new-disk.img"
    sizeGB := uint(15)
    format := "raw"

    // Test Case 1: Simple volume creation
    expectedCmdSimple := fmt.Sprintf("virsh vol-create-as %s %s %dG --format %s",
        shellEscape(poolName), shellEscape(volName), sizeGB, shellEscape(format))
    mockConn.EXPECT().Exec(ctx, expectedCmdSimple, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.CreateVolume(ctx, mockConn, poolName, volName, sizeGB, format, "", "")
    assert.NoError(t, err)

    // Test Case 2: Creation with backing store
    backingVolName := "base.qcow2"
    backingVolFormat := "qcow2"
    expectedCmdBacking := fmt.Sprintf("virsh vol-create-as %s %s %dG --format %s --backing-vol %s --backing-vol-format %s",
        shellEscape(poolName), shellEscape(volName), sizeGB, shellEscape(format), shellEscape(backingVolName), shellEscape(backingVolFormat))
    mockConn.EXPECT().Exec(ctx, expectedCmdBacking, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err = runner.CreateVolume(ctx, mockConn, poolName, volName, sizeGB, format, backingVolName, backingVolFormat)
    assert.NoError(t, err)

    // Test Case 3: Volume already exists (idempotency)
    mockConn.EXPECT().Exec(ctx, expectedCmdSimple, gomock.Any()).
        Return(nil, []byte(fmt.Sprintf("error: operation failed: storage volume '%s' already exists", volName)), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.CreateVolume(ctx, mockConn, poolName, volName, sizeGB, format, "", "")
    assert.NoError(t, err)

    // Test Case 4: Missing backingVolFormat when backingVolName is provided
    err = runner.CreateVolume(ctx, mockConn, poolName, volName, sizeGB, format, backingVolName, "")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "backingVolFormat is required")
}

func TestDefaultRunner_AttachDisk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	vmName := "my-vm"
	diskPath := "/vms/disks/newdisk.qcow2"
	target := "vdc"
	driverType := "qcow2"

	// Test Case 1: Successful attach
	expectedCmd := fmt.Sprintf("virsh attach-disk %s %s %s --driver qemu --subdriver %s --config --live",
		shellEscape(vmName), shellEscape(diskPath), shellEscape(target), shellEscape(driverType))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.AttachDisk(ctx, mockConn, vmName, diskPath, target, "", driverType) // diskType (file/block) omitted for this cmd
	assert.NoError(t, err)

	// Test Case 2: Attach fails
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("error attaching disk"), fmt.Errorf("exec attach error")).Times(1)
	err = runner.AttachDisk(ctx, mockConn, vmName, diskPath, target, "", driverType)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to attach disk")
}

func TestDefaultRunner_SetVMMemory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	vmName := "mem-test-vm"
	memoryMB := uint(2048)
	memoryKiB := memoryMB * 1024

	// Test Case 1: Set memory live and config
	expectedCmdLiveConfig := fmt.Sprintf("virsh setmem %s %dK --live --config", shellEscape(vmName), memoryKiB)
	mockConn.EXPECT().Exec(ctx, expectedCmdLiveConfig, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.SetVMMemory(ctx, mockConn, vmName, memoryMB, true)
	assert.NoError(t, err)

	// Test Case 2: Set memory config only
	expectedCmdConfigOnly := fmt.Sprintf("virsh setmem %s %dK --config", shellEscape(vmName), memoryKiB)
	mockConn.EXPECT().Exec(ctx, expectedCmdConfigOnly, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.SetVMMemory(ctx, mockConn, vmName, memoryMB, false)
	assert.NoError(t, err)

	// Test Case 3: Command fails
	mockConn.EXPECT().Exec(ctx, expectedCmdLiveConfig, gomock.Any()).
		Return(nil, []byte("setmem error"), fmt.Errorf("exec setmem error")).Times(1)
	err = runner.SetVMMemory(ctx, mockConn, vmName, memoryMB, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set memory")
}

func TestDefaultRunner_SetVMCPUs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	vmName := "cpu-test-vm"
	vcpus := uint(4)

	// Test Case 1: Set vCPUs live and config
	expectedCmdLiveConfig := fmt.Sprintf("virsh setvcpus %s %d --live --config", shellEscape(vmName), vcpus)
	mockConn.EXPECT().Exec(ctx, expectedCmdLiveConfig, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.SetVMCPUs(ctx, mockConn, vmName, vcpus, true)
	assert.NoError(t, err)

	// Test Case 2: Set vCPUs config only
	expectedCmdConfigOnly := fmt.Sprintf("virsh setvcpus %s %d --config", shellEscape(vmName), vcpus)
	mockConn.EXPECT().Exec(ctx, expectedCmdConfigOnly, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err = runner.SetVMCPUs(ctx, mockConn, vmName, vcpus, false)
	assert.NoError(t, err)

	// Test Case 3: Command fails
	mockConn.EXPECT().Exec(ctx, expectedCmdLiveConfig, gomock.Any()).
		Return(nil, []byte("setvcpus error"), fmt.Errorf("exec setvcpus error")).Times(1)
	err = runner.SetVMCPUs(ctx, mockConn, vmName, vcpus, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set vCPUs")
}


func TestDefaultRunner_DetachDisk(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl)
	runner := NewDefaultRunner()
	ctx := context.Background()

	vmName := "another-vm"
	target := "vdc"

	// Test Case 1: Successful detach
	expectedCmd := fmt.Sprintf("virsh detach-disk %s %s --config --live", shellEscape(vmName), shellEscape(target))
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.DetachDisk(ctx, mockConn, vmName, target)
	assert.NoError(t, err)

	// Test Case 2: Detach fails (e.g., disk not found) - should be idempotent based on current impl
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("error: No disk found for target 'vdc'"), &connector.CommandError{ExitCode: 1}).Times(1)
	err = runner.DetachDisk(ctx, mockConn, vmName, target)
	assert.NoError(t, err) // Idempotent

	// Test Case 3: Detach fails (other error)
	mockConn.EXPECT().Exec(ctx, expectedCmd, gomock.Any()).
		Return(nil, []byte("some other detach error"), fmt.Errorf("exec detach error")).Times(1)
	err = runner.DetachDisk(ctx, mockConn, vmName, target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to detach disk")
}


func TestDefaultRunner_CreateCloudInitISO(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    vmName := "my-vm-iso"
    isoDestPath := "/opt/isos/my-vm-iso-cloud-init.iso"
    isoDir := "/opt/isos"
    userData := "user-data-content"
    metaData := "meta-data-content"

    // Using gomock.Any() for tmpDirPath as it's time-dependent and hard to predict exactly.
    // We can check for parts of the command.

    // Mock Mkdirp for tmpDirPath and isoDir
    mockConn.EXPECT().Mkdirp(ctx, mockConn, gomock.AssignableToTypeOf("string"), "0700", true).
        DoAndReturn(func(_ context.Context, _ connector.Connector, path string, _ string, _ bool) error {
            assert.Contains(t, path, "/tmp/cloud-init-tmp-"+vmName)
            return nil
        }).Times(1)
    mockConn.EXPECT().Mkdirp(ctx, mockConn, isoDir, "0755", true).Return(nil).Times(1)


    // Mock WriteFile for user-data and meta-data
    mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(userData), gomock.Contains("user-data"), "0644", true).Return(nil).Times(1)
    mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(metaData), gomock.Contains("meta-data"), "0644", true).Return(nil).Times(1)

    // Mock LookPath for genisoimage (assume it exists)
    mockConn.EXPECT().LookPath(ctx, mockConn, "genisoimage").Return("/usr/bin/genisoimage", nil).Times(1)

    // Mock Exec for genisoimage
    // Example: genisoimage -o /opt/isos/my-vm-iso-cloud-init.iso -V cidata -r -J /tmp/cloud-init-tmp-my-vm-iso-167...
    mockConn.EXPECT().Exec(ctx, gomock. दट(func(cmd string) bool {
        return strings.HasPrefix(cmd, "genisoimage -o "+shellEscape(isoDestPath)) &&
               strings.Contains(cmd, "-V cidata -r -J") &&
               strings.Contains(cmd, "/tmp/cloud-init-tmp-"+vmName)
    }), gomock.Any()).Return(nil, []byte{}, nil).Times(1)


    // Mock Remove for cleanup
    mockConn.EXPECT().Remove(ctx, mockConn, gomock.Contains("/tmp/cloud-init-tmp-"+vmName), true).Return(nil).Times(1)


    err := runner.CreateCloudInitISO(ctx, mockConn, vmName, isoDestPath, userData, metaData, "")
    assert.NoError(t, err)

    // Test case with mkisofs fallback
    mockConn.EXPECT().Mkdirp(ctx, mockConn, gomock.AssignableToTypeOf("string"), "0700", true).Return(nil).Times(1)
    mockConn.EXPECT().Mkdirp(ctx, mockConn, isoDir, "0755", true).Return(nil).Times(1)
    mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(userData), gomock.Contains("user-data"), "0644", true).Return(nil).Times(1)
    mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(metaData), gomock.Contains("meta-data"), "0644", true).Return(nil).Times(1)

    mockConn.EXPECT().LookPath(ctx, mockConn, "genisoimage").Return("", fmt.Errorf("not found")).Times(1)
    mockConn.EXPECT().LookPath(ctx, mockConn, "mkisofs").Return("/usr/bin/mkisofs", nil).Times(1)

    mockConn.EXPECT().Exec(ctx, gomock. दट(func(cmd string) bool {
        return strings.HasPrefix(cmd, "mkisofs -o "+shellEscape(isoDestPath))
    }), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    mockConn.EXPECT().Remove(ctx, mockConn, gomock.Contains("/tmp/cloud-init-tmp-"+vmName), true).Return(nil).Times(1)

    err = runner.CreateCloudInitISO(ctx, mockConn, vmName, isoDestPath, userData, metaData, "")
    assert.NoError(t, err)

}
