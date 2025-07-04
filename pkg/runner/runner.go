package runner

import (
	"context"
	"fmt"
	"bytes" // For template rendering
	"os"
	"strconv"
	"strings"
	"text/template" // For template rendering
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"golang.org/x/sync/errgroup"
)

type defaultRunner struct{}

func NewRunner() Runner {
	return &defaultRunner{}
}

func (r *defaultRunner) GatherFacts(ctx context.Context, conn connector.Connector) (*Facts, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil for GatherFacts")
	}
	if !conn.IsConnected() {
		return nil, fmt.Errorf("connector is not connected for GatherFacts")
	}

	facts := &Facts{}
	var err error

	facts.OS, err = conn.GetOS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OS info: %w", err)
	}
	if facts.OS == nil {
		return nil, fmt.Errorf("conn.GetOS returned nil OS without error")
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		hostnameBytes, _, execErr := conn.Exec(gCtx, "hostname -f", nil)
		if execErr != nil {
			hostnameBytes, _, execErr = conn.Exec(gCtx, "hostname", nil)
			if execErr != nil {
				return fmt.Errorf("failed to get hostname: %w", execErr)
			}
		}
		facts.Hostname = strings.TrimSpace(string(hostnameBytes))
		return nil
	})

	facts.Kernel = facts.OS.Kernel

	g.Go(func() error {
		var cpuCmd, memCmd string
		memIsKb := false
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			cpuCmd = "nproc"
			memCmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
			memIsKb = true
		case "darwin":
			cpuCmd = "sysctl -n hw.ncpu"
			memCmd = "sysctl -n hw.memsize"
			memIsKb = false
		default:
			// For unknown OS, try nproc and /proc/meminfo as a common fallback, but be prepared for failure.
			// Alternatively, leave cpuCmd/memCmd empty or return an error earlier.
			// For now, we let it try, and errors will be caught below.
			cpuCmd = "nproc" // Common fallback
			memCmd = "grep MemTotal /proc/meminfo | awk '{print $2}'" // Common fallback
			memIsKb = true
		}
		if cpuCmd != "" {
			cpuBytes, _, execErr := conn.Exec(gCtx, cpuCmd, nil)
			if execErr == nil {
				parsedCPU, parseErr := strconv.Atoi(strings.TrimSpace(string(cpuBytes)))
				if parseErr == nil {
					facts.TotalCPU = parsedCPU
				} else {
					// Return error to errgroup
					return fmt.Errorf("failed to parse CPU output for %s on %s: %w", facts.OS.ID, facts.Hostname, parseErr)
				}
			} else {
				// Return error to errgroup
				return fmt.Errorf("failed to exec CPU command '%s' for %s on %s: %w", cpuCmd, facts.OS.ID, facts.Hostname, execErr)
			}
		}
		if memCmd != "" {
			memBytes, _, execErr := conn.Exec(gCtx, memCmd, nil)
			if execErr == nil {
				memVal, parseErr := strconv.ParseUint(strings.TrimSpace(string(memBytes)), 10, 64)
				if parseErr == nil {
					if memIsKb {
						facts.TotalMemory = memVal / 1024 // Convert KB to MiB
					} else {
						facts.TotalMemory = memVal / (1024 * 1024) // Convert Bytes to MiB
					}
				} else {
					// Return error to errgroup
					return fmt.Errorf("failed to parse Memory output for %s on %s: %w", facts.OS.ID, facts.Hostname, parseErr)
				}
			} else {
				// Return error to errgroup
				return fmt.Errorf("failed to exec Memory command '%s' for %s on %s: %w", memCmd, facts.OS.ID, facts.Hostname, execErr)
			}
		}
		return nil
	})

	g.Go(func() error {
		var ip4Cmd, ip6Cmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			ip4Cmd = "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1"
			ip6Cmd = "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1" // Note: $10 for IPv6 source addr with `ip route get`
		case "darwin":
			// Placeholder for darwin IP logic. Example: ipconfig getifaddr en0 (for primary NIC, usually Wi-Fi or Ethernet)
			// For default route IP: `route -n get default | grep 'interface:' | awk '{print $2}'` then `ifconfig <iface> inet | awk '/inet / {print $2}'`
			// This is more complex and might need specific interface detection.
			// For now, we'll leave it as a warning if not found.
		default:
			// Placeholder for other OS IP logic
		}
		if ip4Cmd != "" {
			ip4Bytes, _, execErr := conn.Exec(gCtx, ip4Cmd, nil)
			if execErr != nil {
				// Log as warning, as IP might not be critical for all operations and might fail on systems without external connectivity.
				fmt.Fprintf(os.Stderr, "Warning: failed to get IPv4 default route for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, execErr)
			} else {
				facts.IPv4Default = strings.TrimSpace(string(ip4Bytes))
			}
		}
		if ip6Cmd != "" {
			ip6Bytes, _, execErr := conn.Exec(gCtx, ip6Cmd, nil)
			if execErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get IPv6 default route for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, execErr)
			} else {
				facts.IPv6Default = strings.TrimSpace(string(ip6Bytes))
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return facts, fmt.Errorf("failed during concurrent fact gathering: %w", err)
	}

	var pmErr, initErr error
	facts.PackageManager, pmErr = r.detectPackageManager(ctx, conn, facts)
	if pmErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect package manager for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, pmErr)
	}
	facts.InitSystem, initErr = r.detectInitSystem(ctx, conn, facts)
	if initErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect init system for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, initErr)
	}
	return facts, nil
}

