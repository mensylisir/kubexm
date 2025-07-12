

请给cluster增加一个Extra字段
type Extra struct{
	Hosts []EtcHost
	Resolves []EtcResolve
}

type EtcHost struct {
	Domain string
	Address string
}

type  EtcResolve struct {
	Address string
}

项目应该有个step收集cluster.hosts的信息，cluster.spec.registry.privateRegistry的信息(其ip对应cluster.spec.hosts中registry角色的ip)，cluster.spec.extra.hosts的信息，写入到所有机器，
当然也得有个step删除这些字段(比如删除集群的时候，这些hosts应该删除掉)
cluster.spec.extra.etcdresolve.address如果不为空，则需要给各节点的/etc/resolv.conf添加这个ip(如果是systemd-resolve管理的机器好像不是这个文件)

cluster.registry.type如果是harbor，则需要有个step在registry角色的机器上部署harbor，如果type是空，则默认是registry，表示在registry的机器上部署registry(支持docker-registry或二进制部署的registry)



# **注意，step有通用的step，但是通用的step不参与组装task，专用的step调用通用step，然后专用的step组装成task，这样task就不需要有具体的业务逻辑了，具体的业务逻辑都在专用step里**，

#### **Module_00: 前置飞行检查 (Preflight) - 万物之始**

**目标**: 在对系统进行任何修改之前，对所有目标节点进行全面、严格的健康状况与合规性检查。任何一项检查失败，整个部署流程将立即中止，并向用户报告精确的错误信息和修复建议。

**任务 (Task): Task_PreflightChecks**
此任务将在所有目标节点上并行执行。

**步骤 (Step): Step_CheckConnectivity**
此步骤的目的是验证主控节点到所有目标节点的网络连通性和权限。它将首先尝试通过 SSH 连接到目标节点的 22 端口。如果连接失败，将报告“无法连接到节点 [IP]:[Port]，请检查网络策略、防火墙或 SSHD 服务状态。”并中止。连接成功后，将执行 whoami 命令，然后执行 sudo whoami 命令。如果 sudo 执行失败，将报告“sudo whoami 执行失败，请确保用户 [user] 具有免密 sudo 权限。”并中止。

**步骤 (Step): Step_CheckOS**
此步骤的目的是确保操作系统版本在 KubeXM 的支持矩阵内。它将读取并解析目标节点上 /etc/os-release 文件中的 ID 和 VERSION_ID。如果解析出的系统信息不在预定义的支持列表（例如 CentOS 7/8, Rocky 8/9, RHEL 7/8, Ubuntu 20.04/22.04）中，将报告“节点 [IP] 的操作系统 '[ID] [VERSION_ID]' 不在支持列表中。”并中止。

**步骤 (Step): Step_CheckHardware**
此步骤的目的是保证节点硬件资源满足 Kubernetes 运行的最低门槛。它将通过 nproc 命令获取 CPU 核心数，并通过 grep MemTotal /proc/meminfo 获取内存总量。如果 CPU 核心数小于 2，或者 Master 节点的内存小于 4GB，或 Worker 节点的内存小于 2GB，将报告“节点 [IP] 资源不足: CPU核心数 [N] (最低要求: 2) / 内存 [M]GB (Master最低要求: 4GB, Worker最低要求: 2GB)。”并中止。

**步骤 (Step): Step_CheckKernel**
此步骤的目的是验证内核版本及关键内核模块的加载情况。它将通过 uname -r 获取内核版本，并通过 lsmod | grep [module_name] 检查 br_netfilter, overlay, ip_vs 等模块是否已加载。如果内核版本过低（例如低于 3.10）或有必要的模块未加载，将报告“节点 [IP] 内核检查失败: 内核版本 [Version] 过低 / 必要的内核模块 [module_name] 未加载。”并中止。

**步骤 (Step): Step_CheckHostname**
此步骤的目的是确保主机名符合 DNS 规范且可被正确解析。它将通过 hostname 命令检查主机名，确保其不包含大写字母或下划线。然后通过 hostname -i 检查其解析的 IP 地址，确保返回值不是 127.0.0.1 或 ::1。如果不合规，将报告“节点 [IP] 主机名 '[hostname]' 不合规或解析错误。请修改主机名并确保 /etc/hosts 中有正确的 [IP] [hostname] 条目。”并中止。

**步骤 (Step): Step_CheckTimeSync**
此步骤的目的是保证集群各节点间的时间同步。它将获取所有节点的时间戳，并计算与主控节点的时间差。同时通过 systemctl is-active chronyd 或 ntpd 检查 NTP 服务状态。如果任何节点的时间差超过 5 秒，或 NTP 服务未运行，将报告“节点 [IP] 与主控节点时间差超过 5 秒 / NTP 服务 (chronyd/ntpd) 未运行。请配置时间同步。”并中止。

**步骤 (Step): Step_CheckExistingInstallation**
此步骤的目的是防止在已有 Kubernetes 环境的节点上进行覆盖安装。它将检查 /etc/kubernetes 目录是否存在，kubelet.service 服务是否存在，以及是否有 K8s 相关容器在运行。如果检测到任何残留，将报告“节点 [IP] 检测到 Kubernetes 残留。请先执行 kubexm reset 或手动清理干净后再试。”并中止。

**步骤 (Step): Step_CheckPorts**
此步骤的目的是确认 Kubernetes 运行所需的核心端口未被其他服务占用。它将使用 ss -tlpn 或 netstat -tlpn 检查 6443, 2379, 2380, 10250, 10257, 10259 等关键端口。如果发现端口被占用，将报告“节点 [IP] 的端口 [Port] 已被进程 '[ProcessName] (PID:[PID])' 占用。请释放该端口。”并中止。

**步骤 (Step): Step_CheckResolvConf**
此步骤的目的是确保 DNS 解析配置健全，这是所有网络操作的生命线。它将检查 /etc/resolv.conf 文件是否存在、非空，并且至少包含一个有效的、可访问的 nameserver IP 地址。如果检查失败，将报告“节点 [IP] 的 /etc/resolv.conf 文件无效 (不存在、为空或未配置可达的 nameserver)。集群部署严重依赖 DNS，请务必配置一个有效的上游 DNS。”并中止。

**步骤 (Step): Step_CheckSELinuxStatus**
**此步骤是新增的、必不可少的检查。** 它的目的是探测当前 SELinux 的状态。它将执行 sestatus 命令并解析其输出。如果状态为 Enforcing，它不会中止流程，但会记录一个明确的警告信息，告知用户 KubeXM 将在后续步骤中自动将其禁用。这个检查确保了后续 DisableSELinux 步骤的必要性和知情权。

**步骤 (Step): Step_CheckFirewallStatus**
**此步骤是新增的、必不可少的检查。** 它的目的是探测当前防火墙服务的状态。它将依次执行 systemctl is-active firewalld 和 ufw status。如果任何一个服务处于 active 状态，它不会中止流程，但会记录一个明确的警告信息，告知用户 KubeXM 将在后续步骤中自动禁用防火墙。

**步骤 (Step): Step_CheckSwapStatus**
**此步骤是新增的、必不可少的检查。** 它的目的是探测当前 Swap 的状态。它将执行 swapon --show 命令。如果该命令有任何输出（表示 Swap 已开启），它不会中止流程，但会记录一个明确的警告信息，告知用户 KubeXM 将在后续步骤中自动禁用 Swap。这个检查对于保证 kubelet 的稳定运行至关重要。

------



#### **Module_01: 节点标准化 (Node Preparation)**

**目标**: 将通过了 Preflight 检查的所有裸机节点，转变为一个完全一致的、标准化的、准备好承载容器运行时的基础环境。

**任务 (Task): Task_SystemTuning**
此任务将在所有目标节点上并行执行。

**步骤 (Step): Step_DisableSELinux**
**此步骤必须在最前面执行之一。** 它的目的是永久禁用 SELinux，因为 SELinux 的强制模式会干扰容器运行时的文件系统挂载等操作。它将首先执行 setenforce 0 命令，将 SELinux 的当前状态立即切换到 Permissive 模式，避免影响当前会话的后续操作。然后，它会使用 sed -i 's/SELINUX=enforcing/SELINUX=disabled/g' /etc/selinux/config 命令，修改配置文件，确保系统重启后 SELinux 状态为 disabled。此操作会先检查当前值，如果已是 disabled 或 permissive，则跳过修改，以保证幂等性。

**步骤 (Step): Step_DisableSwap**
**此步骤必须在最前面执行之一。** 它的目的是永久禁用 Swap 分区，因为 kubelet 要求禁用 Swap 以保证 Pod 的 QoS 和性能。它将首先执行 swapoff -a 命令以立即关闭所有当前活动的 swap 设备。然后，它会备份 /etc/fstab 文件为 /etc/fstab.bak，并使用 sed -i '/swap/s/^/#/' /etc/fstab 命令注释掉其中所有包含 swap 关键字的有效行，以防止系统重启后自动挂载。此操作会检查 swap 是否已关闭，如果已关闭，则跳过 swapoff，以保证幂等性。

**步骤 (Step): Step_DisableFirewall**
此步骤的目的是永久禁用防火墙，以避免复杂的规则配置，简化集群网络。它将探测节点上是否存在 firewalld 或 ufw。如果存在 firewalld，则执行 systemctl disable --now firewalld。如果存在 ufw，则执行 ufw disable。此操作是幂等的，如果服务已禁用，命令不会报错。

