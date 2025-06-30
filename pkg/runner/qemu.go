package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// --- QEMU/libvirt Methods ---

func (r *defaultRunner) CreateVMTemplate(ctx context.Context, conn connector.Connector, name string, osVariant string, memoryMB uint, vcpus uint, diskPath string, diskSizeGB uint, network string, graphicsType string, cloudInitISOPath string) error {
	return fmt.Errorf("not implemented: CreateVMTemplate")
}
func (r *defaultRunner) ImportVMTemplate(ctx context.Context, conn connector.Connector, name string, filePath string) error {
	return fmt.Errorf("not implemented: ImportVMTemplate")
}
func (r *defaultRunner) RefreshStoragePool(ctx context.Context, conn connector.Connector, poolName string) error {
	return fmt.Errorf("not implemented: RefreshStoragePool")
}
func (r *defaultRunner) CreateStoragePool(ctx context.Context, conn connector.Connector, name string, poolType string, targetPath string) error {
	return fmt.Errorf("not implemented: CreateStoragePool")
}
func (r *defaultRunner) StoragePoolExists(ctx context.Context, conn connector.Connector, poolName string) (bool, error) {
	return false, fmt.Errorf("not implemented: StoragePoolExists")
}
func (r *defaultRunner) DeleteStoragePool(ctx context.Context, conn connector.Connector, poolName string) error {
	return fmt.Errorf("not implemented: DeleteStoragePool")
}
func (r *defaultRunner) VolumeExists(ctx context.Context, conn connector.Connector, poolName string, volName string) (bool, error) {
	return false, fmt.Errorf("not implemented: VolumeExists")
}
func (r *defaultRunner) CloneVolume(ctx context.Context, conn connector.Connector, poolName string, origVolName string, newVolName string, newSizeGB uint, format string) error {
	return fmt.Errorf("not implemented: CloneVolume")
}
func (r *defaultRunner) ResizeVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, newSizeGB uint) error {
	return fmt.Errorf("not implemented: ResizeVolume")
}
func (r *defaultRunner) DeleteVolume(ctx context.Context, conn connector.Connector, poolName string, volName string) error {
	return fmt.Errorf("not implemented: DeleteVolume")
}
func (r *defaultRunner) CreateVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, sizeGB uint, format string, backingVolName string, backingVolFormat string) error {
	return fmt.Errorf("not implemented: CreateVolume")
}
func (r *defaultRunner) CreateCloudInitISO(ctx context.Context, conn connector.Connector, vmName string, isoDestPath string, userData string, metaData string, networkConfig string) error {
	return fmt.Errorf("not implemented: CreateCloudInitISO")
}
func (r *defaultRunner) CreateVM(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, vcpus uint, osVariant string, diskPaths []string, networkInterfaces []VMNetworkInterface, graphicsType string, cloudInitISOPath string, bootOrder []string, extraArgs []string) error {
	return fmt.Errorf("not implemented: CreateVM")
}
func (r *defaultRunner) VMExists(ctx context.Context, conn connector.Connector, vmName string) (bool, error) {
	return false, fmt.Errorf("not implemented: VMExists")
}
func (r *defaultRunner) StartVM(ctx context.Context, conn connector.Connector, vmName string) error {
	return fmt.Errorf("not implemented: StartVM")
}
func (r *defaultRunner) ShutdownVM(ctx context.Context, conn connector.Connector, vmName string, force bool, timeout time.Duration) error {
	return fmt.Errorf("not implemented: ShutdownVM")
}
func (r *defaultRunner) DestroyVM(ctx context.Context, conn connector.Connector, vmName string) error {
	return fmt.Errorf("not implemented: DestroyVM")
}
func (r *defaultRunner) UndefineVM(ctx context.Context, conn connector.Connector, vmName string, deleteSnapshots bool, deleteStorage bool, storagePools []string) error {
	return fmt.Errorf("not implemented: UndefineVM")
}
func (r *defaultRunner) GetVMState(ctx context.Context, conn connector.Connector, vmName string) (string, error) {
	return "", fmt.Errorf("not implemented: GetVMState")
}
func (r *defaultRunner) ListVMs(ctx context.Context, conn connector.Connector, all bool) ([]VMInfo, error) {
	return nil, fmt.Errorf("not implemented: ListVMs")
}
func (r *defaultRunner) AttachDisk(ctx context.Context, conn connector.Connector, vmName string, diskPath string, targetDevice string, diskType string, driverType string) error {
	return fmt.Errorf("not implemented: AttachDisk")
}
func (r *defaultRunner) DetachDisk(ctx context.Context, conn connector.Connector, vmName string, targetDeviceOrPath string) error {
	return fmt.Errorf("not implemented: DetachDisk")
}
func (r *defaultRunner) SetVMMemory(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, current bool) error {
	return fmt.Errorf("not implemented: SetVMMemory")
}
func (r *defaultRunner) SetVMCPUs(ctx context.Context, conn connector.Connector, vmName string, vcpus uint, current bool) error {
	return fmt.Errorf("not implemented: SetVMCPUs")
}
