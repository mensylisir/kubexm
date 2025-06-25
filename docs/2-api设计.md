### pkg/apis/kubexms/v1alpha1已存在，已经设计好了

# Kubexm API 设计 (Kubernetes CRD)

本文档描述了 Kubexm 项目的 API 设计。在此上下文中，“API 设计”指的是项目所使用的 **Kubernetes 自定义资源定义 (Custom Resource Definitions - CRDs)**。这些 CRDs 允许用户通过标准的 Kubernetes API（例如，使用 `kubectl` 和 YAML 文件）以声明式的方式定义和管理 Kubexm 所控制的资源。

## 1. 概述

Kubexm 使用 Kubernetes CRDs 来对集群及其组件的期望状态进行建模。用户可以创建这些自定义资源的实例，Kubexm 系统（通常通过一个控制器或操作员模式的组件，或者由命令行工具在特定流程中触发）会监测这些资源的变化，并采取相应行动以使实际状态与期望状态一致。

## 2. API Group 和 Version

Kubexm 定义的自定义资源属于以下 API Group 和 Version：

*   **Group**: `kubexms.io`
*   **Version**: `v1alpha1`

因此，在 YAML 文件中引用这些自定义资源时，`apiVersion` 字段通常会是 `kubexms.io/v1alpha1`。

## 3. 主要自定义资源: Cluster (Kind: `Cluster`)

*   **ShortName**: `km`
*   **Scope**: `Namespaced`
*   **描述**: `Cluster` 是最顶层的自定义资源，代表一个完整的 Kubexm 管理的集群。它封装了集群的全部配置。
*   **核心结构**:
    *   `metadata`: 标准的 Kubernetes `ObjectMeta`。
    *   `spec`: `ClusterSpec` 定义了集群的期望状态。

### 3.1. ClusterSpec 字段详解

以下是 `ClusterSpec` 中主要配置块及其关键字段的概述：

#### 3.1.1. `type` (string)
集群部署类型，例如 `"kubexm"` (二进制部署核心组件) 或 `"kubeadm"` (kubeadm 静态 Pod 部署)。

#### 3.1.2. `hosts` ([]HostSpec)
定义集群中的所有主机。
*   `HostSpec`:
    *   `name` (string): 主机名。
    *   `address` (string): 主机 IP 地址或可解析域名。
    *   `internalAddress` (string, optional): 内部通信地址。
    *   `port` (int, optional): SSH 端口。
    *   `user` (string, optional): SSH 用户。
    *   `password` (string, optional): SSH 密码。
    *   `privateKeyPath` (string, optional): SSH 私钥路径。
    *   `roles` ([]string, optional): 主机扮演的角色 (例如, "master", "worker", "etcd")。
    *   `labels` (map[string]string, optional): Kubernetes 节点标签。
    *   `taints` ([]TaintSpec, optional): Kubernetes 节点污点。

#### 3.1.3. `roleGroups` (*RoleGroupsSpec, optional)
定义不同角色的节点组。
*   `RoleGroupsSpec`:
    *   `master`, `worker`, `etcd`, `loadbalancer`, `storage`, `registry`: 各自定义角色组内包含的 `hosts` (主机名列表)。
    *   `customRoles` ([]CustomRoleSpec, optional): 自定义角色组。

#### 3.1.4. `global` (*GlobalSpec, optional)
全局默认配置。
*   `GlobalSpec`:
    *   `user`, `port`, `password`, `privateKeyPath`: SSH 连接的默认值。
    *   `connectionTimeout` (time.Duration): 连接超时。
    *   `workDir` (string): 本地主机上的工作目录
    *   `hostWorkDir` 远程主机上的工作目录。
    *   `verbose` (bool): 详细输出模式。
    *   `ignoreErr` (bool): 忽略错误继续执行。
    *   `skipPreflight` (bool): 跳过预检检查。

#### 3.1.5. `system` (*SystemSpec, optional)
节点操作系统级别的配置。
*   `SystemSpec`:
    *   `ntpServers` ([]string, optional): NTP 服务器列表。
    *   `timezone` (string, optional): 时区设置。
    *   `rpms`, `debs` ([]string, optional): 预安装的 RPM/DEB 包。
    *   `packageManager` (string, optional): 指定包管理器。
    *   `preInstallScripts`, `postInstallScripts` ([]string, optional): 安装前后执行的脚本。
    *   `skipConfigureOS` (bool): 是否跳过OS配置步骤。
    *   `modules` ([]string, optional): 要加载的内核模块。
    *   `sysctlParams` (map[string]string, optional): 内核 `sysctl` 参数。