**步骤 (Step): Step_UpdateHostName**

此步骤的目的是将所有节点上的名称更改为cluster.spec.hosts中定义的name

**步骤 (Step): Step_UpdateEtcHosts**
此步骤的目的是在所有节点上写入统一的 hosts 记录。它会首先收集 cluster.spec.hosts, cluster.spec.registry.privateRegistry, cluster.spec.extra.hosts 中的所有域名和 IP 映射。然后，在每个节点上，它会查找由 # BEGIN KubeXM Managed Block 和 # END KubeXM Managed Block 包裹的区域。如果该区域存在，则替换其内容；如果不存在，则在文件末尾追加该区域和新的 hosts 记录。这确保了操作的幂等性和可逆性。

**步骤 (Step): Step_UpdateResolvConf**
此步骤的目的是将 cluster.spec.extra.resolves 中定义的 DNS 服务器添加到节点的解析配置中。它会首先探测 /etc/resolv.conf 是否是指向 systemd-resolved stub 文件的符号链接。如果不是，它将直接在 /etc/resolv.conf 文件顶部添加 nameserver [IP] 条目。如果是，它将修改 /etc/systemd/resolved.conf 文件，将 DNS 服务器地址添加到 DNS= 字段，然后执行 systemctl restart systemd-resolved.service。此操作会检查是否已存在该条目，以保证幂等性。

**步骤 (Step): Step_InstallChrony**

此步骤的目的是配置所有节点的时间同步。它会首先检查 cluster.spec.system.ntpServers 的配置。如果该列表只有一个值，且该值是集群中某台主机的名称，那么它将在该主机上安装 chrony 并配置 /etc/chrony.conf，使其作为 NTP 服务器（允许局域网内其他主机同步,。在所有其他节点上，它会安装 chrony 并配置为客户端，将 server 指向那台 NTP 服务器主机。

**步骤 (Step): Step_ConfigureChrony**
此步骤的目的是配置所有节点的时间同步。它会首先检查 cluster.spec.system.ntpServers 的配置。如果该列表只有一个值，且该值是集群中某台主机的名称，那么它将在该主机上安装 chrony 并配置 /etc/chrony.conf，使其作为 NTP 服务器（允许局域网内其他主机同步）。在所有其他节点上，它会安装 chrony 并配置为客户端，将 server 指向那台 NTP 服务器主机。在所有其他情况下（ntpServers 为空或包含多个外部 IP），它将在所有节点上安装 chrony 并配置为客户端，将 server 或 pool 指向 ntpServers 列表中的地址。配置完成后，执行 systemctl enable --now chronyd。

**步骤 (Step): Step_ConfigureSysctl**
此步骤的目的是应用并持久化 Kubernetes 所需的内核参数。它将在 /etc/sysctl.d/ 目录下创建一个名为 /etc/sysctl.d/99-kubexm-cri.conf的文件，并写入如 net.bridge.bridge-nf-call-iptables = 1, net.ipv4.ip_forward = 1 等参数。写入完成后，执行 sysctl --system 命令以立即加载所有系统配置文件中的设置。

**步骤 (Step): Step_LoadKernelModules**
此步骤的目的是加载并持久化必要的内核模块。它将使用 modprobe 命令依次加载 overlay 和 br_netfilter 模块。为了确保系统重启后模块依然被加载，它会将这些模块名写入到 /etc/modules-load.d/kubexm.conf 文件中。

**步骤 (Step): Step_InstallDependencies**
此步骤的目的是安装部署过程中的所有基础软件包依赖。它会根据探测到的操作系统类型（CentOS/RHEL 或 Ubuntu/Debian），使用对应的包管理器（yum 或 apt-get）安装 socat, conntrack-tools, ebtables, ipset, nfs-utils,iscsi,nfs bash-completion等软件包（这些是默认包）。用户还可以通过cluster.spec.system.rpms或cluster.spec.system.debs自定义安装什么包



#### **Module_02: 资源与运行时 (Artifacts & Runtime)**

**目标**: 下载所有必需的二进制文件和容器镜像，并在所有节点上安装和配置用户选定的容器运行时，为后续的 Kubernetes 组件部署做好准备。

**任务 (Task): Task_ArtifactsManagement**
此任务在主控节点上集中执行下载，然后并行分发到所有目标节点。

**步骤 (Step): Step_DownloadFile**--通用下载文件step，其他step调用通用step下载具体的文件

**步骤 (Step): Step_Downloadkubeadm**

**步骤 (Step): Step_Downloadkubelet**

**步骤 (Step): Step_DownloadKubectl**

**步骤 (Step): Step_DownloadEtcd**

**步骤 (Step): Step_DownloadkubeApiserver**

**步骤 (Step): Step_DownloadkubeControllerManager**

**步骤 (Step): Step_DownloadkubeScheduler**

**步骤 (Step): Step_DownloadkubeProxy**

**步骤 (Step): Step_DownloadCalicoCtl**

**步骤 (Step): Step_DownloadContainerd**

**步骤 (Step): Step_DownloadDocker**

**步骤 (Step): Step_DownloadCriDocker**

**步骤 (Step): Step_DownloadCriO**

**步骤 (Step): Step_DownloadIsulad**

**步骤 (Step): Step_DownloadCNI**

**步骤 (Step): Step_DownloadRunc**

**步骤 (Step): Step_DownloadCrictl**

**步骤 (Step): Step_DownloadCalicoCrds**

**步骤 (Step): Step_DownloadNerdctl**

**步骤 (Step): Step_DownloadHelm**

**步骤 (Step): Step_DownloadDockerCompose**

此步骤的目的是根据 cluster.yaml 的完整配置，智能地计算出本次部署所需的所有二进制文件，从 KMZONE 指定的源下载到主控节点（程序所在目录/kubexm），然后分发到所有对应角色的节点上。

- 
- **如果 cluster.kubernetes.type 是 kubeadm**: 下载 kubeadm, kubelet, kubectl 的指定版本二进制包。
- **如果 cluster.kubernetes.type 是 kubexm**: 下载 kube-apiserver, kube-controller-manager, kube-scheduler, kube-proxy, kubelet, kubectl 的指定版本二进制包。
- **如果 cluster.etcd.type 是 kubexm**: 下载 etcd 和 etcdctl 的指定版本二进制包。
- **如果 cluster.runtime.type 是 containerd**: 下载 containerd-[version]-[os]-[arch].tar.gz。
- **如果 cluster.runtime.type 是 docker**: 下载 cri-dockerd-[version]-[arch].tgz。
- **如果 cluster.runtime.type 是 cri-o**: 此步骤跳过 cri-o 的二进制下载，因为它将通过包管理器安装。
- **如果 cluster.runtime.type 是 isulad**: 下载 isulad-[version]-[os]-[arch].tar.gz (如果选择二进制安装)。
- **如果 cluster.loadBalancer.external.type 是 kubexm-kh 或 kubexm-kn**: 如果操作系统包管理器中的版本过低，或者为了版本锁定，此步骤可能会下载指定版本的 keepalived, haproxy 或 nginx 的源码包或预编译包。

所有下载的包将被分发到目标节点上的一个临时目录，例如 /tmp/kubexm_artifacts。



**步骤 (Step): Step_PullImages**
此步骤的目的是在集群启动前，在所有节点上提前拉取所有必需的容器镜像，以避免在核心组件启动时因网络问题导致部署失败，并极大加速部署过程。

它会首先根据 cluster.yaml 的配置（Kubernetes 版本、网络插件类型、CoreDNS 版本等）生成一个完整的镜像列表。镜像地址会根据 KMZONE 进行调整（例如 k8s.gcr.io 替换为 registry.aliyuncs.com/google_containers）。
然后，在所有节点上并行执行，使用对应运行时的客户端命令进行拉取：

- 
- **对于 containerd**: crictl pull [image]
- **对于 docker**: docker pull [image]
- **对于 cri-o**: crictl pull [image]
- **对于 isulad**: isula pull [image]

------



**任务 (Task): Task_RuntimeSetup**
此任务将在所有节点上并行执行，根据用户选择的运行时类型，执行对应的安装和配置流程。

**当 cluster.runtime.type 被指定为 containerd (默认)**:

1. 
2. **步骤 (Step): Step_InstallContainerd**
   此步骤负责安装 containerd。它会在目标节点上解压从主控节点分发过来的 containerd-[version]-[os]-[arch].tar.gz 文件，并将其中的 containerd, ctr, containerd-shim-runc-v2, crictl, nerdctl 等二进制文件复制到 /usr/local/bin 目录下，并赋予可执行权限。
3. **步骤 (Step): Step_GenerateContainerdConfig**
   此步骤负责生成 containerd 的核心配置文件。它会创建 /etc/containerd/config.toml 文件。文件内容中，[plugins."io.containerd.grpc.v1.cri"] 部分是配置的重点，**必须**确保 sandbox_image 指向 KMZONE 指定的 pause 镜像地址，并且 [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options] 下的 SystemdCgroup 被设置为 true。此外，它会检查 cluster.registry 的配置，如果存在私有仓库，则会在此配置文件中动态添加相应的 [plugins."io.containerd.grpc.v1.cri".registry.mirrors."private-registry.com"] 和 [plugins."io.containerd.grpc.v1.cri".registry.configs."private-registry.com".auth] 部分。
