package runner

import (
	"context"
	"crypto/rand"
	"encoding/xml"
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

type OSVariant struct {
	ID string `xml:"id,attr"`
}

type Domain struct {
	XMLName       xml.Name       `xml:"domain"`
	Type          string         `xml:"type,attr"`
	Name          string         `xml:"name"`
	UUID          string         `xml:"uuid"`
	Memory        DomainMemory   `xml:"memory"`
	CurrentMemory DomainMemory   `xml:"currentMemory"`
	VCPU          DomainVCPU     `xml:"vcpu"`
	OS            DomainOS       `xml:"os"`
	Features      DomainFeatures `xml:"features"`
	CPU           DomainCPU      `xml:"cpu"`
	Clock         DomainClock    `xml:"clock"`
	OnPoweroff    string         `xml:"on_poweroff"`
	OnReboot      string         `xml:"on_reboot"`
	OnCrash       string         `xml:"on_crash"`
	PM            DomainPM       `xml:"pm"`
	Devices       DomainDevices  `xml:"devices"`
}
type DomainMemory struct {
	Unit  string `xml:"unit,attr"`
	Value uint   `xml:",chardata"`
}
type DomainVCPU struct {
	Placement string `xml:"placement,attr"`
	Value     uint   `xml:",chardata"`
}
type DomainOS struct {
	Type    DomainOSType `xml:"type"`
	Boot    []DomainBoot `xml:"boot"`
	Variant OSVariant    `xml:"os_variant"`
}
type DomainOSType struct {
	Arch    string `xml:"arch,attr"`
	Machine string `xml:"machine,attr,omitempty"`
	Value   string `xml:",chardata"`
}
type DomainBoot struct {
	Dev string `xml:"dev,attr"`
}
type DomainFeatures struct {
	ACPI   *struct{}     `xml:"acpi"`
	APIC   *struct{}     `xml:"apic"`
	VMPort *DomainVMPort `xml:"vmport"`
}

type DomainVMPort struct {
	State string `xml:"state,attr"`
}

type DomainCPU struct {
	Mode  string `xml:"mode,attr"`
	Check string `xml:"check,attr"`
}
type DomainClock struct {
	Offset string        `xml:"offset,attr"`
	Timer  []DomainTimer `xml:"timer"`
}
type DomainTimer struct {
	Name       string `xml:"name,attr"`
	TickPolicy string `xml:"tickpolicy,attr,omitempty"`
	Present    string `xml:"present,attr,omitempty"`
}
type DomainPM struct {
	SuspendToMem  DomainSuspend `xml:"suspend-to-mem"`
	SuspendToDisk DomainSuspend `xml:"suspend-to-disk"`
}
type DomainSuspend struct {
	Enabled string `xml:"enabled,attr"`
}
type DomainDevices struct {
	Emulator   string            `xml:"emulator"`
	Disks      []DomainDisk      `xml:"disk"`
	Interfaces []DomainInterface `xml:"interface"`
	Serials    []DomainSerial    `xml:"serial"`
	Consoles   []DomainConsole   `xml:"console"`
	Channels   []DomainChannel   `xml:"channel"`
	Inputs     []DomainInput     `xml:"input"`
	Graphics   []DomainGraphic   `xml:"graphics"`
	Videos     []DomainVideo     `xml:"video"`
	MemBalloon *DomainMemBalloon `xml:"memballoon"`
}
type DomainDisk struct {
	Type     string           `xml:"type,attr"`
	Device   string           `xml:"device,attr"`
	Driver   DomainDiskDriver `xml:"driver"`
	Source   DomainSource     `xml:"source"`
	Target   DomainTarget     `xml:"target"`
	ReadOnly *struct{}        `xml:"readonly,omitempty"`
	Address  *DomainAddress   `xml:"address"`
}
type DomainDiskDriver struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Cache string `xml:"cache,attr,omitempty"`
	IO    string `xml:"io,attr,omitempty"`
}
type DomainSource struct {
	File    string `xml:"file,attr,omitempty"`
	Network string `xml:"network,attr,omitempty"`
	Bridge  string `xml:"bridge,attr,omitempty"`
}
type DomainTarget struct {
	Dev string `xml:"dev,attr"`
	Bus string `xml:"bus,attr"`
}
type DomainAddress struct {
	Type       string `xml:"type,attr"`
	Controller string `xml:"controller,attr,omitempty"`
	Bus        string `xml:"bus,attr"`
	Target     string `xml:"target,attr,omitempty"`
	Unit       string `xml:"unit,attr,omitempty"`
	Port       string `xml:"port,attr,omitempty"`
	Slot       string `xml:"slot,attr"`
	Function   string `xml:"function,attr"`
	Domain     string `xml:"domain,attr,omitempty"`
}
type DomainInterface struct {
	Type    string         `xml:"type,attr"`
	Source  DomainSource   `xml:"source"`
	MAC     *DomainMAC     `xml:"mac,omitempty"`
	Model   DomainModel    `xml:"model"`
	Address *DomainAddress `xml:"address"`
	Target  *DomainTarget  `xml:"target,omitempty"`
	XMLName xml.Name       `xml:"interface"`
}

