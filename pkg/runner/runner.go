package runner

import (
	"bytes"
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
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

	osInfo, err := conn.GetOS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OS info: %w", err)
	}
	if osInfo == nil {
		return nil, fmt.Errorf("conn.GetOS returned nil OS without error")
	}
	facts := &Facts{
		OS:     osInfo,
		Kernel: osInfo.Kernel,
	}
	factCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	g, gCtx := errgroup.WithContext(factCtx)
	hostnameChan := make(chan string, 1)
	cpuChan := make(chan resource.Quantity, 1)
	memChan := make(chan resource.Quantity, 1)
	ipv4Chan := make(chan string, 1)
	ipv6Chan := make(chan string, 1)
	disksChan := make(chan []DiskInfo, 1)

	g.Go(func() error {
		defer close(hostnameChan)
		hostnameBytes, _, execErr := conn.Exec(gCtx, "hostname -f", nil)
		if execErr != nil {
			hostnameBytes, _, execErr = conn.Exec(gCtx, "hostname", nil)
			if execErr != nil {
				return fmt.Errorf("failed to get hostname: %w", execErr)
			}
		}
		hostnameChan <- strings.TrimSpace(string(hostnameBytes))
		return nil
	})

	g.Go(func() error {
		defer close(cpuChan)
		var cpuCmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			cpuCmd = "nproc"
		case "darwin":
			cpuCmd = "sysctl -n hw.ncpu"
		default:
			cpuCmd = "nproc"
		}
		cpuBytes, _, execErr := conn.Exec(gCtx, cpuCmd, nil)
		if execErr != nil {
			return fmt.Errorf("failed to exec CPU command '%s' on OS %s: %w", cpuCmd, facts.OS.ID, execErr)
		}
		cpuQuantity, parseErr := ParseCPU(string(cpuBytes))
		if parseErr != nil {
			return fmt.Errorf("failed to parse CPU output '%s': %w", string(cpuBytes), parseErr)
		}
		cpuChan <- *cpuQuantity
		return nil
	})

	g.Go(func() error {
		defer close(memChan)
		var memCmd string
		var memIsKb bool
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			memCmd = "awk '/MemTotal/ {print $2}' /proc/meminfo"
			memIsKb = true
		case "darwin":
			memCmd = "sysctl -n hw.memsize"
			memIsKb = false
		default:
			memCmd = "awk '/MemTotal/ {print $2}' /proc/meminfo"
			memIsKb = true
		}
		memBytes, _, execErr := conn.Exec(gCtx, memCmd, nil)
		if execErr != nil {
			return fmt.Errorf("failed to exec Memory command '%s' on OS %s: %w", memCmd, facts.OS.ID, execErr)
		}
		memStr := strings.TrimSpace(string(memBytes))
		if memIsKb {
			memStr += "Ki"
		}
		memQuantity, parseErr := ParseMemory(memStr)
		if parseErr != nil {
			return fmt.Errorf("failed to parse Memory output '%s': %w", memStr, parseErr)
		}
		memChan <- *memQuantity
		return nil
	})

	g.Go(func() error {
		defer close(disksChan)
		lsblkCmd := "lsblk -p -n -l -b -o NAME,SIZE,TYPE,MOUNTPOINT"

		stdout, _, err := conn.Exec(gCtx, lsblkCmd, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: 'lsblk' command failed, skipping disk fact gathering: %v\n", err)
			disksChan <- []DiskInfo{}
			return nil
		}
		disks, err := parseLsblkOutput(string(stdout))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse lsblk output: %v\n", err)
			disksChan <- []DiskInfo{}
			return nil
		}

		disksChan <- disks
		return nil
	})

	g.Go(func() error {
		defer close(ipv4Chan)

		var ip4Cmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			ip4Cmd = "ip -4 route get 8.8.8.8 | awk 'NR==1{print $7}'"
		case "darwin":
			ip4Cmd = "route -n get default | awk '/interface:/ {iface=$2} /inet/ {ip=$2} END {if (iface) print ip}'"
		}

		if ip4Cmd != "" {
			ip4Bytes, _, execErr := conn.Exec(gCtx, ip4Cmd, nil)
			if execErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get IPv4 default route for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, execErr)
				ipv4Chan <- ""
			} else {
				ipv4Chan <- strings.TrimSpace(string(ip4Bytes))
			}
		}
		return nil
	})

	g.Go(func() error {
		defer close(ipv6Chan)

		var ip6Cmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			ip6Cmd = "ip -6 route get 2001:4860:4860::8888 | awk 'NR==1{print $9}'"
		case "darwin":
			ip6Cmd = "route -n get -inet6 default | awk '/interface:/ {iface=$2} /inet6/ {ip=$2} END {if (iface) print ip}'"
		}

		if ip6Cmd != "" {
			ip6Bytes, _, execErr := conn.Exec(gCtx, ip6Cmd, nil)
			if execErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get IPv6 default route for host %s (%s): %v\n", facts.Hostname, facts.OS.ID, execErr)
				ipv6Chan <- ""
			} else {
				ipv6Chan <- strings.TrimSpace(string(ip6Bytes))
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return facts, fmt.Errorf("failed during concurrent fact gathering: %w", err)
	}

	facts.Hostname = <-hostnameChan
	facts.TotalCPU = <-cpuChan
	facts.TotalMemory = <-memChan
	facts.IPv4Default = <-ipv4Chan
	facts.IPv6Default = <-ipv6Chan
	facts.Disks = <-disksChan

	var totalDiskSize int64
	for _, disk := range facts.Disks {
		if disk.Type == "disk" {
			totalDiskSize += disk.Size.Value()
		}
	}
	facts.TotalDisk = *resource.NewQuantity(totalDiskSize, resource.BinarySI)

	facts.PackageManager, err = r.detectPackageManager(ctx, conn, facts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect package manager for host %s: %v\n", facts.Hostname, err)
	}

	facts.InitSystem, err = r.detectInitSystem(ctx, conn, facts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect init system for host %s: %v\n", facts.Hostname, err)
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
		if _, err := r.LookPath(ctx, conn, "apt-get"); err == nil {
			return &aptInfo, nil
		}
		if _, err := r.LookPath(ctx, conn, "dnf"); err == nil {
			dnfSpecificInfo := yumDnfInfoBase
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
	if _, err := r.LookPath(ctx, conn, "systemctl"); err == nil {
		return &systemdInfo, nil
	}
	if _, err := r.LookPath(ctx, conn, "service"); err == nil {
		return &sysvinitInfo, nil
	}
	if exists, _ := r.Exists(ctx, conn, "/etc/init.d"); exists {
		return &sysvinitInfo, nil
	}
	return nil, fmt.Errorf("unable to detect a supported init system (systemd, sysvinit) on OS ID: %s", facts.OS.ID)
}

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

	effectivePermissions := permissions
	if effectivePermissions == "" {
		effectivePermissions = "0644"
	}
	if err := r.WriteFile(ctx, conn, contentBytes, configPath, effectivePermissions, true); err != nil {
		return fmt.Errorf("failed to write configuration file %s for service %s: %w", configPath, serviceName, err)
	}

	if err := r.DaemonReload(ctx, conn, facts); err != nil {
		return fmt.Errorf("failed to perform daemon-reload after writing config for service %s: %w", serviceName, err)
	}

	if err := r.EnableService(ctx, conn, facts, serviceName); err != nil {
		return fmt.Errorf("failed to enable service %s: %w", serviceName, err)
	}

	if err := r.RestartService(ctx, conn, facts, serviceName); err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}

	return nil
}