4. **步骤 (Step): Step_SetupContainerdService**
   此步骤负责将 containerd 注册为 systemd 服务。它会创建一个 /usr/lib/systemd/system/containerd.service 文件，内容指向 /usr/local/bin/containerd。
5. **步骤 (Step): Step_EnableRuntime**
   此步骤负责启动并设置 containerd 服务开机自启。它将执行 systemctl daemon-reload，然后执行 systemctl enable --now containerd。

**当 cluster.runtime.type 被指定为 docker**:

1. 
2. **步骤 (Step): Step_InstallDockerCE**
   此步骤将通过节点的包管理器安装 docker。它会首先配置 Docker 官方的 yum 或 apt 软件源，然后执行 yum install docker-ce -y 或 apt-get install docker-ce -y。
3. **步骤 (Step): Step_InstallCriDockerd**
   此步骤安装作为 CRI shim 的 cri-dockerd。它会解压分发过来的 cri-dockerd 二进制包，将二进制文件复制到 /usr/local/bin，然后将其附带的 cri-docker.socket 和 cri-docker.service 文件复制到 /usr/lib/systemd/system/。
4. **步骤 (Step): Step_GenerateDockerDaemonJson**
   此步骤配置 Docker daemon。它会创建或修改 /etc/docker/daemon.json 文件。文件内容中，**必须**包含 "exec-opts": ["native.cgroupdriver=systemd"] 以确保 cgroup driver 与 kubelet 一致。同时，会根据 cluster.registry 配置，动态添加 insecure-registries 和 registry-mirrors 字段。
5. **步骤 (Step): Step_EnableRuntime**
   此步骤启动 docker 和 cri-dockerd。它将执行 systemctl daemon-reload，然后执行 systemctl enable --now docker 和 systemctl enable --now cri-docker.socket。

**当 cluster.runtime.type 被指定为 cri-o**:

1. **步骤 (Step): Step_ConfigureCrioRepo**
   此步骤为 cri-o 配置专门的软件源，因为 cri-o 通常不包含在默认的操作系统源中。它会根据 Kubernetes 版本，在 /etc/yum.repos.d/ 或 /etc/apt/sources.list.d/ 下创建对应的 cri-o 源配置文件。
2. **步骤 (Step): Step_InstallCrio**
   此步骤通过包管理器安装 cri-o。它将执行 yum install cri-o cri-tools -y 或 apt-get install cri-o cri-o-runc -y。
3. **步骤 (Step): Step_GenerateCrioConfig**
   此步骤配置 cri-o。它会修改 /etc/crio/crio.conf 文件，**必须**确保 cgroup_manager 的值被设置为 "systemd"，并将 pause_image 指向 KMZONE 指定的 pause 镜像。对于私有仓库，它会在 /etc/crio/registries.conf.d/ 目录下创建一个新的配置文件，用于定义不安全的仓库和镜像 mirror。
4. **步骤 (Step): Step_EnableRuntime**
   此步骤启动 cri-o。它将执行 systemctl daemon-reload，然后执行 systemctl enable --now crio。

**当 cluster.runtime.type 被指定为 isulad**:

1. 
2. **步骤 (Step): Step_InstallIsulad**
   此步骤安装 iSulad。它会解压分发过来的 isulad 二进制包，并将其二进制文件复制到 /usr/local/bin。
3. **步骤 (Step): Step_GenerateIsuladConfig**
   此步骤配置 iSulad。它会创建 /etc/isulad/daemon.json 文件。文件内容中，**必须**确保 group 字段被设置为 "systemd"。同时，会根据 cluster.registry 配置，动态添加私有仓库相关的字段。
4. **步骤 (Step): Step_SetupIsuladService**
   此步骤将 iSulad 注册为 systemd 服务。它会创建一个 /usr/lib/systemd/system/isulad.service 文件。
5. **步骤 (Step): Step_EnableRuntime**
   此步骤启动 iSulad。它将执行 systemctl daemon-reload，然后执行 systemctl enable --now isulad。



#### **任务 (Task): Task_ArtifactsManagement - 真正的原子化流程**

此任务不再是一个单一的 Step，而是一个根据 cluster.yaml 动态生成的、由大量原子 Step 构成的序列。

**1. 下载阶段 (在主控节点上执行)**

- 
- **逻辑**: 部署引擎会首先解析 cluster.yaml，计算出本次部署所需的所有二进制文件列表（binaryList）和镜像列表（imageList）。
- **执行**:
  - 
  - **对于 binaryList 中的每一个 binary**:
    1. 
    2. **Step_CheckCache**:
       - 
       - **输入**: binary.Name, binary.Version, binary.Checksum。
       - **动作**: 检查本地缓存目录（例如 /var/cache/kubexm）中是否存在有效的文件。
       - **输出**: true (缓存命中) 或 false (缓存未命中)。
    3. **Step_DownloadFile** (仅在 Step_CheckCache 返回 false 时执行):
       - 
       - **输入**: binary.URL, 临时下载路径, binary.Checksum。
       - **动作**: 执行 wget 或 curl 下载文件。下载完成后，校验 sha256sum。如果校验失败，删除文件并报错。
       - **输出**: 成功下载的文件路径。
    4. **Step_UpdateCache** (仅在 Step_DownloadFile 成功后执行):
       - 
       - **输入**: 临时下载路径, 缓存中的最终路径。
       - **动作**: 将下载成功的文件移动到本地缓存目录。

**2. 分发阶段 (从主控节点到所有目标节点)**

- 
- **逻辑**: 对于 binaryList 中的每一个 binary，将其分发到所有需要它的角色节点上。
- **执行**:
  - 
  - **对于 binaryList 中的每一个 binary**:
    - 
    - **对于该 binary 需要被部署到的每一个 node**:
      1. 
      2. **Step_DistributeFile**:
         - 
         - **输入**: 本地缓存中的文件路径, 目标节点 node, 远程临时路径 (例如 /tmp/kubexm_artifacts/[binary_filename])。
         - **动作**: 通过 scp 或 sftp 将文件上传到目标节点。

**3. 镜像拉取阶段 (在所有目标节点上并行执行)**

- 
- **逻辑**: 对于 imageList 中的每一个 image，在所有节点上执行拉取。
- **执行**:
  - 
  - **对于 imageList 中的每一个 image**:
    - 
    - **对于所有 node**:
      1. 
      2. **Step_PullImage**:
         - 
         - **输入**: 目标节点 node, image.URL。
         - **动作**: 在目标节点上，根据 runtime.type 执行对应的命令：crictl pull, docker pull, isula pull。

------



#### **任务 (Task): Task_RuntimeSetup - 重新设计的原子化流程**

此任务将根据 cluster.runtime.type 动态地构建一个由多个、更细粒度的原子 Step 组成的执行序列。

#### **当 cluster.runtime.type = containerd**:

此流程将由以下原子 Step 序列组成：

1. 
2. **Step_ExtractTarball**:
   - 
   - **输入**: 目标节点，源 tar 包路径 (例如 /tmp/kubexm_artifacts/containerd.tar.gz)，解压目标目录 (例如 /tmp/containerd_unpacked)。
   - **动作**: 在目标节点上执行 tar -xzvf [source_path] -C [dest_dir]。
3. **Step_CopyFile** (多次调用):
   - 
   - **输入**: 目标节点，源文件路径，目标文件路径。
   - **动作**: 在目标节点上执行 cp [source_path] [dest_path]。
   - **调用序列**:
     - 
     - 将 /tmp/containerd_unpacked/bin/containerd 复制到 /usr/local/bin/。
     - 将 /tmp/containerd_unpacked/bin/ctr 复制到 /usr/local/bin/。
     - 将 /tmp/containerd_unpacked/bin/crictl 复制到 /usr/local/bin/。
     - ... (为 containerd-shim-runc-v2 等所有需要的二进制文件执行此步骤)
4. **Step_SetFilePermission** (多次调用):
   - 
   - **输入**: 目标节点，文件路径，权限模式 (例如 +x)。
   - **动作**: 在目标节点上执行 chmod [mode] [path]。
   - **调用序列**:
     - 
     - 为 /usr/local/bin/containerd, /usr/local/bin/ctr, /usr/local/bin/crictl 等所有二进制文件设置可执行权限。
5. **Step_CreateDirectory**:
   - 
   - **输入**: 目标节点，目录路径 (例如 /etc/containerd)。
   - **动作**: 在目标节点上执行 mkdir -p [path]。
6. **Step_RenderTemplateToFile**:
   - 
   - **输入**: 目标节点，模板内容 (config.toml.tmpl)，模板变量 (从 cluster.yaml 提取的 sandbox_image, 私有仓库配置等)，最终文件路径 (/etc/containerd/config.toml)。
   - **动作**: 在主控节点上渲染模板，然后将渲染好的内容上传到目标节点的最终文件路径。
7. **Step_RenderTemplateToFile** (再次调用):
   - 
   - **输入**: 目标节点，模板内容 (containerd.service.tmpl)，模板变量，最终文件路径 (/usr/lib/systemd/system/containerd.service)。
   - **动作**: 渲染并上传 systemd 服务文件。
8. **Step_SystemdDaemonReload**:
   - 
   - **输入**: 目标节点。
   - **动作**: 在目标节点上执行 systemctl daemon-reload。
9. **Step_EnableSystemdService**:
   - 
   - **输入**: 目标节点，服务名称 (containerd)。
   - **动作**: 在目标节点上执行 systemctl enable --now containerd。