type DomainMAC struct {
	Address string `xml:"address,attr"`
}
type DomainModel struct {
	Type string `xml:"type,attr"`
}
type DomainSerial struct {
	Type   string              `xml:"type,attr"`
	Target *DomainTargetSerial `xml:"target"`
}
type DomainTargetSerial struct {
	Type  string       `xml:"type,attr,omitempty"`
	Port  *int         `xml:"port,attr"`
	Model *DomainModel `xml:"model,omitempty"`
}
type DomainConsole struct {
	Type   string               `xml:"type,attr"`
	Target *DomainTargetConsole `xml:"target"`
}
type DomainTargetConsole struct {
	Type string `xml:"type,attr,omitempty"`
	Port *int   `xml:"port,attr"`
}
type DomainChannel struct {
	Type    string               `xml:"type,attr"`
	Target  *DomainTargetChannel `xml:"target"`
	Address *DomainAddress       `xml:"address"`
}
type DomainTargetChannel struct {
	Type string `xml:"type,attr,omitempty"`
	Name string `xml:"name,attr,omitempty"`
}
type DomainInput struct {
	Type string `xml:"type,attr"`
	Bus  string `xml:"bus,attr"`
}
type DomainGraphic struct {
	Type     string         `xml:"type,attr"`
	Port     string         `xml:"port,attr"`
	Autoport string         `xml:"autoport,attr"`
	Listen   []DomainListen `xml:"listen"`
	Password string         `xml:"passwd,attr,omitempty"`
	Keymap   string         `xml:"keymap,attr,omitempty"`
}
type DomainListen struct {
	Type    string `xml:"type,attr"`
	Address string `xml:"address,attr,omitempty"`
	Network string `xml:"network,attr,omitempty"`
}
type DomainVideo struct {
	Model DomainVideoModel `xml:"model"`
}
type DomainVideoModel struct {
	Type    string `xml:"type,attr"`
	VRAM    int    `xml:"vram,attr"`
	Heads   int    `xml:"heads,attr"`
	Primary string `xml:"primary,attr"`
}
type DomainMemBalloon struct {
	Model   string         `xml:"model,attr"`
	Address *DomainAddress `xml:"address"`
}

type VMSnapshots struct {
	XMLName  xml.Name     `xml:"domainsnapshots"`
	Snapshot []VMSnapshot `xml:"domainsnapshot"`
}
type VMSnapshot struct {
	XMLName      xml.Name          `xml:"domainsnapshot"`
	Name         string            `xml:"name"`
	Description  string            `xml:"description"`
	State        string            `xml:"state"`
	Parent       *VMSnapshotParent `xml:"parent"`
	CreationTime int64             `xml:"creationTime"`
	Disks        *VMSnapshotDisks  `xml:"disks"`
	Children     []VMSnapshot      `xml:"children>domainsnapshot"`
}
type VMSnapshotParent struct {
	Name string `xml:"name"`
}
type VMSnapshotDisks struct {
	Disks []VMSnapshotDisk `xml:"disk"`
}
type VMSnapshotDisk struct {
	Name     string               `xml:"name,attr"`
	Snapshot string               `xml:"snapshot,attr"`
	Source   VMSnapshotDiskSource `xml:"source"`
}

type VMSnapshotDiskSource struct {
	File string `xml:"file,attr"`
}

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

func (r *defaultRunner) CreateVMTemplate(ctx context.Context, conn connector.Connector, name string, osVariant string, memoryMB uint, vcpus uint, diskPath string, diskSizeGB uint, network string, graphicsType string, cloudInitISOPath string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if name == "" || diskPath == "" || osVariant == "" || memoryMB == 0 || vcpus == 0 || diskSizeGB == 0 {
		return errors.New("name, osVariant, memoryMB, vcpus, diskPath and diskSizeGB are required for VM template")
	}

	exists, err := r.Exists(ctx, conn, diskPath)
	if err != nil {
		return errors.Wrapf(err, "failed to check if disk %s exists", diskPath)
	}

	if !exists {
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
	return errors.New("CreateVMTemplate: virsh define from generated XML is not fully implemented via CLI runner; disk creation part is present")
}

func (r *defaultRunner) VMExists(ctx context.Context, conn connector.Connector, vmName string) (bool, error) {
	if conn == nil {
		return false, errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return false, errors.New("vmName cannot be empty")
	}
	cmd := fmt.Sprintf("virsh dominfo %s > /dev/null 2>&1", vmName)
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err == nil {
		return true, nil
	}
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) && cmdErr.ExitCode != 0 { // Any non-zero exit code from dominfo likely means not found or other error
		// More specific check could parse stderr for "Domain not found" or "error: failed to get domain"
		return false, nil // Treat non-zero exit as "does not exist" for simplicity of this check
	}
	return false, errors.Wrapf(err, "failed to check if VM %s exists", vmName)
}

