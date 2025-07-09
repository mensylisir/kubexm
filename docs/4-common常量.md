### pkg/common已存在，存放常量

# Kubexm 公共常量 (`pkg/common`)

本文档描述了 Kubexm 项目中 `pkg/common` 包内定义的全局常量。这些常量主要用于规范默认的目录结构、特殊标识符以及在整个项目代码中可能出现的“魔法字符串”，以提高可维护性和一致性。

## 1. 设计目标

`pkg/common` 包的目标是：

*   集中管理项目中使用的通用常量。
*   避免在代码中硬编码字符串和数值，方便后续修改和维护。
*   为核心组件和操作提供标准的命名和路径约定。

## 2. 主要常量定义

以下是 `pkg/common/constants.go` 中定义的主要常量及其用途：

### 2.1. 默认目录结构常量

这些常量定义了 Kubexm 在执行操作时使用的标准目录名称：

*   **`KUBEXM`**: `".kubexm"`
    *   Kubexm 操作的默认根目录名称。通常在用户主目录或指定的工作目录下创建。
*   **`DefaultLogsDir`**: `"logs"`
    *   在 `KUBEXM` 目录下，用于存放日志文件的默认目录名称。
*   **`DefaultCertsDir`**: `"certs"`
    *   用于存放证书文件的默认目录名称（例如，在集群的产物目录下）。
*   **`DefaultContainerRuntimeDir`**: `"container_runtime"`
    *   用于存放容器运行时相关产物（如二进制文件、配置文件）的默认目录名称。
*   **`DefaultKubernetesDir`**: `"kubernetes"`
    *   用于存放 Kubernetes 组件相关产物（如二进制文件、配置文件）的默认目录名称。
*   **`DefaultEtcdDir`**: `"etcd"`
    *   用于存放 Etcd 相关产物（如二进制文件、配置文件、数据备份）的默认目录名称。

### 2.2. 控制节点标识符

这些常量用于标识在控制机器本地执行的操作或节点：

*   **`ControlNodeHostName`**: `"kubexm-control-node"`
    *   一个特殊的、逻辑上的主机名，代表执行 Kubexm 命令的控制机器本身。当操作需要在本地进行而非远程主机时使用。
*   **`ControlNodeRole`**: `"control-node"`
    *   分配给 `ControlNodeHostName` 的特殊角色名。

## 3. 使用方式

项目中的其他包（如 `pkg/runtime`, `pkg/resource`, `pkg/step` 等）在需要引用这些标准路径或标识符时，应导入 `pkg/common` 包并使用这些已定义的常量。

例如，`runtime.Context` 在构建各种产物下载路径或日志文件路径时，会依赖这些常量来确保一致性。

---

本文档反映了当前 `pkg/common/constants.go` 的主要内容。未来如果添加新的通用常量，
分组存放吧，不要全放到constants中



### 优点 (Strengths)

1. **集中管理 (Centralized Management)**: 这是 pkg/common 包最核心的价值。将所有“魔法字符串”和配置约定集中在一个地方，极大地提高了代码的可读性和可维护性。当需要修改一个默认目录名时（例如，从 certs 改为 pki），只需要修改一处，所有引用该常量的地方都会自动更新，避免了遗漏和不一致。
2. **职责单一 (Single Responsibility Principle)**: 这个包的职责非常纯粹，就是定义常量。它不包含任何逻辑、函数或复杂的结构体，这使得它非常轻量，可以被项目中的任何其他包安全地导入，而不用担心引入不必要的依赖或循环依赖。
3. **命名清晰 (Clear Naming)**: 常量的命名（如 DefaultLogsDir, ControlNodeHostName）直观地反映了它们的用途，降低了新开发人员理解代码的门槛。
4. **标准化约定 (Standardized Conventions)**: 这些常量实际上定义了 kubexm 工具在文件系统上操作的“契约”。例如，DefaultCertsDir 告诉所有相关的Step或Task应该去哪里寻找或存放证书。这种标准化的约定对于一个自动化工具的稳定性和可预测性至关重要。
5. **逻辑主机名的引入 (Logical Hostname for Control Node)**: ControlNodeHostName 和 ControlNodeRole 的设计是一个非常巧妙且实用的抽象。在自动化流程中，经常需要在“执行控制的机器”和“被管理的远程机器”之间进行区分。通过引入一个特殊的逻辑名称，可以让执行引擎（Engine）和连接器（Connector）用统一的方式来处理“本地执行”和“远程执行”的逻辑，而不需要在业务代码中写大量的 if-else 判断。这是一个很好的设计模式。

