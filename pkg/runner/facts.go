package runner

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"golang.org/x/sync/errgroup"
)

// GatherFacts collects various system facts from the connected host.
// It populates and returns a Facts struct.
// This method is part of the defaultRunner implementation of the Runner interface.
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
		// This case should ideally be prevented by conn.GetOS returning an error
		// if it cannot determine the OS or if the OS struct is nil.
		return nil, fmt.Errorf("conn.GetOS returned nil OS without error")
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		hostnameBytes, _, execErr := conn.Exec(gCtx, "hostname -f", nil)
		if execErr != nil {
			// Fallback to short hostname if `hostname -f` fails
			hostnameBytes, _, execErr = conn.Exec(gCtx, "hostname", nil)
			if execErr != nil {
				return fmt.Errorf("failed to get hostname: %w", execErr)
			}
		}
		facts.Hostname = strings.TrimSpace(string(hostnameBytes))
		return nil
	})

	// Kernel information is often part of the OS struct from connector.GetOS()
	// Ensure it's assigned correctly.
	facts.Kernel = facts.OS.Kernel // Assuming facts.OS.Kernel is populated by conn.GetOS

	g.Go(func() error {
		var cpuCmd, memCmd string
		memIsKb := false // Flag to indicate if memory command output is in KB

		// OS-specific commands for CPU and Memory
		// Note: facts.OS.ID should be case-insensitive compared or normalized
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			cpuCmd = "nproc"
			memCmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
			memIsKb = true
		case "darwin": // macOS
			cpuCmd = "sysctl -n hw.ncpu"
			memCmd = "sysctl -n hw.memsize" // This is in bytes for macOS
			memIsKb = false
		default:
			// Fallback for unknown OS types - these might fail or give incorrect results.
			// Consider logging a warning if a default is used for an unrecognized OS.
			fmt.Fprintf(os.Stderr, "Warning: Using default CPU/Memory commands for unrecognized OS ID: %s\n", facts.OS.ID)
			cpuCmd = "nproc" // Common command, might not be available
			memCmd = "grep MemTotal /proc/meminfo | awk '{print $2}'" // Common, might not be available or in KB
			memIsKb = true
		}

		if cpuCmd != "" {
			cpuBytes, _, execErr := conn.Exec(gCtx, cpuCmd, nil)
			if execErr == nil {
				parsedCPU, parseErr := strconv.Atoi(strings.TrimSpace(string(cpuBytes)))
				if parseErr == nil {
					facts.TotalCPU = parsedCPU
				} else {
					return fmt.Errorf("failed to parse CPU output ('%s') for %s on %s: %w", string(cpuBytes), facts.OS.ID, facts.Hostname, parseErr)
				}
			} else {
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
					} else { // Assuming bytes if not KB (e.g., macOS hw.memsize)
						facts.TotalMemory = memVal / (1024 * 1024) // Convert Bytes to MiB
					}
				} else {
					return fmt.Errorf("failed to parse Memory output ('%s') for %s on %s: %w", string(memBytes), facts.OS.ID, facts.Hostname, parseErr)
				}
			} else {
				return fmt.Errorf("failed to exec Memory command '%s' for %s on %s: %w", memCmd, facts.OS.ID, facts.Hostname, execErr)
			}
		}
		return nil
	})

	g.Go(func() error {
		var ip4Cmd, ip6Cmd string
		// OS-specific commands for default IPv4 and IPv6 routes
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			// These commands try to get the source IP used for a specific public destination.
			ip4Cmd = "ip -4 route get 8.8.8.8 | awk '{print $7}' | head -n1"
			ip6Cmd = "ip -6 route get 2001:4860:4860::8888 | awk '{print $10}' | head -n1" // $10 is often the 'src' field
		case "darwin":
			// macOS IP detection can be more complex. `route get default` then `ifconfig <iface>`
			// This is a simplified placeholder. A robust solution might require parsing `netstat -rn` or `route -n get default`.
			// Example: `route -n get default | grep 'interface:' | awk '{print $2}'` gets default iface,
			// then `ifconfig $(route -n get default | grep 'interface:' | awk '{print $2}') inet | awk '/inet / {print $2}'` for IPv4.
			// For now, we'll attempt a common command for the primary interface.
			// It's common for `en0` to be Wi-Fi or Ethernet.
			// ip4Cmd = "ifconfig $(route -n get default | grep 'interface:' | awk '{print $2}') inet | awk '/inet / {print $2}'"
			// ip6Cmd = "ifconfig $(route -n get default | grep 'interface:' | awk '{print $2}') inet6 | awk '/inet6 / {print $2; exit}'" // Get first inet6
			// The above are complex for a simple exec; often, a simpler heuristic or specific library is better.
			// For now, leave blank and rely on user configuration or skip.
			fmt.Fprintf(os.Stderr, "Warning: Automatic default IP detection for macOS (darwin) is basic and may not be accurate. Hostname: %s\n", facts.Hostname)
		default:
			fmt.Fprintf(os.Stderr, "Warning: No specific IP detection logic for OS ID: %s. Hostname: %s\n", facts.OS.ID, facts.Hostname)
		}

		if ip4Cmd != "" {
			ip4Bytes, _, execErr := conn.Exec(gCtx, ip4Cmd, nil)
			if execErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get IPv4 default route for host %s (%s): %v. Command: %s\n", facts.Hostname, facts.OS.ID, execErr, ip4Cmd)
			} else {
				facts.IPv4Default = strings.TrimSpace(string(ip4Bytes))
			}
		}
		if ip6Cmd != "" {
			ip6Bytes, _, execErr := conn.Exec(gCtx, ip6Cmd, nil)
			if execErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get IPv6 default route for host %s (%s): %v. Command: %s\n", facts.Hostname, facts.OS.ID, execErr, ip6Cmd)
			} else {
				facts.IPv6Default = strings.TrimSpace(string(ip6Bytes))
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		// Note: err from g.Wait() will be the first non-nil error returned by any of the g.Go routines.
		// The facts struct might be partially populated.
		return facts, fmt.Errorf("failed during concurrent fact gathering: %w", err)
	}

	// Detect Package Manager and Init System sequentially after OS is known
	var pmErr, initErr error
	// Pass 'r' (the defaultRunner instance) to these methods if they need to call other runner methods like LookPath or Exists.
	facts.PackageManager, pmErr = r.detectPackageManager(ctx, conn, facts)
	if pmErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect package manager for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, pmErr)
		// Continue, as this might not be a fatal error for all operations.
	}

	facts.InitSystem, initErr = r.detectInitSystem(ctx, conn, facts)
	if initErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect init system for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, initErr)
		// Continue.
	}

	return facts, nil // Return partially populated facts even if some warnings occurred.
}

