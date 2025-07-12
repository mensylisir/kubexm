package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// MockConnector implements connector.Connector for integration testing
type MockConnector struct {
	// Configuration
	host     string
	port     int
	user     string
	password string

	// Mock responses
	osInfo           *connector.OS
	commandResponses map[string]*MockCommandResponse
	fileContents     map[string][]byte
	directories      map[string]bool
	
	// Behavior controls
	simulateErrors    bool
	simulateTimeouts  bool
	commandDelay     time.Duration
	
	// Tracking
	executedCommands []string
	lastCommand      string
	lastOptions      *connector.ExecOptions
}

// MockCommandResponse defines how a command should respond
type MockCommandResponse struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
	Delay    time.Duration
}

// NewMockConnector creates a new mock connector
func NewMockConnector() *MockConnector {
	return &MockConnector{
		host:             "mock.example.com",
		port:             22,
		user:             "testuser",
		commandResponses: make(map[string]*MockCommandResponse),
		fileContents:     make(map[string][]byte),
		directories:      make(map[string]bool),
		executedCommands: make([]string, 0),
	}
}

// SetupMockOS configures the mock OS information
func (m *MockConnector) SetupMockOS(id, version, arch string) {
	m.osInfo = &connector.OS{
		ID:         id,
		VersionID:  version,
		PrettyName: fmt.Sprintf("%s %s", strings.Title(id), version),
		Arch:       arch,
		Kernel:     "5.4.0-mock-generic",
	}
}