#### 3.1.6. `kubernetes` (*KubernetesConfig, optional)
Kubernetes 核心组件配置。
*   `KubernetesConfig`:
    *   `type` (string): Kubernetes 部署类型 (同 `ClusterSpec.Type`)。
    *   `version` (string): Kubernetes 版本。
    *   `clusterName` (string, optional): 集群名称。
    *   `dnsDomain` (string, optional): 集群 DNS 域名 (默认 "cluster.local")。
    *   `disableKubeProxy` (*bool, optional): 是否禁用 KubeProxy。
    *   `proxyMode` (string, optional): KubeProxy 模式 (如 "ipvs", "iptables")。
    *   `apiserverCertExtraSans` ([]string, optional): APIServer 证书的额外 SANs。
    *   `featureGates` (map[string]bool, optional): Kubernetes 特性门控。
    *   `apiServer` (*APIServerConfig, optional):
        *   `extraArgs` ([]string): APIServer 额外参数。
        *   `admissionPlugins` ([]string): 启用的准入控制器插件。
        *   `serviceNodePortRange` (string): NodePort 范围。
    *   `controllerManager` (*ControllerManagerConfig, optional): `extraArgs` 等。
    *   `scheduler` (*SchedulerConfig, optional): `extraArgs` 等。
    *   `kubelet` (*KubeletConfig, optional):
        *   `extraArgs` ([]string): Kubelet 额外参数。
        *   `cgroupDriver` (*string): CGroup驱动 (如 "systemd", "cgroupfs")。
        *   `evictionHard` (map[string]string): 硬驱逐阈值。
        *   `podPidsLimit` (*int64): Pod 的 PID 限制。
    *   `kubeProxy` (*KubeProxyConfig, optional): `extraArgs`, 以及针对 IPTables/IPVS 模式的特定配置。
    *   `nodelocaldns` (*NodelocaldnsConfig, optional): NodeLocal DNSCache 配置。

#### 3.1.7. `etcd` (*EtcdConfig, optional)
Etcd 集群配置。
*   `EtcdConfig`:
    *   `type` (string): Etcd 类型 ("kubexm", "external", "kubeadm")。
    *   `version` (string, optional): Etcd 版本 (用于 Kubexm 管理的部署)。
    *   `external` (*ExternalEtcdConfig, optional): 外部 Etcd 集群连接信息 (`endpoints`, `caFile`, `certFile`, `keyFile`)。
    *   `dataDir` (*string, optional): 数据目录。
    *   `extraArgs` ([]string, optional): Etcd 额外参数。
    *   备份配置: `backupDir`, `backupPeriodHours`, `keepBackupNumber`。
    *   性能调优: `heartbeatIntervalMillis`, `electionTimeoutMillis`, `snapshotCount`。

#### 3.1.8. `containerRuntime` (*ContainerRuntimeConfig, optional)
容器运行时配置。
*   `ContainerRuntimeConfig`:
    *   `type` (string): 运行时类型 ("containerd", "docker")。
    *   `version` (string, optional): 运行时版本。
    *   `docker` (*DockerConfig, optional): Docker 特定配置。
        *   `registryMirrors`, `insecureRegistries` ([]string)。
        *   `dataRoot` (*string): Docker 根目录。
        *   `execOpts` ([]string): 如 `native.cgroupdriver=systemd`。
        *   `installCRIDockerd` (*bool): 是否安装 cri-dockerd。
    *   `containerd` (*ContainerdConfig, optional): Containerd 特定配置。
        *   `registryMirrors` (map[string][]string)。
        *   `insecureRegistries` ([]string)。
        *   `useSystemdCgroup` (*bool)。
        *   `extraTomlConfig` (string): 额外的 `config.toml` 内容。

#### 3.1.9. `network` (*NetworkConfig, optional)
集群网络配置。
*   `NetworkConfig`:
    *   `plugin` (string, optional): CNI 插件类型 (如 "calico", "flannel")。
    *   `kubePodsCIDR` (string, optional): Pod 的 CIDR。
    *   `kubeServiceCIDR` (string, optional): Service 的 CIDR。
    *   `calico` (*CalicoConfig, optional): Calico 特定配置。
        *   `ipipMode`, `vxlanMode` (string)。
        *   `vethMTU` (*int)。
        *   `ipPools` ([]CalicoIPPool): 自定义 IP 池。
    *    `Cilium *CiliumConfig` `json:"cilium,omitempty"`
    *   `flannel` (*FlannelConfig, optional): Flannel 特定配置 (`backendMode`)。
    *   `multus` (*MultusCNIConfig, optional): Multus CNI 配置 (`enabled`)。

