package runner

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func (r *defaultRunner) IsPortOpen(ctx context.Context, conn connector.Connector, facts *Facts, port int) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if port <= 0 || port > 65535 {
		return false, fmt.Errorf("invalid port number: %d", port)
	}

	cmdToRun := ""
	if _, err := r.LookPath(ctx, conn, "ss"); err == nil {
		cmdToRun = fmt.Sprintf("ss -ltn | grep -q ':%d '", port)
	} else {
		if _, errNetstat := r.LookPath(ctx, conn, "netstat"); errNetstat == nil {
			cmdToRun = fmt.Sprintf("netstat -ltn | grep -q ':%d\\b.*LISTEN'", port)
		} else {
			return false, fmt.Errorf("neither ss nor netstat found on the remote host")
		}
	}
	return r.Check(ctx, conn, cmdToRun, false)
}

func (r *defaultRunner) WaitForPort(ctx context.Context, conn connector.Connector, facts *Facts, port int, timeout time.Duration) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	opCtx, opCancel := context.WithTimeout(ctx, timeout)
	defer opCancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	isOpenInitial, errInitial := r.IsPortOpen(opCtx, conn, facts, port)
	if errInitial != nil {
		if strings.Contains(errInitial.Error(), "neither ss nor netstat found") {
			return fmt.Errorf("cannot wait for port %d, required tools (ss/netstat) not found: %w", port, errInitial)
		}
	}
	if isOpenInitial {
		return nil
	}

	for {
		select {
		case <-opCtx.Done():
			return fmt.Errorf("timed out waiting for port %d to open after %s: %w", port, timeout, opCtx.Err())
		case <-ticker.C:
			isOpen, err := r.IsPortOpen(opCtx, conn, facts, port)
			if err != nil {
				if strings.Contains(err.Error(), "neither ss nor netstat found") {
					return fmt.Errorf("cannot wait for port %d, required tools (ss/netstat) not found: %w", port, err)
				}
			}
			if isOpen {
				return nil
			}
		}
	}
}

func (r *defaultRunner) SetHostname(ctx context.Context, conn connector.Connector, facts *Facts, hostname string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(hostname) == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	cmd := ""
	if _, err := r.LookPath(ctx, conn, "hostnamectl"); err == nil {
		cmd = fmt.Sprintf("hostnamectl set-hostname %s", hostname)
	} else if _, errHostname := r.LookPath(ctx, conn, "hostname"); errHostname == nil {
		cmd = fmt.Sprintf("hostname %s", hostname)
	} else {
		return fmt.Errorf("no suitable command found to set hostname (checked hostnamectl, hostname)")
	}
	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set hostname to %s using command '%s': %w", hostname, cmd, err)
	}
	if !strings.HasPrefix(cmd, "hostnamectl") {
		applyCmd := fmt.Sprintf("hostname %s", hostname)
		_, _, _ = r.RunWithOptions(ctx, conn, applyCmd, &connector.ExecOptions{Sudo: true})
	}
	return nil
}

func (r *defaultRunner) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(ip) == "" || strings.TrimSpace(fqdn) == "" {
		return fmt.Errorf("IP and FQDN cannot be empty for AddHostEntry")
	}
	allHostnames := []string{fqdn}
	allHostnames = append(allHostnames, hostnames...)
	entryLine := fmt.Sprintf("%s %s", ip, strings.Join(allHostnames, " "))

	checkCmdExact := fmt.Sprintf("grep -Fxq '%s' /etc/hosts", entryLine)
	exactExists, errCheck := r.Check(ctx, conn, checkCmdExact, false)
	if errCheck != nil {
		return fmt.Errorf("failed to check /etc/hosts for existing entry '%s': %w", entryLine, errCheck)
	}
	if exactExists {
		return nil
	}
	appendCmd := fmt.Sprintf("echo '%s' >> /etc/hosts", entryLine)
	_, _, errAppend := r.RunWithOptions(ctx, conn, appendCmd, &connector.ExecOptions{Sudo: true})
	if errAppend != nil {
		return fmt.Errorf("failed to add host entry '%s' to /etc/hosts: %w", entryLine, errAppend)
	}
	return nil
}

