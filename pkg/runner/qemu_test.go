package runner

import (
	"context"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector/mocks"
	"github.com/stretchr/testify/assert"
)

// TestQEMUMethodStubs asserts that all QEMU/libvirt methods currently return "not implemented".
func TestQEMUMethodStubs(t *testing.T) {
	r := &defaultRunner{}
	mockConn := mocks.NewConnector(t) // Create a new mock connector instance for each test or subtest
	ctx := context.Background()

	// Helper to check for "not implemented" error
	assertNotImplemented := func(t *testing.T, err error, methodName string) {
		assert.Error(t, err, "Expected an error for %s", methodName)
		if err != nil {
			assert.Contains(t, err.Error(), "not implemented: "+methodName, "Error message should indicate 'not implemented'")
		}
	}
	assertNotImplementedBool := func(t *testing.T, val bool, err error, methodName string) {
		assert.False(t, val, "Expected bool value to be false for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedString := func(t *testing.T, val string, err error, methodName string) {
		assert.Empty(t, val, "Expected string value to be empty for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}
	assertNotImplementedSlice := func(t *testing.T, val interface{}, err error, methodName string) {
		assert.Nil(t, val, "Expected slice value to be nil for %s", methodName)
		assertNotImplemented(t, err, methodName)
	}

	t.Run("CreateVMTemplate", func(t *testing.T) {
		err := r.CreateVMTemplate(ctx, mockConn, "", "", 0, 0, "", 0, "", "", "")
		assertNotImplemented(t, err, "CreateVMTemplate")
	})
	t.Run("ImportVMTemplate", func(t *testing.T) {
		err := r.ImportVMTemplate(ctx, mockConn, "", "")
		assertNotImplemented(t, err, "ImportVMTemplate")
	})
	t.Run("RefreshStoragePool", func(t *testing.T) {
		err := r.RefreshStoragePool(ctx, mockConn, "")
		assertNotImplemented(t, err, "RefreshStoragePool")
	})
	t.Run("CreateStoragePool", func(t *testing.T) {
		err := r.CreateStoragePool(ctx, mockConn, "", "", "")
		assertNotImplemented(t, err, "CreateStoragePool")
	})
	t.Run("StoragePoolExists", func(t *testing.T) {
		val, err := r.StoragePoolExists(ctx, mockConn, "")
		assertNotImplementedBool(t, val, err, "StoragePoolExists")
	})
	t.Run("DeleteStoragePool", func(t *testing.T) {
		err := r.DeleteStoragePool(ctx, mockConn, "")
		assertNotImplemented(t, err, "DeleteStoragePool")
	})
	t.Run("VolumeExists", func(t *testing.T) {
		val, err := r.VolumeExists(ctx, mockConn, "", "")
		assertNotImplementedBool(t, val, err, "VolumeExists")
	})
	t.Run("CloneVolume", func(t *testing.T) {
		err := r.CloneVolume(ctx, mockConn, "", "", "", 0, "")
		assertNotImplemented(t, err, "CloneVolume")
	})
	t.Run("ResizeVolume", func(t *testing.T) {
		err := r.ResizeVolume(ctx, mockConn, "", "", 0)
		assertNotImplemented(t, err, "ResizeVolume")
	})
	t.Run("DeleteVolume", func(t *testing.T) {
		err := r.DeleteVolume(ctx, mockConn, "", "")
		assertNotImplemented(t, err, "DeleteVolume")
	})
	t.Run("CreateVolume", func(t *testing.T) {
		err := r.CreateVolume(ctx, mockConn, "", "", 0, "", "", "")
		assertNotImplemented(t, err, "CreateVolume")
	})
	t.Run("CreateCloudInitISO", func(t *testing.T) {
		err := r.CreateCloudInitISO(ctx, mockConn, "", "", "", "", "")
		assertNotImplemented(t, err, "CreateCloudInitISO")
	})
	t.Run("CreateVM", func(t *testing.T) {
		err := r.CreateVM(ctx, mockConn, "", 0, 0, "", nil, nil, "", "", nil, nil)
		assertNotImplemented(t, err, "CreateVM")
	})
	t.Run("VMExists", func(t *testing.T) {
		val, err := r.VMExists(ctx, mockConn, "")
		assertNotImplementedBool(t, val, err, "VMExists")
	})
	t.Run("StartVM", func(t *testing.T) {
		err := r.StartVM(ctx, mockConn, "")
		assertNotImplemented(t, err, "StartVM")
	})
	t.Run("ShutdownVM", func(t *testing.T) {
		err := r.ShutdownVM(ctx, mockConn, "", false, 0*time.Second)
		assertNotImplemented(t, err, "ShutdownVM")
	})
	t.Run("DestroyVM", func(t *testing.T) {
		err := r.DestroyVM(ctx, mockConn, "")
		assertNotImplemented(t, err, "DestroyVM")
	})
	t.Run("UndefineVM", func(t *testing.T) {
		err := r.UndefineVM(ctx, mockConn, "", false, false, nil)
		assertNotImplemented(t, err, "UndefineVM")
	})
	t.Run("GetVMState", func(t *testing.T) {
		val, err := r.GetVMState(ctx, mockConn, "")
		assertNotImplementedString(t, val, err, "GetVMState")
	})
	t.Run("ListVMs", func(t *testing.T) {
		val, err := r.ListVMs(ctx, mockConn, false)
		assertNotImplementedSlice(t, val, err, "ListVMs")
	})
	t.Run("AttachDisk", func(t *testing.T) {
		err := r.AttachDisk(ctx, mockConn, "", "", "", "", "")
		assertNotImplemented(t, err, "AttachDisk")
	})
	t.Run("DetachDisk", func(t *testing.T) {
		err := r.DetachDisk(ctx, mockConn, "", "")
		assertNotImplemented(t, err, "DetachDisk")
	})
	t.Run("SetVMMemory", func(t *testing.T) {
		err := r.SetVMMemory(ctx, mockConn, "", 0, false)
		assertNotImplemented(t, err, "SetVMMemory")
	})
	t.Run("SetVMCPUs", func(t *testing.T) {
		err := r.SetVMCPUs(ctx, mockConn, "", 0, false)
		assertNotImplemented(t, err, "SetVMCPUs")
	})
}
