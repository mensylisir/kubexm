package runner

import (
	"context"
	"crypto/rand"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/pkg/errors"
)

var (
	memRegex = regexp.MustCompile(`(\d+)\s*(KiB|MiB|GiB|TiB)`)
)

func generateUUID() string {
	uuid := make([]byte, 16)
	rand.Read(uuid)
	uuid[6] = (uuid[6] & 0x0F) | 0x40
	uuid[8] = (uuid[8] & 0x3F) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func parseIntFromString(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

func parseMemStringToKB(memStr string) (uint64, error) {
	matches := memRegex.FindStringSubmatch(memStr)
	if len(matches) != 3 {
		// Fallback for plain numbers, assume KiB
		val, err := strconv.ParseUint(strings.TrimSpace(memStr), 10, 64)
		if err == nil {
			return val, nil
		}
		return 0, fmt.Errorf("invalid memory format: '%s'", memStr)
	}

	value, err := strconv.ParseUint(matches[1], 10, 64)
	if err != nil {
		return 0, errors.Wrapf(err, "parsing memory value from '%s'", matches[1])
	}

	unit := strings.ToLower(matches[2])
	switch unit {
	case "kib":
		return value, nil
	case "mib":
		return value * 1024, nil
	case "gib":
		return value * 1024 * 1024, nil
	case "tib":
		return value * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown memory unit: '%s'", unit)
	}
}

// CreateVMTemplate defines a new virtual machine configuration that can serve as a template.
// For now, this will be a placeholder as generating complex libvirt XML via shell commands is error-prone.
// A real implementation might use a Go libvirt library or pre-defined XML templates.
// This function will primarily focus on creating the disk if it doesn't exist.
func (r *defaultRunner) CreateVMTemplate(ctx context.Context, conn connector.Connector, name string, osVariant string, memoryMB uint, vcpus uint, diskPath string, diskSizeGB uint, network string, graphicsType string, cloudInitISOPath string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if name == "" || diskPath == "" || osVariant == "" || memoryMB == 0 || vcpus == 0 || diskSizeGB == 0 {
		return errors.New("name, osVariant, memoryMB, vcpus, diskPath and diskSizeGB are required for VM template")
	}

	// 1. Check if disk image exists
	exists, err := r.Exists(ctx, conn, diskPath)
	if err != nil {
		return errors.Wrapf(err, "failed to check if disk %s exists", diskPath)
	}

	if !exists {
		// Create the disk image using qemu-img
		// Ensure parent directory exists
		diskDir := filepath.Dir(diskPath)
		if err := r.Mkdirp(ctx, conn, diskDir, "0755", true); err != nil {
			return errors.Wrapf(err, "failed to create directory %s for disk image", diskDir)
		}

		createDiskCmd := fmt.Sprintf("qemu-img create -f qcow2 %s %dG", diskPath, diskSizeGB)
		_, stderr, err := conn.Exec(ctx, createDiskCmd, &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute})
		if err != nil {
			return errors.Wrapf(err, "failed to create disk image %s with qemu-img. Stderr: %s", diskPath, string(stderr))
		}
	}
	// Placeholder for actual VM definition logic using virsh define
	// A proper implementation would involve generating or using an XML template, then:
	// tempXMLPath := fmt.Sprintf("/tmp/vm_template_%s.xml", name)
	// err = r.WriteFile(ctx, conn, []byte(generatedOrTemplateXML), tempXMLPath, "0600", true)
	// defineCmd := fmt.Sprintf("virsh define %s", tempXMLPath)
	// _, _, err = conn.Exec(ctx, defineCmd, &connector.ExecOptions{Sudo: true})
	// r.Remove(ctx, conn, tempXMLPath, true) // Cleanup
	// if err != nil { return errors.Wrap(err, "failed to define VM template") }
	return errors.New("CreateVMTemplate: virsh define from generated XML is not fully implemented via CLI runner; disk creation part is present")
}

// VMExists checks if a virtual machine with the given name is defined in libvirt.
func (r *defaultRunner) VMExists(ctx context.Context, conn connector.Connector, vmName string) (bool, error) {
	if conn == nil {
		return false, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return false, errors.New("vmName cannot be empty")
	}
	// `virsh dominfo <domain>` exits 0 if domain exists, non-zero otherwise (e.g. 1 if not found)
	cmd := fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", vmName)
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err == nil {
		return true, nil
	} // Exit code 0 means domain exists

	// Check if the error is due to the domain not being found
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) && cmdErr.ExitCode != 0 { // Any non-zero exit code from dominfo likely means not found or other error
		// More specific check could parse stderr for "Domain not found" or "error: failed to get domain"
		return false, nil // Treat non-zero exit as "does not exist" for simplicity of this check
	}
	// For other types of errors (e.g., connection issue), propagate the error
	return false, errors.Wrapf(err, "failed to check if VM %s exists", vmName)
}

