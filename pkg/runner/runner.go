package runner

import (
	"bufio"
	"context"
	"fmt"
	"os" // For stderr output in detect funcs if logger not available
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/utils" // For IsExitCodeIgnored, PathRequiresSudo etc.
	"golang.org/x/sync/errgroup"
)

// defaultRunner implements the Runner interface.
type defaultRunner struct{}

// New creates a new instance of the default Runner.
func New() Runner {
	return &defaultRunner{}
}

// GatherFacts collects system information from the host.
func (r *defaultRunner) GatherFacts(ctx context.Context, conn connector.Connector) (*Facts, error) {
	facts := &Facts{}

	osInfo, err := conn.GetOS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OS info: %w", err)
	}
	facts.OS = osInfo

	// Use an errgroup for concurrent fact gathering that doesn't depend on initial OS info.
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

	g.Go(func() error {
		kernelBytes, _, execErr := conn.Exec(gCtx, "uname -r", nil)
		if execErr != nil {
			return fmt.Errorf("failed to get kernel version: %w", execErr)
		}
		facts.Kernel = strings.TrimSpace(string(kernelBytes))
		return nil
	})

	g.Go(func() error {
		var cpuCmd string
		// OS dependent CPU count. facts.OS is available here.
		switch strings.ToLower(facts.OS.ID) {
		case "darwin":
			cpuCmd = "sysctl -n hw.ncpu"
		default: // Linux and others
			cpuCmd = "nproc"
		}
		cpuBytes, _, execErr := conn.Exec(gCtx, cpuCmd, nil)
		if execErr == nil {
			parsedCPU, errConv := strconv.Atoi(strings.TrimSpace(string(cpuBytes)))
			if errConv == nil {
				facts.TotalCPU = parsedCPU
			} else {
				facts.TotalCPU = 0 // Mark as unknown or handle error
			}
		} else {
			facts.TotalCPU = 0 // Mark as unknown or handle error
		}
		return nil // Don't let CPU count failure fail all facts
	})

	g.Go(func() error {
		var memCmd string
		isKb := false
		isBytes := false
		switch strings.ToLower(facts.OS.ID) {
		case "darwin":
			memCmd = "sysctl -n hw.memsize"
			isBytes = true
		default: // Linux and others
			memCmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
			isKb = true
		}
		memBytes, _, execErr := conn.Exec(gCtx, memCmd, nil)
		if execErr == nil {
			memVal, parseErr := strconv.ParseUint(strings.TrimSpace(string(memBytes)), 10, 64)
			if parseErr == nil {
				if isKb {
					facts.TotalMemory = memVal / 1024 // Convert KB to MiB
				} else if isBytes {
					facts.TotalMemory = memVal / (1024 * 1024) // Convert Bytes to MiB
				} else {
					facts.TotalMemory = memVal // Assume MiB if not KB or Bytes (should not happen)
				}
			} else {
				facts.TotalMemory = 0
			}
		} else {
			facts.TotalMemory = 0
		}
		return nil // Don't let memory count failure fail all facts
	})

	g.Go(func() error {
		// These commands are Linux-specific for IP routing.
		// For other OS, different commands or libraries would be needed.
		if strings.ToLower(facts.OS.ID) == "linux" {
			ip4Cmd := "ip route get 1.1.1.1 | grep -oP 'src \\K\\S+'"
			ip4Bytes, _, _ := conn.Exec(gCtx, ip4Cmd, nil) // Errors ignored as IPs might not exist
			facts.IPv4Default = strings.TrimSpace(string(ip4Bytes))

			ip6Cmd := "ip -6 route get 2001:4860:4860::8888 | grep -oP 'src \\K\\S+' | head -n1"
			ip6Bytes, _, _ := conn.Exec(gCtx, ip6Cmd, nil) // Errors ignored
			facts.IPv6Default = strings.TrimSpace(string(ip6Bytes))
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("failed during concurrent fact gathering: %w", err)
	}

	// These depend on facts.OS, so call them sequentially after OS info is confirmed.
	pmInfo, pmErr := r.detectPackageManager(ctx, conn, facts)
	if pmErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not detect package manager for OS %s: %v\n", facts.OS.ID, pmErr)
		// facts.PackageManager will remain nil or be a minimal unknown struct
		facts.PackageManager = &PackageInfo{Type: PackageManagerUnknown}
	} else {
		facts.PackageManager = pmInfo
	}

	initSystemInfo, initErr := r.detectInitSystem(ctx, conn, facts)
	if initErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not detect init system for OS %s: %v\n", facts.OS.ID, initErr)
		facts.InitSystem = &ServiceInfo{Type: InitSystemUnknown}
	} else {
		facts.InitSystem = initSystemInfo
	}

	return facts, nil
}

