package loadbalancer

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
)

// LoadBalancerType defines the type of load balancer
type LoadBalancerType string

const (
	LoadBalancerTypeHAProxy LoadBalancerType = "haproxy"
	LoadBalancerTypeNginx   LoadBalancerType = "nginx"
	LoadBalancerTypeKubeVIP LoadBalancerType = "kube-vip"
)

// LoadBalancerMode defines whether load balancer is external or internal
type LoadBalancerMode string

const (
	LoadBalancerModeExternal LoadBalancerMode = "external"
	LoadBalancerModeInternal LoadBalancerMode = "internal"
)

// DeploymentType defines how the load balancer is deployed
type DeploymentType string

const (
	DeploymentTypeSystemd   DeploymentType = "systemd"
	DeploymentTypeStaticPod DeploymentType = "static_pod"
)

// LoadBalancerConfig holds the load balancer configuration
type LoadBalancerConfig struct {
	Type            LoadBalancerType `json:"type"`
	Mode            LoadBalancerMode `json:"mode"`
	DeploymentType  DeploymentType   `json:"deployment_type"`
	VIP             string           `json:"vip"`
	Port            int              `json:"port"`
	Servers         []ServerConfig   `json:"servers"`
	Interface       string           `json:"interface,omitempty"`
	VirtualRouterID int              `json:"virtual_router_id,omitempty"`
	Priority        int              `json:"priority,omitempty"`
	AuthPass        string           `json:"auth_pass,omitempty"`
}

// ServerConfig holds backend server configuration
type ServerConfig struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Weight  int    `json:"weight,omitempty"`
}

// LoadBalancerRunner provides load balancer operations
type LoadBalancerRunner struct {
	runner runner.Runner
}

// NewLoadBalancerRunner creates a new load balancer runner
func NewLoadBalancerRunner(r runner.Runner) *LoadBalancerRunner {
	return &LoadBalancerRunner{
		runner: r,
	}
}

// InstallKeepalived installs Keepalived
func (r *LoadBalancerRunner) InstallKeepalived(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()
	startTime := time.Now()

	var installCmd string
	if facts.PackageManager != nil {
		switch facts.PackageManager.Type {
		case runner.PackageManagerApt:
			installCmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
		case runner.PackageManagerYum, runner.PackageManagerDnf:
			installCmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "keepalived")
		default:
			result.MarkFailed(nil, "unsupported package manager")
			return result
		}
	} else {
		result.MarkFailed(nil, "package manager not detected")
		return result
	}

	_, _, err := conn.Exec(ctx, installCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to install keepalived: %v", err))
		return result
	}

	result.MarkSuccess(fmt.Sprintf("keepalived installed successfully in %v", time.Since(startTime)))
	return result
}

// GenerateKeepalivedConfig generates Keepalived configuration
func (r *LoadBalancerRunner) GenerateKeepalivedConfig(
	ctx context.Context,
	conn connector.Connector,
	interfaceName string,
	virtualRouterID int,
	priority int,
	vip string,
	peers []string,
	authPass string,
) ([]byte, *runner.RunnerResult) {
	result := runner.NewRunnerResult()

	config := generateKeepalivedConfContent(interfaceName, virtualRouterID, priority, vip, peers, authPass)

	result.MarkSuccess("keepalived config generated")
	return []byte(config), result
}

// DeployKeepalivedConfig deploys Keepalived configuration
func (r *LoadBalancerRunner) DeployKeepalivedConfig(
	ctx context.Context,
	conn connector.Connector,
	configContent []byte,
	configPath string,
	permissions string,
) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	dir := filepath.Dir(configPath)
	if err := r.runner.Mkdirp(ctx, conn, dir, "0755", true); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to create config directory: %v", err))
		return result
	}

	if err := r.runner.WriteFile(ctx, conn, configContent, configPath, permissions, true); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write keepalived config: %v", err))
		return result
	}

	result.MarkSuccess(fmt.Sprintf("keepalived config deployed to %s", configPath))
	return result
}

