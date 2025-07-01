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

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.VMExists(ctx, mockConn, vmName)
	assert.NoError(t, err)
	assert.True(t, exists)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("error: failed to get domain 'test-vm'"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.VMExists(ctx, mockConn, vmName)
	assert.NoError(t, err)
	assert.False(t, exists)

	execErr := fmt.Errorf("virsh command failed")
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("some other error"), execErr).Times(1)
	exists, err = runner.VMExists(ctx, mockConn, vmName)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "virsh command failed")

	exists, err = runner.VMExists(ctx, mockConn, "")
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "vmName cannot be empty")

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

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
		Return([]byte("running\n"), []byte{}, nil).Times(1)
	state, err := runner.GetVMState(ctx, mockConn, vmName)
	assert.NoError(t, err)
	assert.Equal(t, "running", state)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
		Return([]byte("shut off\n"), []byte{}, nil).Times(1)
	state, err = runner.GetVMState(ctx, mockConn, vmName)
	assert.NoError(t, err)
	assert.Equal(t, "shut off", state)

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
		Return(nil, []byte("error: failed to get domain 'test-vm-state'"), &connector.CommandError{ExitCode: 1}).Times(1)
	state, err = runner.GetVMState(ctx, mockConn, vmName)
	assert.Error(t, err)
	assert.Empty(t, state)
	assert.Contains(t, err.Error(), "failed to get state")

	_, err = runner.GetVMState(ctx, mockConn, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vmName cannot be empty")

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

	// VM is shut off, successfully starts
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("shut off\n"), []byte{}, nil).Times(1)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh start %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.StartVM(ctx, mockConn, vmName)
	assert.NoError(t, err)

	// VM is already running
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("running\n"), []byte{}, nil).Times(1)
	err = runner.StartVM(ctx, mockConn, vmName)
	assert.NoError(t, err)

	// Failed to get VM state
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte("state error"), fmt.Errorf("state exec error")).Times(1)
	err = runner.StartVM(ctx, mockConn, vmName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get current state")

	// VM is shut off, but virsh start command fails
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

    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("running\n"), []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh destroy %s", shellEscape(vmName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.DestroyVM(ctx, mockConn, vmName)
    assert.NoError(t, err)

    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("shut off\n"), []byte{}, nil).Times(1)
    err = runner.DestroyVM(ctx, mockConn, vmName)
    assert.NoError(t, err)

    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).
        Return(nil, []byte("error: failed to get domain 'test-destroy-vm'"), &connector.CommandError{ExitCode: 1}).Times(1)
    err = runner.DestroyVM(ctx, mockConn, vmName)
    assert.NoError(t, err)

    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vmName)), gomock.Any()).Return([]byte("running\n"), []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh destroy %s", shellEscape(vmName)), gomock.Any()).
        Return(nil, []byte("destroy error"), fmt.Errorf("destroy exec error")).Times(1)
    err = runner.DestroyVM(ctx, mockConn, vmName)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to destroy VM")

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
    runner := NewDefaultRunner()
    ctx := context.Background()

    vm1Name, vm2Name := "vm1", "vm2"
    mockConn.EXPECT().Exec(ctx, "virsh list --all --name", gomock.Any()).
        Return([]byte(fmt.Sprintf("%s\n%s\n", vm1Name, vm2Name)), nil, nil).Times(1)

    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vm1Name)), gomock.Any()).
        Return([]byte("running\n"), nil, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh domstate %s", shellEscape(vm2Name)), gomock.Any()).
        Return([]byte("shut off\n"), nil, nil).Times(1)

    vm1Dominfo := "Id: 1\nName: vm1\nUUID: 111\nCPU(s): 2\nMax memory: 2048 KiB\nState: running"
    vm2Dominfo := "Id: -\nName: vm2\nUUID: 222\nCPU(s): 4\nMemory: 4096000 KiB\nState: shut off" // Using "Memory" as a fallback key
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s", shellEscape(vm1Name)), gomock.Any()).
        Return([]byte(vm1Dominfo), nil, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh dominfo %s", shellEscape(vm2Name)), gomock.Any()).
        Return([]byte(vm2Dominfo), nil, nil).Times(1)

    vms, err := runner.ListVMs(ctx, mockConn, true)
    assert.NoError(t, err)
    assert.Len(t, vms, 2)
    assert.Equal(t, "vm1", vms[0].Name); assert.Equal(t, "running", vms[0].State); assert.Equal(t, "111", vms[0].UUID); assert.Equal(t, 2, vms[0].CPUs); assert.Equal(t, uint(2), vms[0].Memory)
    assert.Equal(t, "vm2", vms[1].Name); assert.Equal(t, "shut off", vms[1].State); assert.Equal(t, "222", vms[1].UUID); assert.Equal(t, 4, vms[1].CPUs); assert.Equal(t, uint(4000), vms[1].Memory)
}

func TestDefaultRunner_CreateVMTemplate_DiskCreation(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl)
    runner := NewDefaultRunner()
    ctx := context.Background()

    vmName, osVariant, diskPath, diskDir, diskSizeGB := "tvm", "u22", "/d/tvm.q", "/d", uint(20)
    mockConn.EXPECT().Exists(ctx, mockConn, diskPath).Return(false, nil).Times(1)
    mockConn.EXPECT().Mkdirp(ctx, mockConn, diskDir, "0755", true).Return(nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("qemu-img create -f qcow2 %s %dG", shellEscape(diskPath), diskSizeGB), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.CreateVMTemplate(ctx, mockConn, vmName, osVariant, 2048, 2, diskPath, diskSizeGB, "default", "vnc", "")
    assert.Error(t, err); assert.Contains(t, err.Error(), "not fully implemented")

    mockConn.EXPECT().Exists(ctx, mockConn, diskPath).Return(true, nil).Times(1)
    err = runner.CreateVMTemplate(ctx, mockConn, vmName, osVariant, 2048, 2, diskPath, diskSizeGB, "default", "vnc", "")
    assert.Error(t, err); assert.Contains(t, err.Error(), "not fully implemented")
}

func TestDefaultRunner_StoragePoolExists(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); poolName := "test-pool"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-info %s > /dev/null 2>&1", shellEscape(poolName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.StoragePoolExists(ctx, mockConn, poolName); assert.NoError(t, err); assert.True(t, exists)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-info %s > /dev/null 2>&1", shellEscape(poolName)), gomock.Any()).Return(nil, []byte("err"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.StoragePoolExists(ctx, mockConn, poolName); assert.NoError(t, err); assert.False(t, exists)
}

func TestDefaultRunner_VolumeExists(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); poolName, volName := "tp", "tv"
	cmd := fmt.Sprintf("virsh vol-info --pool %s %s > /dev/null 2>&1", shellEscape(poolName), shellEscape(volName))
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	exists, err := runner.VolumeExists(ctx, mockConn, poolName, volName); assert.NoError(t, err); assert.True(t, exists)
	mockConn.EXPECT().Exec(ctx, cmd, gomock.Any()).Return(nil, []byte("err"), &connector.CommandError{ExitCode: 1}).Times(1)
	exists, err = runner.VolumeExists(ctx, mockConn, poolName, volName); assert.NoError(t, err); assert.False(t, exists)
}