### 与整体架构的契合度

这个 pkg/common 包完美地融入了“世界树”架构的**第二层：基础服务 (The Foundational Services Layer)**。

- 它作为最基础的服务之一，被几乎所有其他层级所依赖：
    - **第一层 (世界观与配置)**: pkg/apis 中定义的结构体，其字段的默认值可以引用这些常量。
    - **第二层 (基础服务)**:
        - pkg/resource: 在构建资源下载路径时会用到 Default...Dir。
        - pkg/logger: 可能会使用 DefaultLogsDir 作为默认日志输出目录。
    - **第三层 (执行与决策)**:
        - pkg/step: 几乎所有与文件操作相关的Step（如 CreateDirStep, DownloadFileStep）都会使用这些目录常量来确定目标路径。
    - **第四层 (运行时与引擎)**:
        - pkg/runtime: 在初始化和构建各种上下文时，会大量使用这些常量来构建工作目录结构。
        - pkg/engine: 在调度时，可能会根据 ControlNodeHostName 来决定一个Step是本地执行还是远程执行。

### 潜在的细微改进建议 (Minor Improvement Suggestions)

这个设计已经非常好了，以下只是一些可以考虑的、锦上添花的细微之处：

1. **按功能分组**: 当前所有常量都在一个constants.go文件中。随着项目发展，常量可能会增多。可以考虑按功能将它们分到不同的文件中，以提高可读性。例如：
    - path_constants.go: 存放所有与目录、文件路径相关的常量。
    - role_constants.go: 存放与主机角色（master, worker, etcd, control-node）相关的常量。
    - resource_constants.go: 存放与资源类型（binary, image）相关的常量。
2. **增加注释**: 虽然命名已经很清晰，但在每个常量或常量组上方添加简短的注释，解释其用途和上下文，总是有益的。
3. **考虑环境变量覆盖**: 在某些场景下，用户可能希望通过环境变量来覆盖这些默认路径。虽然这不属于pkg/common的职责，但在使用这些常量的地方（例如 pkg/runtime 的初始化逻辑中），可以设计成“优先使用环境变量，如果未设置，则使用pkg/common中的默认常量”。

### 总结

这是一个**典范级的基础模块设计**。它简单、清晰、有效，是构建一个健壮、可维护的大型系统的基石。它完美地履行了自己作为“常量注册表”的职责，并为整个“世界树”架构提供了一致性和标准化的基础。无需做大的改动，这个模块已经可以很好地支撑项目的发展。




### **扩展的 pkg/common 常量建议**

#### **1. 角色与标签常量 (Roles & Labels Constants)**

这部分是对现有 ControlNodeRole 的扩展，定义了所有标准和可能的自定义角色。

Generated go

```
// --- Host Roles ---
const (
    RoleMaster         = "master"
    RoleWorker         = "worker"
    RoleEtcd           = "etcd"
    RoleLoadBalancer   = "loadbalancer"
    RoleStorage        = "storage"
    RoleRegistry       = "registry"
    // RoleControlNode is already defined
)

// --- Kubernetes Node Labels & Taints ---
const (
    // 标准的Master节点标签和污点
    LabelNodeRoleMaster = "node-role.kubernetes.io/master"
    TaintKeyNodeRoleMaster = "node-role.kubernetes.io/master"

    // 标准的控制平面节点标签和污点
    LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
    TaintKeyNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"

    // 自定义标签，用于标识节点由kubexm管理
    LabelManagedBy = "app.kubernetes.io/managed-by"
    LabelValueKubexm = "kubexm"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **2. 文件与目录路径常量 (File & Directory Path Constants)**

这是对现有目录常量的极大扩充，涵盖了Kubernetes生态系统的标准路径。

Generated go

```
// --- Kubernetes System Directories ---
const (
    KubeletHomeDir          = "/var/lib/kubelet"
    KubernetesConfigDir     = "/etc/kubernetes"
    KubernetesManifestsDir  = "/etc/kubernetes/manifests"
    KubernetesPKIDir        = "/etc/kubernetes/pki"
    DefaultKubeconfigPath   = "/root/.kube/config"
)

