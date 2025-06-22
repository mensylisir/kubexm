package runner

import (
	"context"
	"fmt"
	"os" // Added for os.Stderr
	// "io" // Removed as unused
	// "net" // Removed as unused
	// "path/filepath" // Removed as unused
	"strconv"
	"strings"
	// "text/template" // Removed as unused
	// "time" // Removed as unused

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
	var err error

	// Step 1: Get OS information synchronously as other facts may depend on it.
	facts.OS, err = conn.GetOS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OS info: %w", err)
	}
	if facts.OS == nil {
		return nil, fmt.Errorf("conn.GetOS returned nil OS without error")
	}

	// Step 2: Gather other facts, potentially in parallel if they are independent.
	g, gCtx := errgroup.WithContext(ctx)

	// Get hostname
	g.Go(func() error {
		hostnameBytes, _, execErr := conn.Exec(gCtx, "hostname -f", nil)
		if execErr != nil {
			// Fallback to simple hostname if hostname -f fails
			hostnameBytes, _, execErr = conn.Exec(gCtx, "hostname", nil)
			if execErr != nil {
				return fmt.Errorf("failed to get hostname: %w", execErr)
			}
		}
		facts.Hostname = strings.TrimSpace(string(hostnameBytes))
		return nil
	})

	// Get kernel version (already part of facts.OS.Kernel, but can be re-verified or kept if OS.Kernel is minimal)
	// For now, we assume facts.OS.Kernel is sufficient. If not, this can be added:
	/*
		g.Go(func() error {
			kernelBytes, _, execErr := conn.Exec(gCtx, "uname -r", nil)
			if execErr != nil {
				return fmt.Errorf("failed to get kernel version: %w", execErr)
			}
			facts.Kernel = strings.TrimSpace(string(kernelBytes)) // This might override OS.Kernel
			return nil
		})
	*/
	// If facts.OS.Kernel is populated by conn.GetOS, facts.Kernel field in Facts struct might be redundant
	// or should be explicitly set from facts.OS.Kernel.
	// For now, let's ensure facts.Kernel is set from facts.OS.Kernel.
	facts.Kernel = facts.OS.Kernel

	// Get CPU and Memory
	g.Go(func() error {
		var cpuCmd, memCmd string
		memIsKb := false

		// Use facts.OS.ID for OS-specific commands
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			cpuCmd = "nproc"
			memCmd = "grep MemTotal /proc/meminfo | awk '{print $2}'" // Output is in KB
			memIsKb = true
		case "darwin":
			cpuCmd = "sysctl -n hw.ncpu"
			memCmd = "sysctl -n hw.memsize" // Output is in Bytes
			memIsKb = false
		default:
			// Fallback or error for unsupported OS for CPU/Mem
			// Try generic, but they might fail.
			cpuCmd = "nproc" // Attempt Linux default
			memCmd = "grep MemTotal /proc/meminfo | awk '{print $2}'" // Attempt Linux default
			memIsKb = true
			// Or return an error/warning:
			// fmt.Fprintf(os.Stderr, "Warning: Unsupported OS '%s' for specific CPU/Mem fact gathering, attempting defaults.\n", facts.OS.ID)
		}

		if cpuCmd != "" {
			cpuBytes, _, execErr := conn.Exec(gCtx, cpuCmd, nil)
			if execErr == nil {
				parsedCPU, parseErr := strconv.Atoi(strings.TrimSpace(string(cpuBytes)))
				if parseErr == nil {
					facts.TotalCPU = parsedCPU
				} else {
					fmt.Fprintf(os.Stderr, "Warning: failed to parse CPU output for %s on %s: %v\n", facts.OS.ID, facts.Hostname, parseErr)
					facts.TotalCPU = 0 // Default or mark as unknown
				}
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to exec CPU command '%s' for %s on %s: %v\n", cpuCmd, facts.OS.ID, facts.Hostname, execErr)
				facts.TotalCPU = 0 // Default or mark as unknown
			}
		}


		if memCmd != "" {
			memBytes, _, execErr := conn.Exec(gCtx, memCmd, nil)
			if execErr == nil {
				memVal, parseErr := strconv.ParseUint(strings.TrimSpace(string(memBytes)), 10, 64)
				if parseErr == nil {
					if memIsKb {
						facts.TotalMemory = memVal / 1024 // Convert KB to MiB
					} else { // Assumed Bytes
						facts.TotalMemory = memVal / (1024 * 1024) // Convert Bytes to MiB
					}
				} else {
					fmt.Fprintf(os.Stderr, "Warning: failed to parse Memory output for %s on %s: %v\n", facts.OS.ID, facts.Hostname, parseErr)
					facts.TotalMemory = 0 // Default
				}
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to exec Memory command '%s' for %s on %s: %v\n", memCmd, facts.OS.ID, facts.Hostname, execErr)
				facts.TotalMemory = 0 // Default
			}
		}
		return nil
	})

	// Get default IP addresses
	g.Go(func() error {
		var ip4Cmd, ip6Cmd string
		// Use facts.OS.ID for OS-specific commands
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			ip4Cmd = "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1"
			ip6Cmd = "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1"
		case "darwin":
			// For macOS, `route get default | grep interface | awk '{print $2}'` gets interface, then `ipconfig getifaddr <iface>`
			// This is more complex; for simplicity, we might leave it or use a simpler heuristic if available.
			// Example: ipconfig getifaddr en0 (but en0 might not be the default)
			// For now, let's keep it Linux-focused for IPs or accept it might be empty for others.
			// A common way:
			// ip4Cmd = "route -n get default | grep 'interface:' | awk '{print $2}' | xargs -I {} ipconfig getifaddr {}" (very basic)
		default:
			// fmt.Fprintf(os.Stderr, "Warning: OS '%s' not specifically handled for IP fact gathering.\n", facts.OS.ID)
		}

		if ip4Cmd != "" {
			ip4Bytes, _, _ := conn.Exec(gCtx, ip4Cmd, nil) // Errors ignored as IPs might not exist or command fails
			facts.IPv4Default = strings.TrimSpace(string(ip4Bytes))
		}
		if ip6Cmd != "" {
			ip6Bytes, _, _ := conn.Exec(gCtx, ip6Cmd, nil) // Errors ignored
			facts.IPv6Default = strings.TrimSpace(string(ip6Bytes))
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		// Log individual errors from goroutines if needed, errgroup returns the first non-nil error.
		return facts, fmt.Errorf("failed during concurrent fact gathering: %w", err) // Return partially filled facts
	}

	// Step 3: Detect package manager and init system (these depend on facts.OS).
	var pmErr, initErr error
	facts.PackageManager, pmErr = r.detectPackageManager(ctx, conn, facts)
	if pmErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect package manager for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, pmErr)
		// facts.PackageManager will be nil, subsequent package operations will fail gracefully.
	}
	facts.InitSystem, initErr = r.detectInitSystem(ctx, conn, facts)
	if initErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect init system for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, initErr)
		// facts.InitSystem will be nil, subsequent service operations will fail gracefully.
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