func (r *defaultRunner) Reboot(ctx context.Context, conn connector.Connector, timeout time.Duration) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for Reboot")
	}
	rebootCmd := "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'"
	_, _, execErr := r.RunWithOptions(ctx, conn, rebootCmd, &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second}) // Short timeout for sending the command
	if execErr != nil {
		if !(strings.Contains(execErr.Error(), "context deadline exceeded") ||
			strings.Contains(execErr.Error(), "session channel closed") ||
			strings.Contains(execErr.Error(), "connection lost") ||
			strings.Contains(execErr.Error(), "EOF")) { // common for abrupt closes
			return fmt.Errorf("failed to issue reboot command: %w", execErr)
		}
		fmt.Fprintf(os.Stderr, "Reboot command initiated, connection may have dropped as expected: %v\n", execErr)
	}

	fmt.Fprintf(os.Stderr, "Reboot command sent. Waiting for shutdown to initiate...\n")
	time.Sleep(10 * time.Second)

	rebootCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "Waiting for host to become responsive after reboot (up to %s)...\n", timeout)

	for {
		select {
		case <-rebootCtx.Done():
			return fmt.Errorf("timed out waiting for host to become responsive after reboot: %w", rebootCtx.Err())
		case <-ticker.C:
			checkCmd := "uptime"
			_, _, checkErr := conn.Exec(rebootCtx, checkCmd, &connector.ExecOptions{Timeout: 5 * time.Second})

			if checkErr == nil {
				fmt.Fprintf(os.Stderr, "Host is responsive after reboot.\n")
				return nil
			}
		}
	}
}

func parseLsblkOutput(output string) ([]DiskInfo, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	diskMap := make(map[string]*DiskInfo)
	var disks []DiskInfo

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := fields[0]
		sizeStr := fields[1]
		diskType := fields[2]
		mountPoint := ""
		if len(fields) > 3 {
			mountPoint = fields[3]
		}

		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil {
			continue
		}
		sizeQuantity := *resource.NewQuantity(size, resource.BinarySI)

		if diskType == "disk" {
			disk := DiskInfo{
				Name:       name,
				Size:       sizeQuantity,
				Type:       diskType,
				MountPoint: mountPoint,
				Partitions: []PartitionInfo{},
			}
			diskMap[name] = &disk
			disks = append(disks, disk)
		} else if diskType == "part" {
			parentDiskName := getParentDisk(name)
			if parentDisk, ok := diskMap[parentDiskName]; ok {
				parentDisk.Partitions = append(parentDisk.Partitions, PartitionInfo{
					Name:       name,
					Size:       sizeQuantity,
					MountPoint: mountPoint,
				})
			}
		}
	}

	for i, disk := range disks {
		if updatedDisk, ok := diskMap[disk.Name]; ok {
			disks[i] = *updatedDisk
		}
	}

	return disks, nil
}

func getParentDisk(partitionName string) string {
	re := regexp.MustCompile(`p\d+$`)
	parent := re.ReplaceAllString(partitionName, "")

	if parent == partitionName {
		re = regexp.MustCompile(`\d+$`)
		parent = re.ReplaceAllString(partitionName, "")
	}

	return parent
}