------



#### **当 cluster.runtime.type = docker**:

此流程将由以下原子 Step 序列组成：

1. 
2. **Step_RunPackageManager**:
   - 
   - **输入**: 目标节点，包管理器 (yum/apt)，动作 (install)，包名 (docker-ce)。
   - **动作**: 在目标节点上执行 yum install docker-ce -y 或 apt-get install -y docker-ce。
3. **Step_ExtractTarball** (for cri-dockerd):
   - 
   - **输入**: 目标节点，源 tar 包路径 (/tmp/kubexm_artifacts/cri-dockerd.tgz)，解压目录 (/tmp/cri-dockerd_unpacked)。
   - **动作**: 执行 tar -xzvf ...。
4. **Step_CopyFile** (for cri-dockerd):
   - 
   - **输入**: 目标节点，/tmp/cri-dockerd_unpacked/cri-dockerd，/usr/local/bin/cri-dockerd。
   - **动作**: 执行 cp ...。
5. **Step_SetFilePermission** (for cri-dockerd):
   - 
   - **输入**: 目标节点，/usr/local/bin/cri-dockerd，+x。
   - **动作**: 执行 chmod ...。
6. **Step_CreateDirectory**:
   - 
   - **输入**: 目标节点，/etc/docker。
   - **动作**: 执行 mkdir -p ...。
7. **Step_RenderTemplateToFile** (for daemon.json):
   - 
   - **输入**: 目标节点，daemon.json.tmpl，变量 (cgroupdriver, 私有仓库配置)，/etc/docker/daemon.json。
   - **动作**: 渲染并上传。
8. **Step_RenderTemplateToFile** (for cri-docker.socket):
   - 
   - **输入**: ... /usr/lib/systemd/system/cri-docker.socket。
   - **动作**: 渲染并上传。
9. **Step_RenderTemplateToFile** (for cri-docker.service):
   - 
   - **输入**: ... /usr/lib/systemd/system/cri-docker.service。
   - **动作**: 渲染并上传。
10. **Step_SystemdDaemonReload**:
    - 
    - **动作**: 执行 systemctl daemon-reload。
11. **Step_EnableSystemdService** (多次调用):
    - 
    - **调用 1**: 输入 docker 服务。
    - **调用 2**: 输入 cri-docker.socket 服务。

------



**对于 cri-o 和 isulad，其执行流程也将同样被分解为上述这些基础的、原子的 Step 组合。** 例如，cri-o 的 Step_ConfigureCrioRepo 将被分解为 Step_RenderTemplateToFile (创建 .repo 文件) 和 Step_RunPackageManager (执行 yum makecache)。



**任务 (Task): Task_EtcdSetup**
此任务的执行逻辑完全由 cluster.etcd.type 决定。

**当 cluster.etcd.type = kubexm (默认)**:
此路径表示 KubeXM 将全权负责从零开始，在 cluster.spec.etcd.nodes 指定的节点上，部署一个高可用的 ETCD 集群。

1. 
2. **证书生成阶段 (在主控节点上执行)**
   1. 
   2. **Step_RenderTemplateToFile** (多次调用):
      - 
      - 创建 etcd-ca-config.json.tmpl 和 etcd-ca-csr.json.tmpl。
      - 创建 etcd-server-csr.json.tmpl (包含 hosts 列表的模板变量)。
   3. **Step_GenerateEtcdCA** (在主控节点执行 openssl工具):
   4. **Step_GenerateEtcdCert** (在主控节点执行openssl 工具):
      - **对于 etcd.nodes 中的每一个 node**:
        - 生成member证书、admin证书、node证书
3. **分发与配置阶段 (在所有 etcd.nodes 上并行执行)**
   1. **Step_CreateDirectory**: 创建 /etc/etcd/pki 和 /var/lib/etcd。
   2. **Step_DistributeFile** (通用分发文件step，其他分发文件step调用这个step):
      - **Step_DistributeEtcdBinaryFile**分发 etcd 和 etcdctl 二进制文件到 /usr/local/bin/。
      - **Step_DistributeEtcdCaFile**分发 etcd-ca.pem 到 /etc/etcd/pki/ca.pem。
      - **Step_DistributeEtcdCertFile** **对于每个 node**: 分发其对应的 member-${node.name}.pem,member-${node.name}-key.pem , admin-${node.name}.pem,admin-${node.name}-key.pem , node-${node.name}.pem,node-${node.name}-key.pem 到 /etc/kubexm/etcd/ssl
   3. **Step_SetFilePermission**: 为 /usr/local/bin/etcd 和 etcdctl 设置可执行权限。
   4. **Step_RenderTemplateToFile**: 渲染 etcd.conf.yml.tmpl，填充 name, ip, 和 **计算出的 initial-cluster 字符串**，生成 /etc/kubexm/etcd/etcd.conf.yml。
   5. **Step_RenderTemplateToFile**: 渲染 etcd.service.tmpl，生成 /etc/systemd/system/etcd.service。
4. **启动与验证阶段 (在所有 etcd.nodes 上执行)**
   1. 
   2. **Step_SystemdDaemonReload**: 执行 systemctl daemon-reload。
   3. **Step_EnableSystemdService**: 执行 systemctl enable --now etcd。
   4. **Step_Sleep**: 等待几秒钟，给集群选举留出时间。
   5. **Step_CheckEtcdHealth** (在任一 ETCD 节点上执行): 执行 etcdctl endpoint health --cluster 命令，并检查输出是否包含所有成员的 is healthy 信息。

**当 cluster.etcd.type = kubeadm**:

- 
- **工具行为**: KubeXM **完全不** 执行 Task_EtcdSetup。ETCD 的部署和管理将由后续 Module_04 中的 kubeadm 自动完成。

**当 cluster.etcd.type = external**:

- 
- **工具行为**: KubeXM **完全不** 执行 Task_EtcdSetup。它假定用户已自行准备好 ETCD 集群。后续 Module_04 将直接使用用户在 cluster.yaml 中提供的 external ETCD 配置。

------



**任务 (Task): Task_RegistrySetup**
此任务的执行逻辑由 cluster.registry.type 决定，并在 registry 角色的节点上执行。

**当 cluster.registry.type = registry**:

1. 
2. **Step_PullImage**: 在 registry 节点上拉取 registry:2 镜像。
3. **Step_CreateDirectory**: 在 registry 节点上创建用于数据持久化的目录，例如 /data/docker_registry。
4. **Step_RunRegistryContainer**: 在 registry 节点上，使用对应的运行时命令（如 docker run 或 crictl run 的 pod sandbox + container 组合）启动 registry:2 容器，并将持久化目录挂载到容器的 /var/lib/registry。

**当 cluster.registry.type = harbor**:

1. **Step_DistributionDocker**: 分发docker到 registry 节点上安装 
2. **Step_DistributionDockerCompose**: 分发 docker-compose到 registry 节点上安装
3. **Step_DistributionHarbor**: 分发Harbor 的离线安装包 harbor-offline-installer-vX.Y.Z.tgz到registry。
4. **Step_ExtractTarball**: 解压 Harbor 安装包。
5. **Step_CopyFile**: 将 harbor.yml.tmpl 复制到解压后的 harbor/ 目录中，并重命名为 harbor.yml。
6. **Step_RenderTemplateToFile**: 使用 cluster.registry.harborConfig 中的变量渲染 harbor.yml 文件。
7. **Step_InstallHarbor**: 在 registry 节点的 harbor/ 目录下，执行 ./install.sh 命令。

**当 cluster.registry.type = 无 或 external**:

- 
- **工具行为**: **完全不** 执行 Task_RegistrySetup。

------



**任务 (Task): Task_LoadBalancerSetup**
此任务的逻辑较为复杂，因为它的一部分（外部LB）在 K8s 启动前执行，另一部分（内部LB）可能与 K8s 启动过程耦合。

**当 cluster.loadBalancer.mode = external**:
此任务在 loadbalancer 角色的节点上执行。

- 
- **当 cluster.loadBalancer.external.type = kubexm-kh (Keepalived + HAProxy)**:
  1. 
  2. **Step_RunPackageManager**: 安装 keepalived 和 haproxy--**这一步需要再最前面安装socat那一步一块安装。**
  3. **Step_RenderTemplateToFile**: 渲染 keepalived.conf.tmpl，填充 virtual_ipaddress 和健康检查脚本，生成 /etc/keepalived/keepalived.conf。
  4. **Step_RenderTemplateToFile**: 渲染 haproxy.cfg.tmpl，填充 backend 服务器为所有 Master 节点的 IP 和端口，生成 /etc/haproxy/haproxy.cfg。
  5. **Step_EnableSystemdService** (多次调用): 启用 keepalived 和 haproxy 服务。
- **当 cluster.loadBalancer.external.type = kubexm-kn (Keepalived + Nginx)**:
  1. 
  2. **Step_RunPackageManager**: 安装 keepalived 和 nginx。--**这一步需要再最前面安装socat那一步一块安装。**
  3. **Step_RenderTemplateToFile**: 渲染 keepalived.conf.tmpl。
  4. **Step_RenderTemplateToFile**: 渲染 nginx.conf.tmpl，**必须** 使用 stream 模块来做四层 TCP 代理，配置 upstream 块指向所有 Master 节点。
  5. **Step_EnableSystemdService** (多次调用): 启用 keepalived 和 nginx 服务。