// StartVM starts a defined (but not running) virtual machine.
func (r *defaultRunner) StartVM(ctx context.Context, conn connector.Connector, vmName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}

	// Check current state first to make it idempotent
	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState != nil {
		return errors.Wrapf(errState, "failed to get current state of VM %s before starting", vmName)
	}
	if state == "running" {
		return nil // Already running
	}
	if state != "shut off" && state != "pmsuspended" { // Add other valid pre-start states if needed
		return errors.Errorf("VM %s is in state '%s', cannot start", vmName, state)
	}

	startCmd := fmt.Sprintf("virsh start %s", vmName)
	_, stderr, err := conn.Exec(ctx, startCmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to start VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

// ShutdownVM attempts a graceful shutdown of a virtual machine.
func (r *defaultRunner) ShutdownVM(ctx context.Context, conn connector.Connector, vmName string, force bool, timeout time.Duration) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}

	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState != nil {
		// If domain not found, consider it already shut down for idempotency.
		if strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain") {
			return nil
		}
		return errors.Wrapf(errState, "failed to get VM state for %s before shutdown", vmName)
	}
	if state == "shut off" {
		return nil // Already shut off
	}
	if state != "running" && state != "paused" { // Can only shutdown running or paused VMs
		return errors.Errorf("VM %s is in state '%s', cannot initiate shutdown", vmName, state)
	}

	shutdownCmd := fmt.Sprintf("virsh shutdown %s", vmName)
	_, stderrShutdown, errShutdown := conn.Exec(ctx, shutdownCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})

	if errShutdown == nil { // Graceful shutdown initiated
		waitCtx, cancelWait := context.WithTimeout(ctx, timeout)
		defer cancelWait()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-waitCtx.Done(): // Timeout reached
				if force {
					return r.DestroyVM(ctx, conn, vmName) // Use original ctx for DestroyVM
				}
				return errors.Errorf("VM %s graceful shutdown timed out after %v", vmName, timeout)
			case <-ticker.C:
				currentState, err := r.GetVMState(waitCtx, conn, vmName)
				if err != nil {
					// If error checking state (e.g., domain disappeared), assume it's off.
					return nil
				}
				if currentState == "shut off" {
					return nil // Successfully shut down
				}
			}
		}
	}

	// Graceful shutdown command failed
	if force {
		return r.DestroyVM(ctx, conn, vmName)
	}
	return errors.Wrapf(errShutdown, "failed to issue graceful shutdown for VM %s. Stderr: %s", vmName, string(stderrShutdown))
}

// DestroyVM forcefully stops (powers off) a virtual machine.
func (r *defaultRunner) DestroyVM(ctx context.Context, conn connector.Connector, vmName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}

	// Check if already off or non-existent for idempotency
	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState != nil {
		if strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain") {
			return nil // Effectively destroyed if not found
		}
		// For other errors getting state, proceed with destroy as a best effort.
	} else if state == "shut off" {
		return nil // Already off
	}

	cmd := fmt.Sprintf("virsh destroy %s", vmName)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		// Idempotency: if already not running or not found, virsh destroy might error but that's okay.
		if strings.Contains(string(stderr), "Domain not found") || strings.Contains(string(stderr), "domain is not running") {
			return nil
		}
		return errors.Wrapf(err, "failed to destroy VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

// UndefineVM removes the definition of a virtual machine from libvirt.
func (r *defaultRunner) UndefineVM(ctx context.Context, conn connector.Connector, vmName string, deleteSnapshots bool, deleteStorage bool, storagePools []string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}

	// Ensure VM is off
	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState == nil && (state == "running" || state == "paused") {
		if errDestroy := r.DestroyVM(ctx, conn, vmName); errDestroy != nil {
			return errors.Wrapf(errDestroy, "failed to destroy VM %s prior to undefine", vmName)
		}
		// Wait a moment for destroy to complete before undefining
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	} else if errState != nil && !(strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain")) {
		return errors.Wrapf(errState, "failed to get state of VM %s before undefine", vmName)
	}
	// If state is "shut off" or domain not found, proceed.

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "undefine", vmName)
	if deleteSnapshots {
		cmdArgs = append(cmdArgs, "--snapshots-metadata") // Deletes snapshot metadata
	}
	if deleteStorage {
		cmdArgs = append(cmdArgs, "--remove-all-storage") // Attempts to remove storage volumes
		// --storage-pools <pool_name>[,<pool_name>...] can be added if storagePools is not empty
		// However, the exact syntax and support for this with --remove-all-storage needs careful checking.
		// For now, --remove-all-storage is a broad attempt.
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "Domain not found") {
			return nil // Idempotent
		}
		return errors.Wrapf(err, "failed to undefine VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

// GetVMState retrieves the current state of a virtual machine.
func (r *defaultRunner) GetVMState(ctx context.Context, conn connector.Connector, vmName string) (string, error) {
	if conn == nil {
		return "", errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return "", errors.New("vmName cannot be empty")
	}
	cmd := fmt.Sprintf("virsh domstate %s", vmName)
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err != nil {
		// If domain not found, virsh domstate errors. This should be handled by caller if needed.
		return "", errors.Wrapf(err, "failed to get state for VM %s. Stderr: %s", vmName, string(stderr))
	}
	return strings.TrimSpace(string(stdout)), nil
}

// ListVMs lists virtual machines known to libvirt.
func (r *defaultRunner) ListVMs(ctx context.Context, conn connector.Connector, all bool) ([]VMInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	listCmdArgs := []string{"virsh", "list", "--name"} // Get names first
	if all {
		listCmdArgs = append(listCmdArgs, "--all")
	}

	stdoutNames, stderrNames, err := conn.Exec(ctx, strings.Join(listCmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list VM names. Stderr: %s", string(stderrNames))
	}

	vmNamesRaw := strings.Split(string(stdoutNames), "\n")
	var vmNames []string
	for _, name := range vmNamesRaw {
		trimmedName := strings.TrimSpace(name)
		if trimmedName != "" {
			vmNames = append(vmNames, trimmedName)
		}
	}

	if len(vmNames) == 0 {
		return []VMInfo{}, nil
	}

	var vms []VMInfo
	for _, vmName := range vmNames {
		// Get state for each VM
		state, errState := r.GetVMState(ctx, conn, vmName)
		if errState != nil {
			// If we can't get the state, we might still list the VM with unknown state or skip it.
			// For now, let's include it with an error state or empty state.
			// Or, more simply, skip if state cannot be determined for robustness.
			// Let's try to get dominfo anyway.
		}

		dominfoCmd := fmt.Sprintf("virsh dominfo %s", vmName)
		infoStdout, _, errInfo := conn.Exec(ctx, dominfoCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
		if errInfo != nil {
			// If dominfo fails, we might have a problem with this VM.
			// Add with available info or log a warning.
			vms = append(vms, VMInfo{Name: vmName, State: state}) // Add with what we have
			continue
		}

		vmInfo := VMInfo{Name: vmName, State: state}
		infoLines := strings.Split(string(infoStdout), "\n")
		for _, line := range infoLines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "UUID":
				vmInfo.UUID = value
			case "CPU(s)":
				vmInfo.CPUs, _ = parseIntFromString(value)
			case "Max memory", "Current memory", "Memory": // Prefer Max memory, fallback to Current memory then Memory
				if vmInfo.Memory == 0 { // Only set if not already set by a preferred key
					memKB, parseErr := parseMemStringToKB(value)
					if parseErr == nil {
						vmInfo.Memory = uint(memKB / 1024) // Convert KiB to MiB
					}
				}
			case "State": // If GetVMState failed earlier, try to get it from dominfo
				if vmInfo.State == "" {
					vmInfo.State = value
				}
			}
		}
		vms = append(vms, vmInfo)
	}
	return vms, nil
}

func (r *defaultRunner) ImportVMTemplate(ctx context.Context, conn connector.Connector, name string, filePath string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if name == "" || filePath == "" {
		return errors.New("name and filePath are required for importing VM template")
	}
	cmd := fmt.Sprintf("virsh define %s", filePath)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to define VM %s from %s. Stderr: %s", name, filePath, string(stderr))
	}
	return nil
}