// --- Etcd System Directories ---
const (
    EtcdDataDir = "/var/lib/etcd"
    EtcdConfigDir = "/etc/etcd"
    EtcdPKIDir    = "/etc/etcd/pki"
)

// --- Container Runtime Directories ---
const (
    ContainerdConfigDir = "/etc/containerd"
    DockerConfigDir     = "/etc/docker"
)

// --- Kubexm Work Directories ---
const (
    DefaultBinDir           = "bin"      // 存放下载的二进制文件
    DefaultConfDir          = "conf"     // 存放生成的配置文件
    DefaultScriptsDir       = "scripts"  // 存放临时脚本
    DefaultBackupDir        = "backup"   // 存放备份文件
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **3. Kubernetes 组件与服务常量 (Component & Service Constants)**

定义了核心组件的名称、默认端口和相关服务的名称。

Generated go

```
// --- Component Names ---
const (
    KubeAPIServer        = "kube-apiserver"
    KubeControllerManager = "kube-controller-manager"
    KubeScheduler        = "kube-scheduler"
    Kubelet              = "kubelet"
    KubeProxy            = "kube-proxy"
    Etcd                 = "etcd"
    Containerd           = "containerd"
    Docker               = "docker"
    Kubeadm              = "kubeadm"
    Kubectl              = "kubectl"
)

// --- Service Names (systemd) ---
const (
    KubeletServiceName      = "kubelet.service"
    ContainerdServiceName   = "containerd.service"
    DockerServiceName       = "docker.service"
)

// --- Default Ports ---
const (
    KubeAPIServerDefaultPort        = 6443
    KubeSchedulerDefaultPort        = 10259 // insecure port, secure is 10259
    KubeControllerManagerDefaultPort = 10257 // insecure port, secure is 10257
    KubeletDefaultPort              = 10250
    EtcdDefaultClientPort           = 2379
    EtcdDefaultPeerPort             = 2380
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **4. 配置与资源名称常量 (Configuration & Resource Name Constants)**

定义了Kubernetes内部资源的标准名称，如ConfigMap、Secret、ClusterRole等。

Generated go

```
// --- CoreDNS ---
const (
    CoreDNSConfigMapName = "coredns"
    CoreDNSDeploymentName = "coredns"
)

// --- Kube-proxy ---
const (
    KubeProxyConfigMapName = "kube-proxy"
    KubeProxyDaemonSetName = "kube-proxy"
)

// --- Cluster Info ---
const (
    ClusterInfoConfigMapName = "cluster-info"
    KubeadmConfigConfigMapName = "kubeadm-config"
)

// --- Secrets ---
const (
    BootstrapTokenSecretPrefix = "bootstrap-token-"
)

// --- RBAC ---
const (
    NodeBootstrapperClusterRoleName = "system:node-bootstrapper"
    NodeAdminClusterRoleBindingName = "kubeadm:node-admins"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **5. 状态与条件常量 (Status & Condition Constants)**

用于表示任务、节点或组件的各种状态。

Generated go

```
// --- Task Status ---
const (
    StatusPending    = "Pending"
    StatusProcessing = "Processing"
    StatusSuccess    = "Success"
    StatusFailed     = "Failed"
)

// --- Node Conditions (from k8s.io/api/core/v1) ---
// 可以直接引用官方库，但有时为了解耦或简化，也会在common中定义别名
const (
    NodeConditionReady = "Ready" // This is a string type from corev1.NodeConditionType
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **6. CNI 插件常量 (CNI Plugin Constants)**

定义了支持的CNI插件的名称。

Generated go

```
const (
    CNICalico   = "calico"
    CNIFlannel  = "flannel"
    CNICilium   = "cilium"
    CNIMultus   = "multus"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **7. 缓存键常量 (Cache Key Constants)**

定义在 pkg/cache 中使用的标准键名，防止拼写错误和冲突。

Generated go

```
// --- Cache Keys ---
const (
    // 示例：缓存主机Facts的键前缀
    CacheKeyHostFactsPrefix = "facts.host."

    // 示例：缓存集群证书的键
    CacheKeyClusterCACert = "pki.ca.cert"
    CacheKeyClusterCAKey  = "pki.ca.key"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

### **如何组织这些常量**

如之前建议的，可以将这些常量按功能分散到不同的文件中，以保持 pkg/common 目录的整洁。

- pkg/common/
    - paths.go (文件与目录路径)
    - roles.go (角色、标签、污点)
    - components.go (组件名称、服务、端口)
    - kubernetes.go (Kubernetes内部资源名称)
    - status.go (状态常量)
    - cache_keys.go (缓存键)
    - constants.go (其他通用常量)

通过这样详尽和结构化的常量定义，您的“世界树”项目将拥有一个极其稳固和标准化的基础，使得上层的业务逻辑（Step和Task）可以更加专注于“做什么”，而不是“叫什么”或“在哪里”。这会极大地提升开发效率和系统的健壮性。



### **超详尽版 pkg/common 常量补充**

#### **1. Etcd 深度常量**

Generated go

```
// --- Etcd ---
const (
    // Directories
    EtcdDefaultDataDir      = "/var/lib/etcd"       // Etcd 默认数据目录
    EtcdDefaultWalDir       = "/var/lib/etcd/wal"   // Etcd 默认 WAL 目录
    EtcdDefaultConfDir      = "/etc/etcd"           // Etcd 配置文件目录
    EtcdDefaultPKIDir       = "/etc/etcd/pki"       // Etcd 证书存放目录
    EtcdDefaultBinDir       = "/usr/local/bin"      // Etcd 二进制文件默认安装目录

    // Files
    EtcdDefaultConfFile     = "/etc/etcd/etcd.conf"        // （如果是文件配置）Etcd 配置文件
    EtcdDefaultSystemdFile  = "/etc/systemd/system/etcd.service" // Etcd systemd 服务文件
    EtcdctlDefaultEndpoint  = "127.0.0.1:2379"             // Etcdctl 默认连接端点

    // Component Names
    EtcdComponentName       = "etcd"
    EtcdctlComponentName    = "etcdctl"

    // PKI file names (standard convention)
    EtcdServerCert          = "server.crt"
    EtcdServerKey           = "server.key"
    EtcdPeerCert            = "peer.crt"
    EtcdPeerKey             = "peer.key"
    EtcdCaCert              = "ca.crt"
    EtcdCaKey               = "ca.key" // 通常只在自签发时存在
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **2. 容器运行时深度常量**

Generated go

```
// --- Containerd ---
const (
    // Directories
    ContainerdDefaultConfDir      = "/etc/containerd"         // Containerd 配置目录
    ContainerdDefaultSocketPath   = "/run/containerd/containerd.sock" // Containerd socket 文件路径
    ContainerdDefaultBinDir       = "/usr/local/bin"          // CNI 插件和 critools 等的安装目录

    // Files
    ContainerdDefaultConfigFile   = "/etc/containerd/config.toml"  // Containerd 主配置文件
    ContainerdDefaultSystemdFile  = "/etc/systemd/system/containerd.service" // Containerd systemd 服务文件
    
    // Runc (used by Containerd)
    RuncComponentName = "runc"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

Generated go

```
// --- Docker ---
const (
    // Directories
    DockerDefaultConfDir      = "/etc/docker"                // Docker 配置目录
    DockerDefaultDataRoot     = "/var/lib/docker"            // Docker 默认数据根目录
    DockerDefaultSocketPath   = "/var/run/docker.sock"       // Docker socket 文件路径

    // Files
    DockerDefaultConfigFile   = "/etc/docker/daemon.json"      // Docker daemon 配置文件
    DockerDefaultSystemdFile  = "/lib/systemd/system/docker.service" // Docker systemd 服务文件

    // CRI-Dockerd (for Docker with recent Kubernetes)
    CniDockerdComponentName   = "cri-dockerd"
    CniDockerdSocketPath      = "/var/run/cri-dockerd.sock"
    CniDockerdSystemdFile     = "/etc/systemd/system/cri-dockerd.service"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **3. Kubernetes 配置文件与组件深度常量**

Generated go

```
// --- Kubernetes Config Files (standard kubeadm layout) ---
const (
    KubeadmConfigFileName       = "kubeadm-config.yaml"  // kubeadm 配置文件名
    KubeletConfigFileName       = "kubelet.conf"         // Kubelet 的 kubeconfig 文件
    KubeletSystemdEnvFileName   = "10-kubeadm.conf"      // Kubelet systemd drop-in 配置文件
    ControllerManagerConfigFileName = "controller-manager.conf" // kube-controller-manager 的 kubeconfig
    SchedulerConfigFileName     = "scheduler.conf"         // kube-scheduler 的 kubeconfig
    AdminConfigFileName         = "admin.conf"           // 集群管理员的 kubeconfig

    // Static Pod manifests
    KubeAPIServerStaticPodFileName       = "kube-apiserver.yaml"
    KubeControllerManagerStaticPodFileName = "kube-controller-manager.yaml"
    KubeSchedulerStaticPodFileName      = "kube-scheduler.yaml"
    EtcdStaticPodFileName               = "etcd.yaml"
)

// --- Kubernetes PKI file names (standard kubeadm layout) ---
const (
    CACertFileName                  = "ca.crt"
    CAKeyFileName                   = "ca.key"
    APIServerCertFileName           = "apiserver.crt"
    APIServerKeyFileName            = "apiserver.key"
    APIServerKubeletClientCertFileName = "apiserver-kubelet-client.crt"
    APIServerKubeletClientKeyFileName  = "apiserver-kubelet-client.key"
    FrontProxyCACertFileName        = "front-proxy-ca.crt"
    FrontProxyCAKeyFileName         = "front-proxy-ca.key"
    FrontProxyClientCertFileName    = "front-proxy-client.crt"
    FrontProxyClientKeyFileName     = "front-proxy-client.key"
    ServiceAccountPublicKeyFileName  = "sa.pub"
    ServiceAccountPrivateKeyFileName = "sa.key"
    // Etcd specific PKI (if managed by kubeadm)
    APIServerEtcdClientCertFileName = "apiserver-etcd-client.crt"
    APIServerEtcdClientKeyFileName  = "apiserver-etcd-client.key"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **4. 高可用性 (HA) 组件常量**

Generated go

```
// --- Keepalived ---
const (
    KeepalivedDefaultConfDir      = "/etc/keepalived"
    KeepalivedDefaultConfigFile   = "/etc/keepalived/keepalived.conf"
    KeepalivedDefaultSystemdFile  = "/etc/systemd/system/keepalived.service"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

Generated go

```
// --- HAProxy ---
const (
    HAProxyDefaultConfDir       = "/etc/haproxy"
    HAProxyDefaultConfigFile    = "/etc/haproxy/haproxy.cfg"
    HAProxyDefaultSystemdFile   = "/etc/systemd/system/haproxy.service"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

Generated go

```
// --- Kube-VIP ---
const (
    KubeVIPManifestFileName = "kube-vip.yaml" // Kube-VIP static pod manifest
    KubeVIPDefaultInterface = "eth0"
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

#### **5. 系统级与预检常量 (System & Preflight Constants)**

Generated go

```
// --- System Configuration ---
const (
    SysctlDefaultConfFile     = "/etc/sysctl.conf"
    ModulesLoadDefaultDir     = "/etc/modules-load.d"
    KubernetesSysctlConfFile  = "/etc/sysctl.d/99-kubernetes-cri.conf" // A common pattern for K8s sysctl settings
    KubeletSystemdDropinDir   = "/etc/systemd/system/kubelet.service.d"

    // Kernel Modules
    KernelModuleBrNetfilter   = "br_netfilter"
    KernelModuleIpvs          = "ip_vs"
)

// --- Preflight Check related ---
const (
    DefaultMinCPUCores        = 2
    DefaultMinMemoryMB        = 2048 // 2GB
)
```

content_copydownload

Use code [with caution](https://support.google.com/legal/answer/13505487).Go

### **总结与组织建议**

这份超详细的列表几乎涵盖了通过二进制或 kubeadm 部署一个高可用Kubernetes集群所涉及到的所有关键路径和文件名。

将这些常量按照其所属的**组件**或**功能领域**组织到不同的文件中，将是最佳实践：

- pkg/common/
    - etcd.go
    - containerd.go
    - docker.go
    - kubernetes_paths.go (存放所有K8s相关的文件、目录名)
    - kubernetes_pki.go (专门存放K8s证书文件名)
    - ha.go (存放Keepalived, HAProxy等)
    - system.go (存放系统级配置、内核模块等)
    - ... (其他已有的文件)

通过这种精细化的常量管理，您的部署Step将会变得极其健壮和清晰。例如，一个InstallKubeAPIServerStep的实现代码中，将不会出现任何硬编码的字符串如 "/etc/kubernetes/manifests/kube-apiserver.yaml"，而是使用 filepath.Join(common.KubernetesManifestsDir, common.KubeAPIServerStaticPodFileName)。这使得整个代码库对文件系统布局的变化具有极强的适应性，并且易于审计和理解。