- **当 cluster.loadBalancer.external.type = external**:
  - 
  - **工具行为**: **完全不** 执行此任务。VIP 地址将直接从 cluster.loadBalancer.vip 字段获取。



#### **Module_04: Kubernetes 引导 (Kubernetes Bootstrap)**

**目标**: 部署 Kubernetes 的核心组件，包括控制平面和节点组件，并将所有节点加入集群。这是整个部署流程中最复杂、组合情况最多的模块。

**任务 (Task): Task_KubernetesBootstrap**
此任务的执行路径由 cluster.kubernetes.type 决定。

------



**当 cluster.kubernetes.type = kubeadm (默认)**:
此路径利用 kubeadm 工具链来引导集群，KubeXM 的主要职责是为其准备好所有前置条件和配置文件。

1. 
2. **准备阶段 (在所有 Master 节点上执行)**
   1. 
   2. **Step_CreateDirectory** (多次调用):
      - 
      - 创建 /etc/kubexm/etcd/ssl 目录（如果使用外部 ETCD）。
      - 创建 /etc/kubexm/ 目录（用于存放一些 KubeXM 生成的额外配置）。
   3. **当 cluster.etcd.type = external 时，执行以下步骤**:
      - 
      - **Step_DistributeFile** (多次调用):
        - 
        - 将用户在 cluster.etcd.externalConfig 中提供的 ca.pem 分发到所有 Master 节点的/etc/kubexm/etcd/ssl 。
        - 分发其对应的 member-${node.name}.pem,member-${node.name}-key.pem , admin-${node.name}.pem,admin-${node.name}-key.pem , node-${node.name}.pem,node-${node.name}-key.pem 到 /etc/kubexm/etcd/ssl
3. **内部负载均衡器部署阶段 (仅当 cluster.loadBalancer.mode = internal 时执行)**
   此阶段在**第一个 Master** 节点上执行，因为这些组件将作为静态 Pod 运行，由 kubelet 在 kubeadm init 之前或期间拉起。
   - 
   - **当 cluster.loadBalancer.internal.type = haproxy 或 nginx**:
     1. 
     2. **Step_RenderTemplateToFile** (生成静态 Pod 清单):
        - 
        - **输入**: haproxy-static-pod.yaml.tmpl 或 nginx-static-pod.yaml.tmpl 模板。
        - **动作**: 渲染并生成最终的 YAML 文件。
     3. **Step_RenderTemplateToFile** (生成配置文件):
        - 
        - **输入**: haproxy.cfg.tmpl 或 nginx.conf.tmpl，其中 backend/upstream 指向所有 Master 节点的 IP。
        - **动作**: 渲染并生成最终的配置文件，保存到例如 /etc/kubexm/haproxy.cfg。
     4. **Step_CopyFile** (部署静态 Pod 清单):
        - 
        - 将生成的静态 Pod YAML 文件复制到第一个 Master 节点的 /etc/kubernetes/manifests/ 目录下。
   - **当 cluster.loadBalancer.internal.type = kube-vip**:
     1. 
     2. **Step_PullImage**: 在第一个 Master 节点上提前拉取 kube-vip 镜像。
     3. **Step_RenderTemplateToFile** (生成 kube-vip 静态 Pod 清单):
        - 
        - **输入**: kube-vip-static-pod.yaml.tmpl，模板变量包括 VIP 地址和网卡接口。
        - **动作**: 渲染并生成最终的 YAML 文件。
     4. **Step_CopyFile** (部署静态 Pod 清单):
        - 
        - 将生成的 kube-vip.yaml 文件复制到第一个 Master 节点的 /etc/kubernetes/manifests/ 目录下。
4. **Kubeadm 初始化阶段**
   1. 
   2. **Step_RenderTemplateToFile** (在主控节点生成 kubeadm-config.yaml):
      - 
      - **这是 kubeadm 路径下最核心的智能步骤。**
      - **输入**: kubeadm-config.yaml.tmpl 和完整的 cluster.yaml 对象。
      - **动作**:
        - 
        - 填充 controlPlaneEndpoint: 使用 cluster.loadBalancer.vip。
        - 填充 kubernetesVersion: 使用 cluster.kubernetes.version。
        - 填充 podSubnet 和 serviceSubnet: 使用 cluster.network.podCidr 和 serviceCidr。
        - 填充 imageRepository: 根据 KMZONE 决定。
        - **填充 etcd 块 (关键逻辑)**:
          - 
          - 如果 etcd.type 是 kubeadm，则此块留空或显式设置为 local。
          - 如果 etcd.type 是 kubexm，则填充 external 块，其 endpoints, caFile, certFile, keyFile 指向 Task_EtcdSetup 创建的集群和证书路径。
          - 如果 etcd.type 是 external，则填充 external 块，其配置指向用户在 cluster.etcd.externalConfig 中提供的信息。
   3. **Step_DistributeFile**: 将生成的 kubeadm-config.yaml 分发到第一个 Master 节点。
   4. **Step_RunCommand** (在第一个 Master 节点上执行 kubeadm init):
      - 
      - **动作**: 执行 kubeadm init --config /path/to/kubeadm-config.yaml --upload-certs。
      - **输出捕获**: **必须** 捕获 kubeadm init 的完整输出，并从中用正则表达式解析出 kubeadm join (for master) 和 kubeadm join (for worker) 的命令，包括 token 和 discovery-token-ca-cert-hash。
5. **集群后初始化阶段 (在主控节点或第一个 Master 上执行)**
   1. 
   2. **Step_CreateDirectory**: 在主控节点为用户创建 .kube 目录。
   3. **Step_DownloadFile**: 从第一个 Master 节点下载 /etc/kubernetes/admin.conf 到主控节点，并保存为 config。
   4. **Step_RunKubectl** (使用刚下载的 admin.conf):
      - 
      - **动作**: 执行 kubectl apply -f [cni_yaml_url]，根据 cluster.network.plugin (calico, flannel, cilium) 安装网络插件。
6. **节点加入阶段**
   1. 
   2. **Step_RunCommand** (在其他所有 Master 节点上并行执行):
      - 
      - **动作**: 执行 kubeadm init 输出中捕获到的 master join 命令。
   3. **Step_RunCommand** (在所有 Worker 节点上并行执行):
      - 
      - **动作**: 执行 kubeadm init 输出中捕获到的 worker join 命令。

------



**当 cluster.kubernetes.type = kubexm**:
此路径完全不依赖 kubeadm，KubeXM 通过二进制文件和 systemd 服务从零构建整个集群。

1. 
2. **证书和 Kubeconfig 生成与分发阶段**
   1. 
   2. **Step_RunCommand** (在主控节点多次调用 cfssl): 生成 K8s CA，以及为 apiserver, controller-manager, scheduler, kubelet, kube-proxy, admin 等所有组件签发证书。
   3. **Step_RunCommand** (在主控节点多次调用 kubectl config set-cluster/set-credentials/set-context): 为 controller-manager, scheduler, kube-proxy, admin 生成对应的 kubeconfig 文件。
   4. **Step_DistributeFile** (多次调用): 将所有生成的证书和 kubeconfig 文件分发到所有对应角色的节点的正确路径下 (例如 /etc/kubernetes/pki, /etc/kubernetes/kubeconfigs)。
3. **控制平面部署阶段 (在所有 Master 节点上并行执行)**
   1. 
   2. **Step_RenderTemplateToFile** (生成 kube-apiserver.service):
      - 
      - **核心逻辑**: 启动参数中，--etcd-servers 等参数根据 etcd.type 指向正确的 ETCD 集群。--bind-address, --advertise-address 等指向节点 IP。--service-cluster-ip-range 指向 serviceCidr。
   3. **Step_RenderTemplateToFile** (生成 kube-controller-manager.service):
      - 
      - **核心逻辑**: --kubeconfig 指向分发过来的 controller-manager.kubeconfig。--cluster-cidr 和 --service-cluster-ip-range 必须正确配置。
   4. **Step_RenderTemplateToFile** (生成 kube-scheduler.service):
      - 
      - **核心逻辑**: --kubeconfig 指向 scheduler.kubeconfig。
   5. **Step_EnableSystemdService** (多次调用): 依次启动 kube-apiserver, kube-controller-manager, kube-scheduler。
4. **节点组件部署阶段 (在所有节点上并行执行)**
   1. 
   2. **Step_RenderTemplateToFile** (生成 kubelet.service 和 kubelet-config.yaml):
      - 
      - **核心逻辑**: kubelet-config.yaml 中 clusterDNS 指向 CoreDNS 的 Service IP (需要预先规划好，例如 10.96.0.10)，clusterDomain 指向 cluster.local。kubelet.service 中 --kubeconfig 指向 kubelet.kubeconfig，--config 指向 kubelet-config.yaml。
   3. **Step_RenderTemplateToFile** (生成 kube-proxy.kubeconfig 和 kube-proxy-config.yaml):
      - 
      - **核心逻辑**: kube-proxy-config.yaml 中 mode 设置为 ipvs，clusterCIDR 设置正确。
   4. **Step_RenderTemplateToFile** (生成 kube-proxy.service):
      - 
      - **核心逻辑**: --config 指向 kube-proxy-config.yaml。
   5. **Step_EnableSystemdService** (多次调用): 启动 kubelet 和 kube-proxy。