// EnableKeepalived enables Keepalived service
func (r *LoadBalancerRunner) EnableKeepalived(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	enableCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, "keepalived")
	_, _, err := conn.Exec(ctx, enableCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to enable keepalived: %v", err))
		return result
	}

	result.MarkSuccess("keepalived enabled")
	return result
}

// StartKeepalived starts Keepalived service
func (r *LoadBalancerRunner) StartKeepalived(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	startCmd := fmt.Sprintf(facts.InitSystem.StartCmd, "keepalived")
	_, _, err := conn.Exec(ctx, startCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to start keepalived: %v", err))
		return result
	}

	result.MarkSuccess("keepalived started")
	return result
}

// RestartKeepalived restarts Keepalived service
func (r *LoadBalancerRunner) RestartKeepalived(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	if facts.InitSystem.DaemonReloadCmd != "" {
		conn.Exec(ctx, facts.InitSystem.DaemonReloadCmd, nil)
	}

	restartCmd := fmt.Sprintf(facts.InitSystem.RestartCmd, "keepalived")
	_, _, err := conn.Exec(ctx, restartCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to restart keepalived: %v", err))
		return result
	}

	result.MarkSuccess("keepalived restarted")
	return result
}

// IsKeepalivedActive checks if Keepalived is active
func (r *LoadBalancerRunner) IsKeepalivedActive(ctx context.Context, conn connector.Connector, facts *runner.Facts) (bool, *runner.RunnerResult) {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return false, result
	}

	isActiveCmd := fmt.Sprintf(facts.InitSystem.IsActiveCmd, "keepalived")
	_, _, err := conn.Exec(ctx, isActiveCmd, nil)
	if err != nil {
		result.MarkSkipped("keepalived not active")
		return false, result
	}

	result.MarkSuccess("keepalived is active")
	return true, result
}

// InstallHAProxy installs HAProxy
func (r *LoadBalancerRunner) InstallHAProxy(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.PackageManager == nil {
		result.MarkFailed(nil, "package manager not detected")
		return result
	}

	var installCmd string
	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		installCmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "haproxy")
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		installCmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "haproxy")
	default:
		result.MarkFailed(nil, "unsupported package manager")
		return result
	}

	_, _, err := conn.Exec(ctx, installCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to install haproxy: %v", err))
		return result
	}

	result.MarkSuccess("haproxy installed")
	return result
}

// GenerateHAProxyConfig generates HAProxy configuration
func (r *LoadBalancerRunner) GenerateHAProxyConfig(servers []ServerConfig, frontendPort int, mode string) ([]byte, *runner.RunnerResult) {
	result := runner.NewRunnerResult()

	config := generateHAProxyConfigContent(servers, frontendPort, mode)
	result.MarkSuccess("haproxy config generated")

	return []byte(config), result
}

// DeployHAProxyConfig deploys HAProxy configuration
func (r *LoadBalancerRunner) DeployHAProxyConfig(
	ctx context.Context,
	conn connector.Connector,
	configContent []byte,
	configPath string,
	permissions string,
) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if err := r.runner.WriteFile(ctx, conn, configContent, configPath, permissions, true); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write haproxy config: %v", err))
		return result
	}

	result.MarkSuccess(fmt.Sprintf("haproxy config deployed to %s", configPath))
	return result
}

// EnableHAProxy enables HAProxy service
func (r *LoadBalancerRunner) EnableHAProxy(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	enableCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, "haproxy")
	_, _, err := conn.Exec(ctx, enableCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to enable haproxy: %v", err))
		return result
	}

	result.MarkSuccess("haproxy enabled")
	return result
}

// StartHAProxy starts HAProxy service
func (r *LoadBalancerRunner) StartHAProxy(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	startCmd := fmt.Sprintf(facts.InitSystem.StartCmd, "haproxy")
	_, _, err := conn.Exec(ctx, startCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to start haproxy: %v", err))
		return result
	}

	result.MarkSuccess("haproxy started")
	return result
}

// RestartHAProxy restarts HAProxy service
func (r *LoadBalancerRunner) RestartHAProxy(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	restartCmd := fmt.Sprintf(facts.InitSystem.RestartCmd, "haproxy")
	_, _, err := conn.Exec(ctx, restartCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to restart haproxy: %v", err))
		return result
	}

	result.MarkSuccess("haproxy restarted")
	return result
}

