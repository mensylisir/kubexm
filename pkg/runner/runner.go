package runner

import (
	"context"
	"fmt"
	"io" // Keep for future use, e.g. Render
	"net" // Keep for future use
	"path/filepath" // Keep for future use
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"golang.org/x/sync/errgroup"
)

// defaultRunner is a stateless struct that implements the Runner interface.
type defaultRunner struct{}

// New creates a new stateless Runner service.
func New() Runner {
	return &defaultRunner{}
}

// GatherFacts gathers information about the host.
func (r *defaultRunner) GatherFacts(ctx context.Context, conn connector.Connector) (*Facts, error) {
	facts := &Facts{}
	var getOSError error // To capture errors from GetOS if it's the first one in errgroup

	g, gCtx := errgroup.WithContext(ctx)

	// Get OS information
	g.Go(func() error {
		var err error
		facts.OS, err = conn.GetOS(gCtx) // Use conn from parameter
		if err != nil {
			getOSError = fmt.Errorf("failed to get OS info: %w", err)
			return getOSError // Return error to errgroup
		}
		if facts.OS == nil { // Should be caught by GetOS error, but defensive
			getOSError = fmt.Errorf("conn.GetOS returned nil OS without error")
			return getOSError
		}
		return nil
	})

	// Get hostname and kernel version
	g.Go(func() error {
		// Wait for OS info to be available if needed for OS-specific commands,
		// or if GetOS fails, errgroup context gCtx will be cancelled.
		if err := gCtx.Err(); err != nil {
			return err
		}
		hostnameBytes, _, execErr := conn.Exec(gCtx, "hostname -f", nil) // Use conn
		if execErr != nil {
			// Fallback to simple hostname if hostname -f fails (e.g. not configured)
			hostnameBytes, _, execErr = conn.Exec(gCtx, "hostname", nil)
			if execErr != nil {
				return fmt.Errorf("failed to get hostname (hostname -f and hostname): %w", execErr)
			}
		}
		facts.Hostname = strings.TrimSpace(string(hostnameBytes))

		kernelBytes, _, execErr := conn.Exec(gCtx, "uname -r", nil) // Use conn
		if execErr != nil {
			return fmt.Errorf("failed to get kernel version: %w", execErr)
		}
		facts.Kernel = strings.TrimSpace(string(kernelBytes))
		return nil
	})

	// Get CPU and Memory
	g.Go(func() error {
		if err := gCtx.Err(); err != nil {
			return err
		}
		// Ensure facts.OS is populated before attempting OS-specific commands.
		// This relies on the GetOS goroutine completing successfully first if its result is needed.
		// If GetOS fails, gCtx.Err() will be non-nil, and this goroutine will return early.
		// We need a mechanism to wait for facts.OS or handle its potential nilness if GetOS is slow or fails.
		// For now, we'll assume if GetOS fails, gCtx is cancelled. If it succeeds, facts.OS is set.
		// A more robust way would be to have dependent goroutines.

		// This temporary read of facts.OS might occur before GetOS goroutine finishes writing to it.
		// This is a race condition. The GetOS must complete before this goroutine can safely use facts.OS.
		// The errgroup doesn't guarantee order of execution between goroutines, only that all complete or one fails.
		// A better pattern: GetOS runs first, then other fact gathering that depends on OS info.
		// For this refactoring, let's assume facts.OS will be populated OR getOSError will cancel context.
		// A more robust solution is needed if facts.OS is critical for command choice here AND GetOS can be slow.

		// Given the current errgroup, we cannot reliably use facts.OS here if it's set by another goroutine.
		// So, for CPU/Mem detection, we'll try generic commands first or make OS-specific logic self-contained
		// by calling conn.GetOS() *within* this goroutine if needed for command selection.
		// For simplicity, let's assume generic commands or handle OS variance carefully.

		// CPU detection:
		cpuCmd := "nproc" // Default for Linux
		// If we could reliably get facts.OS.ID here, we'd use it.
		// Tentative OS check (less reliable due to potential race with GetOS goroutine):
		// Note: This is a simplified approach. A proper solution would ensure GetOS completes first.
		// For this iteration, we'll proceed with a common command and acknowledge this limitation.

		cpuBytes, _, execErr := conn.Exec(gCtx, cpuCmd, nil) // Use conn
		if execErr == nil {
			parsedCPU, err := strconv.Atoi(strings.TrimSpace(string(cpuBytes)))
			if err == nil {
				facts.TotalCPU = parsedCPU
			} else {
				facts.TotalCPU = 0 // Default or mark as unknown
			}
		} else {
			// Fallback for systems without nproc (e.g., macOS)
			// This would ideally use facts.OS.ID if available and reliable.
			// Trying sysctl for macOS as a common fallback.
			cpuBytesMac, _, execErrMac := conn.Exec(gCtx, "sysctl -n hw.ncpu", nil)
			if execErrMac == nil {
				parsedCPUMac, errMac := strconv.Atoi(strings.TrimSpace(string(cpuBytesMac)))
				if errMac == nil {
					facts.TotalCPU = parsedCPUMac
				} else {
					facts.TotalCPU = 0
				}
			} else {
				facts.TotalCPU = 0 // Default or mark as unknown
			}
		}

		// Memory detection:
		memCmd := "grep MemTotal /proc/meminfo | awk '{print $2}'" // KB, for Linux
		isKb := true
		// Similar OS detection issue as above for command choice.
		// Assuming Linux for /proc/meminfo.
		memBytes, _, execErr := conn.Exec(gCtx, memCmd, nil) // Use conn
		if execErr == nil {
			memVal, parseErr := strconv.ParseUint(strings.TrimSpace(string(memBytes)), 10, 64)
			if parseErr == nil {
				if isKb {
					facts.TotalMemory = memVal / 1024 // Convert KB to MiB
				} else {
					facts.TotalMemory = memVal / (1024 * 1024) // Convert Bytes to MiB
				}
			} else {
				facts.TotalMemory = 0 // Default
			}
		} else {
			// Fallback for macOS (hw.memsize returns bytes)
			memBytesMac, _, execErrMac := conn.Exec(gCtx, "sysctl -n hw.memsize", nil)
			if execErrMac == nil {
				memValMac, parseErrMac := strconv.ParseUint(strings.TrimSpace(string(memBytesMac)), 10, 64)
				if parseErrMac == nil {
					facts.TotalMemory = memValMac / (1024 * 1024) // Bytes to MiB
				} else {
					facts.TotalMemory = 0
				}
			} else {
				facts.TotalMemory = 0 // Default
			}
		}
		return nil
	})

	// Get default IP addresses
	g.Go(func() error {
		if err := gCtx.Err(); err != nil {
			return err
		}
		// These commands are Linux-specific.
		// Again, OS-specific command selection is hard here without facts.OS reliably.
		ip4Cmd := "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1"
		ip6Cmd := "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1" // Changed from $NF to $10 based on common ip route output

		ip4Bytes, _, _ := conn.Exec(gCtx, ip4Cmd, nil) // Use conn, errors ignored as IPs might not exist
		facts.IPv4Default = strings.TrimSpace(string(ip4Bytes))

		ip6Bytes, _, _ := conn.Exec(gCtx, ip6Cmd, nil) // Use conn, errors ignored
		facts.IPv6Default = strings.TrimSpace(string(ip6Bytes))
		return nil
	})

	if err := g.Wait(); err != nil {
		if getOSError != nil { // Prioritize GetOS error as it's foundational
			return nil, getOSError
		}
		return nil, fmt.Errorf("failed to gather some host facts: %w", err)
	}

	// After all goroutines, facts.OS should be reliably set if GetOS was successful.
	if facts.OS == nil { // Should have been caught by getOSError if GetOS failed and returned error
	    return nil, fmt.Errorf("critical: OS information is nil after fact gathering")
	}

	// These must be called after facts.OS is available.
	var pmErr, initErr error
	facts.PackageManager, pmErr = r.detectPackageManager(ctx, conn, facts) // Pass original ctx
	if pmErr != nil {
		// Log or handle error, but don't necessarily fail all fact gathering
		// For now, we'll let it be nil and proceed.
		// Consider if this should be a fatal error for GatherFacts.
	}
	facts.InitSystem, initErr = r.detectInitSystem(ctx, conn, facts) // Pass original ctx
	if initErr != nil {
		// Similar handling for init system detection error
	}

	return facts, nil
}