5. **集群后初始化阶段 (在主控节点执行)**
   1. 
   2. **Step_RunKubectl**: 使用 admin.kubeconfig 安装 CNI 网络插件。
   3. **Step_RunKubectl**: 部署 CoreDNS 的 YAML。在二进制部署模式下，CoreDNS 需要手动部署。

------



#### **Module_05: 集群终装 (Cluster Finalization)**

**目标**: 部署集群运行所必需的附加组件，并完成最后的配置。

**任务 (Task): Task_ClusterFinalization**
此任务在主控节点上执行，面向整个集群进行操作。

1. 
2. **Step_RunKubectl** (多次调用):
   - 
   - **动作**: kubectl label node ... 和 kubectl taint node ...，根据 cluster.spec.hosts 中定义的角色，为节点打上正确的标签（如 node-role.kubernetes.io/master）和污点。
3. **Step_RunKubectl**:
   - 
   - **动作**: kubectl apply -f [nodelocaldns.yaml]，部署 NodeLocal DNSCache 以提升集群 DNS 解析性能和稳定性。
4. **Step_RunKubectl 或 Step_RunHelm** (循环执行):
   - 
   - **逻辑**: 遍历 cluster.spec.addons 列表。
   - **对于每个 addon**:
     - 
     - 如果 addon.type 是 yaml，则执行 Step_RunKubectl，apply 其指定的 URL 或本地文件路径。
     - 如果 addon.type 是 helm，则执行 Step_RunHelm，依次执行 helm repo add, helm repo update, helm install 等命令，并传入 addon.values 中的配置。





### **KubeXM 终极部署矩阵与执行路径 v12.0 (扩展篇)**

#### **Pipeline_AddNode: 添加节点流水线**

**目标**: 向一个已经存在的、由 KubeXM 部署的集群中，平滑地添加一个新的 Worker 或 Master 节点。

**前置条件**:

- 
- 主控节点上保留着初次部署时生成的集群状态文件（包含证书、token 等关键信息）。
- 新节点必须满足 Module_00: Preflight 的所有检查要求。

**任务 (Task): Task_AddNode**

1. 

2. **准备阶段 (在主控节点和新节点上执行)**

   1. 
   2. **Step_RunTask**: 在新节点上完整地执行 Task_PreflightChecks。
   3. **Step_RunTask**: 在新节点上完整地执行 Task_SystemTuning。
   4. **Step_RunTask**: 在新节点上完整地执行 Task_ArtifactsManagement（只下载和分发该节点角色所需的二进制和镜像）。
   5. **Step_RunTask**: 在新节点上完整地执行 Task_RuntimeSetup。