func (r *defaultRunner) detectPackageManager(ctx context.Context, conn connector.Connector, facts *Facts) (*PackageInfo, error) {
	if facts == nil || facts.OS == nil || facts.OS.ID == "" {
		return nil, fmt.Errorf("OS facts (ID) not available, cannot reliably detect package manager")
	}
	osID := strings.ToLower(facts.OS.ID)

	switch osID {
	case "ubuntu", "debian", "raspbian", "linuxmint": // Add other debian family OSs
		if _, err := conn.LookPath(ctx, "apt-get"); err == nil {
			return &PackageInfo{
				Type:          PackageManagerApt,
				UpdateCmd:     "apt-get update -y",
				InstallCmd:    "apt-get install -y -q --no-install-recommends %s",
				RemoveCmd:     "apt-get remove -y -q %s",
				PkgQueryCmd:   "dpkg-query -W -f='${Status}\\n${Version}' %s",
				CacheCleanCmd: "apt-get clean",
			}, nil
		}
	case "centos", "rhel", "fedora", "almalinux", "rocky": // Add other RHEL family OSs
		if _, err := conn.LookPath(ctx, "dnf"); err == nil {
			return &PackageInfo{
				Type:          PackageManagerDnf,
				UpdateCmd:     "dnf makecache",
				InstallCmd:    "dnf install -y %s",
				RemoveCmd:     "dnf remove -y %s",
				PkgQueryCmd:   "rpm -q --queryformat '%{NAME}-%{VERSION}-%{RELEASE}' %s",
				CacheCleanCmd: "dnf clean all",
			}, nil
		}
		if _, err := conn.LookPath(ctx, "yum"); err == nil {
			return &PackageInfo{
				Type:          PackageManagerYum,
				UpdateCmd:     "yum makecache fast",
				InstallCmd:    "yum install -y %s",
				RemoveCmd:     "yum remove -y %s",
				PkgQueryCmd:   "rpm -q --queryformat '%{NAME}-%{VERSION}-%{RELEASE}' %s",
				CacheCleanCmd: "yum clean all",
			}, nil
		}
	}
	return nil, fmt.Errorf("unsupported OS or package manager for OS ID: %s", osID)
}

func (r *defaultRunner) detectInitSystem(ctx context.Context, conn connector.Connector, facts *Facts) (*ServiceInfo, error) {
	// Systemd is dominant. Check for systemctl.
	if _, err := conn.LookPath(ctx, "systemctl"); err == nil {
		// Further check if it's actually systemd running (pid 1 is systemd)
		// cmd := "stat /proc/1/exe | grep systemd" // This is one way
		// _, _, errStat := conn.Exec(ctx, cmd, nil)
		// if errStat == nil { ... }
		return &ServiceInfo{
			Type:            InitSystemSystemd,
			StartCmd:        "systemctl start %s",
			StopCmd:         "systemctl stop %s",
			EnableCmd:       "systemctl enable %s",
			DisableCmd:      "systemctl disable %s",
			RestartCmd:      "systemctl restart %s",
			IsActiveCmd:     "systemctl is-active %s",
			IsEnabledCmd:    "systemctl is-enabled %s",
			DaemonReloadCmd: "systemctl daemon-reload",
		}, nil
	}
	// Add checks for other init systems like SysV if needed.
	return nil, fmt.Errorf("init system not detected or unsupported (only systemd currently supported)")
}

func (r *defaultRunner) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	opts := &connector.ExecOptions{Sudo: sudo}
	stdout, stderr, err := conn.Exec(ctx, cmd, opts)
	if err != nil {
		return string(stdout), fmt.Errorf("command '%s' failed. Stderr: '%s': %w", cmd, string(stderr), err)
	}
	return string(stdout), nil
}

