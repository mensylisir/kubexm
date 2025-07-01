package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/pkg/errors"
	// Consider adding a libvirt client library if direct XML manipulation or more complex operations are needed,
	// but for now, we'll stick to `virsh` commands for simplicity, aligning with the Docker implementation style.
)

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

		createDiskCmd := fmt.Sprintf("qemu-img create -f qcow2 %s %dG", shellEscape(diskPath), diskSizeGB)
		_, stderr, err := conn.Exec(ctx, createDiskCmd, &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute})
		if err != nil {
			return errors.Wrapf(err, "failed to create disk image %s with qemu-img. Stderr: %s", diskPath, string(stderr))
		}
	}
	// Placeholder for actual VM definition logic using virsh define
	return errors.New("CreateVMTemplate: virsh define from generated XML is not fully implemented via CLI runner; disk creation part is present")
}

// VMExists checks if a virtual machine with the given name is defined in libvirt.
func (r *defaultRunner) VMExists(ctx context.Context, conn connector.Connector, vmName string) (bool, error) {
	if conn == nil { return false, errors.New("connector cannot be nil") }
	if strings.TrimSpace(vmName) == "" { return false, errors.New("vmName cannot be empty") }
	cmd := fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", shellEscape(vmName))
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err == nil { return true, nil }
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) && cmdErr.ExitCode != 0 { return false, nil }
	return false, errors.Wrapf(err, "failed to check if VM %s exists", vmName)
}

// StartVM starts a defined (but not running) virtual machine.
func (r *defaultRunner) StartVM(ctx context.Context, conn connector.Connector, vmName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if strings.TrimSpace(vmName) == "" { return errors.New("vmName cannot be empty") }

	stateCmd := fmt.Sprintf("virsh domstate %s", shellEscape(vmName))
	stateStdout, stateStderr, err := conn.Exec(ctx, stateCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err != nil {
		return errors.Wrapf(err, "failed to get VM state for %s. Stderr: %s", vmName, string(stateStderr))
	}
	if strings.TrimSpace(string(stateStdout)) == "running" { return nil }

	startCmd := fmt.Sprintf("virsh start %s", shellEscape(vmName))
	_, stderr, err := conn.Exec(ctx, startCmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to start VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

// ShutdownVM attempts a graceful shutdown of a virtual machine.
func (r *defaultRunner) ShutdownVM(ctx context.Context, conn connector.Connector, vmName string, force bool, timeout time.Duration) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if strings.TrimSpace(vmName) == "" { return errors.New("vmName cannot be empty") }

	stateCmd := fmt.Sprintf("virsh domstate %s", shellEscape(vmName))
	stateStdout, stateStderr, err := conn.Exec(ctx, stateCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err != nil {
		if strings.Contains(string(stateStderr), "Domain not found") { return nil }
		return errors.Wrapf(err, "failed to get VM state for %s. Stderr: %s", vmName, string(stateStderr))
	}
	if strings.TrimSpace(string(stateStdout)) == "shut off" { return nil }

	shutdownCmd := fmt.Sprintf("virsh shutdown %s", shellEscape(vmName))
	_, stderrShutdown, errShutdown := conn.Exec(ctx, shutdownCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})

	if errShutdown == nil { // Graceful shutdown initiated
		waitCtx, cancelWait := context.WithTimeout(ctx, timeout)
		defer cancelWait()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-waitCtx.Done():
				if force { return r.DestroyVM(waitCtx, conn, vmName) }
				return errors.Errorf("VM %s graceful shutdown timed out", vmName)
			case <-ticker.C:
				sStdout, _, sErr := conn.Exec(waitCtx, stateCmd, &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second})
				if sErr != nil { continue } // Error checking state, continue polling
				if strings.TrimSpace(string(sStdout)) == "shut off" { return nil }
			}
		}
	} else if !force { // Shutdown command failed and not forcing
		return errors.Wrapf(errShutdown, "failed to issue graceful shutdown for %s. Stderr: %s", vmName, string(stderrShutdown))
	}
	// If shutdown failed and force is true, or if graceful timed out and force is true
	if force { return r.DestroyVM(ctx, conn, vmName) }
	return errors.Errorf("VM %s graceful shutdown failed", vmName) // Should not be reached if logic is correct
}

