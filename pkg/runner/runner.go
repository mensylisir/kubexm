package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runner/helpers"
	"k8s.io/apimachinery/pkg/api/resource"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"golang.org/x/sync/errgroup"
)

type defaultRunner struct {
	logger *logger.Logger
}

func NewRunner() Runner {
	return &defaultRunner{
		logger: logger.Get(),
	}
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
	interfaceChan := make(chan string, 1)
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
		cpuQuantity, parseErr := helpers.ParseCPU(string(cpuBytes))
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
		memQuantity, parseErr := helpers.ParseMemory(memStr)
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
			r.logger.Errorf("%v Warning: 'lsblk' command failed, skipping disk fact gathering: %v\n", os.Stderr, err)
			disksChan <- []DiskInfo{}
			return nil
		}
		disks, err := parseLsblkOutput(string(stdout))
		if err != nil {
			r.logger.Errorf("%v Warning: failed to parse lsblk output: %v\n", os.Stderr, err)
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
				r.logger.Errorf("%v Warning: failed to get IPv4 default route for host %s (%s): %v\n", os.Stderr, facts.Hostname, facts.OS.ID, execErr)
				ipv4Chan <- ""
			} else {
				ipv4Chan <- strings.TrimSpace(string(ip4Bytes))
			}
		}
		return nil
	})

	g.Go(func() error {
		defer close(interfaceChan)

		var ifaceCmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			ifaceCmd = "ip -4 route get 8.8.8.8 | awk 'NR==1{print $5}'"
		case "darwin":
			ifaceCmd = "route -n get default | awk '/interface:/ {print $2}'"
		}

		if ifaceCmd != "" {
			ifaceBytes, _, execErr := conn.Exec(gCtx, ifaceCmd, nil)
			if execErr != nil {
				r.logger.Errorf("Warning: failed to get default interface for host %s (%s): %v", facts.Hostname, facts.OS.ID, execErr)
				interfaceChan <- ""
			} else {
				interfaceChan <- strings.TrimSpace(string(ifaceBytes))
			}
		} else {
			interfaceChan <- ""
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
				r.logger.Errorf("%v Warning: failed to get IPv6 default route for host %s (%s): %v\n", os.Stderr, facts.Hostname, facts.OS.ID, execErr)
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
	facts.DefaultInterface = <-interfaceChan
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
		r.logger.Errorf("%v Warning: failed to detect package manager for host %s: %v\n", os.Stderr, facts.Hostname, err)
	}

	facts.InitSystem, err = r.detectInitSystem(ctx, conn, facts)
	if err != nil {
		r.logger.Errorf("%v Warning: failed to detect init system for host %s: %v\n", os.Stderr, facts.Hostname, err)
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

func (r *defaultRunner) detectHostPackageManager(ctx context.Context, conn connector.Connector, facts *HostFacts) (*PackageInfo, error) {
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

func (r *defaultRunner) detectHostInitSystem(ctx context.Context, conn connector.Connector, facts *HostFacts) (*ServiceInfo, error) {
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
			strings.Contains(execErr.Error(), "EOF")) {
			return fmt.Errorf("failed to issue reboot command: %w", execErr)
		}
		r.logger.Errorf("%v Reboot command initiated, connection may have dropped as expected: %v\n", os.Stderr, execErr)
	}

	r.logger.Errorf("%v Reboot command sent. Waiting for shutdown to initiate...\n", os.Stderr)
	time.Sleep(10 * time.Second)

	rebootCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	r.logger.Errorf("%v Waiting for host to become responsive after reboot (up to %s)...\n", os.Stderr, timeout)

	for {
		select {
		case <-rebootCtx.Done():
			return fmt.Errorf("timed out waiting for host to become responsive after reboot: %w", rebootCtx.Err())
		case <-ticker.C:
			checkCmd := "uptime"
			_, _, checkErr := conn.Exec(rebootCtx, checkCmd, &connector.ExecOptions{Timeout: 5 * time.Second})

			if checkErr == nil {
				r.logger.Errorf("%v Host is responsive after reboot.\n", os.Stderr)
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

var (
	sudoCache   = make(map[string]bool)
	sudoCacheMu sync.RWMutex
)

func (r *defaultRunner) DetermineSudo(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	cfg := conn.GetConnectionConfig()

	if cfg.User == "root" {
		return false, nil
	}

	dir := filepath.Dir(path)
	sudoCacheMu.RLock()
	needsSudo, found := sudoCache[dir]
	sudoCacheMu.RUnlock()
	if found {
		return needsSudo, nil
	}
	testCmd := fmt.Sprintf("test -w %s", dir)
	_, _, err := r.RunWithOptions(ctx, conn, testCmd, &connector.ExecOptions{Sudo: false})

	if err == nil {

		needsSudo = false
	} else {
		needsSudo = true
	}

	sudoCacheMu.Lock()
	sudoCache[dir] = needsSudo
	sudoCacheMu.Unlock()

	return needsSudo, nil
}

func (r *defaultRunner) GatherHostFacts(ctx context.Context, conn connector.Connector) (*HostFacts, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil for GatheHostFacts")
	}
	if !conn.IsConnected() {
		return nil, fmt.Errorf("connector is not connected for GatheHostFacts")
	}

	osInfo, err := conn.GetOS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OS info: %w", err)
	}
	if osInfo == nil {
		return nil, fmt.Errorf("conn.GetOS returned nil OS without error")
	}

	facts := &HostFacts{
		OS:            osInfo,
		Kernel:        osInfo.Kernel,
		KernelModules: make(map[string]bool),
	}

	factCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	g, gCtx := errgroup.WithContext(factCtx)

	hostnameChan := make(chan string, 1)
	cpuChan := make(chan *CPUInfo, 1)
	memChan := make(chan *MemoryInfo, 1)
	ipv4Chan := make(chan string, 1)
	ipv6Chan := make(chan string, 1)
	disksChan := make(chan []DiskInfo, 1)
	netChan := make(chan []NetworkInterface, 1)
	securityChan := make(chan *SecurityProfile, 1)
	swapChan := make(chan bool, 1)

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
		var cmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			cmd = "lscpu"
		case "darwin":
			cmd = "sysctl -a | grep machdep.cpu"
		default:
			cpuChan <- &CPUInfo{Architecture: facts.OS.Arch}
			return nil
		}

		stdout, _, err := conn.Exec(gCtx, cmd, nil)
		if err != nil {
			r.logger.Warn("lscpu/sysctl for CPU info failed, providing basic info", "error", err)
			cpuChan <- &CPUInfo{Architecture: facts.OS.Arch}
			return nil
		}

		cpuInfo, parseErr := parseCPUInfo(string(stdout), facts.OS.ID)
		if parseErr != nil {
			return fmt.Errorf("failed to parse CPU info: %w", parseErr)
		}
		if cpuInfo.Architecture == "" {
			cpuInfo.Architecture = facts.OS.Arch
		}
		cpuChan <- cpuInfo
		return nil
	})

	g.Go(func() error {
		defer close(memChan)
		var cmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			cmd = "cat /proc/meminfo"
		case "darwin":
			cmd = "sysctl -n hw.memsize"
		default:
			cmd = "cat /proc/meminfo"
		}

		stdout, _, err := conn.Exec(gCtx, cmd, nil)
		if err != nil {
			return fmt.Errorf("failed to get memory info: %w", err)
		}

		memInfo, parseErr := parseMemoryInfo(string(stdout), facts.OS.ID)
		if parseErr != nil {
			return fmt.Errorf("failed to parse memory info: %w", parseErr)
		}
		memChan <- memInfo
		return nil
	})

	g.Go(func() error {
		defer close(netChan)
		var cmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			cmd = "ip -j addr"
		case "darwin":
			cmd = ""
		}
		if cmd == "" {
			netChan <- []NetworkInterface{}
			return nil
		}
		stdout, _, err := conn.Exec(gCtx, cmd, nil)
		if err != nil {
			r.logger.Warn("Failed to get network interface info", "error", err)
			netChan <- []NetworkInterface{}
			return nil
		}
		interfaces, parseErr := parseNetworkInterfaces(string(stdout), facts.OS.ID)
		if parseErr != nil {
			return fmt.Errorf("failed to parse network interfaces: %w", parseErr)
		}
		netChan <- interfaces
		return nil
	})

	g.Go(func() error {
		defer close(securityChan)
		profile := &SecurityProfile{}
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			sestatus, _, err := conn.Exec(gCtx, "sestatus", nil)
			if err == nil {
				profile.SELinuxStatus = parseSELinuxStatus(string(sestatus))
			} else {
				profile.SELinuxStatus = "NotAvailable"
			}
			aaStatus, _, err := conn.Exec(gCtx, "aa-status", nil)
			if err == nil {
				profile.AppArmorEnabled = strings.Contains(string(aaStatus), "apparmor module is loaded")
			}
		}
		securityChan <- profile
		return nil
	})

	g.Go(func() error {
		defer close(swapChan)
		var swapOn bool
		stdout, _, err := conn.Exec(gCtx, "swapon -s", nil)
		if err == nil && len(strings.Split(strings.TrimSpace(string(stdout)), "\n")) > 1 {
			swapOn = true
		}
		swapChan <- swapOn
		return nil
	})

	g.Go(func() error {
		defer close(disksChan)
		lsblkCmd := "lsblk -p -n -l -b -o NAME,SIZE,TYPE,MOUNTPOINT"
		stdout, _, err := conn.Exec(gCtx, lsblkCmd, nil)
		if err != nil {
			r.logger.Warn("lsblk command failed, skipping disk fact gathering", "error", err)
			disksChan <- []DiskInfo{}
			return nil
		}
		disks, err := parseLsblkOutput(string(stdout))
		if err != nil {
			r.logger.Warn("Failed to parse lsblk output", "error", err)
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
		}
		if ip4Cmd != "" {
			ip4Bytes, _, execErr := conn.Exec(gCtx, ip4Cmd, nil)
			if execErr == nil {
				ipv4Chan <- strings.TrimSpace(string(ip4Bytes))
			} else {
				ipv4Chan <- ""
			}
		} else {
			ipv4Chan <- ""
		}
		return nil
	})

	g.Go(func() error {
		defer close(ipv6Chan)
		var ip6Cmd string
		switch strings.ToLower(facts.OS.ID) {
		case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
			ip6Cmd = "ip -6 route get 2001:4860:4860::8888 | awk 'NR==1{print $9}'"
		}
		if ip6Cmd != "" {
			ip6Bytes, _, execErr := conn.Exec(gCtx, ip6Cmd, nil)
			if execErr == nil {
				ipv6Chan <- strings.TrimSpace(string(ip6Bytes))
			} else {
				ipv6Chan <- ""
			}
		} else {
			ipv6Chan <- ""
		}
		return nil
	})

	modulesToCheck := []string{"br_netfilter", "overlay", "ip_vs"}
	for _, mod := range modulesToCheck {
		cmd := fmt.Sprintf("lsmod | grep -w ^%s", mod)
		_, _, err := conn.Exec(gCtx, cmd, nil)
		facts.KernelModules[mod] = (err == nil)
	}

	if err := g.Wait(); err != nil {
		return facts, fmt.Errorf("failed during concurrent fact gathering: %w", err)
	}

	facts.Hostname = <-hostnameChan
	facts.CPU = <-cpuChan
	facts.Memory = <-memChan
	facts.IPv4Default = <-ipv4Chan
	facts.IPv6Default = <-ipv6Chan
	facts.Disks = <-disksChan
	facts.NetworkInterfaces = <-netChan
	facts.Security = <-securityChan
	facts.SwapOn = <-swapChan

	var totalDiskSize int64
	for _, disk := range facts.Disks {
		if disk.Type == "disk" {
			totalDiskSize += disk.Size.Value()
		}
	}
	facts.TotalDisk = *resource.NewQuantity(totalDiskSize, resource.BinarySI)

	facts.PackageManager, err = r.detectHostPackageManager(ctx, conn, facts)
	if err != nil {
		r.logger.Warn("Failed to detect package manager", "host", facts.Hostname, "error", err)
	}

	facts.InitSystem, err = r.detectHostInitSystem(ctx, conn, facts)
	if err != nil {
		r.logger.Warn("Failed to detect init system", "host", facts.Hostname, "error", err)
	}

	return facts, nil
}