func (r *defaultRunner) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string {
	stdout, err := r.Run(ctx, conn, cmd, sudo)
	if err != nil {
		panic(fmt.Sprintf("MustRun command '%s' failed: %v. Stdout: %s", cmd, err, stdout))
	}
	return stdout
}

func (r *defaultRunner) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) {
	opts := &connector.ExecOptions{Sudo: sudo}
	_, _, err := conn.Exec(ctx, cmd, opts)
	if err != nil {
		if _, ok := err.(*connector.CommandError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *defaultRunner) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error) {
	return conn.Exec(ctx, cmd, opts)
}

func (r *defaultRunner) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	return conn.Exists(ctx, path)
}

func (r *defaultRunner) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	// Use `test -d` for POSIX environments.
	cmd := fmt.Sprintf("test -d %s", path)
	// Sudo typically not needed for `test -d`.
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: false})
	if err == nil {
		return true, nil // Exit code 0 means true (it is a directory)
	}
	if e, ok := err.(*connector.CommandError); ok && e.ExitCode == 1 {
		return false, nil // Exit code 1 means false (not a directory or does not exist)
	}
	return false, fmt.Errorf("failed to check if '%s' is a directory: %w", path, err) // Other errors
}

func (r *defaultRunner) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
	return conn.ReadFile(ctx, path)
}

func (r *defaultRunner) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	fs := connector.FileStat{Permissions: permissions, Sudo: sudo}
	return conn.CopyContent(ctx, string(content), destPath, fs)
}

func (r *defaultRunner) Mkdirp(ctx context.Context, conn connector.Connector, path string, permissions string, sudo bool) error {
	cmd := fmt.Sprintf("mkdir -p %s", path)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to mkdir -p %s (stderr: %s): %w", path, string(stderr), err)
	}
	if permissions != "" { // Apply permissions if specified
		return r.Chmod(ctx, conn, path, permissions, sudo)
	}
	return nil
}

func (r *defaultRunner) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	cmd := fmt.Sprintf("rm -rf %s", path)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to remove %s (stderr: %s): %w", path, string(stderr), err)
	}
	return nil
}

func (r *defaultRunner) Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	cmd := fmt.Sprintf("chmod %s %s", permissions, path)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to chmod %s to %s (stderr: %s): %w", path, permissions, string(stderr), err)
	}
	return nil
}

func (r *defaultRunner) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool, sudo bool) error {
	ownerGroup := owner
	if group != "" {
		ownerGroup = fmt.Sprintf("%s:%s", owner, group)
	}
	recursiveFlag := ""
	if recursive {
		recursiveFlag = "-R"
	}
	// Ensure no leading/trailing spaces on flags or paths
	cmd := strings.TrimSpace(fmt.Sprintf("chown %s %s %s", recursiveFlag, ownerGroup, path))
	cmd = regexp.MustCompile(`\s+`).ReplaceAllString(cmd, " ") // Compact multiple spaces

	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to chown %s to %s (stderr: %s): %w", path, ownerGroup, string(stderr), err)
	}
	return nil
}

func (r *defaultRunner) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) {
    // This command should work on most Linux systems. macOS needs `shasum -a 256`.
    // For simplicity, assuming Linux environment or that `sha256sum` is available.
    // A more robust solution would check facts.OS.ID.
    cmd := fmt.Sprintf("sha256sum %s | awk '{print $1}'", path)
    stdout, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to get sha256sum for %s (stderr: %s): %w", path, string(stderr), err)
    }
    return strings.TrimSpace(string(stdout)), nil
}

func (r *defaultRunner) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) {
    return conn.LookPath(ctx, file)
}

func (r *defaultRunner) IsPortOpen(ctx context.Context, conn connector.Connector, facts *Facts, port int) (bool, error) {
	// This is a simplified check. `ss` is preferred, `netstat` is legacy.
	// Behavior might differ based on OS.
	// Example for Linux using ss:
	if facts.OS != nil && strings.ToLower(facts.OS.ID) == "linux" {
		// Check TCP and UDP, listening ports. -n for numeric, -l for listening, -p for process (optional), -t for tcp, -u for udp
		cmd := fmt.Sprintf("ss -nl | grep -q ':%d\\s'", port) // Basic check for listening on port
		_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{})
		return err == nil, nil // If grep finds it, exit 0 (no error)
	}
	return false, fmt.Errorf("IsPortOpen not reliably implemented for OS: %s", facts.OS.ID)
}