// detectPackageManager attempts to identify the package manager on the host.
// This method is called by GatherFacts and relies on the OS information from the Facts struct.
// It needs the defaultRunner receiver `r` to potentially use methods like `r.LookPath`.
func (r *defaultRunner) detectPackageManager(ctx context.Context, conn connector.Connector, facts *Facts) (*PackageInfo, error) {
	if facts == nil || facts.OS == nil {
		return nil, fmt.Errorf("OS facts not available, cannot detect package manager")
	}

	// Define standard package manager command templates
	aptInfo := PackageInfo{
		Type: PackageManagerApt, UpdateCmd: "apt-get update -y", InstallCmd: "apt-get install -y %s",
		RemoveCmd: "apt-get remove -y %s", PkgQueryCmd: "dpkg-query -W -f='${Status}' %s", CacheCleanCmd: "apt-get clean",
	}
	yumDnfInfoBase := PackageInfo{ // Base for YUM/DNF, default to YUM
		Type: PackageManagerYum, UpdateCmd: "yum update -y", InstallCmd: "yum install -y %s",
		RemoveCmd: "yum remove -y %s", PkgQueryCmd: "rpm -q %s", CacheCleanCmd: "yum clean all",
	}

	// OS-specific detection logic
	switch strings.ToLower(facts.OS.ID) {
	case "ubuntu", "debian", "raspbian", "linuxmint":
		return &aptInfo, nil
	case "centos", "rhel", "fedora", "almalinux", "rocky":
		// For these distributions, check for DNF first.
		if _, err := r.LookPath(ctx, conn, "dnf"); err == nil {
			dnfSpecificInfo := yumDnfInfoBase // Start with base
			dnfSpecificInfo.Type = PackageManagerDnf
			dnfSpecificInfo.UpdateCmd = "dnf update -y"
			dnfSpecificInfo.InstallCmd = "dnf install -y %s"
			dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"
			dnfSpecificInfo.CacheCleanCmd = "dnf clean all"
			return &dnfSpecificInfo, nil
		}
		// DNF not found, explicitly check for YUM for these distros.
		if _, err := r.LookPath(ctx, conn, "yum"); err == nil {
			return &yumDnfInfoBase, nil
		}
		// If neither DNF nor YUM found for these specific OS IDs, it's an issue.
		return nil, fmt.Errorf("package manager detection failed: neither dnf nor yum found for OS ID %s", facts.OS.ID)
	default:
		// Fallback for unknown OS: try to detect by command existence
		if _, err := r.LookPath(ctx, conn, "apt-get"); err == nil {
			return &aptInfo, nil
		}
		if _, err := r.LookPath(ctx, conn, "dnf"); err == nil {
			dnfSpecificInfo := yumDnfInfoBase // Start with base
			dnfSpecificInfo.Type = PackageManagerDnf
			dnfSpecificInfo.UpdateCmd = "dnf update -y"
			dnfSpecificInfo.InstallCmd = "dnf install -y %s"
			dnfSpecificInfo.RemoveCmd = "dnf remove -y %s"
			dnfSpecificInfo.CacheCleanCmd = "dnf clean all"
			return &dnfSpecificInfo, nil
		}
		if _, err := r.LookPath(ctx, conn, "yum"); err == nil {
			return &yumDnfInfoBase, nil
		}
		return nil, fmt.Errorf("unsupported OS or unable to detect package manager by command for OS ID: %s", facts.OS.ID)
	}
}