func TestDefaultRunner_CreateStoragePool_DirType(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    poolName, targetPath, poolType := "mdp", "/d/v/mdp", "dir"

	mockConn.EXPECT().StoragePoolExists(ctx, mockConn, poolName).Return(false, nil).Times(1)
    mockConn.EXPECT().Mkdirp(ctx, mockConn, targetPath, "0755", true).Return(nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-define-as %s dir --target %s", shellEscape(poolName), shellEscape(targetPath)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-build %s", shellEscape(poolName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-start %s", shellEscape(poolName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-autostart %s", shellEscape(poolName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.CreateStoragePool(ctx, mockConn, poolName, poolType, targetPath)
    assert.NoError(t, err)

	// Test idempotency: pool already exists
	mockConn.EXPECT().StoragePoolExists(ctx, mockConn, poolName).Return(true, nil).Times(1)
	err = runner.CreateStoragePool(ctx, mockConn, poolName, poolType, targetPath)
	assert.NoError(t, err) // Should do nothing and return no error
}

func TestDefaultRunner_DeleteStoragePool(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); poolName := "ptd"
    destroyCmd, undefineCmd := fmt.Sprintf("virsh pool-destroy %s", shellEscape(poolName)), fmt.Sprintf("virsh pool-undefine %s", shellEscape(poolName))
    mockConn.EXPECT().Exec(ctx, destroyCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, undefineCmd, gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.DeleteStoragePool(ctx, mockConn, poolName); assert.NoError(t, err)
}

func TestDefaultRunner_RefreshStoragePool(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); poolName := "ptr"
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh pool-refresh %s", shellEscape(poolName)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.RefreshStoragePool(ctx, mockConn, poolName); assert.NoError(t, err)
}

func TestDefaultRunner_ImportVMTemplate(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); vmName, filePath := "ivm", "/t/vm.xml"
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh define %s", shellEscape(filePath)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.ImportVMTemplate(ctx, mockConn, vmName, filePath); assert.NoError(t, err)
}

func TestDefaultRunner_CreateVM_Basic(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	vmName, memMB, vcpus := "tvc", uint(1024), uint(2)
	disks := []string{"/d/tvc.q"}; nets := []VMNetworkInterface{{Type: "network", Source: "default"}}

	mockConn.EXPECT().VMExists(ctx, mockConn, vmName).Return(false, nil).Times(1)
	mockConn.EXPECT().WriteFile(ctx, mockConn, gomock.Any(), gomock.Contains(fmt.Sprintf("/tmp/kubexm-vmdef-%s-", vmName)), "0600", true).Return(nil).Times(1)
	mockConn.EXPECT().Exec(ctx, gomock.Contains("virsh define /tmp/kubexm-vmdef-"), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	mockConn.EXPECT().Remove(ctx, mockConn, gomock.Contains(fmt.Sprintf("/tmp/kubexm-vmdef-%s-", vmName)), true).Return(nil).Times(1)
	mockConn.EXPECT().StartVM(ctx, mockConn, vmName).Return(nil).Times(1)
	err := runner.CreateVM(ctx, mockConn, vmName, memMB, vcpus, "rhel9", disks, nets, "vnc", "", nil, nil)
	assert.NoError(t, err)
}

func TestDefaultRunner_CloneVolume(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	pool, orig, newVol := "dp", "base.q", "clone.q"

	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh vol-clone --pool %s %s %s", shellEscape(pool), shellEscape(orig), shellEscape(newVol)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.CloneVolume(ctx, mockConn, pool, orig, newVol, 0, "qcow2"); assert.NoError(t, err)
}

func TestDefaultRunner_ResizeVolume(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	pool, vol, sizeGB := "imgs", "disk.q", uint(50)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh vol-resize --pool %s %s %dG", shellEscape(pool), shellEscape(vol), sizeGB), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.ResizeVolume(ctx, mockConn, pool, vol, sizeGB); assert.NoError(t, err)
}

func TestDefaultRunner_DeleteVolume(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background(); pool, vol := "def", "del.q"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh vol-delete --pool %s %s", shellEscape(pool), shellEscape(vol)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.DeleteVolume(ctx, mockConn, pool, vol); assert.NoError(t, err)
}

func TestDefaultRunner_CreateVolume(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    pool, vol, sizeGB, format := "vms_pool", "new.img", uint(10), "raw"

	mockConn.EXPECT().VolumeExists(ctx, mockConn, pool, vol).Return(false, nil).Times(1)
    mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh vol-create-as %s %s %dG --format %s", shellEscape(pool), shellEscape(vol), sizeGB, shellEscape(format)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    err := runner.CreateVolume(ctx, mockConn, pool, vol, sizeGB, format, "", ""); assert.NoError(t, err)

	// Test idempotency
	mockConn.EXPECT().VolumeExists(ctx, mockConn, pool, vol).Return(true, nil).Times(1)
	err = runner.CreateVolume(ctx, mockConn, pool, vol, sizeGB, format, "", ""); assert.NoError(t, err)
}

func TestDefaultRunner_AttachDisk(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	vm, disk, target, driver := "avm", "/d/new.q", "vdc", "qcow2"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh attach-disk %s %s %s --driver qemu --subdriver %s --config --live", shellEscape(vm), shellEscape(disk), shellEscape(target), shellEscape(driver)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.AttachDisk(ctx, mockConn, vm, disk, target, "", driver); assert.NoError(t, err)
}

func TestDefaultRunner_SetVMMemory(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	vm, memMB := "memvm", uint(2048)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh setmem %s %dK --live --config", shellEscape(vm), memMB*1024), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.SetVMMemory(ctx, mockConn, vm, memMB, true); assert.NoError(t, err)
}

func TestDefaultRunner_SetVMCPUs(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	vm, cpus := "cpuvm", uint(4)
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh setvcpus %s %d --live --config", shellEscape(vm), cpus), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.SetVMCPUs(ctx, mockConn, vm, cpus, true); assert.NoError(t, err)
}

func TestDefaultRunner_DetachDisk(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	vm, target := "dvm", "vdc"
	mockConn.EXPECT().Exec(ctx, fmt.Sprintf("virsh detach-disk %s %s --config --live", shellEscape(vm), shellEscape(target)), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
	err := runner.DetachDisk(ctx, mockConn, vm, target); assert.NoError(t, err)
}

func TestDefaultRunner_CreateCloudInitISO(t *testing.T) {
    ctrl := gomock.NewController(t); defer ctrl.Finish()
    mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
    vm, isoPath, uData, mData := "iso-vm", "/opt/iso.iso", "ud", "md"

    mockConn.EXPECT().Mkdirp(ctx, mockConn, gomock.Contains("/tmp/kubexm-cloud-init-tmp-"), "0700", true).Return(nil).Times(1)
    mockConn.EXPECT().Mkdirp(ctx, mockConn, "/opt", "0755", true).Return(nil).Times(1) // Dir of isoPath
    mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(uData), gomock.Contains("user-data"), "0644", true).Return(nil).Times(1)
    mockConn.EXPECT().WriteFile(ctx, mockConn, []byte(mData), gomock.Contains("meta-data"), "0644", true).Return(nil).Times(1)
    mockConn.EXPECT().LookPath(ctx, mockConn, "genisoimage").Return("/bin/genisoimage", nil).Times(1)
    mockConn.EXPECT().Exec(ctx, gomock.Contains(fmt.Sprintf("genisoimage -o %s", shellEscape(isoPath))), gomock.Any()).Return(nil, []byte{}, nil).Times(1)
    mockConn.EXPECT().Remove(ctx, mockConn, gomock.Contains("/tmp/kubexm-cloud-init-tmp-"), true).Return(nil).Times(1)
    err := runner.CreateCloudInitISO(ctx, mockConn, vm, isoPath, uData, mData, ""); assert.NoError(t, err)
}

func TestDefaultRunner_EnsureLibvirtDaemonRunning(t *testing.T) {
	ctrl := gomock.NewController(t); defer ctrl.Finish()
	mockConn := mocks.NewMockConnector(ctrl); runner := NewDefaultRunner(); ctx := context.Background()
	facts := &Facts{InitSystem: &ServiceInfo{Type: InitSystemSystemd, IsActiveCmd: "systemctl is-active", EnableCmd: "systemctl enable", StartCmd: "systemctl start"}}

	// Case 1: Service is active and enabled
	mockConn.EXPECT().IsServiceActive(ctx, mockConn, facts, "libvirtd").Return(true, nil).Times(1)
	mockConn.EXPECT().EnableService(ctx, mockConn, facts, "libvirtd").Return(nil).Times(1) // Enable is called for assurance
	err := runner.EnsureLibvirtDaemonRunning(ctx, mockConn, facts); assert.NoError(t, err)

	// Case 2: Service is not active, then started and enabled
	mockConn.EXPECT().IsServiceActive(ctx, mockConn, facts, "libvirtd").Return(false, nil).Times(1)
	mockConn.EXPECT().StartService(ctx, mockConn, facts, "libvirtd").Return(nil).Times(1)
	mockConn.EXPECT().EnableService(ctx, mockConn, facts, "libvirtd").Return(nil).Times(1)
	err = runner.EnsureLibvirtDaemonRunning(ctx, mockConn, facts); assert.NoError(t, err)

	// Case 3: IsServiceActive fails, but start and enable succeed
	mockConn.EXPECT().IsServiceActive(ctx, mockConn, facts, "libvirtd").Return(false, fmt.Errorf("is-active failed")).Times(1)
	mockConn.EXPECT().StartService(ctx, mockConn, facts, "libvirtd").Return(nil).Times(1)
	mockConn.EXPECT().EnableService(ctx, mockConn, facts, "libvirtd").Return(nil).Times(1)
	err = runner.EnsureLibvirtDaemonRunning(ctx, mockConn, facts); assert.NoError(t, err)


	// Case 4: Start fails
	mockConn.EXPECT().IsServiceActive(ctx, mockConn, facts, "libvirtd").Return(false, nil).Times(1)
	mockConn.EXPECT().StartService(ctx, mockConn, facts, "libvirtd").Return(fmt.Errorf("start failed")).Times(1)
	// CheckDockerInstalled might be called if start fails - mock it if needed by that function.
	// For this test, assume CheckDockerInstalled is not part of EnsureLibvirtDaemonRunning directly.
	err = runner.EnsureLibvirtDaemonRunning(ctx, mockConn, facts); assert.Error(t, err)
}