func (r *defaultRunner) GetInterfaceAddresses(ctx context.Context, conn connector.Connector, interfaceName string) (map[string][]string, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil for GetInterfaceAddresses")
	}
	if strings.TrimSpace(interfaceName) == "" {
		return nil, fmt.Errorf("interfaceName cannot be empty for GetInterfaceAddresses")
	}

	osInfo, err := conn.GetOS(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OS info for GetInterfaceAddresses: %w", err)
	}
	if osInfo == nil {
		return nil, fmt.Errorf("OS info returned as nil by connector, cannot determine command for GetInterfaceAddresses")
	}

	var cmdStr string
	var outputParser func(output string) map[string][]string

	linuxInetRegex := regexp.MustCompile(`^\s*inet\s+([0-9a-fA-F:.]+)/`)
	linuxInet6Regex := regexp.MustCompile(`^\s*inet6\s+([0-9a-fA-F:.]+)/`)

	darwinInetRegex := regexp.MustCompile(`^\s*inet\s+([0-9a-fA-F:.]+)\s+netmask`)
	darwinInet6Regex := regexp.MustCompile(`^\s*inet6\s+([0-9a-fA-F:.]+)%[a-zA-Z0-9]+\s+prefixlen`)
	darwinInet6SimpleRegex := regexp.MustCompile(`^\s*inet6\s+([0-9a-fA-F:.]+)\s+prefixlen`)

	switch strings.ToLower(osInfo.ID) {
	case "linux", "ubuntu", "debian", "centos", "rhel", "fedora", "almalinux", "rocky", "raspbian", "linuxmint":
		cmdStr = fmt.Sprintf("ip addr show dev %s", interfaceName)
		outputParser = func(output string) map[string][]string {
			res := map[string][]string{"ipv4": {}, "ipv6": {}}
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				if matches := linuxInetRegex.FindStringSubmatch(line); len(matches) > 1 {
					res["ipv4"] = append(res["ipv4"], matches[1])
				} else if matches := linuxInet6Regex.FindStringSubmatch(line); len(matches) > 1 {
					res["ipv6"] = append(res["ipv6"], matches[1])
				}
			}
			return res
		}
	case "darwin":
		cmdStr = fmt.Sprintf("ifconfig %s", interfaceName)
		outputParser = func(output string) map[string][]string {
			res := map[string][]string{"ipv4": {}, "ipv6": {}}
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				trimmedLine := strings.TrimSpace(line)
				if matches := darwinInetRegex.FindStringSubmatch(trimmedLine); len(matches) > 1 {
					res["ipv4"] = append(res["ipv4"], matches[1])
				} else if matches := darwinInet6Regex.FindStringSubmatch(trimmedLine); len(matches) > 1 {
					res["ipv6"] = append(res["ipv6"], matches[1])
				} else if matches := darwinInet6SimpleRegex.FindStringSubmatch(trimmedLine); len(matches) > 1 {
					res["ipv6"] = append(res["ipv6"], matches[1])
				}
			}
			return res
		}
	default:
		return nil, fmt.Errorf("GetInterfaceAddresses not supported for OS ID: %s", osInfo.ID)
	}

	stdout, stderr, execErr := r.RunWithOptions(ctx, conn, cmdStr, &connector.ExecOptions{Sudo: false})
	if execErr != nil {
		errStr := strings.ToLower(string(stderr))
		if strings.Contains(errStr, "does not exist") || strings.Contains(errStr, "no such device") {
			return map[string][]string{"ipv4": {}, "ipv6": {}}, nil
		}
		return nil, fmt.Errorf("failed to execute command '%s' for interface %s: %w (stderr: %s)", cmdStr, interfaceName, execErr, string(stderr))
	}

	return outputParser(string(stdout)), nil
}