func (r *defaultRunner) detectPackageManager(ctx context.Context, conn connector.Connector, facts *Facts) (*PackageInfo, error) {
	if facts == nil || facts.OS == nil {
		return nil, fmt.Errorf("OS facts not available, cannot detect package manager")
	}
	aptInfo := PackageInfo{
		Type: PackageManagerApt, UpdateCmd: "apt-get update -y", InstallCmd: "apt-get install -y %s",
		RemoveCmd: "apt-get remove -y %s", PkgQueryCmd: "dpkg-query -W -f='${Status}' %s", CacheCleanCmd: "apt-get clean",
	}
	yumDnfInfoBase := PackageInfo{
		Type: PackageManagerYum, UpdateCmd: "yum update -y", InstallCmd: "yum install -y %s",
		RemoveCmd: "yum remove -y %s", PkgQueryCmd: "rpm -q %s", CacheCleanCmd: "yum clean all",
	}
	switch strings.ToLower(facts.OS.ID) {
	case "ubuntu", "debian", "raspbian", "linuxmint":
		return &aptInfo, nil
	case "centos", "rhel", "fedora", "almalinux", "rocky":
		if _, err := r.LookPath(ctx, conn, "dnf"); err == nil {
			dnfSpecificInfo := yumDnfInfoBase
			dnfSpecificInfo.Type = PackageManagerDnf
			dnfSpecificInfo.UpdateCmd = "dnf update -y"
			dnfSpecificInfo.InstallCmd = "dnf install -y %s"
			dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"
			dnfSpecificInfo.CacheCleanCmd = "dnf clean all"
			return &dnfSpecificInfo, nil
		}
		return &yumDnfInfoBase, nil
	default:
		if _, err := r.LookPath(ctx, conn, "apt-get"); err == nil { return &aptInfo, nil }
		if _, err := r.LookPath(ctx, conn, "dnf"); err == nil {
			// Apply multiline formatting for readability
			dnfSpecificInfo := yumDnfInfoBase // Start with yum base
			dnfSpecificInfo.Type = PackageManagerDnf
			dnfSpecificInfo.UpdateCmd = "dnf update -y"
			dnfSpecificInfo.InstallCmd = "dnf install -y %s"
			dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"
			dnfSpecificInfo.CacheCleanCmd = "dnf clean all"
			return &dnfSpecificInfo, nil
		}
		if _, err := r.LookPath(ctx, conn, "yum"); err == nil { return &yumDnfInfoBase, nil }
		return nil, fmt.Errorf("unsupported OS or unable to detect package manager for OS ID: %s", facts.OS.ID)
	}
}

func (r *defaultRunner) detectInitSystem(ctx context.Context, conn connector.Connector, facts *Facts) (*ServiceInfo, error) {
	if facts == nil || facts.OS == nil {
		return nil, fmt.Errorf("OS facts not available, cannot detect init system")
	}
	systemdInfo := ServiceInfo{
		Type: InitSystemSystemd, StartCmd: "systemctl start %s", StopCmd: "systemctl stop %s",
		EnableCmd: "systemctl enable %s", DisableCmd: "systemctl disable %s", RestartCmd: "systemctl restart %s",
		IsActiveCmd: "systemctl is-active --quiet %s", DaemonReloadCmd: "systemctl daemon-reload",
	}
	sysvinitInfo := ServiceInfo{
		Type: InitSystemSysV, StartCmd: "service %s start", StopCmd: "service %s stop",
		EnableCmd: "chkconfig %s on", DisableCmd: "chkconfig %s off", RestartCmd: "service %s restart",
		IsActiveCmd: "service %s status", DaemonReloadCmd: "",
	}
	if _, err := r.LookPath(ctx, conn, "systemctl"); err == nil { return &systemdInfo, nil }
	if _, err := r.LookPath(ctx, conn, "service"); err == nil { return &sysvinitInfo, nil }
	if exists, _ := r.Exists(ctx, conn, "/etc/init.d"); exists { return &sysvinitInfo, nil }
	return nil, fmt.Errorf("unable to detect a supported init system (systemd, sysvinit) on OS ID: %s", facts.OS.ID)
}