// InstallNginx installs Nginx
func (r *LoadBalancerRunner) InstallNginx(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.PackageManager == nil {
		result.MarkFailed(nil, "package manager not detected")
		return result
	}

	var installCmd string
	switch facts.PackageManager.Type {
	case runner.PackageManagerApt:
		installCmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "nginx")
	case runner.PackageManagerYum, runner.PackageManagerDnf:
		installCmd = fmt.Sprintf(facts.PackageManager.InstallCmd, "nginx")
	default:
		result.MarkFailed(nil, "unsupported package manager")
		return result
	}

	_, _, err := conn.Exec(ctx, installCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to install nginx: %v", err))
		return result
	}

	result.MarkSuccess("nginx installed")
	return result
}

// GenerateNginxLBConfig generates Nginx load balancer configuration
func (r *LoadBalancerRunner) GenerateNginxLBConfig(servers []ServerConfig, listenPort int, lbMethod string) ([]byte, *runner.RunnerResult) {
	result := runner.NewRunnerResult()

	config := generateNginxLBConfigContent(servers, listenPort, lbMethod)
	result.MarkSuccess("nginx lb config generated")

	return []byte(config), result
}

// DeployNginxConfig deploys Nginx configuration
func (r *LoadBalancerRunner) DeployNginxConfig(
	ctx context.Context,
	conn connector.Connector,
	configContent []byte,
	configPath string,
) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if err := r.runner.WriteFile(ctx, conn, configContent, configPath, "0644", true); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write nginx config: %v", err))
		return result
	}

	_, _, err := conn.Exec(ctx, "nginx -t", nil)
	if err != nil {
		result.MarkFailed(err, "nginx config test failed")
		return result
	}

	result.MarkSuccess(fmt.Sprintf("nginx config deployed to %s", configPath))
	return result
}

// EnableNginx enables Nginx service
func (r *LoadBalancerRunner) EnableNginx(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	enableCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, "nginx")
	_, _, err := conn.Exec(ctx, enableCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to enable nginx: %v", err))
		return result
	}

	result.MarkSuccess("nginx enabled")
	return result
}

// StartNginx starts Nginx service
func (r *LoadBalancerRunner) StartNginx(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	startCmd := fmt.Sprintf(facts.InitSystem.StartCmd, "nginx")
	_, _, err := conn.Exec(ctx, startCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to start nginx: %v", err))
		return result
	}

	result.MarkSuccess("nginx started")
	return result
}

// RestartNginx restarts Nginx service
func (r *LoadBalancerRunner) RestartNginx(ctx context.Context, conn connector.Connector, facts *runner.Facts) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if facts.InitSystem == nil {
		result.MarkFailed(nil, "init system not detected")
		return result
	}

	restartCmd := fmt.Sprintf(facts.InitSystem.RestartCmd, "nginx")
	_, _, err := conn.Exec(ctx, restartCmd, nil)
	if err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to restart nginx: %v", err))
		return result
	}

	result.MarkSuccess("nginx restarted")
	return result
}

// DeployKubeVIPStaticPod deploys Kube-VIP static pod
func (r *LoadBalancerRunner) DeployKubeVIPStaticPod(
	ctx context.Context,
	conn connector.Connector,
	vip string,
	interfaceName string,
	controlPlaneHosts []string,
	manifestPath string,
) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	manifest := generateKubeVIPManifest(vip, interfaceName, controlPlaneHosts)

	if err := r.runner.WriteFile(ctx, conn, []byte(manifest), manifestPath, "0644", false); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write kube-vip manifest: %v", err))
		return result
	}

	result.MarkSuccess(fmt.Sprintf("kube-vip static pod deployed to %s", manifestPath))
	return result
}

// RemoveKubeVIPStaticPod removes Kube-VIP static pod
func (r *LoadBalancerRunner) RemoveKubeVIPStaticPod(
	ctx context.Context,
	conn connector.Connector,
	manifestPath string,
) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	if err := r.runner.Remove(ctx, conn, manifestPath, false, false); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to remove kube-vip manifest: %v", err))
		return result
	}

	result.MarkSuccess("kube-vip static pod removed")
	return result
}