// SetupCommandResponse configures how a specific command should respond
func (m *MockConnector) SetupCommandResponse(cmd string, stdout, stderr string, exitCode int) {
	m.commandResponses[cmd] = &MockCommandResponse{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
}

// SetupCommandError configures a command to return an error
func (m *MockConnector) SetupCommandError(cmd string, exitCode int, stderr string) {
	m.commandResponses[cmd] = &MockCommandResponse{
		Stderr:   stderr,
		ExitCode: exitCode,
		Error:    &connector.CommandError{ExitCode: exitCode, Stderr: stderr},
	}
}

// SetupCommandTimeout configures a command to timeout
func (m *MockConnector) SetupCommandTimeout(cmd string) {
	m.commandResponses[cmd] = &MockCommandResponse{
		Error: errors.New("command timeout"),
		Delay: 10 * time.Second, // Longer than typical test timeouts
	}
}

// GetOS returns mock OS information
func (m *MockConnector) GetOS(ctx context.Context) (*connector.OS, error) {
	if m.osInfo == nil {
		m.SetupMockOS("ubuntu", "20.04", "amd64") // Default
	}
	return m.osInfo, nil
}

// Exec simulates command execution
func (m *MockConnector) Exec(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
	m.lastCommand = cmd
	m.lastOptions = options
	m.executedCommands = append(m.executedCommands, cmd)

	// Add artificial delay if configured
	if m.commandDelay > 0 {
		time.Sleep(m.commandDelay)
	}

	// Check for specific command responses
	if response, exists := m.commandResponses[cmd]; exists {
		if response.Delay > 0 {
			select {
			case <-time.After(response.Delay):
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		
		if response.Error != nil {
			return []byte(response.Stdout), []byte(response.Stderr), response.Error
		}
		
		if response.ExitCode != 0 {
			return []byte(response.Stdout), []byte(response.Stderr), &connector.CommandError{
				ExitCode: response.ExitCode,
				Stderr:   response.Stderr,
				Stdout:   response.Stdout,
			}
		}
		
		return []byte(response.Stdout), []byte(response.Stderr), nil
	}

	// Default command handling based on command patterns
	return m.handleDefaultCommand(ctx, cmd)
}

// handleDefaultCommand provides default responses for common commands
func (m *MockConnector) handleDefaultCommand(ctx context.Context, cmd string) ([]byte, []byte, error) {
	cmd = strings.TrimSpace(cmd)
	
	// Handle common commands
	switch {
	case strings.HasPrefix(cmd, "echo "):
		// Extract the echo content
		content := strings.TrimPrefix(cmd, "echo ")
		content = strings.Trim(content, "'\"")
		return []byte(content), nil, nil
		
	case cmd == "hostname":
		return []byte("mock-hostname"), nil, nil
		
	case cmd == "nproc":
		return []byte("4"), nil, nil
		
	case strings.Contains(cmd, "free -m"):
		return []byte("MemTotal: 8192 MiB"), nil, nil
		
	case strings.Contains(cmd, "df -h"):
		return []byte("Filesystem      Size  Used Avail Use% Mounted on\n/dev/sda1        20G  5.0G   14G  27% /"), nil, nil
		
	case strings.HasPrefix(cmd, "cat "):
		filepath := strings.TrimPrefix(cmd, "cat ")
		if content, exists := m.fileContents[filepath]; exists {
			return content, nil, nil
		}
		return nil, []byte("cat: " + filepath + ": No such file or directory"), &connector.CommandError{ExitCode: 1}
		
	case strings.HasPrefix(cmd, "test -f "):
		filepath := strings.TrimPrefix(cmd, "test -f ")
		if _, exists := m.fileContents[filepath]; exists {
			return nil, nil, nil // exit code 0
		}
		return nil, nil, &connector.CommandError{ExitCode: 1}
		
	case strings.HasPrefix(cmd, "test -d "):
		dirpath := strings.TrimPrefix(cmd, "test -d ")
		if exists, _ := m.directories[dirpath]; exists {
			return nil, nil, nil // exit code 0
		}
		return nil, nil, &connector.CommandError{ExitCode: 1}
		
	case strings.HasPrefix(cmd, "mkdir -p "):
		dirpath := strings.TrimPrefix(cmd, "mkdir -p ")
		m.directories[dirpath] = true
		return nil, nil, nil
		
	case strings.HasPrefix(cmd, "rm "):
		// Handle file/directory removal
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			target := parts[len(parts)-1]
			delete(m.fileContents, target)
			delete(m.directories, target)
		}
		return nil, nil, nil
		
	case strings.Contains(cmd, "systemctl"):
		// Mock systemctl commands
		return []byte("mock systemctl output"), nil, nil
		
	case strings.Contains(cmd, "apt-get") || strings.Contains(cmd, "yum") || strings.Contains(cmd, "dnf"):
		// Mock package manager commands
		return []byte("mock package manager output"), nil, nil
		
	case strings.Contains(cmd, "which ") || strings.Contains(cmd, "type "):
		// Mock which/type commands
		return []byte("/usr/bin/mock"), nil, nil
		
	case strings.HasPrefix(cmd, "sleep "):
		// Handle sleep commands
		return nil, nil, nil

	case strings.Contains(cmd, "docker"):
		// Handle Docker commands
		return m.handleDockerCommand(ctx, cmd)
		
	case strings.Contains(cmd, "kubectl"):
		// Handle kubectl commands
		return m.handleKubectlCommand(ctx, cmd)
		
	case strings.Contains(cmd, "kubeadm"):
		// Handle kubeadm commands
		return m.handleKubeadmCommand(ctx, cmd)
		
	case strings.Contains(cmd, "ping"):
		// Handle ping commands
		return []byte("PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.\n64 bytes from 8.8.8.8: icmp_seq=1 ttl=119 time=10.1 ms"), nil, nil
		
	case strings.Contains(cmd, "ssh-keygen"):
		// Handle SSH key generation
		return []byte("Generating public/private rsa key pair."), nil, nil
		
	case strings.Contains(cmd, "openssl"):
		// Handle OpenSSL commands
		return []byte("Certificate generated successfully"), nil, nil
		
	case strings.HasPrefix(cmd, "stat "):
		// Handle file stats
		filepath := strings.TrimPrefix(cmd, "stat ")
		if _, exists := m.fileContents[filepath]; exists {
			return []byte("  File: " + filepath + "\n  Size: 1024\n  Access: (0644/-rw-r--r--)  Uid: (1000/user)   Gid: (1000/user)"), nil, nil
		}
		return nil, []byte("stat: cannot stat '" + filepath + "': No such file or directory"), &connector.CommandError{ExitCode: 1}
		
	case strings.HasPrefix(cmd, "ls -la "):
		// Handle detailed file listing
		dirpath := strings.TrimPrefix(cmd, "ls -la ")
		if exists, _ := m.directories[dirpath]; exists {
			return []byte("total 8\ndrwxr-xr-x 2 user user 4096 Jan  1 12:00 .\ndrwxr-xr-x 3 user user 4096 Jan  1 12:00 .."), nil, nil
		}
		return nil, []byte("ls: cannot access '" + dirpath + "': No such file or directory"), &connector.CommandError{ExitCode: 1}
		
	case strings.Contains(cmd, "netstat") || strings.Contains(cmd, "ss"):
		// Handle network stats
		return []byte("tcp    0    0 0.0.0.0:22    0.0.0.0:*    LISTEN\ntcp    0    0 0.0.0.0:80    0.0.0.0:*    LISTEN"), nil, nil
		
	case strings.Contains(cmd, "ps"):
		// Handle process listing
		return []byte("  PID TTY          TIME CMD\n 1234 ?        00:00:01 systemd\n 5678 ?        00:00:00 sshd"), nil, nil
		
	case strings.Contains(cmd, "ip addr") || strings.Contains(cmd, "ifconfig"):
		// Handle network interface listing
		return []byte("1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536\n    inet 127.0.0.1/8 scope host lo\n2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500\n    inet 192.168.1.100/24 brd 192.168.1.255 scope global eth0"), nil, nil
		
	case strings.Contains(cmd, "ufw status") || strings.Contains(cmd, "iptables"):
		// Handle firewall status
		return []byte("Status: active"), nil, nil
		
	case strings.Contains(cmd, "df"):
		// Handle disk space
		return []byte("Filesystem     1K-blocks    Used Available Use% Mounted on\n/dev/sda1       20971520 5242880  15728640  26% /"), nil, nil

	default:
		// Unknown command
		return nil, []byte("mock: command not found: " + cmd), &connector.CommandError{ExitCode: 127}
	}
}

// LookPath simulates looking up executable paths
func (m *MockConnector) LookPath(ctx context.Context, file string) (string, error) {
	// Common executables
	commonPaths := map[string]string{
		"bash":       "/bin/bash",
		"sh":         "/bin/sh",
		"systemctl":  "/usr/bin/systemctl",
		"apt-get":    "/usr/bin/apt-get",
		"yum":        "/usr/bin/yum",
		"dnf":        "/usr/bin/dnf",
		"docker":     "/usr/bin/docker",
		"kubectl":    "/usr/local/bin/kubectl",
		"kubeadm":    "/usr/local/bin/kubeadm",
		"kubelet":    "/usr/local/bin/kubelet",
		"containerd": "/usr/bin/containerd",
		"runc":       "/usr/bin/runc",
	}
	
	if path, exists := commonPaths[file]; exists {
		return path, nil
	}
	
	return "", errors.New("executable not found: " + file)
}

// SetFileContent sets the content for a mock file
func (m *MockConnector) SetFileContent(filepath string, content []byte) {
	m.fileContents[filepath] = content
}

// SetDirectoryExists marks a directory as existing
func (m *MockConnector) SetDirectoryExists(dirpath string) {
	m.directories[dirpath] = true
}

// GetExecutedCommands returns all executed commands for verification
func (m *MockConnector) GetExecutedCommands() []string {
	return m.executedCommands
}

// GetLastCommand returns the last executed command
func (m *MockConnector) GetLastCommand() string {
	return m.lastCommand
}

// GetLastOptions returns the last execution options
func (m *MockConnector) GetLastOptions() *connector.ExecOptions {
	return m.lastOptions
}

// SimulateErrors enables error simulation
func (m *MockConnector) SimulateErrors(enable bool) {
	m.simulateErrors = enable
}

// SimulateTimeouts enables timeout simulation
func (m *MockConnector) SimulateTimeouts(enable bool) {
	m.simulateTimeouts = enable
}

// SetCommandDelay sets a delay for all commands
func (m *MockConnector) SetCommandDelay(delay time.Duration) {
	m.commandDelay = delay
}

// Reset clears all mock state
func (m *MockConnector) Reset() {
	m.commandResponses = make(map[string]*MockCommandResponse)
	m.fileContents = make(map[string][]byte)
	m.directories = make(map[string]bool)
	m.executedCommands = make([]string, 0)
	m.lastCommand = ""
	m.lastOptions = nil
	m.simulateErrors = false
	m.simulateTimeouts = false
	m.commandDelay = 0
}

// handleDockerCommand handles Docker-specific commands
func (m *MockConnector) handleDockerCommand(ctx context.Context, cmd string) ([]byte, []byte, error) {
	switch {
	case strings.Contains(cmd, "docker images"):
		return []byte("REPOSITORY    TAG       IMAGE ID       CREATED        SIZE\nnginx         latest    abcd1234       2 days ago     142MB"), nil, nil
	case strings.Contains(cmd, "docker ps"):
		return []byte("CONTAINER ID   IMAGE     COMMAND                  CREATED        STATUS        PORTS     NAMES\n1234567890ab   nginx     \"/docker-entrypoint.…\"   2 hours ago    Up 2 hours    80/tcp    test-nginx"), nil, nil
	case strings.Contains(cmd, "docker pull"):
		return []byte("Using default tag: latest\nlatest: Pulling from library/nginx\nDigest: sha256:abcd1234\nStatus: Downloaded newer image for nginx:latest"), nil, nil
	case strings.Contains(cmd, "docker run"):
		return []byte("1234567890abcdef"), nil, nil // Container ID
	case strings.Contains(cmd, "docker start"):
		return []byte("Container started"), nil, nil
	case strings.Contains(cmd, "docker stop"):
		return []byte("Container stopped"), nil, nil
	case strings.Contains(cmd, "docker rm"):
		return []byte("Container removed"), nil, nil
	case strings.Contains(cmd, "docker rmi"):
		return []byte("Image removed"), nil, nil
	case strings.Contains(cmd, "docker network"):
		if strings.Contains(cmd, "ls") {
			return []byte("NETWORK ID     NAME      DRIVER    SCOPE\nabcd1234       bridge    bridge    local\nefgh5678       host      host      local"), nil, nil
		}
		return []byte("Network operation completed"), nil, nil
	case strings.Contains(cmd, "docker volume"):
		if strings.Contains(cmd, "ls") {
			return []byte("DRIVER    VOLUME NAME\nlocal     test-volume"), nil, nil
		}
		return []byte("Volume operation completed"), nil, nil
	case strings.Contains(cmd, "docker-compose"):
		return []byte("Docker Compose operation completed"), nil, nil
	default:
		return []byte("Docker operation completed"), nil, nil
	}
}

// handleKubectlCommand handles kubectl-specific commands
func (m *MockConnector) handleKubectlCommand(ctx context.Context, cmd string) ([]byte, []byte, error) {
	switch {
	case strings.Contains(cmd, "kubectl cluster-info"):
		return []byte("Kubernetes control plane is running at https://127.0.0.1:6443\nCoreDNS is running at https://127.0.0.1:6443/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy"), nil, nil
	case strings.Contains(cmd, "kubectl version"):
		return []byte("Client Version: version.Info{Major:\"1\", Minor:\"28\", GitVersion:\"v1.28.0\"}\nServer Version: version.Info{Major:\"1\", Minor:\"28\", GitVersion:\"v1.28.0\"}"), nil, nil
	case strings.Contains(cmd, "kubectl get pods"):
		return []byte("[{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"name\":\"test-pod\",\"namespace\":\"default\"}}]"), nil, nil
	case strings.Contains(cmd, "kubectl get deployments"):
		return []byte("[{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"name\":\"test-deployment\",\"namespace\":\"default\"}}]"), nil, nil
	case strings.Contains(cmd, "kubectl get services"):
		return []byte("[{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-service\",\"namespace\":\"default\"}}]"), nil, nil
	case strings.Contains(cmd, "kubectl apply"):
		return []byte("configmap/test-config created"), nil, nil
	case strings.Contains(cmd, "kubectl delete"):
		return []byte("configmap \"test-config\" deleted"), nil, nil
	case strings.Contains(cmd, "kubectl exec"):
		return []byte("hello"), nil, nil
	case strings.Contains(cmd, "kubectl scale"):
		return []byte("deployment.apps/test-deployment scaled"), nil, nil
	case strings.Contains(cmd, "kubectl rollout history"):
		return []byte("deployment.apps/test-deployment\nREVISION  CHANGE-CAUSE\n1         Initial deployment"), nil, nil
	case strings.Contains(cmd, "kubectl rollout undo"):
		return []byte("deployment.apps/test-deployment rolled back"), nil, nil
	case strings.Contains(cmd, "kubectl rollout status"):
		return []byte("deployment \"test-deployment\" successfully rolled out"), nil, nil
	case strings.Contains(cmd, "kubectl logs"):
		return []byte("Mock log output\nAnother log line"), nil, nil
	case strings.Contains(cmd, "kubectl port-forward"):
		return []byte("Forwarding from 127.0.0.1:8080 -> 80"), nil, nil
	default:
		return []byte("kubectl operation completed"), nil, nil
	}
}

// handleKubeadmCommand handles kubeadm-specific commands
func (m *MockConnector) handleKubeadmCommand(ctx context.Context, cmd string) ([]byte, []byte, error) {
	switch {
	case strings.Contains(cmd, "kubeadm version"):
		return []byte("kubeadm version: &version.Info{Major:\"1\", Minor:\"28\", GitVersion:\"v1.28.0\"}"), nil, nil
	case strings.Contains(cmd, "kubeadm init"):
		return []byte("Your Kubernetes control-plane has initialized successfully!\nTo start using your cluster, you need to run the following as a regular user:\n  mkdir -p $HOME/.kube\n  sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config\n  sudo chown $(id -u):$(id -g) $HOME/.kube/config"), nil, nil
	case strings.Contains(cmd, "kubeadm join"):
		return []byte("This node has joined the cluster"), nil, nil
	case strings.Contains(cmd, "kubeadm reset"):
		return []byte("The reset process does not clean CNI configuration. To do so, you must remove /etc/cni/net.d"), nil, nil
	case strings.Contains(cmd, "kubeadm token"):
		return []byte("abcdef.0123456789abcdef"), nil, nil
	default:
		return []byte("kubeadm operation completed"), nil, nil
	}
}

// Verify interface compliance
var _ connector.Connector = (*MockConnector)(nil)