// detectInitSystem attempts to identify the init system on the host.
// This method is called by GatherFacts and relies on the OS information.
// It needs the defaultRunner receiver `r` to potentially use methods like `r.LookPath` or `r.Exists`.
func (r *defaultRunner) detectInitSystem(ctx context.Context, conn connector.Connector, facts *Facts) (*ServiceInfo, error) {
	if facts == nil || facts.OS == nil {
		return nil, fmt.Errorf("OS facts not available, cannot detect init system")
	}

	systemdInfo := ServiceInfo{
		Type: InitSystemSystemd, StartCmd: "systemctl start %s", StopCmd: "systemctl stop %s",
		EnableCmd: "systemctl enable %s", DisableCmd: "systemctl disable %s", RestartCmd: "systemctl restart %s",
		IsActiveCmd: "systemctl is-active --quiet %s", DaemonReloadCmd: "systemctl daemon-reload",
	}
	sysvinitInfo := ServiceInfo{ // Generic SysV commands
		Type: InitSystemSysV, StartCmd: "service %s start", StopCmd: "service %s stop",
		// Enable/Disable for SysV is highly distribution-specific (chkconfig, update-rc.d, etc.)
		// Providing generic ones that might not always work or be relevant.
		EnableCmd: "chkconfig %s on", DisableCmd: "chkconfig %s off", // Common on RHEL/CentOS like
		// For Debian/Ubuntu: update-rc.d %s defaults; update-rc.d %s remove
		RestartCmd: "service %s restart", IsActiveCmd: "service %s status", // Status might be unreliable
		DaemonReloadCmd: "", // No standard generic daemon-reload for SysV
	}

	// Check for systemctl command first (strong indicator of systemd)
	if _, err := r.LookPath(ctx, conn, "systemctl"); err == nil {
		return &systemdInfo, nil
	}
	// Check for service command (common in SysV and sometimes a wrapper in systemd)
	if _, err := r.LookPath(ctx, conn, "service"); err == nil {
		// To be more certain it's not systemd masquerading, one might check `pidof systemd` or similar,
		// but for simplicity, if systemctl isn't found, service command suggests SysV or similar.
		// A common indicator for SysV is the presence of /etc/init.d scripts.
		if exists, _ := r.Exists(ctx, conn, "/etc/init.d"); exists {
			// Adjust SysV commands for Debian/Ubuntu if OS ID matches
			switch strings.ToLower(facts.OS.ID) {
			case "ubuntu", "debian", "raspbian", "linuxmint":
				sysvinitInfo.EnableCmd = "update-rc.d %s defaults"
				sysvinitInfo.DisableCmd = "update-rc.d -f %s remove"
			}
			return &sysvinitInfo, nil
		}
	}
	// If /etc/init.d exists, it's likely SysV even if `service` command wasn't found (less common scenario)
	if exists, _ := r.Exists(ctx, conn, "/etc/init.d"); exists {
		switch strings.ToLower(facts.OS.ID) {
		case "ubuntu", "debian", "raspbian", "linuxmint":
			sysvinitInfo.EnableCmd = "update-rc.d %s defaults"
			sysvinitInfo.DisableCmd = "update-rc.d -f %s remove"
		}
		return &sysvinitInfo, nil
	}

	return nil, fmt.Errorf("unable to detect a supported init system (systemd, sysvinit) by command or known paths for OS ID: %s", facts.OS.ID)
}