3. **节点加入阶段 (根据集群类型决定)**

   **当集群类型 (cluster.kubernetes.type) = kubeadm**:

   - 
   - **如果要添加 Worker 节点**:
     1. 
     2. **Step_RunCommand** (在新节点上执行):
        - 
        - **动作**: 执行初次部署时保存的 worker join 命令。如果 token 已过期，KubeXM 需要先在 Master 节点上执行 kubeadm token create --print-join-command 来生成一条新的 join 命令。
   - **如果要添加 Master 节点**:
     1. 
     2. **Step_RunCommand** (在新节点上执行):
        - 
        - **动作**: 执行初次部署时保存的 master join 命令 (--control-plane）。同样，如果 token 过期，需要重新生成。

   **当集群类型 (cluster.kubernetes.type) = kubexm**:

   - 
   - **如果要添加 Worker 节点**:
     1. 
     2. **Step_RunCommand** (在主控节点执行 cfssl): 为新节点生成专用的 kubelet 和 kube-proxy 证书。
     3. **Step_RunCommand** (在主控节点执行 kubectl config): 为新节点的 kubelet 和 kube-proxy 生成专用的 kubeconfig 文件。
     4. **Step_DistributeFile** (多次调用): 将新生成的证书、kubeconfig 以及公共的 CA 证书分发到新节点。
     5. **Step_RunTask**: 在新节点上执行一个简化的 Task_DeployNodeComponents，只包含部署 kubelet 和 kube-proxy 的 systemd 服务的相关 Step。
   - **如果要添加 Master 节点**:
     1. 
     2. **Step_RunTask**: 在新节点上完整地执行 Task_DeployControlPlaneComponents (部署 apiserver, scheduler, controller-manager)。
     3. **Step_RunTask**: 在新节点上完整地执行 Task_DeployNodeComponents (部署 kubelet, kube-proxy)。
     4. **Step_UpdateLoadBalancerConfig**:
        - 
        - **动作**: 如果使用了负载均衡器，KubeXM 需要自动更新 LB 的后端服务器池，将新的 Master 节点 IP 添加进去，并触发 LB 配置重载。
        - **对于 kubexm-kh/kn**: 登录到 LB 节点，执行 Step_RenderTemplateToFile 更新 haproxy.cfg/nginx.conf，然后执行 Step_RunCommand (systemctl reload haproxy)。
        - **对于内部静态 Pod LB**: 更新 /etc/kubexm/[lb].cfg 并触发 Pod 重启（例如通过删除 Pod）。

------



#### **Pipeline_DeleteNode: 删除节点流水线**

**目标**: 从集群中安全、优雅地移除一个节点，并清理该节点上的所有相关资源。

**任务 (Task): Task_DeleteNode**

1. 
2. **集群侧操作 (在主控节点执行)**
   1. 
   2. **Step_RunKubectl** (执行 drain):
      - 
      - **动作**: 执行 kubectl drain [node_name] --delete-emptydir-data --force --ignore-daemonsets，驱逐节点上的所有 Pod。
   3. **Step_RunKubectl** (执行 delete):
      - 
      - **动作**: 执行 kubectl delete node [node_name]，从集群中移除该节点对象。
   4. **如果删除的是 Master 节点**:
      - 
      - **当集群类型 = kubeadm**:
        - 
        - **Step_RunCommand**: 在一个健康的 Master 节点上执行 kubeadm reset --force 可能不足以从etcd中移除成员，需要手动执行 etcdctl member remove [member_id]。
      - **当集群类型 = kubexm**:
        - 
        - **Step_RunCommand**: 在一个健康的 ETCD 节点上，使用 etcdctl member remove [member_id] 将被删除节点的 ETCD 成员移除。
        - **Step_UpdateLoadBalancerConfig**: 从 LB 的后端池中移除该 Master 节点的 IP。
3. **节点侧清理 (在被删除的节点上执行)**
   1. 
   2. **Step_RunCommand**:
      - 
      - **当集群类型 = kubeadm**: 执行 kubeadm reset --force --cri-socket unix:///var/run/crio/crio.sock (根据实际运行时调整socket路径)。
      - **当集群类型 = kubexm**:
        - 
        - SubStep: systemctl stop kubelet kube-proxy ...
        - SubStep: systemctl disable kubelet kube-proxy ...
   3. **Step_RemoveFilesAndDirs** (多次调用):
      - 
      - **动作**: 强制删除 /etc/kubernetes, /var/lib/kubelet, /var/lib/etcd, /etc/cni/net.d 等目录。
   4. **Step_RunTask** (执行反向任务):
      - 
      - **动作**: 执行一个反向的 Task_SystemTuning_Revert，用于清理 hosts 条目、NTP 配置、sysctl 配置等。

------



#### **Pipeline_DeleteCluster: 销毁集群流水线**

**目标**: 彻底、干净地销毁整个集群，将所有节点恢复到接近初始安装 KubeXM 之前的状态。

**任务 (Task): Task_DeleteCluster**

此任务将在所有节点上并行执行。

1. 
2. **Step_RunCommand** (在所有节点执行):
   - 
   - **当集群类型 = kubeadm**: kubeadm reset -f --cri-socket ...
   - **当集群类型 = kubexm**: systemctl stop kubelet kube-proxy kube-apiserver ...
3. **Step_StopAndDisableServices** (在所有节点并行执行):
   - 
   - **动作**: 停止并禁用所有由 KubeXM 安装的服务，包括 containerd, docker, crio, isulad, etcd, keepalived, haproxy 等。
4. **Step_RemoveFilesAndDirs** (在所有节点并行、多次调用):
   - 
   - **动作**: 强制递归删除 /etc/kubernetes, /var/lib/kubelet, /var/lib/etcd, /var/lib/cni, /etc/cni, /opt/cni, /etc/kubexm, /var/cache/kubexm (仅主控) 等所有相关目录。
5. **Step_RevertSystemConfig** (在所有节点并行执行):
   - 
   - SubStep: RevertEtcHosts: 清理 /etc/hosts 中由 KubeXM 管理的区块。
   - SubStep: RevertResolvConf: 清理 /etc/resolv.conf 或 systemd-resolved 的配置。
   - SubStep: UninstallDependencies: (可选) 卸载 socat, conntrack-tools 等依赖包。
   - SubStep: RevertSysctl: 删除 /etc/sysctl.d/99-kubernetes-cri.conf。
   - SubStep: RevertKernelModules: 删除 /etc/modules-load.d/k8s.conf。

------



#### **错误处理与回滚机制**

这是一个产品级工具必须具备的高级特性。

- 
- **Step 级别的错误处理**:
  - 
  - 每个 Step 的执行都必须有明确的超时时间。
  - 每个 Step 执行后，必须有验证环节。例如，Step_EnableSystemdService 执行后，必须紧跟一个 Step_CheckSystemdServiceStatus 来确认服务是否真的 active。如果验证失败，则认为该 Step 失败。
- **Task 级别的回滚**:
  - 
  - KubeXM 引擎在执行每个 Step 前，会将其对应的 Revert Step 推入一个回滚栈。
  - 如果 Task 中的任何一个 Step 执行失败，引擎将停止正向执行，并开始按后进先出（LIFO）的顺序，依次执行回滚栈中的所有 Revert Step。
  - 例如，如果 Task_RuntimeSetup 在 Step_EnableRuntime 失败了，它会依次执行 Revert_SetupContainerdService (删除 .service 文件), Revert_GenerateContainerdConfig (删除 config.toml), Revert_InstallContainerd (删除二进制文件)。
- **Pipeline 级别的断点续传**:
  - 
  - KubeXM 引擎在每成功执行完一个 Task 后，会将其状态持久化到本地的状态文件（例如 .kubexm_state.json）。
  - 如果整个 Pipeline 因为网络中断等原因失败，用户可以执行 kubexm create cluster --resume。
  - KubeXM 会读取状态文件，跳过所有已成功完成的 Task，从失败的那个 Task 开始继续执行。



### **KubeXM 终极部署矩阵与执行路径 v12.0 (高级运维篇)**

#### **Pipeline_UpgradeCluster: 集群升级流水线**

**目标**: 安全、平滑地将 Kubernetes 集群从一个版本升级到另一个受支持的版本（例如 v1.27.x -> v1.28.y），支持灰度、可回滚的升级策略。

**核心原则**: 遵循 Kubernetes 官方推荐的升级顺序：**ETCD (如果由 KubeXM 管理) -> 控制平面 -> 节点组件**。绝不能跨次要版本升级（例如不能从 1.26 直接升到 1.28）。

**任务 (Task): Task_UpgradeCluster**

1. 
2. **准备阶段 (Pre-Upgrade)**
   1. 
   2. **Step_RunPreflightChecks_Upgrade**:
      - 
      - **动作**: 这是一个专门的升级前检查。
      - SubStep_CheckVersionSkew: 确认当前集群版本与目标版本兼容，不能跨版本。
      - SubStep_CheckClusterHealth: 使用 kubectl get nodes, kubectl get cs, etcdctl endpoint health 确认集群当前处于健康状态。
      - SubStep_CheckDeprecatedAPIs: 使用 pluto 或类似工具，扫描集群中是否存在将在目标版本中被废弃的 API 对象，并发出严重警告。
      - SubStep_BackupClusterState: **(关键)** 执行一次全量备份（详见备份与恢复模块）。
   3. **Step_DownloadAndDistributeBinaries_Upgrade**:
      - 
      - **动作**: 下载**目标版本**的 Kubernetes 二进制文件和容器镜像，并分发到所有节点。
3. **ETCD 升级阶段 (仅当 cluster.etcd.type = kubexm 时执行)**
   1. 
   2. **Step_UpgradeEtcdMember** (逐个节点执行):
      - 
      - **动作**: 在一个 ETCD 节点上，systemctl stop etcd，替换 etcd, etcdctl 二进制文件为新版本，然后 systemctl start etcd。
      - **验证**: 启动后，立即执行 etcdctl endpoint health 确认该成员重新加入集群并健康。
   3. **Step_Sleep**: 在升级下一个成员之前，等待一小段时间以确保集群稳定。
4. **控制平面升级阶段 (Control Plane Upgrade)**
   - 
   - **当集群类型 (cluster.kubernetes.type) = kubeadm**:
     1. 
     2. **Step_RunCommand** (在第一个 Master 节点执行):
        - 
        - **动作**: 执行 kubeadm upgrade plan 查看升级计划。
        - **动作**: 执行 kubeadm upgrade apply v[target_version]。kubeadm 会自动升级静态 Pod 清单 (kube-apiserver, kube-controller-manager, kube-scheduler, etcd)。
     3. **Step_UpgradeKubeletAndKubectl_Node** (在第一个 Master 节点执行):
        - 
        - SubStep_RunPackageManager 或 Step_CopyFile: 升级 kubelet 和 kubectl 二进制文件。
        - SubStep_SystemdDaemonReload
        - SubStep_RestartService: systemctl restart kubelet。
     4. **Step_RunCommand** (在其他所有 Master 节点上**逐个**执行):
        - 
        - SubStep_RunCommand: kubeadm upgrade node。
        - SubStep_UpgradeKubeletAndKubectl_Node (在该节点上执行)。
   - **当集群类型 (cluster.kubernetes.type) = kubexm**:
     1. 
     2. **Step_UpgradeControlPlaneComponent** (在所有 Master 节点上**逐个**执行):
        - 
        - SubStep_DrainNode: (可选，更安全) kubectl drain [node_name] --ignore-daemonsets。
        - SubStep_StopService: systemctl stop kube-apiserver。
        - SubStep_ReplaceBinary: 替换 /usr/local/bin/kube-apiserver 为新版本。
        - SubStep_StartService: systemctl start kube-apiserver。
        - SubStep_UncordonNode: kubectl uncordon [node_name]。
        - **对 kube-controller-manager 和 kube-scheduler 重复以上步骤。**
5. **节点组件升级阶段 (Node Upgrade)**
   - 
   - **Step_UpgradeNodeComponent** (在所有节点上，可以分批次并行执行):
     1. 
     2. **Step_DrainNode**: kubectl drain [node_name] --ignore-daemonsets。
     3. **当集群类型 = kubeadm**:
        - 
        - SubStep_RunCommand: kubeadm upgrade node。
     4. **当集群类型 = kubexm 或 kubeadm**:
        - 
        - SubStep_UpgradeKubeletAndKubectl_Node: 升级 kubelet 和 kubectl 二进制文件。
        - SubStep_SystemdDaemonReload
        - SubStep_RestartService: systemctl restart kubelet。
        - SubStep_UpgradeKubeProxy: (如果是 kubexm) 替换 kube-proxy 二进制并重启服务。对于 kubeadm，通常是通过 DaemonSet 管理，升级 Master 会自动触发更新。
     5. **Step_UncordonNode**: kubectl uncordon [node_name]。

------



#### **证书生命周期管理**

**目标**: 提供自动化工具来检查、续订即将过期的 Kubernetes 证书。

**任务 (Task): Task_ManageCerts**

- 
- **Step_CheckCertsExpiration**:
  - 
  - **动作**:
    - 
    - **当集群类型 = kubeadm**: 在 Master 节点上执行 kubeadm certs check-expiration。
    - **当集群类型 = kubexm**: 编写一个脚本或 Go 程序，使用 openssl x509 -in [cert_path] -noout -enddate 遍历 /etc/kubernetes/pki 下的所有证书，并检查其有效期。
  - **输出**: 报告所有证书的到期时间，对 90 天内即将到期的证书发出警告。
- **Step_RenewCerts**:
  - 
  - **动作**:
    - 
    - **当集群类型 = kubeadm**: 执行 kubeadm certs renew all。
    - **当集群类型 = kubexm**:
      - 
      - SubStep_RenewCert_Component: 需要重新执行 Task_EtcdSetup 和 Task_KubernetesBootstrap 中所有与 cfssl 相关的 Step，使用已有的 CA 重新签发所有证书。
      - 这是一个复杂的过程，需要精确替换所有节点上的证书，并**依次重启**使用这些证书的组件（apiserver, etcd, kubelet 等）。
  - **后续动作**: 证书续订后，需要重新分发 admin.conf 等 kubeconfig 文件。

------



#### **备份与恢复 (Backup & Restore)**

**目标**: 提供对集群关键数据的快照备份和灾难恢复能力。

**任务 (Task): Task_BackupCluster**

- 
- **Step_BackupETCD**:
  - 
  - **动作**: 在一个 ETCD 节点上，执行 etcdctl snapshot save [backup_path]。**必须** 使用 ETCD 的 API v3，并带上所有 TLS 认证参数 (--endpoints, --cacert, --cert, --key)。
- **Step_BackupPKI**:
  - 
  - **动作**: 将 Master 节点上的整个 /etc/kubernetes/pki 目录和 /etc/etcd/pki (如果 etcd.type=kubexm) 打包成一个 tar.gz 文件。
- **Step_BackupKubeadmConfig** (仅当 kubeadm):
  - 
  - **动作**: 备份 /etc/kubernetes/ 目录下的 kubeadm-config.yaml（如果有）。
- **Step_StoreBackup**:
  - 
  - **动作**: 将 ETCD 快照和 PKI 压缩包加密后，上传到远端存储（如 S3, NFS）。

**任务 (Task): Task_RestoreCluster** (灾难恢复场景)

这是一个高风险操作，通常需要在所有节点都重置（或全新的）环境下进行。

1. 
2. **Step_RestorePKIAndConfig**: 将备份的 PKI 目录和 kubeadm-config.yaml 恢复到第一个 Master 节点的正确位置。
3. **Step_RestoreETCD**:
   - 
   - **动作**: 在第一个 ETCD 节点上，执行 etcdctl snapshot restore [snapshot_path] --data-dir [new_data_dir]。
   - **关键**: 恢复后，需要用新的数据目录启动一个**单节点**的 ETCD 集群，后续再将其他节点作为新成员加入。
4. **Step_ReBootstrapCluster**:
   - 
   - **动作**: 实际上是重新执行一个修改版的 Task_KubernetesBootstrap，但这次它会使用恢复的 PKI 和 ETCD 数据，而不是从头生成。所有组件的启动参数需要指向恢复后的数据。

------



#### **系统可观测性 (Observability)**

**目标**: KubeXM 本身需要提供对其部署过程的可观测性。

- 
- **结构化日志 (Structured Logging)**:
  - 
  - 所有 Step 的输出都必须是结构化的 JSON 日志。
  - 日志中应包含 timestamp, level, pipeline_id, module, task, step, node, message, 以及 duration_ms 等字段。
- **Metrics 暴露**:
  - 
  - KubeXM 引擎可以内置一个 Prometheus Exporter，暴露部署过程的度量指标，例如：
    - 
    - kubexm_pipeline_total{pipeline="create_cluster", status="success/failure"}: 流水线执行总次数。
    - kubexm_pipeline_duration_seconds{pipeline="create_cluster"}: 流水线执行耗时。
    - kubexm_step_duration_seconds{step="InstallContainerd"}: 每个 Step 的执行耗时。
- **事件记录 (Event Recording)**:
  - 
  - KubeXM 引擎可以将关键事件（如 PipelineStarted, TaskSucceeded, StepFailed）记录到本地的 SQLite 数据库或发送到消息队列，以便于审计和事后分析。





### **KubeXM 终极部署矩阵与执行路径 v12.0 (终极运维篇)**

#### **Pipeline_RenewCA: 根证书 (CA) 续期流水线**

**核心挑战**:

1. 
2. **信任链断裂**: 新的 CA 是一个全新的信任根，旧客户端（如 kubelet, kubectl）不信任由新 CA 签发的新服务端证书（如 apiserver）。
3. **双向不信任**: 旧服务端（apiserver）也不信任由新 CA 签发的新客户端证书（如 kubelet）。
4. **操作的原子性**: 整个过程必须精心编排，任何一步的失败都可能导致集群永久性失联。

**设计策略**: 采用 **“双 CA 过渡”** 策略。在一段时间内，让集群同时信任新旧两个 CA，逐步替换所有证书，最后再废弃旧 CA。这是一个复杂但安全的方法。

**任务 (Task): Task_RenewCA**

1. 

2. **阶段一: 准备与分发新 CA (Prepare New CA)**

   1. 
   2. **Step_GenerateNewCA**:
      - 
      - **动作**: 在主控节点上，执行 cfssl gencert -initca ...，生成一套全新的 CA 证书和私钥，例如 ca-new.pem 和 ca-new-key.pem。
   3. **Step_CreateCombinedCA**:
      - 
      - **动作**: 将新旧 CA 的公钥合并成一个文件: cat ca-old.pem ca-new.pem > ca-combined.pem。这个合并后的 CA 文件是整个过渡期的信任关键。
   4. **Step_DistributeCombinedCA**:
      - 
      - **动作**: 将 ca-combined.pem 分发到集群中的**所有**节点，替换掉原来只包含旧 CA 的 ca.pem 文件。例如，更新 /etc/kubernetes/pki/ca.crt。
      - **注意**: 此时只替换 CA 公钥，不替换任何组件的证书和私钥。

3. **阶段二: 更新控制平面信任 (Update Control Plane Trust)**

   - 
   - **目标**: 让控制平面组件（apiserver, etcd）能够验证由**新 CA**签发的客户端证书。

   1. 
   2. **Step_UpdateApiServerTrust**:
      - 
      - **动作**: 修改 kube-apiserver 的启动参数（或静态 Pod 清单），将其 --client-ca-file 参数指向 ca-combined.pem。
   3. **Step_UpdateEtcdTrust** (仅当 etcd.type=kubexm):
      - 
      - **动作**: 修改 etcd.conf.yml，将其 trusted-ca-file 和 peer-trusted-ca-file 指向 ca-combined.pem。
   4. **Step_RestartControlPlane**:
      - 
      - **动作**: **逐个、滚动重启**所有 ETCD 节点和所有 Master 节点上的 kube-apiserver。每次重启后，都必须严格验证其健康状况。

   - 
   - **此时状态**: 控制平面现在信任新旧两个 CA。

4. **阶段三: 重新签发并部署所有证书 (Reissue All Certificates)**

   - 
   - **目标**: 使用**新的 CA 私钥** (ca-new-key.pem) 为集群中的每一个组件重新签发证书。

   1. 
   2. **Step_ReissueCertificates**:
      - 
      - **动作**: 在主控节点上，重新执行所有与证书生成相关的 Step，但这次 -ca 和 -ca-key 参数指向的是新 CA。这将生成一套全新的、由新 CA 签发的叶子证书。
   3. **Step_DistributeNewCertificates**:
      - 
      - **动作**: 将这些新生成的叶子证书（如 apiserver.pem, kubelet.pem 等）分发到所有对应节点的正确位置，**覆盖**掉旧的证书和私钥。
   4. **Step_DistributeNewKubeconfigs**:
      - 
      - **动作**: 由于 admin, kubelet, scheduler 等的证书已变，需要重新生成它们的 kubeconfig 文件并分发。

5. **阶段四: 全集群滚动重启 (Full Cluster Rolling Restart)**

   - 
   - **目标**: 让所有组件加载新的证书和私钥，并开始使用它们进行通信。

   1. 
   2. **Step_RollingRestartAll**:
      - 
      - **动作**: 按照 **ETCD -> kube-apiserver -> kube-controller-manager -> kube-scheduler -> kubelet -> kube-proxy** 的顺序，**逐个、滚动重启**集群中的每一个组件。这是一个极其缓慢且需要精密验证的过程。

6. **阶段五: 清理旧 CA (Deprecate Old CA)**

   - 
   - **目标**: 在确认集群使用新证书链稳定运行一段时间后（例如 24 小时），彻底移除对旧 CA 的信任。

   1. 
   2. **Step_ReplaceCombinedCAWithNew**:
      - 
      - **动作**: 将所有节点上的 ca-combined.pem 替换为只包含新 CA 内容的 ca-new.pem。
   3. **Step_FinalRollingRestart**:
      - 
      - **动作**: 再次执行一次全集群的滚动重启，让所有组件加载这个最终的、只包含新 CA 的信任根。
   4. **Step_DeleteOldCA**:
      - 
      - **动作**: 从主控节点安全地删除旧的 CA 私钥。

------



#### **其他高级运维场景**

**目标**: 允许用户修改 cluster.yaml 中的某些字段，并让 KubeXM 智能地应用这些变更，而不是强制重新创建集群。

**任务 (Task): Task_ApplyConfigChange**

- 
- **Step_DiffConfig**:
  - 
  - **动作**: KubeXM 会在 kubectl 的 annotation 中保存一份初次部署的 cluster.yaml 的 hash。执行 apply 时，会对比新旧配置的差异。
- **智能应用逻辑**:
  - 
  - **网络插件变更**: 例如从 calico 换到 cilium。这是一个高风险操作。KubeXM 会发出严重警告，并执行 Step_UninstallCNI (删除旧 CNI 的 DaemonSet 和配置)，然后执行 Step_InstallCNI (安装新 CNI)。这会导致全网短暂中断。
  - **kube-proxy 模式变更**: 从 iptables 换到 ipvs。KubeXM 会修改所有节点上的 kube-proxy 配置文件，并滚动重启所有 kube-proxy Pod/服务。
  - **特性门控 (Feature Gates) 变更**: KubeXM 会修改 kube-apiserver, kube-controller-manager, kubelet 等组件的启动参数，并滚动重启它们。
  - **不可变更项**: 对于 podSubnet, serviceSubnet 等核心 CIDR，KubeXM 会拒绝变更，并提示用户这些字段只能在创建集群时指定。

**目标**: 在不中断业务的前提下，安全地滚动升级集群所有节点的操作系统内核。

**任务 (Task): Task_KernelUpgrade**

- 
- **Step_RollingUpgradeNode** (逐个节点执行):
  1. 
  2. **Step_DrainNode**: kubectl drain [node_name] ...，安全驱逐业务 Pod。
  3. **Step_StopKubelet**: systemctl stop kubelet，避免在升级过程中干扰集群。
  4. **Step_RunPackageManager**: 执行 yum update kernel 或 apt-get dist-upgrade。
  5. **Step_RebootNode**: 执行 reboot。
  6. **Step_VerifyNodeAfterReboot**: 等待节点重启后，SSH 连接并确认新内核 (uname -r) 已被加载，且容器运行时 (containerd 等) 正常运行。
  7. **Step_StartKubelet**: systemctl start kubelet。
  8. **Step_UncordonNode**: kubectl uncordon [node_name]，让节点重新接受调度。

**目标**: 在线增加或减少 ETCD 集群的成员。

- 
- **扩容 (Add ETCD Member)**:
  1. 
  2. 在新节点上准备好环境和 ETCD 二进制文件。
  3. **Step_RunCommand** (在现有 ETCD 节点): etcdctl member add [new_node_name] --peer-urls=https://[new_node_ip]:2380。
  4. **Step_ConfigureAndStartNewMember**: 在新节点上，使用 existing 的 initial-cluster-state 来启动 etcd 服务，它会自动加入现有集群。
- **缩容 (Remove ETCD Member)**:
  1. 
  2. **Step_RunCommand** (在健康的 ETCD 节点): etcdctl member remove [member_id_to_remove]。
  3. **Step_StopAndCleanupNode**: 在被移除的节点上，停止 etcd 服务并清理其数据目录。