// Stubs for methods implemented in specialized files (command.go, archive.go, file.go, network.go, package.go, service.go, template.go, user.go, system.go)
// are NOT duplicated here. Those files define these methods for defaultRunner.

// Stubs ONLY for very high-level or miscellaneous "enriched interface" methods
// that don't clearly fit into the existing specialized files yet.
func (r *defaultRunner) DeployAndEnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName, configContent, configPath, permissions string, templateData interface{}) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for DeployAndEnableService")
	}
	if facts == nil {
		return fmt.Errorf("facts cannot be nil for DeployAndEnableService")
	}
	if serviceName == "" {
		return fmt.Errorf("serviceName cannot be empty")
	}
	if configPath == "" {
		return fmt.Errorf("configPath cannot be empty")
	}

	var contentBytes []byte

	// 1. Render configuration if templateData is provided
	if templateData != nil {
		if configContent == "" {
			return fmt.Errorf("configContent cannot be empty if templateData is provided")
		}
		tmpl, err := template.New(serviceName + "-config").Parse(configContent)
		if err != nil {
			return fmt.Errorf("failed to parse config content as template for service %s: %w", serviceName, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, templateData); err != nil {
			return fmt.Errorf("failed to execute template for service %s with data: %w", serviceName, err)
		}
		contentBytes = buf.Bytes()
	} else {
		contentBytes = []byte(configContent)
	}

	// 2. Write configuration file
	// Assuming sudo is required for writing service configuration files.
	// Permissions should be appropriate for config files, e.g., "0644" or "0600".
	// If permissions is empty, WriteFile might use a default or the connector's default.
	// Let's ensure a sensible default if not provided.
	effectivePermissions := permissions
	if effectivePermissions == "" {
		effectivePermissions = "0644"
	}
	if err := r.WriteFile(ctx, conn, contentBytes, configPath, effectivePermissions, true); err != nil {
		return fmt.Errorf("failed to write configuration file %s for service %s: %w", configPath, serviceName, err)
	}

	// 3. Daemon Reload (important after changing service unit files or some configs)
	if err := r.DaemonReload(ctx, conn, facts); err != nil {
		// Non-fatal for some init systems (like basic SysV), but log or return if critical.
		// For now, let's consider it important enough to return error.
		return fmt.Errorf("failed to perform daemon-reload after writing config for service %s: %w", serviceName, err)
	}

	// 4. Enable Service
	if err := r.EnableService(ctx, conn, facts, serviceName); err != nil {
		return fmt.Errorf("failed to enable service %s: %w", serviceName, err)
	}

	// 5. Restart Service (or Start if preferred, Restart is often safer for config changes)
	if err := r.RestartService(ctx, conn, facts, serviceName); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}

	return nil
}

