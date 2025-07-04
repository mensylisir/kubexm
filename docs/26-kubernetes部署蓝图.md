#### **第零阶段：准备工作与蓝图审查**

1. **读懂蓝图**: 施工队首先完整地阅读并理解你给的 config.yaml 蓝图。他们会检查所有配置项的合法性和逻辑一致性，例如，etcd 节点的数量是否满足高可用要求（奇数个），网络 CIDR 是否冲突等。
2. **清点工具和材料**: 根据蓝图，施工队会自动生成一份详细的“物料清单”，确定需要下载哪些特定版本的二进制文件（如 kubelet, etcd, containerd）和容器镜像（如 coredns, calico-node, pause）。
3. **联系所有工地（节点）**: 施工队会使用你提供的 SSH 账号和密码/密钥，并发地连接到蓝图上列出的每一台机器，确保指挥通路畅通。
4. **工地勘测（预检）**: 在正式动工前，派人到每台机器上进行严格的自动化检查：
   - **系统环境**: 操作系统版本、内核版本是否满足要求？机器架构是 x86 还是 ARM？
   - **资源与网络**: 必要的端口（如 6443, 2379, 10250）是否被占用？主机名是否重复？防火墙和 SELinux 是否需要配置？
   - **环境一致性**: 所有机器的时间是否同步？（如果蓝图要求，就自动为它们配置好 NTP 时间服务器）。所有机器的时区是否已按要求设置？
   - **环境清理**: 机器上是否已存在旧的 Kubernetes、Docker 或相关配置文件？根据策略进行清理或给出警告。

#### **第一阶段：搭建基础设施**

1. **搭建集群的“大门”（控制平面负载均衡器）**: 根据蓝图 controlPlaneEndpoint 部分的规划，选择一种方式来确保控制平面的高可用：
   - **方案A (外部负载均衡 - Keepalived+HAProxy)**: 找来你指定的 loadbalancer 角色的机器，在上面安装并配置好 keepalived 和 haproxy，让它们共同提供一个稳定的虚拟 IP (VIP) 地址。这个 VIP 将是集群的统一入口。
   - **方案B (外部负载均衡 - Kube-VIP)**: 使用 kube-vip 技术，在 Master 节点上直接选举并创建一个 VIP，这是一种更云原生的轻量级方案。
   - **方案C (使用外部已有 LB)**: 如果你提供了外部 F5 或云厂商负载均衡器的地址，施工队会记下该地址，并在后续配置中使用它。
   - **方案D (无外部 LB，采用内部代理)**: 如果蓝图明确指出不使用外部负载均衡器，施工队会记下所有 Master 节点的 IP 地址列表，为后续在 Worker 节点上部署本地代理做准备。
2. **建立“大脑”的核心数据库 (Etcd)**: 集群的所有状态都存在 Etcd 中，必须高可用。
   - **方案A (二进制部署 - kubexm)**: 施工队将 etcd 的二进制程序亲自安装到你指定的 etcd 节点上，生成证书，配置成一个高可用的 TLS 加密集群，并创建 systemd 守护进程来管理它。
   - **方案B (Kubeadm 方式)**: 施工队会在 Master 节点上，准备好启动 etcd 的静态 Pod 配置文件。这样 kubelet 一启动，就会自动把 etcd 以容器的形式拉起来，由 Kubernetes 自己管理。
   - **方案C (使用外部 Etcd)**: 如果你提供了外部 Etcd 集群的访问地址和证书，施工队会验证其连通性，并将这些信息安全地保存下来，供后续的 Kubernetes 组件使用。
3. **安装容器“发动机” (Container Runtime)**: 在**每一台**机器上（包括 Master 和 Worker），安装并配置好容器运行时（如 containerd）。同时，会按照你的要求，配置好国内的镜像加速器地址、私有仓库的认证信息等。

#### **第二阶段：组装 Kubernetes 控制平面**

1. **启动“大脑”的各个部件 (Master 节点)**: 在所有 master 角色的节点上启动 Kubernetes 的核心组件。
   - **方案A (二进制部署 - kubexm)**: 施工队将 kube-apiserver, kube-scheduler, kube-controller-manager 的二进制程序分别安装好，生成所有必需的证书和配置文件，并创建 systemd 服务来启动它们。它们启动时会连接到第一阶段建好的 Etcd 集群。
   - **方案B (Kubeadm 方式)**: 施工队在 Master 节点上准备好这些核心组件的静态 Pod 配置文件，让 kubelet 自动将它们作为容器启动。

#### **第三阶段：扩大集群规模并打通网络**

1. **招募“工人”并建立内部通信代理 (Worker 节点)**: 让所有 worker 角色的节点加入到已经启动的 Master 集群中。
   - **对于有外部 LB 的方案 (A, B, C)**: Worker 节点的 kubelet 会被配置为直接连接那个唯一的 VIP 地址。
   - **对于无外部 LB 的方案 (D)**: 在每个 Worker 节点加入集群**之前**，施工队会先在它上面启动一个本地的 HAProxy 或 Nginx 服务。这个服务被配置为反向代理到所有 Master 节点的 API Server。Worker 节点的 kubelet 则被配置为连接自己本地的代理地址 (127.0.0.1:6443)，从而巧妙地实现了不依赖外部设备的高可用。
2. **铺设“交通网络” (CNI 网络插件)**: 集群节点已就位，但 Pod 间网络不通。施工队会根据你的选择 (calico, flannel, cilium, kube-ovn, hybridnet 等)，将对应的网络插件的配置文件和守护进程部署到集群中，建立起一个覆盖所有节点的 Pod 网络。如果蓝图要求，还会预先安装 multus 来支持多网卡。

#### **第四阶段：安装附加功能和服务**

1. **配置“域名解析服务” (DNS)**:
   - 安装 CoreDNS，让集群内的服务可以通过服务名互相访问。
   - 如果蓝图要求，还会安装 NodeLocalDNS，在每个节点上部署 DNS 缓存，以提高解析性能和稳定性。
2. **部署“共享仓库” (Storage)**: 根据你的选择 (openebs-localpv, longhorn, rook-ceph 等)，施工队会自动部署对应的存储 Operator 和组件，为集群提供动态创建持久化存储卷的能力。
3. **安装“对外服务暴露”工具 (Service LoadBalancer)**:
   - 如果你需要在裸金属环境里暴露 LoadBalancer 类型的 Service，施工队会安装 Metallb，让它能从你指定的 IP 地址池里为 Service 分配可访问的外部 IP。

#### **收尾阶段：交付与清理**

1. **生成“通行证” (Kubeconfig)**: 所有工作完成后，施工队会生成一个包含管理员凭证的 kubeconfig 文件。它会把这个文件安全地交给你，你用 kubectl 拿着它就能立即管理这个全新的集群。
2. **打扫战场**: 清理掉安装过程中在各个节点上产生的临时文件，最后向你报告：“老板，集群已按照您的蓝图完美建成，所有组件均已就绪，请验收！”