func (r *defaultRunner) RefreshStoragePool(ctx context.Context, conn connector.Connector, poolName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if poolName == "" {
		return errors.New("poolName cannot be empty")
	}
	cmd := fmt.Sprintf("virsh pool-refresh %s", poolName)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute}) // Refresh can take time
	if err != nil {
		return errors.Wrapf(err, "failed to refresh storage pool %s. Stderr: %s", poolName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CreateStoragePool(ctx context.Context, conn connector.Connector, name string, poolType string, targetPath string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if name == "" || poolType == "" || targetPath == "" {
		return errors.New("name, poolType, and targetPath are required")
	}

	// Idempotency: Check if pool already exists
	exists, errExists := r.StoragePoolExists(ctx, conn, name)
	if errExists != nil {
		return errors.Wrapf(errExists, "failed to check if storage pool %s exists", name)
	}
	if exists {
		return nil
	} // Already exists

	if poolType == "dir" { // Ensure directory exists for dir-type pools
		if err := r.Mkdirp(ctx, conn, targetPath, "0755", true); err != nil {
			return errors.Wrapf(err, "failed to create directory %s for pool %s", targetPath, name)
		}
	}

	var defineCmd string
	switch poolType {
	case "dir":
		defineCmd = fmt.Sprintf("virsh pool-define-as %s dir --target %s", name, targetPath)
	// Add other pool types (logical, iscsi, etc.) as needed
	default:
		return errors.Errorf("unsupported poolType: %s", poolType)
	}

	_, stderrDefine, errDefine := conn.Exec(ctx, defineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDefine != nil {
		// If it already defined but we missed in exists check (race or other issue), that's fine.
		if !(strings.Contains(string(stderrDefine), "already defined") || strings.Contains(string(stderrDefine), "already exists")) {
			return errors.Wrapf(errDefine, "failed to define pool %s. Stderr: %s", name, string(stderrDefine))
		}
	}

	buildCmd := fmt.Sprintf("virsh pool-build %s", name)
	_, stderrBuild, errBuild := conn.Exec(ctx, buildCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errBuild != nil {
		if !(strings.Contains(string(stderrBuild), "already built") || strings.Contains(string(stderrBuild), "No action required")) {
			return errors.Wrapf(errBuild, "failed to build pool %s. Stderr: %s", name, string(stderrBuild))
		}
	}

	startCmd := fmt.Sprintf("virsh pool-start %s", name)
	_, stderrStart, errStart := conn.Exec(ctx, startCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errStart != nil && !strings.Contains(string(stderrStart), "already active") {
		return errors.Wrapf(errStart, "failed to start pool %s. Stderr: %s", name, string(stderrStart))
	}

	autostartCmd := fmt.Sprintf("virsh pool-autostart %s", name)
	if _, _, errAutostart := conn.Exec(ctx, autostartCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second}); errAutostart != nil {
		// Log as warning, not a critical failure if pool is otherwise operational
		// fmt.Printf("Warning: failed to set autostart for pool %s: %v\n", name, errAutostart)
	}
	return nil
}

func (r *defaultRunner) StoragePoolExists(ctx context.Context, conn connector.Connector, poolName string) (bool, error) {
	if conn == nil {
		return false, errors.New("connector cannot be nil")
	}
	if poolName == "" {
		return false, errors.New("poolName cannot be empty")
	}
	cmd := fmt.Sprintf("virsh pool-info %s > /dev/null 2>&1", poolName)
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err == nil {
		return true, nil
	} // Exit 0 means pool exists
	if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 {
		// Non-zero exit usually means not found (e.g. "error: failed to get pool '...'")
		return false, nil
	}
	return false, errors.Wrapf(err, "failed to check storage pool %s existence", poolName)
}

func (r *defaultRunner) DeleteStoragePool(ctx context.Context, conn connector.Connector, poolName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if poolName == "" {
		return errors.New("poolName cannot be empty")
	}

	// Attempt to destroy (stop) the pool first. This is idempotent.
	destroyCmd := fmt.Sprintf("virsh pool-destroy %s", poolName)
	_, stderrDestroy, errDestroy := conn.Exec(ctx, destroyCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDestroy != nil {
		// Ignore errors if pool not found or not active, as that's fine for deletion.
		if !(strings.Contains(string(stderrDestroy), "not found") ||
			strings.Contains(string(stderrDestroy), "not active") ||
			strings.Contains(string(stderrDestroy), "not running")) { // "not running" for some virsh versions
			return errors.Wrapf(errDestroy, "failed to destroy pool %s. Stderr: %s", poolName, string(stderrDestroy))
		}
	}

	// Undefine the pool. This is also idempotent.
	undefineCmd := fmt.Sprintf("virsh pool-undefine %s", poolName)
	_, stderrUndefine, errUndefine := conn.Exec(ctx, undefineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errUndefine != nil {
		if !strings.Contains(string(stderrUndefine), "not found") { // Ignore "not found" for undefine
			return errors.Wrapf(errUndefine, "failed to undefine pool %s. Stderr: %s", poolName, string(stderrUndefine))
		}
	}
	return nil
}

func (r *defaultRunner) VolumeExists(ctx context.Context, conn connector.Connector, poolName string, volName string) (bool, error) {
	if conn == nil {
		return false, errors.New("connector cannot be nil")
	}
	if poolName == "" || volName == "" {
		return false, errors.New("poolName and volName cannot be empty")
	}
	cmd := fmt.Sprintf("virsh vol-info --pool %s %s > /dev/null 2>&1", poolName, volName)
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err == nil {
		return true, nil
	}
	if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 {
		// e.g. "error: Failed to get volume '...' from pool '...'"
		return false, nil
	}
	return false, errors.Wrapf(err, "failed to check volume %s in pool %s", volName, poolName)
}

func (r *defaultRunner) CloneVolume(ctx context.Context, conn connector.Connector, poolName string, origVolName string, newVolName string, newSizeGB uint, format string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if poolName == "" || origVolName == "" || newVolName == "" {
		return errors.New("poolName, origVolName, and newVolName are required")
	}

	// virsh vol-clone [--pool <string>] <vol> <newname> [--new-format <string>] [--new-capacity <bytes>]
	cloneCmdArgs := []string{"virsh", "vol-clone", "--pool", poolName, origVolName, newVolName}
	if format != "" { // Assuming format here refers to new-format for clone, not capacity format.
		cloneCmdArgs = append(cloneCmdArgs, "--new-format", format)
	}
	// Note: vol-clone can also take --new-capacity. If newSizeGB is for the final state after potential resize,
	// it might be better to clone first, then resize. Or, if libvirt supports it well, use --new-capacity here.
	// For simplicity and explicit control, we'll clone then resize if newSizeGB > 0.

	_, stderr, err := conn.Exec(ctx, strings.Join(cloneCmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute}) // Cloning can take time
	if err != nil {
		return errors.Wrapf(err, "failed to clone volume %s to %s in pool %s. Stderr: %s", origVolName, newVolName, poolName, string(stderr))
	}

	if newSizeGB > 0 {
		// Check current size first? Or just attempt resize.
		// For simplicity, just attempt resize.
		if errResize := r.ResizeVolume(ctx, conn, poolName, newVolName, newSizeGB); errResize != nil {
			return errors.Wrapf(errResize, "volume %s cloned successfully, but failed to resize to %dGB", newVolName, newSizeGB)
		}
	}
	return nil
}

func (r *defaultRunner) ResizeVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, newSizeGB uint) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if poolName == "" || volName == "" || newSizeGB == 0 {
		return errors.New("poolName, volName, and non-zero newSizeGB are required")
	}
	capacityStr := fmt.Sprintf("%dG", newSizeGB) // virsh vol-resize takes size like "10G"
	cmd := fmt.Sprintf("virsh vol-resize --pool %s %s %s", poolName, volName, capacityStr)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to resize volume %s in pool %s to %s. Stderr: %s", volName, poolName, capacityStr, string(stderr))
	}
	return nil
}

func (r *defaultRunner) DeleteVolume(ctx context.Context, conn connector.Connector, poolName string, volName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if poolName == "" || volName == "" {
		return errors.New("poolName and volName are required")
	}
	cmd := fmt.Sprintf("virsh vol-delete --pool %s %s", poolName, volName)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		// Idempotency: if volume not found, it's okay.
		if strings.Contains(string(stderr), "Failed to get volume") || strings.Contains(string(stderr), "Storage volume not found") {
			return nil
		}
		return errors.Wrapf(err, "failed to delete volume %s from pool %s. Stderr: %s", volName, poolName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CreateVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, sizeGB uint, format string, backingVolName string, backingVolFormat string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if poolName == "" || volName == "" || sizeGB == 0 {
		return errors.New("poolName, volName, and non-zero sizeGB are required")
	}

	// Idempotency: Check if volume already exists
	exists, errExists := r.VolumeExists(ctx, conn, poolName, volName)
	if errExists != nil {
		return errors.Wrapf(errExists, "failed to check if volume %s in pool %s exists", volName, poolName)
	}
	if exists {
		return nil
	} // Already exists

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "vol-create-as", poolName, volName, fmt.Sprintf("%dG", sizeGB))
	if format != "" {
		cmdArgs = append(cmdArgs, "--format", format)
	}
	if backingVolName != "" {
		if backingVolFormat == "" {
			return errors.New("backingVolFormat is required when backingVolName is provided")
		}
		cmdArgs = append(cmdArgs, "--backing-vol", backingVolName)
		cmdArgs = append(cmdArgs, "--backing-vol-format", backingVolFormat)
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		// Double check for already exists, in case of race or if Exists check failed silently
		if strings.Contains(string(stderr), "already exists") {
			return nil
		}
		return errors.Wrapf(err, "failed to create volume %s in pool %s. Stderr: %s", volName, poolName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CreateCloudInitISO(ctx context.Context, conn connector.Connector, vmName string, isoDestPath string, userData string, metaData string, networkConfig string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || isoDestPath == "" || userData == "" || metaData == "" {
		return errors.New("vmName, isoDestPath, userData, and metaData are required")
	}

	remoteBaseTmpDir := "/tmp" // Standard temporary directory on Linux
	tmpDirName := fmt.Sprintf("kubexm-cloud-init-tmp-%s-%d", vmName, time.Now().UnixNano())
	tmpDirPath := filepath.Join(remoteBaseTmpDir, tmpDirName)

	// Ensure the base temp directory exists (usually /tmp does, but good practice)
	// r.Mkdirp(ctx, conn, remoteBaseTmpDir, "0777", true) // /tmp should exist and be writable

	if err := r.Mkdirp(ctx, conn, tmpDirPath, "0700", true); err != nil { // 0700 for restricted access
		return errors.Wrapf(err, "failed to create temporary directory %s on remote host", tmpDirPath)
	}
	defer r.Remove(ctx, conn, tmpDirPath, true) // Cleanup temporary directory

	if err := r.WriteFile(ctx, conn, []byte(userData), filepath.Join(tmpDirPath, "user-data"), "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write user-data to %s", tmpDirPath)
	}
	if err := r.WriteFile(ctx, conn, []byte(metaData), filepath.Join(tmpDirPath, "meta-data"), "0644", true); err != nil {
		return errors.Wrapf(err, "failed to write meta-data to %s", tmpDirPath)
	}
	if networkConfig != "" {
		if err := r.WriteFile(ctx, conn, []byte(networkConfig), filepath.Join(tmpDirPath, "network-config"), "0644", true); err != nil {
			return errors.Wrapf(err, "failed to write network-config to %s", tmpDirPath)
		}
	}

	// Ensure destination directory for ISO exists
	isoDir := filepath.Dir(isoDestPath)
	if err := r.Mkdirp(ctx, conn, isoDir, "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create directory %s for ISO image", isoDir)
	}

	// Determine which ISO creation tool to use
	isoCmdTool := "genisoimage" // Default
	if _, err := r.LookPath(ctx, conn, "genisoimage"); err != nil {
		// genisoimage not found, try mkisofs
		if _, errMkisofs := r.LookPath(ctx, conn, "mkisofs"); errMkisofs == nil {
			isoCmdTool = "mkisofs"
		} else {
			return errors.New("neither genisoimage nor mkisofs found on the remote host")
		}
	}

	// Command to create ISO: genisoimage -o <output.iso> -V cidata -r -J <path_to_cloud_init_files_dir>
	// or: mkisofs -o <output.iso> -V cidata -r -J <path_to_cloud_init_files_dir>
	isoCmd := fmt.Sprintf("%s -o %s -V cidata -r -J %s",
		isoCmdTool,
		isoDestPath,
		tmpDirPath,
	)

	_, stderr, err := conn.Exec(ctx, isoCmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to create cloud-init ISO %s using %s. Stderr: %s", isoDestPath, isoCmdTool, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CreateVM(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, vcpus uint, osVariant string, diskPaths []string, networkInterfaces []VMNetworkInterface, graphicsType string, cloudInitISOPath string, bootOrder []string, extraArgs []string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || memoryMB == 0 || vcpus == 0 || len(diskPaths) == 0 || diskPaths[0] == "" {
		return errors.New("vmName, memoryMB, vcpus, and at least one diskPath are required")
	}
	if len(networkInterfaces) == 0 {
		return errors.New("at least one networkInterface is required")
	}

	exists, err := r.VMExists(ctx, conn, vmName)
	if err != nil {
		return errors.Wrapf(err, "failed to check if VM %s already exists", vmName)
	}
	if exists {
		return errors.Errorf("VM %s already exists", vmName)
	}

	var xmlBuilder strings.Builder
	xmlBuilder.WriteString(fmt.Sprintf("<domain type='kvm'>\n"))
	xmlBuilder.WriteString(fmt.Sprintf("  <name>%s</name>\n", vmName))
	xmlBuilder.WriteString(fmt.Sprintf("  <uuid>%s</uuid>\n", generateUUID())) // Generate a UUID
	xmlBuilder.WriteString(fmt.Sprintf("  <memory unit='MiB'>%d</memory>\n", memoryMB))
	xmlBuilder.WriteString(fmt.Sprintf("  <currentMemory unit='MiB'>%d</currentMemory>\n", memoryMB))
	xmlBuilder.WriteString(fmt.Sprintf("  <vcpu placement='static'>%d</vcpu>\n", vcpus))
	xmlBuilder.WriteString("  <os>\n")
	if osVariant == "" {
		osVariant = "generic"
	} // Default OS variant if not specified
	xmlBuilder.WriteString(fmt.Sprintf("    <type arch='x86_64' machine='pc-q35-rhel8.6.0'>hvm</type>\n")) // Machine type might need to be more generic or detected
	// Add os-variant if provided, or a loader for UEFI if needed
	// <smbios mode='sysinfo'/> might be useful
	if len(bootOrder) == 0 {
		bootOrder = []string{"hd"}
		if cloudInitISOPath != "" {
			bootOrder = append(bootOrder, "cdrom")
		}
	}
	for _, dev := range bootOrder {
		xmlBuilder.WriteString(fmt.Sprintf("    <boot dev='%s'/>\n", dev))
	}
	xmlBuilder.WriteString("  </os>\n")

	xmlBuilder.WriteString("  <features><acpi/><apic/><vmport state='off'/></features>\n")
	xmlBuilder.WriteString("  <cpu mode='host-passthrough' check='none'/>\n") // host-passthrough for better perf, or host-model
	xmlBuilder.WriteString("  <clock offset='utc'><timer name='rtc' tickpolicy='catchup'/><timer name='pit' tickpolicy='delay'/><timer name='hpet' present='no'/></clock>\n")
	xmlBuilder.WriteString("  <on_poweroff>destroy</on_poweroff>\n  <on_reboot>restart</on_reboot>\n  <on_crash>destroy</on_crash>\n")
	xmlBuilder.WriteString("  <pm><suspend-to-mem enabled='no'/><suspend-to-disk enabled='no'/></pm>\n")
	xmlBuilder.WriteString("  <devices>\n    <emulator>/usr/libexec/qemu-kvm</emulator>\n") // Path might vary based on distro

	diskTargetLetter := 'a'
	pciSlot := 0x04 // Starting PCI slot for disks
	for _, diskPath := range diskPaths {
		if diskPath == "" {
			continue
		}
		xmlBuilder.WriteString("    <disk type='file' device='disk'>\n")
		xmlBuilder.WriteString("      <driver name='qemu' type='qcow2' cache='none' io='native'/>\n") // Common settings
		xmlBuilder.WriteString(fmt.Sprintf("      <source file='%s'/>\n", diskPath))
		xmlBuilder.WriteString(fmt.Sprintf("      <target dev='vd%c' bus='virtio'/>\n", diskTargetLetter))
		// Assign unique PCI address
		xmlBuilder.WriteString(fmt.Sprintf("      <address type='pci' domain='0x0000' bus='0x00' slot='0x%02x' function='0x0'/>\n", pciSlot))
		xmlBuilder.WriteString("    </disk>\n")
		diskTargetLetter++
		pciSlot++
	}

	if cloudInitISOPath != "" {
		xmlBuilder.WriteString("    <disk type='file' device='cdrom'>\n")
		xmlBuilder.WriteString("      <driver name='qemu' type='raw'/>\n")
		xmlBuilder.WriteString(fmt.Sprintf("      <source file='%s'/>\n", cloudInitISOPath))
		xmlBuilder.WriteString("      <target dev='sda' bus='sata'/>\n") // Use a common bus like sata for cdrom
		xmlBuilder.WriteString("      <readonly/>\n")
		xmlBuilder.WriteString(fmt.Sprintf("      <address type='drive' controller='0' bus='0' target='0' unit='%d'/>\n", 0)) // Unit 0 for first SATA CD-ROM
		xmlBuilder.WriteString("    </disk>\n")
	}

	networkPCISlot := pciSlot // Continue PCI slot numbering for NICs
	for _, nic := range networkInterfaces {
		xmlBuilder.WriteString(fmt.Sprintf("    <interface type='%s'>\n", nic.Type))           // e.g. network, bridge
		xmlBuilder.WriteString(fmt.Sprintf("      <source %s='%s'/>\n", nic.Type, nic.Source)) // e.g. <source network='default'/> or <source bridge='br0'/>
		if nic.MACAddress != "" {
			xmlBuilder.WriteString(fmt.Sprintf("      <mac address='%s'/>\n", nic.MACAddress))
		}
		if nic.Model == "" {
			nic.Model = "virtio"
		}
		xmlBuilder.WriteString(fmt.Sprintf("      <model type='%s'/>\n", nic.Model))
		xmlBuilder.WriteString(fmt.Sprintf("      <address type='pci' domain='0x0000' bus='0x00' slot='0x%02x' function='0x0'/>\n", networkPCISlot))
		xmlBuilder.WriteString("    </interface>\n")
		networkPCISlot++
	}

	if graphicsType == "" {
		graphicsType = "vnc"
	}
	if graphicsType != "none" {
		xmlBuilder.WriteString(fmt.Sprintf("    <graphics type='%s' port='-1' autoport='yes' listen='0.0.0.0'>\n", graphicsType))
		xmlBuilder.WriteString("      <listen type='address' address='0.0.0.0'/>\n    </graphics>\n")
		xmlBuilder.WriteString("    <video>\n      <model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1' primary='yes'/>\n    </video>\n") // qxl is a good default
	}

	xmlBuilder.WriteString("    <serial type='pty'><target type='isa-serial' port='0'><model name='isa-serial'/></target></serial>\n")
	xmlBuilder.WriteString("    <console type='pty'><target type='serial' port='0'/></console>\n")
	xmlBuilder.WriteString("    <channel type='unix'>\n      <target type='virtio' name='org.qemu.guest_agent.0'/>\n") // QEMU guest agent channel
	xmlBuilder.WriteString(fmt.Sprintf("      <address type='virtio-serial' controller='0' bus='0' port='%d'/>\n", 1)) // Port 1 for guest agent
	xmlBuilder.WriteString("    </channel>\n")
	xmlBuilder.WriteString("    <input type='tablet' bus='usb'/><input type='mouse' bus='ps2'/><input type='keyboard' bus='ps2'/>\n")
	xmlBuilder.WriteString(fmt.Sprintf("    <memballoon model='virtio'><address type='pci' domain='0x0000' bus='0x00' slot='0x%02x' function='0x0'/></memballoon>\n", networkPCISlot)) // Next available PCI slot
	xmlBuilder.WriteString("  </devices>\n</domain>\n")

	vmXML := xmlBuilder.String()
	tempXMLPath := fmt.Sprintf("/tmp/kubexm-vmdef-%s-%d.xml", vmName, time.Now().UnixNano())
	if err = r.WriteFile(ctx, conn, []byte(vmXML), tempXMLPath, "0600", true); err != nil {
		return errors.Wrapf(err, "failed to write temp VM XML to %s", tempXMLPath)
	}
	defer r.Remove(ctx, conn, tempXMLPath, true)

	defineCmd := fmt.Sprintf("virsh define %s", tempXMLPath)
	_, stderrDefine, errDefine := conn.Exec(ctx, defineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDefine != nil {
		return errors.Wrapf(errDefine, "failed to define VM %s. Stderr: %s\nXML:\n%s", vmName, string(stderrDefine), vmXML)
	}

	if errStart := r.StartVM(ctx, conn, vmName); errStart != nil {
		return errors.Wrapf(errStart, "VM %s defined, but failed to start", vmName)
	}
	return nil
}

func (r *defaultRunner) AttachDisk(ctx context.Context, conn connector.Connector, vmName string, diskPath string, targetDevice string, diskType string, driverType string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || diskPath == "" || targetDevice == "" {
		return errors.New("vmName, diskPath, and targetDevice are required")
	}
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "attach-disk", vmName, diskPath, targetDevice)
	if driverType != "" {
		cmdArgs = append(cmdArgs, "--driver", "qemu", "--subdriver", driverType)
	}
	// diskType for attach-disk usually means 'cdrom', 'floppy', 'disk', 'lun'. 'file' or 'block' are source types.
	// For simplicity, we'll assume it's a regular disk and not set --type unless explicitly needed.
	cmdArgs = append(cmdArgs, "--config", "--live") // Make persistent and apply live

	cmd := strings.Join(cmdArgs, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to attach disk %s to VM %s as %s. Stderr: %s", diskPath, vmName, targetDevice, string(stderr))
	}
	return nil
}

func (r *defaultRunner) DetachDisk(ctx context.Context, conn connector.Connector, vmName string, targetDeviceOrPath string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || targetDeviceOrPath == "" {
		return errors.New("vmName and targetDeviceOrPath are required")
	}
	cmd := fmt.Sprintf("virsh detach-disk %s %s --config --live", vmName, targetDeviceOrPath)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "No disk found for target") || strings.Contains(string(stderr), "no target device found") || strings.Contains(string(stderr), "not found") {
			return nil // Idempotent
		}
		return errors.Wrapf(err, "failed to detach disk %s from VM %s. Stderr: %s", targetDeviceOrPath, vmName, string(stderr))
	}
	return nil
}
func (r *defaultRunner) SetVMMemory(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, current bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || memoryMB == 0 {
		return errors.New("vmName and non-zero memoryMB are required")
	}
	memoryKiB := memoryMB * 1024

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "setmem", vmName, fmt.Sprintf("%dK", memoryKiB))

	if current {
		cmdArgs = append(cmdArgs, "--live", "--config")
	} else {
		cmdArgs = append(cmdArgs, "--config")
	}
	cmd := strings.Join(cmdArgs, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to set memory for VM %s to %dMiB. Stderr: %s", vmName, memoryMB, string(stderr))
	}
	return nil
}

func (r *defaultRunner) SetVMCPUs(ctx context.Context, conn connector.Connector, vmName string, vcpus uint, current bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || vcpus == 0 {
		return errors.New("vmName and non-zero vcpus are required")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "setvcpus", vmName, fmt.Sprintf("%d", vcpus))
	if current {
		cmdArgs = append(cmdArgs, "--live", "--config")
	} else {
		cmdArgs = append(cmdArgs, "--config")
	}
	cmd := strings.Join(cmdArgs, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to set vCPUs for VM %s to %d. Stderr: %s", vmName, vcpus, string(stderr))
	}
	return nil
}

// EnsureLibvirtDaemonRunning checks and ensures libvirtd is running.
func (r *defaultRunner) EnsureLibvirtDaemonRunning(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if facts == nil || facts.InitSystem == nil || facts.InitSystem.Type == InitSystemUnknown {
		return errors.New("cannot ensure libvirtd state without init system info from facts")
	}

	serviceName := "libvirtd" // Common name for libvirtd service

	isActive, err := r.IsServiceActive(ctx, conn, facts, serviceName)
	if err != nil {
		// If IsServiceActive fails (e.g. service not found), try starting it anyway
		// as IsServiceActive might be restrictive.
	}
	if isActive {
		// If active, ensure it's enabled
		if errEnable := r.EnableService(ctx, conn, facts, serviceName); errEnable != nil {
			// Log warning or return error? For "Ensure", probably error.
			return errors.Wrapf(errEnable, "libvirtd service is active but failed to enable")
		}
		return nil // Active and enabled
	}

	// Not active, try to start
	if errStart := r.StartService(ctx, conn, facts, serviceName); errStart != nil {
		return errors.Wrapf(errStart, "failed to start libvirtd service")
	}
	// Started, now enable
	if errEnable := r.EnableService(ctx, conn, facts, serviceName); errEnable != nil {
		return errors.Wrapf(errEnable, "libvirtd service started but failed to enable")
	}
	return nil
}

// Placeholder for AttachNetInterface - requires complex XML manipulation or specific virsh commands
func (r *defaultRunner) AttachNetInterface(ctx context.Context, conn connector.Connector, vmName string, iface VMNetworkInterface, persistent bool) error {
	return errors.New("AttachNetInterface not fully implemented via CLI runner")
}

// Placeholder for DetachNetInterface
func (r *defaultRunner) DetachNetInterface(ctx context.Context, conn connector.Connector, vmName string, macAddress string, persistent bool) error {
	return errors.New("DetachNetInterface not fully implemented via CLI runner")
}

// Placeholder for ListNetInterfaces
func (r *defaultRunner) ListNetInterfaces(ctx context.Context, conn connector.Connector, vmName string) ([]VMNetworkInterfaceDetail, error) {
	return nil, errors.New("ListNetInterfaces not fully implemented via CLI runner")
}

// Placeholder for CreateSnapshot
func (r *defaultRunner) CreateSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName, description string, diskSpecs []VMSnapshotDiskSpec, noMetadata, halt, diskOnly, reuseExisting, quiesce, atomic bool) error {
	return errors.New("CreateSnapshot not fully implemented via CLI runner")
}

// Placeholder for DeleteSnapshot
func (r *defaultRunner) DeleteSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName string, children, metadata bool) error {
	return errors.New("DeleteSnapshot not fully implemented via CLI runner")
}

// Placeholder for ListSnapshots
func (r *defaultRunner) ListSnapshots(ctx context.Context, conn connector.Connector, vmName string) ([]VMSnapshotInfo, error) {
	return nil, errors.New("ListSnapshots not fully implemented via CLI runner")
}

// Placeholder for RevertToSnapshot
func (r *defaultRunner) RevertToSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName string, force, running bool) error {
	return errors.New("RevertToSnapshot not fully implemented via CLI runner")
}

// Placeholder for GetVMInfo (more detailed than dominfo)
func (r *defaultRunner) GetVMInfo(ctx context.Context, conn connector.Connector, vmName string) (*VMDetails, error) {
	return nil, errors.New("GetVMInfo (detailed XML parsing) not fully implemented via CLI runner")
}

// Placeholder for GetVNCPort
func (r *defaultRunner) GetVNCPort(ctx context.Context, conn connector.Connector, vmName string) (string, error) {
	return "", errors.New("GetVNCPort not fully implemented via CLI runner (requires XML parsing)")
}