func (r *defaultRunner) StartVM(ctx context.Context, conn connector.Connector, vmName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}
	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState != nil {
		return errors.Wrapf(errState, "failed to get current state of VM %s before starting", vmName)
	}
	if state == "running" {
		return nil // Already running
	}
	if state != "shut off" && state != "pmsuspended" {
		return errors.Errorf("VM %s is in state '%s', cannot start", vmName, state)
	}

	startCmd := fmt.Sprintf("virsh start %s", vmName)
	_, stderr, err := conn.Exec(ctx, startCmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to start VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) ShutdownVM(ctx context.Context, conn connector.Connector, vmName string, force bool, timeout time.Duration) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}

	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState != nil {
		if strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain") {
			return nil
		}
		return errors.Wrapf(errState, "failed to get VM state for %s before shutdown", vmName)
	}
	if state == "shut off" {
		return nil // Already shut off
	}
	if state != "running" && state != "paused" {
		return errors.Errorf("VM %s is in state '%s', cannot initiate shutdown", vmName, state)
	}

	shutdownCmd := fmt.Sprintf("virsh shutdown %s", vmName)
	_, stderrShutdown, errShutdown := conn.Exec(ctx, shutdownCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})

	if errShutdown == nil {
		waitCtx, cancelWait := context.WithTimeout(ctx, timeout)
		defer cancelWait()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-waitCtx.Done():
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

	if force {
		return r.DestroyVM(ctx, conn, vmName)
	}
	return errors.Wrapf(errShutdown, "failed to issue graceful shutdown for VM %s. Stderr: %s", vmName, string(stderrShutdown))
}

