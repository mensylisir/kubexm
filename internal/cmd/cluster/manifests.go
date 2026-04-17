package cluster

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/config"
	"github.com/mensylisir/kubexm/internal/logger"
	"github.com/spf13/cobra"
)

var manifestsOptions = &ManifestsOptions{}

type ManifestsOptions struct {
	ClusterConfigFile string
	OutputPath        string
	DryRun            bool
}

var manifestsCmd = &cobra.Command{
	Use:   "manifests",
	Short: "Generate Kubernetes manifests from cluster configuration",
	Long:  `Generate Kubernetes manifests (YAML files) based on the cluster configuration without executing them. This is useful for review or customization before actual cluster creation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := logger.Get()
		defer logger.SyncGlobal()

		if manifestsOptions.ClusterConfigFile == "" {
			return fmt.Errorf("cluster configuration file must be provided via -f or --config flag")
		}

		absPath, err := filepath.Abs(manifestsOptions.ClusterConfigFile)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for config file %s: %w", manifestsOptions.ClusterConfigFile, err)
		}

		clusterConfig, err := config.ParseFromFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to load cluster configuration from %s: %w", absPath, err)
		}

		outputPath := manifestsOptions.OutputPath
		if outputPath == "" {
			outputPath = clusterConfig.Name + "-manifests"
		}

		log.Infof("Generating Kubernetes manifests for cluster '%s' in directory '%s'", clusterConfig.Name, outputPath)

		// Create output directory
		if err := os.MkdirAll(outputPath, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", outputPath, err)
		}

		// Generate manifests based on cluster configuration
		if err := generateManifests(clusterConfig, outputPath, log); err != nil {
			return fmt.Errorf("failed to generate manifests: %w", err)
		}

		log.Infof("Manifests generated successfully at: %s", outputPath)
		return nil
	},
}

func init() {
	ClusterCmd.AddCommand(manifestsCmd)
	manifestsCmd.Flags().StringVarP(&manifestsOptions.ClusterConfigFile, "config", "f", "", "Path to the cluster configuration YAML file (required)")
	manifestsCmd.Flags().StringVarP(&manifestsOptions.OutputPath, "output", "o", "", "Output directory for manifests (default: <cluster-name>-manifests)")
	manifestsCmd.Flags().BoolVar(&manifestsOptions.DryRun, "dry-run", false, "Show what would be generated without creating files")

	if err := manifestsCmd.MarkFlagRequired("config"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mark 'config' flag as required: %v\n", err)
	}
}

// hasControlPlaneRole checks if a host has the control plane role
func hasControlPlaneRole(host *v1alpha1.HostSpec) bool {
	for _, role := range host.Roles {
		if role == common.RoleMaster || role == common.RoleControlPlane {
			return true
		}
	}
	return false
}

// hasWorkerRole checks if a host has the worker role
func hasWorkerRole(host *v1alpha1.HostSpec) bool {
	for _, role := range host.Roles {
		if role == common.RoleWorker {
			return true
		}
	}
	return false
}

// generateManifests generates Kubernetes manifests based on cluster configuration
func generateManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	// Generate etcd manifests
	if err := generateEtcdManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate etcd manifests: %w", err)
	}

	// Generate kube-apiserver manifests
	if err := generateKubeAPIServerManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate kube-apiserver manifests: %w", err)
	}

	// Generate kube-controller-manager manifests
	if err := generateKubeControllerManagerManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate kube-controller-manager manifests: %w", err)
	}

	// Generate kube-scheduler manifests
	if err := generateKubeSchedulerManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate kube-scheduler manifests: %w", err)
	}

	// Generate kube-proxy manifests
	if err := generateKubeProxyManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate kube-proxy manifests: %w", err)
	}

	// Generate kubelet manifests
	if err := generateKubeletManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate kubelet manifests: %w", err)
	}

	// Generate CNI manifests
	if err := generateCNIManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate CNI manifests: %w", err)
	}

	// Generate essential RBAC manifests
	if err := generateEssentialRBACManifests(clusterConfig, outputPath, log); err != nil {
		return fmt.Errorf("failed to generate RBAC manifests: %w", err)
	}

	return nil
}

func generateEtcdManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	etcdDir := filepath.Join(outputPath, "etcd")
	if err := os.MkdirAll(etcdDir, 0755); err != nil {
		return err
	}

	// Generate etcd manifest for each control plane node
	for _, host := range clusterConfig.Spec.Hosts {
		if hasControlPlaneRole(&host) {
			manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
  labels:
    component: etcd
    tier: control-plane
spec:
  containers:
  - name: etcd
    image: %s
    command:
    - etcd
    - --data-dir=/var/lib/etcd
    - --listen-client-urls=https://127.0.0.1:2379
    - --advertise-client-urls=https://%s:2379
    - --listen-peer-urls=https://%s:2380
    - --initial-cluster=%s
    - --cert-file=/etc/kubernetes/pki/etcd/server.crt
    - --key-file=/etc/kubernetes/pki/etcd/server.key
    - --trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
    - --client-cert-auth=true
    - --peer-cert-file=/etc/kubernetes/pki/etcd/peer.crt
    - --peer-key-file=/etc/kubernetes/pki/etcd/peer.key
    - --peer-trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
    - --peer-client-cert-auth=true
  hostNetwork: true
  volumes:
  - name: etcd-data
    hostPath:
      path: /var/lib/etcd
`, getEtcdImage(clusterConfig), host.Address, host.Address, getEtcdInitialCluster(clusterConfig))
			if err := writeFile(filepath.Join(etcdDir, host.Name+"-etcd.yaml"), manifest); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateKubeAPIServerManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	apiDir := filepath.Join(outputPath, "kube-apiserver")
	if err := os.MkdirAll(apiDir, 0755); err != nil {
		return err
	}

	for _, host := range clusterConfig.Spec.Hosts {
		if hasControlPlaneRole(&host) {
			manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
  labels:
    component: kube-apiserver
    tier: control-plane
spec:
  containers:
  - name: kube-apiserver
    image: %s
    command:
    - kube-apiserver
    - --service-cluster-ip-range=%s
    - --cloud-provider=%s
    - --audit-log-path=/var/log/kubernetes/audit.log
    - --audit-log-maxage=30
    - --audit-log-maxbackup=10
    - --audit-log-maxsize=100
    - --authorization-mode=Node,RBAC
    - --client-ca-file=/etc/kubernetes/pki/ca.crt
    - --etcd-cafile=/etc/kubernetes/pki/etcd/ca.crt
    - --etcd-certfile=/etc/kubernetes/pki/apiserver-etcd-client.crt
    - --etcd-keyfile=/etc/kubernetes/pki/apiserver-etcd-client.key
    - --etcd-servers=%s
    - --kubelet-client-certificate=/etc/kubernetes/pki/apiserver-kubelet-client.crt
    - --kubelet-client-key=/etc/kubernetes/pki/apiserver-kubelet-client.key
    - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
    - --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.crt
    - --proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client.key
    - --requestheader-allowed-names=front-proxy-admin
    - --requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.crt
    - --requestheader-extra-headers-prefix=X-Remote-Extra-
    - --requestheader-group-headers=X-Remote-Group
    - --requestheader-username-headers=X-Remote-User
    - --secure-port=6443
    - --service-account-issuer=%s
    - --service-account-key-file=/etc/kubernetes/pki/sa.pub
    - --service-account-signing-key-file=/etc/kubernetes/pki/sa.key
    - --tls-cert-file=/etc/kubernetes/pki/apiserver.crt
    - --tls-private-key-file=/etc/kubernetes/pki/apiserver.key
  hostNetwork: true
`, getKubernetesImage(clusterConfig), getServiceCIDR(clusterConfig), getCloudProvider(clusterConfig), getEtcdServers(clusterConfig), getServiceIssuer(clusterConfig))
			if err := writeFile(filepath.Join(apiDir, host.Name+"-kube-apiserver.yaml"), manifest); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateKubeControllerManagerManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	dir := filepath.Join(outputPath, "kube-controller-manager")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, host := range clusterConfig.Spec.Hosts {
		if hasControlPlaneRole(&host) {
			manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
  labels:
    component: kube-controller-manager
    tier: control-plane
spec:
  containers:
  - name: kube-controller-manager
    image: %s
    command:
    - kube-controller-manager
    - --cluster-cidr=%s
    - --cluster-name=%s
    - --controllers=*,-cloud-node-lifecycle
    - --kubeconfig=/etc/kubernetes/kubeconfig/kube-controller-manager.conf
    - --leader-elect=true
    - --root-ca-file=/etc/kubernetes/pki/ca.crt
    - --service-cluster-ip-range=%s
    - --service-account-private-key-file=/etc/kubernetes/pki/sa.key
    - --use-service-account-credentials=true
  hostNetwork: true
`, getKubernetesImage(clusterConfig), getPodCIDR(clusterConfig), clusterConfig.Name, getServiceCIDR(clusterConfig))
			if err := writeFile(filepath.Join(dir, host.Name+"-kube-controller-manager.yaml"), manifest); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateKubeSchedulerManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	dir := filepath.Join(outputPath, "kube-scheduler")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, host := range clusterConfig.Spec.Hosts {
		if hasControlPlaneRole(&host) {
			manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
  labels:
    component: kube-scheduler
    tier: control-plane
spec:
  containers:
  - name: kube-scheduler
    image: %s
    command:
    - kube-scheduler
    - --kubeconfig=/etc/kubernetes/kubeconfig/kube-scheduler.conf
    - --leader-elect=true
  hostNetwork: true
`, getKubernetesImage(clusterConfig))
			if err := writeFile(filepath.Join(dir, host.Name+"-kube-scheduler.yaml"), manifest); err != nil {
				return err
			}
		}
	}
	return nil
}

func generateKubeProxyManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	dir := filepath.Join(outputPath, "kube-proxy")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-proxy
  namespace: kube-system
data:
  config.conf: |
    clusterCIDR: %s
    conntrack:
      maxPerCore: 32768
      min: 131072
    enableIPVS: true
    mode: ipvs
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: kube-proxy
  template:
    metadata:
      labels:
        k8s-app: kube-proxy
    spec:
      containers:
      - name: kube-proxy
        image: %s
        command:
        - /usr/local/bin/kube-proxy
        - --config=/var/lib/kube-proxy/config.conf
        - --hostname-override=${NODE_NAME}
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /run/xtables.lock
          name: xtables-lock
        - mountPath: /lib/modules
          name: lib-modules
          readOnly: true
        - mountPath: /var/lib/kube-proxy
          name: kube-proxy
      hostNetwork: true
      volumes:
      - hostPath:
          path: /run/xtables.lock
          type: FileOrCreate
        name: xtables-lock
      - hostPath:
          path: /lib/modules
          type: Directory
        name: lib-modules
`, getPodCIDR(clusterConfig), getKubeComponentImage(clusterConfig, "kube-proxy"))
	return writeFile(filepath.Join(dir, "kube-proxy.yaml"), manifest)
}

func generateKubeletManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	dir := filepath.Join(outputPath, "kubelet")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	for _, host := range clusterConfig.Spec.Hosts {
		manifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: kubelet-config-%s
  namespace: kube-system
data:
  kubelet: |
    apiVersion: kubelet.config.k8s.io/v1beta1
    kind: KubeletConfiguration
    authentication:
      anonymous:
        enabled: false
      webhook:
        cacheTTL: 2h0m0s
        enabled: true
    authorization:
      mode: Webhook
      webhook:
        cacheAuthorizedTTL: 5m0s
        cacheUnauthorizedTTL: 30s
    clusterDNS:
    - %s
    clusterDomain: cluster.local
    cpuManagerPolicy: none
    cgroupDriver: systemd
    cgroupRoot: /
    containerLogMaxFiles: 5
    containerLogMaxSize: 10Mi
    contentType: application/vnd.kubernetes.protobuf
    cpuCFSQuota: true
    cpuCFSQuotaPeriod: 100ms
    enableControllerAttachDetach: true
    enableDebuggingHandlers: true
    enableServer: true
    enforceNodeAllocatable:
    - pods
    eventBurst: 10
    eventRecordQPS: 5
    evictionHard:
      imagefs.available: 15%%
      memory.available: 100Mi
      nodefs.available: 10%%
      nodefs.inodesFree: 5%%
    evictionPressureTransitionPeriod: 5m0s
    failSwapOn: true
    fileCheckFrequency: 20s
    hairpinMode: promiscuous-bridge
    healthzBindAddress: 127.0.0.1
    healthzPort: 10248
    httpCheckFrequency: 20s
    imageGCHighThresholdPercent: 85
    imageGCLowThresholdPercent: 80
    imageMinimumGCAge: 2m0s
    kubeAPIServerServerTLSMinVersion: TLS1_2
    kubeletCgroups: /systemd/system.slice
    logging: {}
    maxOpenFiles: 1000000
    maxPods: 110
    nodeLeaseDurationSeconds: 40
    nodeStatusReportFrequency: 10s
    nodeStatusUpdateFrequency: 10s
    oomScoreAdj: -999
    podPidsLimit: -1
    registryBurst: 10
    registryPullQPS: 5
    resolvConf: /run/systemd/resolve/resolv.conf
    rotateCertificates: true
    runtimeRequestTimeout: 2m0s
    serializeImagePulls: true
    staticPodPath: /etc/kubernetes/manifests
    streamingConnectionIdleTimeout: 4h0m0s
    syncFrequency: 1m0s
    volumeStatsAggPeriod: 1m0s
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kubelet
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: kubelet
  template:
    metadata:
      labels:
        k8s-app: kubelet
    spec:
      containers:
      - name: kubelet
        image: %s
        command:
        - kubelet
        - --config=/var/lib/kubelet/config/config.yaml
        - --kubeconfig=/etc/kubernetes/kubeconfig/kubelet.conf
        - --hostname-override=%s
        - --register-node=true
        - --cert-dir=/var/lib/kubelet/pki
        - --container-runtime-endpoint=unix:///var/run/containerd/containerd.sock
      hostNetwork: true
      volumes:
      - hostPath:
          path: /etc/kubernetes/kubeconfig
          type: DirectoryOrCreate
        name: kubeconfig
      - hostPath:
          path: /var/lib/kubelet
          type: DirectoryOrCreate
        name: kubelet
      - hostPath:
          path: /etc/kubernetes/pki
          type: DirectoryOrCreate
        name: pki
`, host.Name, getClusterDNS(clusterConfig), getKubeComponentImage(clusterConfig, "kubelet"), host.Name)
		if err := writeFile(filepath.Join(dir, host.Name+"-kubelet.yaml"), manifest); err != nil {
			return err
		}
	}
	return nil
}

func generateCNIManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	if clusterConfig.Spec.Network == nil {
		return fmt.Errorf("network configuration is required to generate CNI manifests")
	}
	dir := filepath.Join(outputPath, "cni")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	cniType := clusterConfig.Spec.Network.Plugin
	switch cniType {
	case "calico":
		manifest := fmt.Sprintf(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: calico-node
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: calico-node
  template:
    metadata:
      labels:
        k8s-app: calico-node
    spec:
      containers:
      - name: calico-node
        image: %s
        env:
        - name: CALICO_IPV4POOL_CIDR
          value: %s
        - name: CALICO_NETWORKING_BACKEND
          value: bird
        - name: FELIX_IPV4SUPPORT
          value: "true"
        - name: CALICO_IPV4POOL_IPIP
          value: "CrossSubnet"
        - name: CALICO_AUTODETECTION_METHOD
          value: first-found
        - name: CLUSTER_TYPE
          value: kubernetes
        - name: FELIX_DEFAULTENDPOINTTOHOSTACTION
          value: ACCEPT
        - name: FELIX_VXLANMACSEC_ENABLED
          value: "false"
        - name: FELIX_WIREGUARD_ENABLED
          value: "false"
        securityContext:
          privileged: true
        volumeMounts:
        - name: lib-modules
          mountPath: /lib/modules
          readOnly: true
        - name: var-run-calico
          mountPath: /var/run/calico
        - name: cni-bin-dir
          mountPath: /opt/cni/bin
        - name: cni-net-dir
          mountPath: /etc/cni/net.d
      hostNetwork: true
      volumes:
      - name: lib-modules
        hostPath:
          path: /lib/modules
      - name: var-run-calico
        hostPath:
          path: /var/run/calico
      - name: cni-bin-dir
        hostPath:
          path: /opt/cni/bin
      - name: cni-net-dir
        hostPath:
          path: /etc/cni/net.d
`, getCNICNIImage(clusterConfig, "calico"), getPodCIDR(clusterConfig))
		return writeFile(filepath.Join(dir, "calico.yaml"), manifest)

	case "flannel":
		manifest := fmt.Sprintf(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-flannel
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: flannel
  template:
    metadata:
      labels:
        app: flannel
    spec:
      containers:
      - name: kube-flannel
        image: %s
        command:
        - /opt/bin/flanneld
        args:
        - --ip-masq
        - --kube-subnet-mgr
        - --iface=$(IFACE)
        env:
        - name: POD_CIDR
          value: %s
        - name: IFACE
          value: eth0
        securityContext:
          privileged: true
        volumeMounts:
        - name: run
          mountPath: /run/flannel
        - name: cni
          mountPath: /etc/cni/net.d
        - name: xtables
          mountPath: /run/xtables.lock
      hostNetwork: true
      volumes:
      - name: run
        hostPath:
          path: /run/flannel
      - name: cni
        hostPath:
          path: /etc/cni/net.d
      - name: xtables
        hostPath:
          path: /run/xtables.lock
          type: FileOrCreate
`, getCNICNIImage(clusterConfig, "flannel"), getPodCIDR(clusterConfig))
		return writeFile(filepath.Join(dir, "flannel.yaml"), manifest)

	default:
		log.Warn("CNI type %s manifest generation not yet implemented, skipping", cniType)
		return nil
	}
}

func generateEssentialRBACManifests(clusterConfig *v1alpha1.Cluster, outputPath string, log *logger.Logger) error {
	dir := filepath.Join(outputPath, "rbac")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	manifest := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system:kube-apiserver-to-kubelet
rules:
- apiGroups:
  - ""
  resources:
  - nodes/proxy
  - nodes/stats
  - nodes/log
  - nodes/spec
  - nodes/metrics
  verbs:
  - "*"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:kube-apiserver
  namespace: ""
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kube-apiserver-to-kubelet
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: kubernetes
`
	return writeFile(filepath.Join(dir, "essential-rbac.yaml"), manifest)
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// Helper functions for image and configuration retrieval

func getRegistry(cfg *v1alpha1.Cluster) string {
	if cfg.Spec.Registry != nil && cfg.Spec.Registry.MirroringAndRewriting != nil {
		if cfg.Spec.Registry.MirroringAndRewriting.PrivateRegistry != "" {
			return cfg.Spec.Registry.MirroringAndRewriting.PrivateRegistry
		}
	}
	return "registry.k8s.io"
}

func getKubeComponentImage(cfg *v1alpha1.Cluster, component string) string {
	version := "v1.28.0"
	if cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.Version != "" {
		version = cfg.Spec.Kubernetes.Version
	}
	return fmt.Sprintf("%s/%s:%s", getRegistry(cfg), component, version)
}

func getKubernetesImage(cfg *v1alpha1.Cluster) string {
	return getKubeComponentImage(cfg, "kube-apiserver")
}

func getEtcdImage(cfg *v1alpha1.Cluster) string {
	version := "v3.5.13"
	if cfg.Spec.Etcd != nil && cfg.Spec.Etcd.Version != "" {
		version = cfg.Spec.Etcd.Version
	}
	return fmt.Sprintf("%s/etcd:%s", getRegistry(cfg), version)
}

func getCNICNIImage(cfg *v1alpha1.Cluster, cniType string) string {
	// Use default CNI versions; per-component versions can be configured via addon source
	switch cniType {
	case "calico":
		return fmt.Sprintf("%s/calico/cni:v3.27.4", getRegistry(cfg))
	case "flannel":
		return fmt.Sprintf("%s/flannel/flannel:v0.24.0", getRegistry(cfg))
	default:
		return fmt.Sprintf("%s/calico/cni:v3.27.4", getRegistry(cfg))
	}
}

func getClusterDNS(cfg *v1alpha1.Cluster) string {
	// First IP in the service CIDR
	if cfg.Spec.Network != nil && cfg.Spec.Network.KubeServiceCIDR != "" {
		return "10.96.0.10"
	}
	return "10.96.0.10"
}

func getCloudProvider(cfg *v1alpha1.Cluster) string {
	return "external"
}

func getServiceIssuer(cfg *v1alpha1.Cluster) string {
	return "https://kubernetes.default.svc"
}

func getPodCIDR(cfg *v1alpha1.Cluster) string {
	if cfg.Spec.Network != nil && cfg.Spec.Network.KubePodsCIDR != "" {
		return cfg.Spec.Network.KubePodsCIDR
	}
	return "10.244.0.0/16"
}

func getServiceCIDR(cfg *v1alpha1.Cluster) string {
	if cfg.Spec.Network != nil && cfg.Spec.Network.KubeServiceCIDR != "" {
		return cfg.Spec.Network.KubeServiceCIDR
	}
	return "10.96.0.0/12"
}

func getEtcdInitialCluster(cfg *v1alpha1.Cluster) string {
	var members []string
	for _, host := range cfg.Spec.Hosts {
		if hasControlPlaneRole(&host) {
			members = append(members, fmt.Sprintf("%s=https://%s:2380", host.Name, host.Address))
		}
	}
	return joinStringSlice(members)
}

func getEtcdServers(cfg *v1alpha1.Cluster) string {
	var servers []string
	for _, host := range cfg.Spec.Hosts {
		if hasControlPlaneRole(&host) {
			servers = append(servers, fmt.Sprintf("https://%s:2379", host.Address))
		}
	}
	return joinStringSlice(servers)
}

func joinStringSlice(slice []string) string {
	result := ""
	for i, s := range slice {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