func parseCPUInfo(output string, osID string) (*CPUInfo, error) {
	info := &CPUInfo{}
	lines := strings.Split(output, "\n")

	if strings.ToLower(osID) == "darwin" {
		for _, line := range lines {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			switch key {
			case "machdep.cpu.brand_string":
				info.ModelName = val
			case "machdep.cpu.core_count":
				info.CoresPerSocket, _ = strconv.Atoi(val)
			case "machdep.cpu.thread_count":
				info.LogicalCount, _ = strconv.Atoi(val)
			}
		}
		info.Sockets = 1
		if info.CoresPerSocket > 0 {
			info.ThreadsPerCore = info.LogicalCount / info.CoresPerSocket
		}
		return info, nil
	}

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Architecture":
			info.Architecture = val
		case "Model name":
			info.ModelName = val
		case "CPU(s)":
			info.LogicalCount, _ = strconv.Atoi(val)
		case "Socket(s)":
			info.Sockets, _ = strconv.Atoi(val)
		case "Core(s) per socket":
			info.CoresPerSocket, _ = strconv.Atoi(val)
		case "Thread(s) per core":
			info.ThreadsPerCore, _ = strconv.Atoi(val)
		}
	}
	return info, nil
}

func parseMemoryInfo(output string, osID string) (*MemoryInfo, error) {
	info := &MemoryInfo{}
	if strings.ToLower(osID) == "darwin" {
		memBytes, err := strconv.ParseInt(strings.TrimSpace(output), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse darwin memory size: %w", err)
		}
		info.Total = *resource.NewQuantity(memBytes, resource.BinarySI)
		return info, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		val := parts[1]
		switch key {
		case "MemTotal:":
			q, err := resource.ParseQuantity(val + "Ki")
			if err == nil {
				info.Total = q
			}
		case "SwapTotal:":
			q, err := resource.ParseQuantity(val + "Ki")
			if err == nil {
				info.SwapTotal = q
			}
		case "SwapFree:":
			q, err := resource.ParseQuantity(val + "Ki")
			if err == nil {
				info.SwapFree = q
			}
		}
	}
	return info, nil
}