func (r *defaultRunner) WaitForPort(ctx context.Context, conn connector.Connector, facts *Facts, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		isOpen, _ := r.IsPortOpen(ctx, conn, facts, port)
		if isOpen {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for port %d to open after %v", port, timeout)
}

func (r *defaultRunner) SetHostname(ctx context.Context, conn connector.Connector, facts *Facts, hostname string) error {
	if facts.OS == nil || facts.OS.ID == "" {
		return fmt.Errorf("cannot set hostname: OS facts unavailable")
	}
	var cmd string
	switch strings.ToLower(facts.OS.ID) {
	case "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky": // Most systemd systems
		cmd = fmt.Sprintf("hostnamectl set-hostname %s", hostname)
	default:
		return fmt.Errorf("SetHostname not implemented for OS: %s", facts.OS.ID)
	}
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set hostname to %s (stderr: %s): %w", hostname, string(stderr), err)
	}
	// Also update /etc/hosts for current hostname resolution if possible
	// This is complex due to various formats of /etc/hosts.
	// A simple approach: remove old hostname line for 127.0.1.1, add new one.
	// sed -i '/^127.0.1.1/d' /etc/hosts
	// echo "127.0.1.1 $(hostname) $(hostname -s)" >> /etc/hosts
	// This is best handled by a dedicated hosts file management step for robustness.
	return nil
}

func (r *defaultRunner) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error {
    if ip == "" || fqdn == "" {
        return fmt.Errorf("IP address and FQDN must be provided to AddHostEntry")
    }
    allNames := []string{fqdn}
    allNames = append(allNames, hostnames...)
    entry := fmt.Sprintf("%s %s", ip, strings.Join(allNames, " "))

    // Check if entry already exists to avoid duplicates (simple check)
    // A more robust check would parse /etc/hosts properly.
    hostsContent, err := r.ReadFile(ctx, conn, "/etc/hosts")
    if err == nil { // If file readable
        if strings.Contains(string(hostsContent), entry) {
            return nil // Entry seems to exist
        }
    }

    // Append entry. Sudo needed for /etc/hosts.
    // Use shell redirection which requires `sh -c` or similar.
    // Ensure entry does not contain characters that break the shell command.
    // For simplicity, assuming entry is safe.
    appendCmd := fmt.Sprintf("echo '%s' >> /etc/hosts", entry)
    // The command needs to be run via shell for '>>' redirection if conn.Exec isn't a full shell.
    // If conn.Exec is like `ssh host cmd`, then `sh -c 'echo ... >> ...'` is needed.
    // Assuming conn.Exec can handle this or a more direct file append method is available on conn.
    // If conn.WriteFile appends or can write with append mode, that's better.
    // For now, a simple echo with append.
    // This is not robust if multiple operations happen concurrently or if line already exists partially.
    // A proper /etc/hosts management step (like ManageHostsFileEntryStepSpec) is better.
    // This is a simplified version.
    _, stderr, err := conn.Exec(ctx, appendCmd, &connector.ExecOptions{Sudo: true, Shell: "sh"}) // Shell: "sh" to ensure redirection works
    if err != nil {
        return fmt.Errorf("failed to add host entry '%s' (stderr: %s): %w", entry, string(stderr), err)
    }
    return nil
}


func (r *defaultRunner) InstallPackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error {
	if facts.PackageManager == nil || facts.PackageManager.Type == PackageManagerUnknown {
		return fmt.Errorf("cannot install packages: package manager unknown for host %s", facts.Hostname)
	}
	if len(packages) == 0 { return nil }
	cmd := fmt.Sprintf(facts.PackageManager.InstallCmd, strings.Join(packages, " "))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to install packages '%s' (stderr: %s): %w", strings.Join(packages, " "), string(stderr), err)
	}
	return nil
}