func (r *defaultRunner) DestroyVM(ctx context.Context, conn connector.Connector, vmName string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}

	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState != nil {
		if strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain") {
			return nil
		}
	} else if state == "shut off" {
		return nil // Already off
	}

	cmd := fmt.Sprintf("virsh destroy %s", vmName)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "Domain not found") || strings.Contains(string(stderr), "domain is not running") {
			return nil
		}
		return errors.Wrapf(err, "failed to destroy VM %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) UndefineVM(ctx context.Context, conn connector.Connector, vmName string, deleteSnapshots bool, deleteStorage bool, storagePools []string) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if strings.TrimSpace(vmName) == "" {
		return errors.New("vmName cannot be empty")
	}

	state, errState := r.GetVMState(ctx, conn, vmName)
	if errState == nil && (state == "running" || state == "paused") {
		if errDestroy := r.DestroyVM(ctx, conn, vmName); errDestroy != nil {
			return errors.Wrapf(errDestroy, "failed to destroy VM %s prior to undefine", vmName)
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	} else if errState != nil && !(strings.Contains(errState.Error(), "Domain not found") || strings.Contains(errState.Error(), "failed to get domain")) {
		return errors.Wrapf(errState, "failed to get state of VM %s before undefine", vmName)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "undefine", vmName)
	if deleteSnapshots {
		cmdArgs = append(cmdArgs, "--snapshots-metadata")
	}
	if deleteStorage {
		cmdArgs = append(cmdArgs, "--remove-all-storage")
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

func (r *defaultRunner) ListVMs(ctx context.Context, conn connector.Connector, all bool) ([]VMInfo, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	listCmdArgs := []string{"virsh", "list", "--name"}
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
		details, err := r.GetVMInfo(ctx, conn, vmName)
		if err != nil {
			state, _ := r.GetVMState(ctx, conn, vmName)
			vms = append(vms, VMInfo{Name: vmName, State: state, Error: err.Error()})
			continue
		}
		vms = append(vms, details.VMInfo)
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
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
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

	exists, errExists := r.StoragePoolExists(ctx, conn, name)
	if errExists != nil {
		return errors.Wrapf(errExists, "failed to check if storage pool %s exists", name)
	}
	if exists {
		return nil
	}

	if poolType == "dir" {
		if err := r.Mkdirp(ctx, conn, targetPath, "0755", true); err != nil {
			return errors.Wrapf(err, "failed to create directory %s for pool %s", targetPath, name)
		}
	}

	var defineCmd string
	switch poolType {
	case "dir":
		defineCmd = fmt.Sprintf("virsh pool-define-as %s dir --target %s", name, targetPath)
	default:
		return errors.Errorf("unsupported poolType: %s", poolType)
	}

	_, stderrDefine, errDefine := conn.Exec(ctx, defineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDefine != nil {
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
	}
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

	destroyCmd := fmt.Sprintf("virsh pool-destroy %s", poolName)
	_, stderrDestroy, errDestroy := conn.Exec(ctx, destroyCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDestroy != nil {
		if !(strings.Contains(string(stderrDestroy), "not found") ||
			strings.Contains(string(stderrDestroy), "not active") ||
			strings.Contains(string(stderrDestroy), "not running")) {
			return errors.Wrapf(errDestroy, "failed to destroy pool %s. Stderr: %s", poolName, string(stderrDestroy))
		}
	}

	undefineCmd := fmt.Sprintf("virsh pool-undefine %s", poolName)
	_, stderrUndefine, errUndefine := conn.Exec(ctx, undefineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errUndefine != nil {
		if !strings.Contains(string(stderrUndefine), "not found") {
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

	cloneCmdArgs := []string{"virsh", "vol-clone", "--pool", poolName, origVolName, newVolName}
	if format != "" {
		cloneCmdArgs = append(cloneCmdArgs, "--new-format", format)
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cloneCmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute}) // Cloning can take time
	if err != nil {
		return errors.Wrapf(err, "failed to clone volume %s to %s in pool %s. Stderr: %s", origVolName, newVolName, poolName, string(stderr))
	}

	if newSizeGB > 0 {
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
	capacityStr := fmt.Sprintf("%dG", newSizeGB)
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

	exists, errExists := r.VolumeExists(ctx, conn, poolName, volName)
	if errExists != nil {
		return errors.Wrapf(errExists, "failed to check if volume %s in pool %s exists", volName, poolName)
	}
	if exists {
		return nil
	}

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

	remoteBaseTmpDir := "/tmp"
	tmpDirName := fmt.Sprintf("kubexm-cloud-init-tmp-%s-%d", vmName, time.Now().UnixNano())
	tmpDirPath := filepath.Join(remoteBaseTmpDir, tmpDirName)

	if err := r.Mkdirp(ctx, conn, tmpDirPath, "0700", true); err != nil {
		return errors.Wrapf(err, "failed to create temporary directory %s on remote host", tmpDirPath)
	}
	defer r.Remove(ctx, conn, tmpDirPath, true, true)

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

	isoDir := filepath.Dir(isoDestPath)
	if err := r.Mkdirp(ctx, conn, isoDir, "0755", true); err != nil {
		return errors.Wrapf(err, "failed to create directory %s for ISO image", isoDir)
	}

	isoCmdTool := "genisoimage"
	if _, err := r.LookPath(ctx, conn, "genisoimage"); err != nil {
		if _, errMkisofs := r.LookPath(ctx, conn, "mkisofs"); errMkisofs == nil {
			isoCmdTool = "mkisofs"
		} else {
			return errors.New("neither genisoimage nor mkisofs found on the remote host")
		}
	}

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

func (r *defaultRunner) CreateVM(
	ctx context.Context,
	conn connector.Connector,
	facts *Facts,
	vmName string,
	memoryMB uint,
	vcpus uint,
	osVariant string,
	diskPaths []string,
	networkInterfaces []VMNetworkInterface,
	graphicsType string,
	cloudInitISOPath string,
	bootOrder []string,
	extraArgs []string,
) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || memoryMB == 0 || vcpus == 0 || len(diskPaths) == 0 || diskPaths[0] == "" {
		return errors.New("vmName, memoryMB, vcpus, and at least one diskPath are required")
	}
	if facts == nil || facts.OS == nil {
		return errors.New("facts (including OS) are required to create a VM")
	}

	exists, err := r.VMExists(ctx, conn, vmName)
	if err != nil {
		return errors.Wrapf(err, "failed to check if VM %s already exists", vmName)
	}
	if exists {
		return errors.Errorf("VM %s already exists", vmName)
	}

	emulatorPath, err := r.LookPath(ctx, conn, "qemu-system-"+facts.OS.Arch)
	if err != nil {
		emulatorPath, err = r.LookPath(ctx, conn, "qemu-kvm")
		if err != nil {
			return errors.New("could not find a valid QEMU emulator (qemu-system-<arch> or qemu-kvm) on the host")
		}
	}

	port0 := 0
	domain := Domain{
		Type:          "kvm",
		Name:          vmName,
		UUID:          generateUUID(),
		Memory:        DomainMemory{Unit: "MiB", Value: memoryMB},
		CurrentMemory: DomainMemory{Unit: "MiB", Value: memoryMB},
		VCPU:          DomainVCPU{Placement: "static", Value: vcpus},
		OS: DomainOS{
			Type:    DomainOSType{Arch: facts.OS.Arch, Value: "hvm"},
			Variant: OSVariant{ID: osVariant},
		},
		Features: DomainFeatures{
			ACPI:   &struct{}{},
			APIC:   &struct{}{},
			VMPort: &DomainVMPort{State: "off"},
		},
		CPU:        DomainCPU{Mode: "host-passthrough", Check: "none"},
		Clock:      DomainClock{Offset: "utc", Timer: []DomainTimer{{Name: "rtc", TickPolicy: "catchup"}, {Name: "pit", TickPolicy: "delay"}, {Name: "hpet", Present: "no"}}},
		OnPoweroff: "destroy",
		OnReboot:   "restart",
		OnCrash:    "destroy",
		PM:         DomainPM{SuspendToMem: DomainSuspend{Enabled: "no"}, SuspendToDisk: DomainSuspend{Enabled: "no"}},
		Devices: DomainDevices{
			Emulator: emulatorPath,
		},
	}

	if len(bootOrder) == 0 {
		bootOrder = []string{"hd"}
		if cloudInitISOPath != "" {
			bootOrder = append(bootOrder, "cdrom")
		}
	}
	for _, dev := range bootOrder {
		domain.OS.Boot = append(domain.OS.Boot, DomainBoot{Dev: dev})
	}

	pciSlot := 4
	diskTargetLetter := 'a'
	for _, diskPath := range diskPaths {
		domain.Devices.Disks = append(domain.Devices.Disks, DomainDisk{
			Type:    "file",
			Device:  "disk",
			Driver:  DomainDiskDriver{Name: "qemu", Type: "qcow2", Cache: "none", IO: "native"},
			Source:  DomainSource{File: diskPath},
			Target:  DomainTarget{Dev: fmt.Sprintf("vd%c", diskTargetLetter), Bus: "virtio"},
			Address: &DomainAddress{Type: "pci", Bus: "0x01", Slot: fmt.Sprintf("0x%02x", pciSlot), Function: "0x0", Domain: "0x0000"},
		})
		diskTargetLetter++
		pciSlot++
	}

	if cloudInitISOPath != "" {
		domain.Devices.Disks = append(domain.Devices.Disks, DomainDisk{
			Type:     "file",
			Device:   "cdrom",
			Driver:   DomainDiskDriver{Name: "qemu", Type: "raw"},
			Source:   DomainSource{File: cloudInitISOPath},
			Target:   DomainTarget{Dev: "sda", Bus: "sata"},
			ReadOnly: &struct{}{},
			Address:  &DomainAddress{Type: "drive", Controller: "0", Bus: "0", Target: "0", Unit: "0"},
		})
	}

	for _, nic := range networkInterfaces {

		var domainSource DomainSource
		if nic.Type == "network" {
			domainSource.Network = nic.Source
		} else if nic.Type == "bridge" {
			domainSource.Bridge = nic.Source
		}

		iface := DomainInterface{
			Type:    nic.Type,
			Source:  domainSource,
			Model:   DomainModel{Type: "virtio"},
			Address: &DomainAddress{Type: "pci", Bus: "0x01", Slot: fmt.Sprintf("0x%02x", pciSlot), Function: "0x0", Domain: "0x0000"},
		}
		if nic.MACAddress != "" {
			iface.MAC = &DomainMAC{Address: nic.MACAddress}
		}
		domain.Devices.Interfaces = append(domain.Devices.Interfaces, iface)
		pciSlot++
	}

	domain.Devices.Serials = []DomainSerial{
		{Type: "pty", Target: &DomainTargetSerial{Port: &port0, Model: &DomainModel{Type: "isa-serial"}}},
	}
	domain.Devices.Consoles = []DomainConsole{
		{Type: "pty", Target: &DomainTargetConsole{Type: "serial", Port: &port0}},
	}
	domain.Devices.Channels = []DomainChannel{
		{
			Type:    "unix",
			Target:  &DomainTargetChannel{Type: "virtio", Name: "org.qemu.guest_agent.0"},
			Address: &DomainAddress{Type: "virtio-serial", Controller: "0", Bus: "0", Port: "1"},
		},
	}
	domain.Devices.Inputs = []DomainInput{
		{Type: "tablet", Bus: "usb"},
		{Type: "mouse", Bus: "ps2"},
		{Type: "keyboard", Bus: "ps2"},
	}

	if graphicsType == "" {
		graphicsType = "vnc"
	}
	if graphicsType != "none" {
		domain.Devices.Graphics = []DomainGraphic{
			{
				Type:     graphicsType,
				Port:     "-1",
				Autoport: "yes",
				Listen:   []DomainListen{{Type: "address", Address: "0.0.0.0"}},
			},
		}
		domain.Devices.Videos = []DomainVideo{
			{Model: DomainVideoModel{Type: "qxl", VRAM: 65536, Heads: 1, Primary: "yes"}},
		}
	}

	domain.Devices.MemBalloon = &DomainMemBalloon{
		Model:   "virtio",
		Address: &DomainAddress{Type: "pci", Bus: "0x01", Slot: fmt.Sprintf("0x%02x", pciSlot), Function: "0x0", Domain: "0x0000"},
	}

	xmlHeader := `<?xml version="1.0" encoding="UTF-8"?>`
	xmlBytes, err := xml.MarshalIndent(domain, "  ", "    ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal VM domain to XML")
	}
	vmXML := xmlHeader + "\n" + string(xmlBytes)

	tempXMLPath := fmt.Sprintf("/tmp/kubexm-vmdef-%s-%d.xml", vmName, time.Now().UnixNano())
	if err = r.WriteFile(ctx, conn, []byte(vmXML), tempXMLPath, "0600", true); err != nil {
		return errors.Wrapf(err, "failed to write temp VM XML to %s", tempXMLPath)
	}
	defer r.Remove(ctx, conn, tempXMLPath, true, true)

	defineCmd := fmt.Sprintf("virsh define %s", tempXMLPath)
	_, stderrDefine, errDefine := conn.Exec(ctx, defineCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if errDefine != nil {
		return errors.Wrapf(errDefine, "failed to define VM %s. Stderr: %s\n--- XML DEFINITION ---\n%s", vmName, string(stderrDefine), vmXML)
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
	cmdArgs = append(cmdArgs, "--config", "--live")

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
			return nil
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

func (r *defaultRunner) EnsureLibvirtDaemonRunning(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if facts == nil || facts.InitSystem == nil || facts.InitSystem.Type == InitSystemUnknown {
		return errors.New("cannot ensure libvirtd state without init system info from facts")
	}

	serviceName := "libvirtd"

	isActive, err := r.IsServiceActive(ctx, conn, facts, serviceName)
	if err != nil {
	}
	if isActive {
		if errEnable := r.EnableService(ctx, conn, facts, serviceName); errEnable != nil {
			return errors.Wrapf(errEnable, "libvirtd service is active but failed to enable")
		}
		return nil
	}

	if errStart := r.StartService(ctx, conn, facts, serviceName); errStart != nil {
		return errors.Wrapf(errStart, "failed to start libvirtd service")
	}
	if errEnable := r.EnableService(ctx, conn, facts, serviceName); errEnable != nil {
		return errors.Wrapf(errEnable, "libvirtd service started but failed to enable")
	}
	return nil
}

func (r *defaultRunner) AttachNetInterface(ctx context.Context, conn connector.Connector, vmName string, iface VMNetworkInterface, persistent bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || iface.Type == "" || iface.Source == "" {
		return errors.New("vmName, interface type, and source are required")
	}

	ifaceModel := iface.Model
	if ifaceModel == "" {
		ifaceModel = "virtio"
	}

	newIface := DomainInterface{
		XMLName: xml.Name{Local: "interface"},
		Type:    iface.Type,
		Model:   DomainModel{Type: ifaceModel},
	}
	if iface.Type == "network" {
		newIface.Source.Network = iface.Source
	} else if iface.Type == "bridge" {
		newIface.Source.Bridge = iface.Source
	} else {
		return errors.Errorf("unsupported interface type: %s", iface.Type)
	}
	if iface.MACAddress != "" {
		newIface.MAC = &DomainMAC{Address: iface.MACAddress}
	}

	xmlBytes, err := xml.Marshal(newIface)
	if err != nil {
		return errors.Wrap(err, "failed to marshal interface to XML")
	}

	tempXMLPath := fmt.Sprintf("/tmp/kubexm-iface-attach-%d.xml", time.Now().UnixNano())
	if err := r.WriteFile(ctx, conn, xmlBytes, tempXMLPath, "0600", true); err != nil {
		return errors.Wrapf(err, "failed to write temp interface XML to %s", tempXMLPath)
	}
	defer r.Remove(ctx, conn, tempXMLPath, true, true)

	cmdArgs := []string{"virsh", "attach-device", vmName, tempXMLPath}
	if persistent {
		cmdArgs = append(cmdArgs, "--config")
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to attach network interface to %s. Stderr: %s", vmName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) DetachNetInterface(ctx context.Context, conn connector.Connector, vmName string, macAddress string, persistent bool) error {
	if conn == nil {
		return errors.New("connector cannot be nil")
	}
	if vmName == "" || macAddress == "" {
		return errors.New("vmName and macAddress are required")
	}

	cmdArgs := []string{"virsh", "detach-interface", vmName, "--type", "network", "--mac", macAddress}
	if persistent {
		cmdArgs = append(cmdArgs, "--config", "--live")
	} else {
		cmdArgs = append(cmdArgs, "--live")
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "no interface found with matching mac address") || strings.Contains(string(stderr), "not found") {
			return nil
		}
		return errors.Wrapf(err, "failed to detach network interface %s from %s. Stderr: %s", macAddress, vmName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) ListNetInterfaces(ctx context.Context, conn connector.Connector, vmName string) ([]VMNetworkInterfaceDetail, error) {
	details, err := r.GetVMInfo(ctx, conn, vmName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get VM details for listing interfaces")
	}

	var domain Domain
	if err := xml.Unmarshal([]byte(details.RawXML), &domain); err != nil {
		return nil, errors.Wrap(err, "failed to parse RawXML from VMDetails")
	}

	var result []VMNetworkInterfaceDetail
	for _, ifaceInfo := range details.NetworkInterfaces {
		var ifaceType string
		var ifaceModel string
		for _, xmlIface := range domain.Devices.Interfaces {
			if xmlIface.MAC != nil && xmlIface.MAC.Address == ifaceInfo.MAC {
				ifaceType = xmlIface.Type
				ifaceModel = xmlIface.Model.Type
				break
			}
		}

		result = append(result, VMNetworkInterfaceDetail{
			VMNetworkInterface: VMNetworkInterface{
				Type:       ifaceType,
				Source:     ifaceInfo.Source,
				MACAddress: ifaceInfo.MAC,
				Model:      ifaceModel,
			},
			InterfaceName: ifaceInfo.Name,
			State:         "active",
		})
	}
	return result, nil
}

func (r *defaultRunner) CreateSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName, description string, diskSpecs []VMSnapshotDiskSpec, noMetadata, halt, diskOnly, reuseExisting, quiesce, atomic bool) error {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "snapshot-create-as", vmName)

	if snapshotName != "" {
		cmdArgs = append(cmdArgs, snapshotName)
	}
	if description != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("'%s'", description))
	}
	if noMetadata {
		cmdArgs = append(cmdArgs, "--no-metadata")
	}
	if halt {
		cmdArgs = append(cmdArgs, "--halt")
	}
	if diskOnly {
		cmdArgs = append(cmdArgs, "--disk-only")
	}
	if reuseExisting {
		cmdArgs = append(cmdArgs, "--reuse-external")
	}
	if quiesce {
		cmdArgs = append(cmdArgs, "--quiesce")
	}
	if atomic {
		cmdArgs = append(cmdArgs, "--atomic")
	}

	for _, spec := range diskSpecs {
		arg := fmt.Sprintf("--diskspec %s,snapshot=%s", spec.Name, spec.Snapshot)
		if spec.Snapshot == "external" {
			if spec.File == "" || spec.DriverType == "" {
				return errors.New("external snapshot spec requires file and driverType")
			}
			arg += fmt.Sprintf(",file=%s,driver=%s", spec.File, spec.DriverType)
		}
		cmdArgs = append(cmdArgs, arg)
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to create snapshot '%s' for vm '%s'. Stderr: %s", snapshotName, vmName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) DeleteSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName string, children, metadata bool) error {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "snapshot-delete", vmName, snapshotName)

	if children {
		cmdArgs = append(cmdArgs, "--children")
	}
	if metadata {
		cmdArgs = append(cmdArgs, "--metadata")
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		if strings.Contains(string(stderr), "snapshot not found") {
			return nil
		}
		return errors.Wrapf(err, "failed to delete snapshot '%s' from vm '%s'. Stderr: %s", snapshotName, vmName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) ListSnapshots(ctx context.Context, conn connector.Connector, vmName string) ([]VMSnapshotInfo, error) {
	listCmd := fmt.Sprintf("virsh snapshot-list %s --name", vmName)
	stdout, stderr, err := conn.Exec(ctx, listCmd, &connector.ExecOptions{Sudo: true, Timeout: 1 * time.Minute})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list snapshot names for vm '%s'. Stderr: %s", vmName, string(stderr))
	}

	snapshotNames := strings.Fields(string(stdout))
	if len(snapshotNames) == 0 {
		return []VMSnapshotInfo{}, nil
	}

	var infos []VMSnapshotInfo
	for _, snapName := range snapshotNames {
		infoCmd := fmt.Sprintf("virsh snapshot-info %s %s --xml", vmName, snapName)
		xmlBytes, _, err := conn.Exec(ctx, infoCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
		if err != nil {
			fmt.Printf("Warning: could not get info for snapshot %s: %v\n", snapName, err)
			continue
		}

		var snap VMSnapshot
		if err := xml.Unmarshal(xmlBytes, &snap); err != nil {
			fmt.Printf("Warning: could not parse xml for snapshot %s: %v\n", snapName, err)
			continue
		}

		info := VMSnapshotInfo{
			Name:        snap.Name,
			Description: snap.Description,
			CreatedAt:   time.Unix(snap.CreationTime, 0).Format(time.RFC3339),
			State:       snap.State,
			HasParent:   snap.Parent != nil && snap.Parent.Name != "",
		}
		if snap.Children != nil {
			for _, child := range snap.Children {
				info.Children = append(info.Children, child.Name)
			}
		}
		if snap.Disks != nil {
			info.Disks = make(map[string]string)
			for _, disk := range snap.Disks.Disks {
				info.Disks[disk.Name] = disk.Source.File
			}
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (r *defaultRunner) RevertToSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName string, force, running bool) error {
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "virsh", "snapshot-revert", vmName, snapshotName)

	if force {
		cmdArgs = append(cmdArgs, "--force")
	}
	if running {
		cmdArgs = append(cmdArgs, "--running")
	} else {
		cmdArgs = append(cmdArgs, "--paused")
	}

	_, stderr, err := conn.Exec(ctx, strings.Join(cmdArgs, " "), &connector.ExecOptions{Sudo: true, Timeout: 2 * time.Minute})
	if err != nil {
		return errors.Wrapf(err, "failed to revert vm '%s' to snapshot '%s'. Stderr: %s", vmName, snapshotName, string(stderr))
	}
	return nil
}

func (r *defaultRunner) GetVMInfo(ctx context.Context, conn connector.Connector, vmName string) (*VMDetails, error) {
	if conn == nil {
		return nil, errors.New("connector cannot be nil")
	}
	if vmName == "" {
		return nil, errors.New("vmName cannot be empty")
	}

	dumpxmlCmd := fmt.Sprintf("virsh dumpxml %s", vmName)
	xmlBytes, stderr, err := conn.Exec(ctx, dumpxmlCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get XML for VM %s. Stderr: %s", vmName, string(stderr))
	}
	rawXML := string(xmlBytes)

	var domain Domain
	if err := xml.Unmarshal(xmlBytes, &domain); err != nil {
		return nil, errors.Wrap(err, "failed to parse VM XML")
	}

	state, err := r.GetVMState(ctx, conn, vmName)
	if err != nil {
		state = "unknown"
	}

	dominfoCmd := fmt.Sprintf("virsh dominfo %s", vmName)
	infoStdout, _, _ := conn.Exec(ctx, dominfoCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
	autostart := strings.Contains(string(infoStdout), "Autostart:      enable")
	persistent := strings.Contains(string(infoStdout), "Persistent:     yes")

	memMB := domain.Memory.Value
	unit := strings.ToLower(domain.Memory.Unit)
	if unit == "kib" {
		memMB /= 1024
	} else if unit == "gib" {
		memMB *= 1024
	}

	details := &VMDetails{
		VMInfo: VMInfo{
			Name:   domain.Name,
			State:  state,
			CPUs:   int(domain.VCPU.Value),
			Memory: memMB,
			UUID:   domain.UUID,
		},
		OSVariant:        domain.OS.Variant.ID,
		DomainType:       domain.Type,
		Architecture:     domain.OS.Type.Arch,
		EmulatorPath:     domain.Devices.Emulator,
		PersistentConfig: persistent,
		Autostart:        autostart,
		EffectiveMemory:  domain.CurrentMemory.Value,
		EffectiveVCPUs:   domain.VCPU.Value,
		RawXML:           rawXML,
	}

	for _, g := range domain.Devices.Graphics {
		gi := VMGraphicsInfo{
			Type:     g.Type,
			Port:     g.Port,
			Password: g.Password,
			Keymap:   g.Keymap,
		}
		if len(g.Listen) > 0 {
			gi.Listen = g.Listen[0].Address
		}
		details.Graphics = append(details.Graphics, gi)
	}

	for _, d := range domain.Devices.Disks {
		bdi := VMBlockDeviceInfo{
			Device:     d.Target.Dev,
			Type:       d.Device,
			SourceFile: d.Source.File,
			DriverType: d.Driver.Type,
			TargetBus:  d.Target.Bus,
		}
		details.Disks = append(details.Disks, bdi)
	}

	for _, iface := range domain.Devices.Interfaces {
		if iface.MAC == nil {
			continue
		}
		vii := VMInterfaceInfo{
			Name:       iface.Target.Dev,
			MAC:        iface.MAC.Address,
			DeviceName: iface.Target.Dev,
		}
		if iface.Source.Network != "" {
			vii.Source = iface.Source.Network
		} else if iface.Source.Bridge != "" {
			vii.Source = iface.Source.Bridge
		}

		domifaddrCmd := fmt.Sprintf("virsh domifaddr %s %s", vmName, vii.MAC)
		ipStdout, _, err := conn.Exec(ctx, domifaddrCmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
		if err == nil {
			lines := strings.Split(string(ipStdout), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 4 && (parts[2] == "ipv4" || parts[2] == "ipv6") {
					ipParts := strings.Split(parts[3], "/")
					if len(ipParts) == 2 {
						prefix, _ := strconv.Atoi(ipParts[1])
						vii.IPAddresses = append(vii.IPAddresses, VMInterfaceAddress{
							Addr:   ipParts[0],
							Prefix: prefix,
						})
					}
				}
			}
		}
		details.NetworkInterfaces = append(details.NetworkInterfaces, vii)
	}

	return details, nil
}

func (r *defaultRunner) GetVNCPort(ctx context.Context, conn connector.Connector, vmName string) (string, error) {
	details, err := r.GetVMInfo(ctx, conn, vmName)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get VM details for VNC port lookup")
	}

	for _, graphic := range details.Graphics {
		if strings.ToLower(graphic.Type) == "vnc" {
			if graphic.Port != "-1" && graphic.Port != "" {
				return graphic.Port, nil
			}
			cmd := fmt.Sprintf("virsh vncdisplay %s", vmName)
			stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true, Timeout: 30 * time.Second})
			if err != nil {
				return "", errors.Wrapf(err, "failed to get VNC display for vm %s. Stderr: %s", vmName, string(stderr))
			}

			displayStr := strings.TrimSpace(string(stdout))
			parts := strings.Split(displayStr, ":")
			if len(parts) < 2 {
				return "", errors.Errorf("unexpected output from 'virsh vncdisplay': %s", displayStr)
			}
			displayNum, err := strconv.Atoi(parts[len(parts)-1])
			if err != nil {
				return "", errors.Wrapf(err, "failed to parse VNC display number from '%s'", displayStr)
			}
			return strconv.Itoa(5900 + displayNum), nil
		}
	}

	return "", errors.New("no VNC graphics device found for VM")
}