// detectPackageManager attempts to identify the package manager on the host.
func (r *defaultRunner) detectPackageManager(ctx context.Context, conn connector.Connector, facts *Facts) (*PackageInfo, error) {
	if facts == nil || facts.OS == nil {
		return nil, fmt.Errorf("OS facts not available, cannot detect package manager")
	}

	// Define package manager info structures (as in the old runner)
	aptInfo := PackageInfo{
		Type: PackageManagerApt, UpdateCmd: "apt-get update -y", InstallCmd: "apt-get install -y %s",
		RemoveCmd: "apt-get remove -y %s", PkgQueryCmd: "dpkg-query -W -f='${Status}' %s", CacheCleanCmd: "apt-get clean",
	}
	yumDnfInfoBase := PackageInfo{ // Base for yum/dnf
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
		if _, err := r.LookPath(ctx, conn, "apt-get"); err == nil {
			return &aptInfo, nil
		}
		if _, err := r.LookPath(ctx, conn, "dnf"); err == nil {
			dnfSpecificInfo := yumDnfInfoBase
			dnfSpecificInfo.Type = PackageManagerDnf; dnfSpecificInfo.UpdateCmd = "dnf update -y"; dnfSpecificInfo.InstallCmd = "dnf install -y %s"; dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"; dnfSpecificInfo.CacheCleanCmd = "dnf clean all"
			return &dnfSpecificInfo, nil
		}
		if _, err := r.LookPath(ctx, conn, "yum"); err == nil {
			return &yumDnfInfoBase, nil
		}
		return nil, fmt.Errorf("unsupported OS or unable to detect package manager for OS ID: %s", facts.OS.ID)
	}
}