func (r *defaultRunner) Reboot(ctx context.Context, conn connector.Connector, timeout time.Duration) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for Reboot")
	}

	// Issue the reboot command.
	// Using "reboot" command which is common. Sudo is typically required.
	// We don't necessarily wait for this command to complete if it hangs the session.
	// A common strategy is to run it in a way that it detaches, e.g. `nohup reboot &`
	// or use a command that inherently does that like `shutdown -r +1 "Rebooting..."`.
	// For simplicity, just `reboot`. If the connection drops immediately, the error might be suppressed by some shells.
	// Let's use a slightly delayed reboot to allow the command to likely return.
	// `systemd-run --on-active=5s reboot` or `sh -c "sleep 5 && reboot" &`
	// Adding a small delay and running in background for robustness: "sh -c 'sleep 2 && reboot' > /dev/null 2>&1 &"
	rebootCmd := "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'" // Basic reboot command, slight delay, backgrounded

	// Attempt to issue the reboot command. We might not get a clean exit if the system reboots too fast.
	_, _, execErr := r.RunWithOptions(ctx, conn, rebootCmd, &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second}) // Short timeout for sending the command

	// We don't strictly fail on execErr here, as the command might succeed in rebooting even if the SSH session is terminated abruptly.
	// However, if execErr indicates command not found or immediate permission denied, that's a failure.
	if execErr != nil {
		// Check if the error is a context deadline exceeded, which is expected if the command is backgrounded and connection closes.
		// Or if the error message suggests the connection was closed.
		if !(strings.Contains(execErr.Error(), "context deadline exceeded") ||
			strings.Contains(execErr.Error(), "session channel closed") ||
			strings.Contains(execErr.Error(), "connection lost") || // common with some SSH libraries
			strings.Contains(execErr.Error(), "EOF")) { // common for abrupt closes
			return fmt.Errorf("failed to issue reboot command: %w", execErr)
		}
		fmt.Fprintf(os.Stderr, "Reboot command initiated, connection may have dropped as expected: %v\n", execErr)
	}

	// Wait a grace period for the shutdown to initiate properly.
	fmt.Fprintf(os.Stderr, "Reboot command sent. Waiting for shutdown to initiate...\n")
	time.Sleep(10 * time.Second) // Grace period for shutdown to start

	rebootCtx, cancel := context.WithTimeout(ctx, timeout) // Overall timeout for waiting
	defer cancel()

	ticker := time.NewTicker(2 * time.Second) // Poll every 2 seconds
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "Waiting for host to become responsive after reboot (up to %s)...\n", timeout)

	for {
		select {
		case <-rebootCtx.Done():
			return fmt.Errorf("timed out waiting for host to become responsive after reboot: %w", rebootCtx.Err())
		case <-ticker.C:
			// Attempt a simple command to check if the host is back up and responsive.
			// The existing 'conn' might be stale. Ideally, we'd re-establish a connection.
			// Since defaultRunner is stateless and doesn't store ConnectionCfg,
			// we rely on the passed 'conn' object. If it has internal reconnect logic, it might work.
			// If not, this check will likely fail until a new 'conn' is provided externally after reboot.
			// This implementation of Reboot is therefore limited by the statelessness of Runner.
			// A more robust reboot-and-wait would be part of a higher-level stateful orchestration.

			// For now, we'll just try a simple check. If it fails, we assume host is not ready.
			// If the original connection is truly dead, this check will always fail.
			// This highlights a limitation of a stateless runner handling reboot-and-wait fully.
			checkCmd := "uptime" // A simple command that should work on a booted system
			_, _, checkErr := conn.Exec(rebootCtx, checkCmd, &connector.ExecOptions{Timeout: 5 * time.Second}) // Use rebootCtx for timeout of this check

			if checkErr == nil {
				fmt.Fprintf(os.Stderr, "Host is responsive after reboot.\n")
				return nil // Host is back up
			}
			// If checkErr is not nil, continue waiting.
			// fmt.Fprintf(os.Stderr, "Host not yet responsive: %v\n", checkErr) // Verbose
		}
	}
}

// --- Stubs for methods defined in interface.go but not yet implemented in defaultRunner ---
// These are primarily for the extensive QEMU/libvirt and Docker functionalities
// that are part of the Runner interface but not yet implemented in the core defaultRunner
// or its specialized files (like archive.go, file.go, etc.).

// --- QEMU/libvirt Methods ---
// Implementations for RefreshStoragePool, CreateStoragePool, StoragePoolExists, DeleteStoragePool
// are in qemu.go. These stubs are removed to prevent redeclaration.

// All QEMU/libvirt related method stubs that are implemented in qemu.go
// have been removed from this file to prevent redeclaration errors.
// This includes:
// VolumeExists, CloneVolume, ResizeVolume, DeleteVolume, CreateVolume,
// CreateCloudInitISO, CreateVM, VMExists, StartVM, ShutdownVM, DestroyVM,
// UndefineVM, GetVMState, ListVMs, AttachDisk, DetachDisk, SetVMMemory, SetVMCPUs,
// EnsureLibvirtDaemonRunning.

// --- Docker Methods (Stubs remain if not implemented elsewhere yet) ---
// All Docker related method stubs that are implemented in docker.go
// have been removed from this file to prevent redeclaration errors.
// This includes:
// PullImage, ImageExists, ListImages, RemoveImage, BuildImage,
// CreateContainer, ContainerExists, StartContainer, StopContainer, RestartContainer, RemoveContainer,
// ListContainers, GetContainerLogs, GetContainerStats, InspectContainer, PauseContainer, UnpauseContainer, ExecInContainer,
// CreateDockerNetwork, RemoveDockerNetwork, ListDockerNetworks, ConnectContainerToNetwork, DisconnectContainerFromNetwork,
// CreateDockerVolume, RemoveDockerVolume, ListDockerVolumes, InspectDockerVolume,
// DockerInfo, DockerPrune, GetDockerDaemonConfig, ConfigureDockerDaemon, EnsureDefaultDockerConfig.