func (r *defaultRunner) RemovePackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error {
	if facts.PackageManager == nil || facts.PackageManager.Type == PackageManagerUnknown {
		return fmt.Errorf("cannot remove packages: package manager unknown for host %s", facts.Hostname)
	}
	if len(packages) == 0 { return nil }
	cmd := fmt.Sprintf(facts.PackageManager.RemoveCmd, strings.Join(packages, " "))
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to remove packages '%s' (stderr: %s): %w", strings.Join(packages, " "), string(stderr), err)
	}
	return nil
}

func (r *defaultRunner) UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if facts.PackageManager == nil || facts.PackageManager.Type == PackageManagerUnknown {
		return fmt.Errorf("cannot update package cache: package manager unknown for host %s", facts.Hostname)
	}
	cmd := facts.PackageManager.UpdateCmd
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to update package cache (stderr: %s): %w", string(stderr), err)
	}
	return nil
}

func (r *defaultRunner) IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *Facts, packageName string) (installed bool, version string, err error) {
    if facts.PackageManager == nil || facts.PackageManager.Type == PackageManagerUnknown {
        return false, "", fmt.Errorf("cannot check package installation: package manager unknown for host %s", facts.Hostname)
    }
    if facts.PackageManager.PkgQueryCmd == "" {
        return false, "", fmt.Errorf("package query command not defined for package manager type %s on host %s", facts.PackageManager.Type, facts.Hostname)
    }

    queryCmd := strings.ReplaceAll(facts.PackageManager.PkgQueryCmd, "%s", packageName)

    stdout, stderr, execErr := conn.Exec(ctx, queryCmd, &connector.ExecOptions{Sudo: false})

    switch facts.PackageManager.Type {
    case PackageManagerApt:
        // dpkg-query -W -f='${Status}\n${Version}' <pkg>
        // Success output: "install ok installed\n<version>"
        // Not found: exits non-zero.
        if execErr != nil {
            return false, "", nil
        }
        outputStr := string(stdout)
        lines := strings.Split(outputStr, "\n")
        // Expected: "install ok installed" on first line for successfully installed package.
        // Some systems might just have "installed" or similar. Using "installed".
        if len(lines) >= 1 && strings.Contains(lines[0], "installed") {
            if len(lines) >=2 { // Version is on the second line
                 return true, strings.TrimSpace(lines[1]), nil
            }
            return true, "", nil // Installed, but version format unexpected
        }
         // If output is not as expected but command succeeded, treat as not installed or version unknown
        return false, "", fmt.Errorf("unexpected output from dpkg-query for %s: %s", packageName, outputStr)

    case PackageManagerYum, PackageManagerDnf:
        // rpm -q --queryformat '%{NAME}-%{VERSION}-%{RELEASE}' <pkg> or just '%{VERSION}-%{RELEASE}'
        // Success output: <name>-<version>-<release> or <version>-<release>
        // Not found: exits 1.
        if execErr != nil {
            if e, ok := execErr.(*connector.CommandError); ok && e.ExitCode == 1 {
                return false, "", nil // Not installed
            }
            return false, "", fmt.Errorf("rpm query for %s failed (stderr: %s): %w", packageName, string(stderr), execErr)
        }
        // RPM query format might include name. Try to strip it if present.
        // Example: mypackage-1.2.3-4.el7 -> 1.2.3-4.el7
        versionStr := strings.TrimSpace(string(stdout))
        if strings.HasPrefix(versionStr, packageName+"-") {
            versionStr = strings.TrimPrefix(versionStr, packageName+"-")
        }
        return true, versionStr, nil
    default:
        return false, "", fmt.Errorf("IsPackageInstalled not implemented for package manager type %s", facts.PackageManager.Type)
    }
}


func (r *defaultRunner) AddRepository(ctx context.Context, conn connector.Connector, facts *Facts, repoConfig string, isFilePath bool) error {
	return fmt.Errorf("AddRepository not yet implemented")
}