// detectInitSystem attempts to identify the init system on the host.
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

	if _, err := r.LookPath(ctx, conn, "systemctl"); err == nil {
		return &systemdInfo, nil
	}
	if _, err := r.LookPath(ctx, conn, "service"); err == nil {
		return &sysvinitInfo, nil
	}
	// Simplified Exists check for /etc/init.d (original runner.Exists uses conn.Stat)
	if exists, _ := r.Exists(ctx, conn, "/etc/init.d"); exists {
		return &sysvinitInfo, nil
	}
	return nil, fmt.Errorf("unable to detect a supported init system (systemd, sysvinit) on OS ID: %s", facts.OS.ID)
}


// --- Command Execution (Implementations will be moved from command.go) ---
// Implementations are now in command.go


// --- Archive Operations (Implementations will be moved from archive.go) ---
// Implementations are now in archive.go


// --- File Operations (Implementations will be moved from file.go) ---
// Implementations are now in file.go


// --- Network Operations (Implementations will be moved from network.go) ---
// Implementations are now in network.go


// --- Package Operations (Implementations will be moved from package.go) ---
// Implementations are now in package.go


// --- Service Operations (Implementations will be moved from service.go) ---
// Implementations are now in service.go


// --- Template Operations (Implementations will be moved from template.go) ---
// Implementation is now in template.go


// --- User Operations (Implementations will be moved from user.go) ---
// Implementations are now in user.go


// Ensure connector.Connector has ReadFile and WriteFile methods
// These are assumed by the defaultRunner implementations of ReadFile/WriteFile.
// If not, those methods need to be implemented via Exec.
type extendedConnector interface {
	connector.Connector
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error
}