type IpAddrInfo struct {
	Ifname    string `json:"ifname"`
	Operstate string `json:"operstate"`
	LinkType  string `json:"link_type"`
	Address   string `json:"address"`
	AddrInfo  []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		Prefixlen int    `json:"prefixlen"`
	} `json:"addr_info"`
}

func parseNetworkInterfaces(output string, osID string) ([]NetworkInterface, error) {
	if strings.ToLower(osID) != "linux" {
		return []NetworkInterface{}, nil
	}

	var infos []IpAddrInfo
	if err := json.Unmarshal([]byte(output), &infos); err != nil {
		return nil, fmt.Errorf("failed to unmarshal 'ip -j addr' output: %w", err)
	}

	var interfaces []NetworkInterface
	for _, info := range infos {
		if info.Operstate == "DOWN" || info.LinkType == "loopback" {
			continue
		}
		iface := NetworkInterface{
			Name:       info.Ifname,
			MACAddress: info.Address,
			IPv4:       []string{},
			IPv6:       []string{},
		}
		for _, addr := range info.AddrInfo {
			if addr.Family == "inet" {
				iface.IPv4 = append(iface.IPv4, fmt.Sprintf("%s/%d", addr.Local, addr.Prefixlen))
			} else if addr.Family == "inet6" {
				iface.IPv6 = append(iface.IPv6, fmt.Sprintf("%s/%d", addr.Local, addr.Prefixlen))
			}
		}
		interfaces = append(interfaces, iface)
	}
	return interfaces, nil
}

func parseSELinuxStatus(output string) string {
	re := regexp.MustCompile(`Current mode:\s+(\w+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		status := strings.ToLower(matches[1])
		switch status {
		case "enforcing":
			return "Enforcing"
		case "permissive":
			return "Permissive"
		case "disabled":
			return "Disabled"
		}
	}
	return "Unknown"
}