// DeployStaticPodManifest deploys static pod manifest
func (r *LoadBalancerRunner) DeployStaticPodManifest(
	ctx context.Context,
	conn connector.Connector,
	manifestContent []byte,
	manifestName string,
	staticPodDir string,
) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	manifestPath := filepath.Join(staticPodDir, manifestName+".yaml")

	if err := r.runner.WriteFile(ctx, conn, manifestContent, manifestPath, "0644", false); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to write static pod manifest: %v", err))
		return result
	}

	result.MarkSuccess(fmt.Sprintf("static pod manifest deployed to %s", manifestPath))
	return result
}

// RemoveStaticPodManifest removes static pod manifest
func (r *LoadBalancerRunner) RemoveStaticPodManifest(
	ctx context.Context,
	conn connector.Connector,
	manifestName string,
	staticPodDir string,
) *runner.RunnerResult {
	result := runner.NewRunnerResult()

	manifestPath := filepath.Join(staticPodDir, manifestName+".yaml")

	if err := r.runner.Remove(ctx, conn, manifestPath, false, false); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to remove static pod manifest: %v", err))
		return result
	}

	result.MarkSuccess("static pod manifest removed")
	return result
}

func generateKeepalivedConfContent(interfaceName string, virtualRouterID int, priority int, vip string, peers []string, authPass string) string {
	peerStr := ""
	for _, peer := range peers {
		peerStr += fmt.Sprintf("    unicast_peer %s\n", peer)
	}

	return fmt.Sprintf(`! Keepalived configuration
global_defs {
    router_id LVS_%s
}

vrrp_instance VI_KUBERNETES {
    state BACKUP
    interface %s
    virtual_router_id %d
    priority %d
    advert_int 1
    authentication {
        auth_type PASS
        auth_pass %s
    }
    unicast_src_ip ${SRC_IP}
%s    virtual_ipaddress {
        %s dev %s label %s:vip
    }
}
`, interfaceName, interfaceName, virtualRouterID, priority, authPass, peerStr, vip, interfaceName, interfaceName)
}

func generateHAProxyConfigContent(servers []ServerConfig, frontendPort int, mode string) string {
	backendServers := ""
	for _, server := range servers {
		backendServers += fmt.Sprintf(`    server %s %s:%d check inter 2000 rise 2 fall 3
`, server.Name, server.Address, server.Port)
	}

	return fmt.Sprintf(`global
    log /dev/log local0
    log /dev/log local1 notice
    chroot /var/lib/haproxy
    stats socket /var/run/haproxy/admin.sock mode 660 level admin
    stats timeout 30s
    user haproxy
    group haproxy
    daemon

defaults
    log global
    mode %s
    option httplog
    option dontlognull
    timeout connect 5000
    timeout client 50000
    timeout server 50000

frontend kubernetes-api
    bind *:%d
    mode %s
    default_backend kubernetes-api-servers

backend kubernetes-api-servers
    mode %s
    balance roundrobin
%s`, mode, frontendPort, mode, mode, backendServers)
}

func generateNginxLBConfigContent(servers []ServerConfig, listenPort int, lbMethod string) string {
	upstreamServers := ""
	for _, server := range servers {
		upstreamServers += fmt.Sprintf(`    server %s:%d;
`, server.Address, server.Port)
	}

	return fmt.Sprintf(`stream {
    upstream kubernetes-api {
        %s
        %s
    }

    server {
        listen %d;
        proxy_pass kubernetes-api;
    }
}
`, lbMethod, upstreamServers, listenPort)
}

func generateKubeVIPManifest(vip string, interfaceName string, controlPlaneHosts []string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: kube-vip
  namespace: kube-system
spec:
  containers:
  - name: kube-vip
    image: docker.io/plndr/kube-vip:latest
    args:
    - manager
    - --arp
    - --leader-elect
    - --services
    - --controlplane
    - --interface=%s
    env:
    - name: vip_ip
      value: %s
    securityContext:
      capabilities:
        add:
        - NET_ADMIN
        - SYS_TIME
  hostNetwork: true
`, interfaceName, vip)
}