func (r *defaultRunner) DisableFirewall(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.OS == nil {
		return fmt.Errorf("OS facts not available, cannot determine how to disable firewall")
	}
	if facts.InitSystem == nil {
		fmt.Printf("Warning: InitSystem facts not available for DisableFirewall. Will rely on LookPath only for some checks.\n")
	}

	if _, err := r.LookPath(ctx, conn, "firewall-cmd"); err == nil {
		if facts.InitSystem != nil && facts.InitSystem.Type == InitSystemSystemd {
			stopCmd := fmt.Sprintf(facts.InitSystem.StopCmd, "firewalld")
			disableServiceCmd := fmt.Sprintf(facts.InitSystem.DisableCmd, "firewalld")

			_, _, errStop := r.RunWithOptions(ctx, conn, stopCmd, &connector.ExecOptions{Sudo: true})
			if errStop != nil {
				fmt.Printf("Warning: command '%s' failed during DisableFirewall: %v. Attempting to disable service.\n", stopCmd, errStop)
			}
			_, _, errDisable := r.RunWithOptions(ctx, conn, disableServiceCmd, &connector.ExecOptions{Sudo: true})
			if errDisable != nil {
				return fmt.Errorf("failed to disable firewalld service using systemctl: %w", errDisable)
			}
			fmt.Println("firewalld service stopped and disabled.")
			return nil
		} else {
			return fmt.Errorf("firewall-cmd found but not on a recognized systemd system for service management; automatic disable not fully supported")
		}
	}

	if _, err := r.LookPath(ctx, conn, "ufw"); err == nil {
		cmd := "ufw disable"
		_, stderr, errExec := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
		if errExec != nil {
			var cmdErr *connector.CommandError
			if errors.As(errExec, &cmdErr) && strings.Contains(strings.ToLower(string(stderr)), "firewall not enabled") {
				fmt.Println("ufw is already disabled.")
				return nil
			}
			return fmt.Errorf("failed to execute 'ufw disable': %w (stderr: %s)", errExec, string(stderr))
		}
		fmt.Println("ufw disabled.")
		return nil
	}

	if _, err := r.LookPath(ctx, conn, "iptables"); err == nil {
		fmt.Println("Attempting to disable firewall by flushing iptables rules and setting default policies to ACCEPT.")
		commands := []string{
			"iptables -P INPUT ACCEPT",
			"iptables -P FORWARD ACCEPT",
			"iptables -P OUTPUT ACCEPT",
			"iptables -F",
			"iptables -X",
			"iptables -Z",
		}
		if _, errIp6 := r.LookPath(ctx, conn, "ip6tables"); errIp6 == nil {
			commands = append(commands,
				"ip6tables -P INPUT ACCEPT",
				"ip6tables -P FORWARD ACCEPT",
				"ip6tables -P OUTPUT ACCEPT",
				"ip6tables -F",
				"ip6tables -X",
				"ip6tables -Z",
			)
		}

		var encounteredError bool
		for _, cmd := range commands {
			_, stderr, errExec := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
			if errExec != nil {
				fmt.Printf("Warning: command '%s' failed during iptables configuration: %v (stderr: %s). Continuing...\n", cmd, errExec, string(stderr))
				encounteredError = true
			}
		}
		if encounteredError {
			return fmt.Errorf("one or more iptables commands failed; firewall may not be fully open. Check warnings.")
		}
		fmt.Println("iptables rules flushed and default policies set to ACCEPT. Note: This may not prevent rules from being reloaded by a persistence service.")
		return nil
	}

	return fmt.Errorf("no known firewall management tool (firewalld, ufw, iptables) found. Cannot automatically disable firewall.")
}

func (r *defaultRunner) EnsureHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(ip) == "" || strings.TrimSpace(fqdn) == "" {
		return fmt.Errorf("IP and FQDN cannot be empty")
	}

	allNewHostnames := append([]string{fqdn}, hostnames...)
	uniqueNewHostnames := []string{}
	seen := make(map[string]bool)
	for _, h := range allNewHostnames {
		if !seen[h] {
			seen[h] = true
			uniqueNewHostnames = append(uniqueNewHostnames, h)
		}
	}
	newLine := fmt.Sprintf("%s %s", ip, strings.Join(uniqueNewHostnames, " "))

	checkCmdExact := fmt.Sprintf("grep -Fxq %s /etc/hosts", newLine)
	if exists, _ := r.Check(ctx, conn, checkCmdExact, false); exists {
		return nil
	}

	checkCmdIP := fmt.Sprintf("grep -w %s /etc/hosts", ip)
	stdout, _, err := r.RunWithOptions(ctx, conn, checkCmdIP, &connector.ExecOptions{Sudo: false})

	var finalCmd string

	if err != nil {
		finalCmd = fmt.Sprintf("echo %s >> /etc/hosts", newLine)
	} else {
		oldLine := strings.TrimSpace(string(stdout))
		oldFields := strings.Fields(oldLine)
		existingHostnames := make(map[string]bool)
		if len(oldFields) > 1 {
			for _, h := range oldFields[1:] {
				existingHostnames[h] = true
			}
		}

		hostnamesToAppend := []string{}
		for _, newHost := range uniqueNewHostnames {
			if !existingHostnames[newHost] {
				hostnamesToAppend = append(hostnamesToAppend, newHost)
			}
		}

		if len(hostnamesToAppend) > 0 {
			updatedLine := fmt.Sprintf("%s %s", oldLine, strings.Join(hostnamesToAppend, " "))
			escapedOldLine := strings.ReplaceAll(oldLine, "/", "\\/")
			escapedUpdatedLine := strings.ReplaceAll(updatedLine, "/", "\\/")
			finalCmd = fmt.Sprintf("sed -i 's/%s/%s/g' /etc/hosts", escapedOldLine, escapedUpdatedLine)
		} else {
			return nil
		}
	}

	_, _, execErr := r.RunWithOptions(ctx, conn, finalCmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to ensure host entry '%s': %w", newLine, execErr)
	}

	return nil
}