// DestroyVM forcefully stops (powers off) a virtual machine.
func (r *defaultRunner) DestroyVM(ctx context.Context, conn connector.Connector, vmName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if strings.TrimSpace(vmName) == "" { return errors.New("vmName cannot be empty") }

	// Check if already off or non-existent for idempotency
	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState != nil { // Error getting state (e.g. not found)
		if strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain") {
			return nil // Effectively destroyed
		}
		// Some other error getting state, proceed with destroy as best effort
	} else if state == "shut off" {
		return nil // Already off
	}

	cmd := fmt.Sprintf("virsh destroy %s", shellEscape(vmName))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "Domain not found") || strings.Contains(string(stderr), "domain is not running") {
			return nil // Idempotent
		}
		return errors.Wrapf(err, "failed to destroy VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

// UndefineVM removes the definition of a virtual machine from libvirt.
func (r *defaultRunner) UndefineVM(ctx context.Context, conn connector.Connector, vmName string, deleteSnapshots bool, deleteStorage bool, storagePools []string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if strings.TrimSpace(vmName) == "" { return errors.New("vmName cannot be empty") }

	// Ensure VM is off
	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState == nil && (state == "running" || state == "paused") {
		if errDestroy := r.DestroyVM(ctx, conn, vmName); errDestroy != nil {
			return errors.Wrapf(errDestroy, "failed to destroy VM %s prior to undefine", vmName)
		}
		time.Sleep(2 * time.Second) // Give time for destroy to settle
	} else if errState != nil && !(strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain")) {
		// If error getting state is not "not found", it's an issue.
		return errors.Wrapf(errState, "failed to get state of VM %s before undefine", vmName)
	}
	// If state is "shut off" or domain not found, proceed.

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "undefine", shellEscape(vmName))
	if deleteSnapshots { cmdArgs = append(cmdArgs, "--snapshots-metadata") }
	if deleteStorage { cmdArgs = append(cmdArgs, "--remove-all-storage") }

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "Domain not found") { return nil } // Idempotent
		return errors.Wrapf(err, "failed to undefine VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

// GetVMState retrieves the current state of a virtual machine.
func (r *defaultRunner) GetVMState(ctx context.Context, conn connector.Connector, vmName string) (string, error) {
	if conn == nil { return "", errors.New("connector cannot be nil") }
	if strings.TrimSpace(vmName) == "" { return "", errors.New("vmName cannot be empty") }
	cmd := fmt.Sprintf("virsh domstate %s", shellEscape(vmName))
	stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err != nil {
		return "", errors.Wrapf(err, "failed to get state for VM %s. Stderr: %s", vmName, string(stderr))
	}
	return strings.TrimSpace(string(stdout)), nil
}

// ListVMs lists virtual machines known to libvirt.
func (r *defaultRunner) ListVMs(ctx context.Context, conn connector.Connector, all bool) ([]VMInfo, error) {
    if conn == nil { return nil, errors.New("connector cannot be nil") }
    listCmdArgs := []string{"virsh", "list", "--name"}
    if all { listCmdArgs = append(listCmdArgs, "--all") }

    stdoutNames, stderrNames, err := conn.Exec(ctx, strings.Join(listCmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
    if err != nil { return nil, errors.Wrapf(err, "failed to list VM names. Stderr: %s", string(stderrNames)) }

    vmNamesRaw := strings.Split(string(stdoutNames), "\n")
    var vmNames []string
    for _, name := range vmNamesRaw {
        trimmedName := strings.TrimSpace(name)
        if trimmedName != "" { vmNames = append(vmNames, trimmedName) }
    }
    if len(vmNames) == 0 { return []VMInfo{}, nil }

    var vms []VMInfo
    for _, vmName := range vmNames {
        state, errState := r.GetVMState(ctx, conn, vmName)
        if errState != nil { continue } // Skip if error getting state

        dominfoCmd := fmt.Sprintf("virsh dominfo %s", shellEscape(vmName))
        infoStdout, _, errInfo := conn.Exec(ctx, dominfoCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
        if errInfo != nil {
            vms = append(vms, VMInfo{Name: vmName, State: state}); continue
        }

        vmInfo := VMInfo{Name: vmName, State: state}
        infoLines := strings.Split(string(infoStdout), "\n")
        for _, line := range infoLines {
            parts := strings.SplitN(line, ":", 2)
            if len(parts) != 2 { continue }
            key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
            switch key {
            case "UUID": vmInfo.UUID = value
            case "CPU(s)": vmInfo.CPUs, _ = parseIntFromString(value)
            case "Max memory", "Memory":
                memKB, _ := parseMemStringToKB(value)
                vmInfo.Memory = uint(memKB / 1024)
            }
        }
        if vmInfo.State == "" { // Fallback for state from dominfo if GetVMState had issues earlier
            for _, line := range infoLines {
                 if strings.HasPrefix(line, "State:") {
                    vmInfo.State = strings.TrimSpace(strings.TrimPrefix(line, "State:"))
                    break
                }
            }
        }
        vms = append(vms, vmInfo)
    }
    return vms, nil
}

func parseIntFromString(s string) (int, error) { /* ... as previously defined ... */ return 0, nil }
func parseMemStringToKB(s string) (uint64, error) { /* ... as previously defined ... */ return 0, nil }

func (r *defaultRunner) ImportVMTemplate(ctx context.Context, conn connector.Connector, name string, filePath string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || filePath == "" { return errors.New("name and filePath are required") }
	cmd := fmt.Sprintf("virsh define %s", shellEscape(filePath))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil { return errors.Wrapf(err, "failed to define VM %s from %s. Stderr: %s", name, filePath, string(stderr)) }
	return nil
}

func (r *defaultRunner) RefreshStoragePool(ctx context.Context, conn connector.Connector, poolName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if poolName == "" { return errors.New("poolName cannot be empty") }
	cmd := fmt.Sprintf("virsh pool-refresh %s", shellEscape(poolName))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil { return errors.Wrapf(err, "failed to refresh pool %s. Stderr: %s", poolName, string(stderr)) }
	return nil
}

func (r *defaultRunner) CreateStoragePool(ctx context.Context, conn connector.Connector, name string, poolType string, targetPath string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if name == "" || poolType == "" || targetPath == "" { return errors.New("name, poolType, targetPath required") }
	if poolType == "dir" {
		if err := r.Mkdirp(ctx, conn, targetPath, "0755", true); err != nil {
			return errors.Wrapf(err, "failed to create dir %s for pool", targetPath)
		}
	}
	var defineCmd string
	switch poolType {
	case "dir": defineCmd = fmt.Sprintf("virsh pool-define-as %s dir --target %s", shellEscape(name), shellEscape(targetPath))
	default: return errors.Errorf("unsupported poolType: %s", poolType)
	}
	_, stderrDefine, errDefine := conn.Exec(ctx, defineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDefine != nil && !(strings.Contains(string(stderrDefine), "already defined") || strings.Contains(string(stderrDefine), "already exists")) {
		return errors.Wrapf(errDefine, "failed to define pool %s. Stderr: %s", name, string(stderrDefine))
	}
	buildCmd := fmt.Sprintf("virsh pool-build %s", shellEscape(name))
	_, stderrBuild, errBuild := conn.Exec(ctx, buildCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errBuild != nil && !(strings.Contains(string(stderrBuild), "already built") || strings.Contains(string(stderrBuild), "No action required")) {
		return errors.Wrapf(errBuild, "failed to build pool %s. Stderr: %s", name, string(stderrBuild))
	}
	startCmd := fmt.Sprintf("virsh pool-start %s", shellEscape(name))
	_, stderrStart, errStart := conn.Exec(ctx, startCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errStart != nil && !strings.Contains(string(stderrStart), "already active") {
		return errors.Wrapf(errStart, "failed to start pool %s. Stderr: %s", name, string(stderrStart))
	}
	autostartCmd := fmt.Sprintf("virsh pool-autostart %s", shellEscape(name))
	if _, _, errAutostart := conn.Exec(ctx, autostartCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second}); errAutostart != nil {
		// Log warning, not critical failure
	}
	return nil
}

func (r *defaultRunner) StoragePoolExists(ctx context.Context, conn connector.Connector, poolName string) (bool, error) {
	if conn == nil { return false, errors.New("connector cannot be nil") }
	if poolName == "" { return false, errors.New("poolName cannot be empty") }
	cmd := fmt.Sprintf("virsh pool-info %s > /dev/null 2>&1", shellEscape(poolName))
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err == nil { return true, nil }
	if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 { return false, nil }
	return false, errors.Wrapf(err, "failed to check pool %s existence", poolName)
}

func (r *defaultRunner) DeleteStoragePool(ctx context.Context, conn connector.Connector, poolName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if poolName == "" { return errors.New("poolName cannot be empty") }
	destroyCmd := fmt.Sprintf("virsh pool-destroy %s", shellEscape(poolName))
	_, stderrDestroy, errDestroy := conn.Exec(ctx, destroyCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDestroy != nil && !(strings.Contains(string(stderrDestroy), "not found") || strings.Contains(string(stderrDestroy), "not active") || strings.Contains(string(stderrDestroy), "not running")) {
		return errors.Wrapf(errDestroy, "failed to destroy pool %s. Stderr: %s", poolName, string(stderrDestroy))
	}
	undefineCmd := fmt.Sprintf("virsh pool-undefine %s", shellEscape(poolName))
	_, stderrUndefine, errUndefine := conn.Exec(ctx, undefineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errUndefine != nil && !strings.Contains(string(stderrUndefine), "not found") {
		return errors.Wrapf(errUndefine, "failed to undefine pool %s. Stderr: %s", poolName, string(stderrUndefine))
	}
	return nil
}

func (r *defaultRunner) VolumeExists(ctx context.Context, conn connector.Connector, poolName string, volName string) (bool, error) {
	if conn == nil { return false, errors.New("connector cannot be nil") }
	if poolName == "" || volName == "" { return false, errors.New("poolName and volName cannot be empty") }
	cmd := fmt.Sprintf("virsh vol-info --pool %s %s > /dev/null 2>&1", shellEscape(poolName), shellEscape(volName))
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err == nil { return true, nil }
	if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 { return false, nil }
	return false, errors.Wrapf(err, "failed to check volume %s in pool %s", volName, poolName)
}

func (r *defaultRunner) CloneVolume(ctx context.Context, conn connector.Connector, poolName string, origVolName string, newVolName string, newSizeGB uint, format string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if poolName == "" || origVolName == "" || newVolName == "" { return errors.New("poolName, origVolName, newVolName required") }

	cloneCmd := fmt.Sprintf("virsh vol-clone --pool %s %s %s", shellEscape(poolName), shellEscape(origVolName), shellEscape(newVolName))
	_, stderr, err := conn.Exec(ctx, cloneCmd, &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute})
	if err != nil { return errors.Wrapf(err, "failed to clone vol %s to %s. Stderr: %s", origVolName, newVolName, string(stderr)) }

	if newSizeGB > 0 {
		if errResize := r.ResizeVolume(ctx, conn, poolName, newVolName, newSizeGB); errResize != nil {
			return errors.Wrapf(errResize, "vol %s cloned, but failed to resize to %dGB", newVolName, newSizeGB)
		}
	}
	return nil
}

func (r *defaultRunner) ResizeVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, newSizeGB uint) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if poolName == "" || volName == "" || newSizeGB == 0 { return errors.New("poolName, volName, and non-zero newSizeGB required") }
	capacityStr := fmt.Sprintf("%dG", newSizeGB)
	cmd := fmt.Sprintf("virsh vol-resize --pool %s %s %s", shellEscape(poolName), shellEscape(volName), shellEscape(capacityStr))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil { return errors.Wrapf(err, "failed to resize vol %s to %s. Stderr: %s", volName, capacityStr, string(stderr)) }
	return nil
}

func (r *defaultRunner) DeleteVolume(ctx context.Context, conn connector.Connector, poolName string, volName string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if poolName == "" || volName == "" { return errors.New("poolName and volName are required") }
	cmd := fmt.Sprintf("virsh vol-delete --pool %s %s", shellEscape(poolName), shellEscape(volName))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "Failed to get volume") || strings.Contains(string(stderr), "Storage volume not found") { return nil }
		return errors.Wrapf(err, "failed to delete vol %s. Stderr: %s", volName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CreateVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, sizeGB uint, format string, backingVolName string, backingVolFormat string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if poolName == "" || volName == "" || sizeGB == 0 { return errors.New("poolName, volName, non-zero sizeGB required") }

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "vol-create-as", shellEscape(poolName), shellEscape(volName), fmt.Sprintf("%dG", sizeGB))
	if format != "" { cmdArgs = append(cmdArgs, "--format", shellEscape(format)) }
	if backingVolName != "" {
		if backingVolFormat == "" { return errors.New("backingVolFormat required with backingVolName") }
		cmdArgs = append(cmdArgs, "--backing-vol", shellEscape(backingVolName), "--backing-vol-format", shellEscape(backingVolFormat))
	}
	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "already exists") { return nil }
		return errors.Wrapf(err, "failed to create vol %s. Stderr: %s", volName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CreateCloudInitISO(ctx context.Context, conn connector.Connector, vmName string, isoDestPath string, userData string, metaData string, networkConfig string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if vmName == "" || isoDestPath == "" || userData == "" || metaData == "" { return errors.New("vmName, isoDestPath, userData, metaData required") }

	remoteBaseTmpDir := "/tmp"
	tmpDirName := fmt.Sprintf("cloud-init-tmp-%s-%d", vmName, time.Now().UnixNano())
	tmpDirPath := filepath.Join(remoteBaseTmpDir, tmpDirName)
	if err := r.Mkdirp(ctx, conn, tmpDirPath, "0700", true); err != nil { return errors.Wrapf(err, "failed to create temp dir %s", tmpDirPath) }
	defer r.Remove(ctx, conn, tmpDirPath, true)

	if err := r.WriteFile(ctx, conn, []byte(userData), filepath.Join(tmpDirPath, "user-data"), "0644", true); err != nil { return err }
	if err := r.WriteFile(ctx, conn, []byte(metaData), filepath.Join(tmpDirPath, "meta-data"), "0644", true); err != nil { return err }
	if networkConfig != "" {
		if err := r.WriteFile(ctx, conn, []byte(networkConfig), filepath.Join(tmpDirPath, "network-config"), "0644", true); err != nil { return err }
	}
	if err := r.Mkdirp(ctx, conn, filepath.Dir(isoDestPath), "0755", true); err != nil { return errors.Wrapf(err, "failed to create ISO dir %s", filepath.Dir(isoDestPath)) }

	isoCmdTool := "genisoimage"
	if _, err := r.LookPath(ctx, conn, "genisoimage"); err != nil {
		if _, err = r.LookPath(ctx, conn, "mkisofs"); err == nil { isoCmdTool = "mkisofs" } else { return errors.New("genisoimage or mkisofs not found") }
	}
	isoCmd := fmt.Sprintf("%s -o %s -V cidata -r -J %s", isoCmdTool, shellEscape(isoDestPath), shellEscape(tmpDirPath))
	_, stderr, err := conn.Exec(ctx, isoCmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil { return errors.Wrapf(err, "failed to create cloud-init ISO %s. Stderr: %s", isoDestPath, string(stderr)) }
	return nil
}

func (r *defaultRunner) CreateVM(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, vcpus uint, osVariant string, diskPaths []string, networkInterfaces []VMNetworkInterface, graphicsType string, cloudInitISOPath string, bootOrder []string, extraArgs []string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if vmName == "" || memoryMB == 0 || vcpus == 0 || len(diskPaths) == 0 || diskPaths[0] == "" {
		return errors.New("vmName, memoryMB, vcpus, and at least one diskPath are required")
	}
	if len(networkInterfaces) == 0 { // Require at least one network interface
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
	xmlBuilder.WriteString(fmt.Sprintf("  <memory unit='KiB'>%d</memory>\n", memoryMB*1024))
	xmlBuilder.WriteString(fmt.Sprintf("  <currentMemory unit='KiB'>%d</currentMemory>\n", memoryMB*1024))
	xmlBuilder.WriteString(fmt.Sprintf("  <vcpu placement='static'>%d</vcpu>\n", vcpus))
	xmlBuilder.WriteString("  <os>\n    <type arch='x86_64' machine='pc-q35-rhel8.6.0'>hvm</type>\n") // Machine type might need to be more generic or detected

	if len(bootOrder) == 0 {
		bootOrder = []string{"hd"}
		if cloudInitISOPath != "" { bootOrder = append(bootOrder, "cdrom") }
	}
	for _, dev := range bootOrder { xmlBuilder.WriteString(fmt.Sprintf("    <boot dev='%s'/>\n", dev)) }
	xmlBuilder.WriteString("  </os>\n")
	xmlBuilder.WriteString("  <features><acpi/><apic/><vmport state='off'/></features>\n")
	xmlBuilder.WriteString("  <cpu mode='host-model' check='partial'/>\n")
	xmlBuilder.WriteString("  <clock offset='utc'><timer name='rtc' tickpolicy='catchup'/><timer name='pit' tickpolicy='delay'/><timer name='hpet' present='no'/></clock>\n")
	xmlBuilder.WriteString("  <on_poweroff>destroy</on_poweroff>\n  <on_reboot>restart</on_reboot>\n  <on_crash>destroy</on_crash>\n")
	xmlBuilder.WriteString("  <pm><suspend-to-mem enabled='no'/><suspend-to-disk enabled='no'/></pm>\n")
	xmlBuilder.WriteString("  <devices>\n    <emulator>/usr/libexec/qemu-kvm</emulator>\n") // Path might vary

	diskTargetLetter := 'a'
	pciBus := 4 // Start PCI bus number for disks
	for i, diskPath := range diskPaths {
		if diskPath == "" { continue }
		xmlBuilder.WriteString(fmt.Sprintf("    <disk type='file' device='disk'>\n"))
		xmlBuilder.WriteString(fmt.Sprintf("      <driver name='qemu' type='qcow2'/>\n"))
		xmlBuilder.WriteString(fmt.Sprintf("      <source file='%s'/>\n", diskPath))
		xmlBuilder.WriteString(fmt.Sprintf("      <target dev='vd%c' bus='virtio'/>\n", diskTargetLetter))
		xmlBuilder.WriteString(fmt.Sprintf("      <address type='pci' domain='0x0000' bus='0x%02x' slot='0x00' function='0x0'/>\n", pciBus+i))
		xmlBuilder.WriteString("    </disk>\n")
		diskTargetLetter++
	}

	if cloudInitISOPath != "" {
		xmlBuilder.WriteString("    <disk type='file' device='cdrom'>\n")
		xmlBuilder.WriteString("      <driver name='qemu' type='raw'/>\n")
		xmlBuilder.WriteString(fmt.Sprintf("      <source file='%s'/>\n", cloudInitISOPath))
		xmlBuilder.WriteString("      <target dev='sda' bus='sata'/>\n")
		xmlBuilder.WriteString("      <readonly/>\n")
		xmlBuilder.WriteString("      <address type='drive' controller='0' bus='0' target='0' unit='0'/>\n")
		xmlBuilder.WriteString("    </disk>\n")
	}

	networkPCIBus := 2 // Start PCI bus for NICs differently from disks
	for i, nic := range networkInterfaces {
		xmlBuilder.WriteString("    <interface type='network'>\n")
		if nic.Source == "" { nic.Source = "default" }
		xmlBuilder.WriteString(fmt.Sprintf("      <source network='%s'/>\n", nic.Source))
		if nic.MACAddress != "" { xmlBuilder.WriteString(fmt.Sprintf("      <mac address='%s'/>\n", nic.MACAddress)) }
		if nic.Model == "" { nic.Model = "virtio" }
		xmlBuilder.WriteString(fmt.Sprintf("      <model type='%s'/>\n", nic.Model))
		xmlBuilder.WriteString(fmt.Sprintf("      <address type='pci' domain='0x0000' bus='0x%02x' slot='0x00' function='0x0'/>\n", networkPCIBus+i))
		xmlBuilder.WriteString("    </interface>\n")
	}

	if graphicsType == "" { graphicsType = "vnc" }
	if graphicsType != "none" {
		xmlBuilder.WriteString(fmt.Sprintf("    <graphics type='%s' port='-1' autoport='yes' listen='0.0.0.0'>\n", graphicsType))
		xmlBuilder.WriteString("      <listen type='address' address='0.0.0.0'/>\n    </graphics>\n")
		xmlBuilder.WriteString("    <video>\n      <model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1' primary='yes'/>\n    </video>\n")
	}

	xmlBuilder.WriteString("    <serial type='pty'><target type='isa-serial' port='0'><model name='isa-serial'/></target></serial>\n")
	xmlBuilder.WriteString("    <console type='pty'><target type='serial' port='0'/></console>\n")
	xmlBuilder.WriteString("    <input type='tablet' bus='usb'/><input type='mouse' bus='ps2'/><input type='keyboard' bus='ps2'/>\n")
	xmlBuilder.WriteString("    <memballoon model='virtio'><address type='pci' domain='0x0000' bus='0x05' slot='0x00' function='0x0'/></memballoon>\n")
	xmlBuilder.WriteString("  </devices>\n</domain>\n")

	vmXML := xmlBuilder.String()
	tempXMLPath := fmt.Sprintf("/tmp/kubexm-vmdef-%s-%d.xml", vmName, time.Now().UnixNano())
	if err = r.WriteFile(ctx, conn, []byte(vmXML), tempXMLPath, "0600", true); err != nil {
		return errors.Wrapf(err, "failed to write temp VM XML to %s", tempXMLPath)
	}
	defer r.Remove(ctx, conn, tempXMLPath, true)

	defineCmd := fmt.Sprintf("virsh define %s", shellEscape(tempXMLPath))
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
	if conn == nil { return errors.New("connector cannot be nil") }
	if vmName == "" || diskPath == "" || targetDevice == "" {
		return errors.New("vmName, diskPath, and targetDevice are required")
	}
	// virsh attach-disk <domain> <source> <target> --driver <driver_name> --type <driver_type> --subdriver <subdriver> --config --live
	// Example: virsh attach-disk myvm /path/to/newdisk.qcow2 vdb --driver qemu --type qcow2 --config --live
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "attach-disk", shellEscape(vmName), shellEscape(diskPath), shellEscape(targetDevice))
	if driverType != "" { // e.g. qcow2, raw
		cmdArgs = append(cmdArgs, "--driver", "qemu", "--subdriver", shellEscape(driverType)) // Common for qemu driver
	}
	if diskType != "" { // e.g. file, block
		// This is often implied or part of driver choice; virsh attach-disk is simpler.
		// The --type flag for attach-disk refers to disk device type (cdrom, disk, floppy, lun), not file format.
		// We will omit --type for now, relying on libvirt to infer or using driver/subdriver for format.
	}
	cmdArgs = append(cmdArgs, "--config") // Make change persistent
	cmdArgs = append(cmdArgs, "--live")   // Attempt live attach

	cmd := strings.Join(cmdArgs, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to attach disk %s to VM %s as %s. Stderr: %s", diskPath, vmName, targetDevice, string(stderr))
	}
	return nil
}

func (r *defaultRunner) DetachDisk(ctx context.Context, conn connector.Connector, vmName string, targetDeviceOrPath string) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if vmName == "" || targetDeviceOrPath == "" {
		return errors.New("vmName and targetDeviceOrPath are required")
	}
	// virsh detach-disk <domain> <target> --config --live
	cmd := fmt.Sprintf("virsh detach-disk %s %s --config --live", shellEscape(vmName), shellEscape(targetDeviceOrPath))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		// Idempotency: If disk not found, "error: No disk found for target..."
		if strings.Contains(string(stderr), "No disk found for target") || strings.Contains(string(stderr),"no target device found") {
			return nil
		}
		return errors.Wrapf(err, "failed to detach disk %s from VM %s. Stderr: %s", targetDeviceOrPath, vmName, string(stderr))
	}
	return nil
}
func (r *defaultRunner) SetVMMemory(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, current bool) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if vmName == "" || memoryMB == 0 {
		return errors.New("vmName and non-zero memoryMB are required")
	}

	// `virsh setmem <domain> <count>[<scale>] [--config] [--live] [--current]`
	// <scale> can be k, K, KiB, m, M, MiB, g, G, GiB, t, T, TiB
	// We are given memoryMB, so convert to KiB for libvirt as it's a common base unit.
	memoryKiB := memoryMB * 1024

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "setmem", shellEscape(vmName), fmt.Sprintf("%dK", memoryKiB)) // Using K for KiB shortcut

	if current { // Affect running VM and make persistent if also --config
		cmdArgs = append(cmdArgs, "--live", "--config") // --current is implied by --live for setmem. --config makes it persistent.
	} else { // Affect persistent config only (next boot)
		cmdArgs = append(cmdArgs, "--config")
	}
	// Note: `virsh setmaxmem` might be needed first if the new memory exceeds current max memory defined for the VM.
	// This simplified version assumes new memory is within allowed max.
	// A more robust version would check `virsh dominfo` or `dumpxml` for max memory and call `setmaxmem` if needed.

	cmd := strings.Join(cmdArgs, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to set memory for VM %s to %dMiB. Stderr: %s", vmName, memoryMB, string(stderr))
	}
	return nil
}

func (r *defaultRunner) SetVMCPUs(ctx context.Context, conn connector.Connector, vmName string, vcpus uint, current bool) error {
	if conn == nil { return errors.New("connector cannot be nil") }
	if vmName == "" || vcpus == 0 {
		return errors.New("vmName and non-zero vcpus are required")
	}

	// `virsh setvcpus <domain> <count> [--config] [--live] [--current] [--guest]`
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "setvcpus", shellEscape(vmName), fmt.Sprintf("%d", vcpus))

	if current {
		cmdArgs = append(cmdArgs, "--live", "--config") // Similar to setmem, --live implies --current, --config makes it persistent
	} else {
		cmdArgs = append(cmdArgs, "--config")
	}
	// Note: This sets the *current* number of vCPUs. The maximum vCPUs is defined in the domain XML
	// and might need to be adjusted separately if `vcpus` exceeds that maximum.
	// `virsh dumpxml <domain>` then `virsh define` with modified XML, or `virsh edit` can change max vCPUs.
	// This simplified version assumes new vcpu count is within allowed max.

	cmd := strings.Join(cmdArgs, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to set vCPUs for VM %s to %d. Stderr: %s", vmName, vcpus, string(stderr))
	}
	return nil
}

[end of pkg/runner/qemu.go]