#### 3.1.10. `controlPlaneEndpoint` (*ControlPlaneEndpointSpec, optional)
控制平面接入点。
*   `ControlPlaneEndpointSpec`:
    *   `domain` (string, optional): 控制平面域名。
    *   `address` (string, optional): 控制平面 IP 地址 (如 VIP)。
    *   `port` (int, optional): 控制平面端口 (默认 6443)。
    *   `externalLoadBalancerType` (string, optional): 外部负载均衡器类型 ("kubexm", "external")。
    *   `internalLoadBalancerType` (string, optional): 内部负载均衡器类型 ("haproxy", "nginx", "kube-vip")。

#### 3.1.11. `highAvailability` (*HighAvailabilityConfig, optional)
高可用性配置。
*   `HighAvailabilityConfig`:
    *   `enabled` (*bool, optional): 是否启用 HA。
    *   `external` (*ExternalLoadBalancerConfig, optional): 外部负载均衡配置。
        *   `type` (string): 如 "UserProvided", "ManagedKeepalivedHAProxy"。
        *   `keepalived` (*KeepalivedConfig): Keepalived 配置 (`vrid`, `priority`, `interface`)。
        *   `haproxy` (*HAProxyConfig): HAProxy 配置 (`frontendBindAddress`, `frontendPort`, `backendServers`)。
        *   `nginxLB` (*NginxLBConfig): Nginx LB 配置。
    *   `internal` (*InternalLoadBalancerConfig, optional): 内部负载均衡配置。
        *   `type` (string): 如 "KubeVIP"。
        *   `kubevip` (*KubeVIPConfig): KubeVIP 配置 (`mode`, `vip`, `interface`)。

#### 3.1.12. `storage` (*StorageConfig, optional)
存储配置。
*   `StorageConfig`:
    *   `defaultStorageClass` (*string, optional): 默认的 StorageClass 名称。
    *   `openebs` (*OpenEBSConfig, optional): OpenEBS 配置。
        *   `enabled` (*bool)。
        *   `basePath` (string): OpenEBS LocalPV 存储路径。
        *   `engines`: 可配置不同的 OpenEBS 存储引擎如 `localHostPath`。

#### 3.1.13. `registry` (*RegistryConfig, optional)
镜像仓库配置。
*   `RegistryConfig`:
    *   `privateRegistry` (string, optional): 私有镜像仓库地址，用于覆盖默认镜像前缀。
    *   `namespaceOverride` (string, optional): 统一的命名空间覆盖。
    *   `auths` (map[string]RegistryAuth, optional): 各个仓库的认证信息。
        *   `RegistryAuth`: `username`, `password`, `auth` (base64), `skipTLSVerify`, `plainHTTP`。
    *   `type` (*string, optional): 本地部署的仓库类型 (如 "registry", "harbor")。
    *   `dataRoot` (*string, optional): 本地部署仓库的数据目录。

#### 3.1.14. `addons` ([]string, optional)
要安装的集群插件列表 (字符串形式的插件名称)。
*   注意: `addon_types.go` 中定义了更复杂的 `AddonConfig` 结构，包含 `Sources` (Chart/YAML)，但 `ClusterSpec.Addons` 当前为 `[]string`。这可能意味着字符串名称会映射到别处定义的详细配置，或者 `AddonConfig` 用于更细致的内部处理。

#### 3.1.15. `preflight` (*PreflightConfig, optional)
预检检查配置。
*   `PreflightConfig`:
    *   `minCPUCores` (*int32, optional): 最小 CPU 核心数。
    *   `minMemoryMB` (*uint64, optional): 最小内存 (MB)。
    *   `disableSwap` (*bool, optional): 是否禁用 Swap (默认 true)。

## 4. 定义位置

所有这些自定义资源的 Go 语言类型定义（即 API Schema）均位于项目的以下目录中：

`pkg/apis/kubexms/v1alpha1/`

该目录下的 `*_types.go` 文件包含了具体的结构体定义。`register.go` 文件负责将这些类型注册到 Kubernetes 的 scheme 中。

## 5. 使用方式

用户通常会创建一个 `Cluster` 类型的 YAML 文件，然后使用 `kubectl apply -f <filename>.yaml` 命令将其提交到 Kubernetes 集群。Kubexm 的控制逻辑会响应该资源的创建或变更，并执行相应的部署、配置或管理操作。
用户也会使用`kubexm create cluster -f <filename>.yaml`命令来创建集群,这样会走命令行参数。

---

本文档旨在提供 Kubexm CRD API 的高级概述。有关每个字段的详细信息和确切结构，请直接参考 `pkg/apis/kubexms/v1alpha1/` 目录下的源代码。
我已经将这份更详尽的拟定内容通过此消息发送给您。