func (r *defaultRunner) serviceCommand(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string, commandTemplate string, useSudo bool) error {
	if facts.InitSystem == nil || facts.InitSystem.Type == InitSystemUnknown {
		return fmt.Errorf("cannot manage service '%s': init system unknown for host %s", serviceName, facts.Hostname)
	}
	if commandTemplate == "" {
		return fmt.Errorf("command template empty for service '%s' action on host %s", serviceName, facts.Hostname)
	}
	cmd := fmt.Sprintf(commandTemplate, serviceName)
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: useSudo})
	if err != nil {
		return fmt.Errorf("failed to %s (stderr: %s): %w", cmd, string(stderr), err)
	}
	return nil
}

func (r *defaultRunner) StartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	return r.serviceCommand(ctx, conn, facts, serviceName, facts.InitSystem.StartCmd, true)
}
func (r *defaultRunner) StopService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	return r.serviceCommand(ctx, conn, facts, serviceName, facts.InitSystem.StopCmd, true)
}
func (r *defaultRunner) RestartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	return r.serviceCommand(ctx, conn, facts, serviceName, facts.InitSystem.RestartCmd, true)
}
func (r *defaultRunner) EnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	return r.serviceCommand(ctx, conn, facts, serviceName, facts.InitSystem.EnableCmd, true)
}
func (r *defaultRunner) DisableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	return r.serviceCommand(ctx, conn, facts, serviceName, facts.InitSystem.DisableCmd, true)
}
func (r *defaultRunner) IsServiceActive(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error) {
	if facts.InitSystem == nil || facts.InitSystem.Type == InitSystemUnknown {
		return false, fmt.Errorf("cannot check service active status: init system unknown for host %s", facts.Hostname)
	}
	cmd := fmt.Sprintf(facts.InitSystem.IsActiveCmd, serviceName)
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true})
	return err == nil, nil
}
func (r *defaultRunner) IsServiceEnabled(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error) {
	if facts.InitSystem == nil || facts.InitSystem.Type == InitSystemUnknown {
		return false, fmt.Errorf("cannot check service enabled status: init system unknown for host %s", facts.Hostname)
	}
	cmd := fmt.Sprintf(facts.InitSystem.IsEnabledCmd, serviceName)
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true})
	return err == nil, nil
}
func (r *defaultRunner) DaemonReload(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if facts.InitSystem == nil || facts.InitSystem.Type == InitSystemUnknown {
		return fmt.Errorf("cannot daemon-reload: init system unknown for host %s", facts.Hostname)
	}
	cmd := facts.InitSystem.DaemonReloadCmd
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to daemon-reload (stderr: %s): %w", string(stderr), err)
	}
	return nil
}
func (r *defaultRunner) Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}
	return r.WriteFile(ctx, conn, buf.Bytes(), destPath, permissions, sudo)
}
func (r *defaultRunner) UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error) {
	cmd := fmt.Sprintf("id -u %s", username)
	_, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{})
	return err == nil, nil // Exit 0 if user exists
}
func (r *defaultRunner) GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error) {
	cmd := fmt.Sprintf("getent group %s", groupname)
    _, _, err := conn.Exec(ctx, cmd, &connector.ExecOptions{})
    return err == nil, nil // Exit 0 if group exists
}
func (r *defaultRunner) AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool, sudo bool) error {
	cmdParts := []string{"useradd"}
	if systemUser { cmdParts = append(cmdParts, "-r") }
	if createHome { cmdParts = append(cmdParts, "-m") } else { cmdParts = append(cmdParts, "-M") }
	if shell != "" { cmdParts = append(cmdParts, "-s", shell) }
	if group != "" { cmdParts = append(cmdParts, "-g", group) }
	if homeDir != "" { cmdParts = append(cmdParts, "-d", homeDir) }
	cmdParts = append(cmdParts, username)
	cmd := strings.Join(cmdParts, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to add user %s (stderr: %s): %w", username, string(stderr), err)
	}
	return nil
}
func (r *defaultRunner) AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool, sudo bool) error {
	cmdParts := []string{"groupadd"}
	if systemGroup { cmdParts = append(cmdParts, "-r") }
	cmdParts = append(cmdParts, groupname)
	cmd := strings.Join(cmdParts, " ")
	_, stderr, err := conn.Exec(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to add group %s (stderr: %s): %w", groupname, string(stderr), err)
	}
	return nil
}

// Ensure defaultRunner implements Runner.
var _ Runner = (*defaultRunner)(nil)
