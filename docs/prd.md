## 1. 项目介绍
     1. 我的项目分为connector层，封装了ssh和local，其中ssh是远程操作，local是本地操作，都支持root和sudo，且支持密码和密钥
     2. connector之上我封装的runner层，runner层封装了一系列好用的函数
     3. 在上面是step层，step层通过调用runner，实现了很多原子性和幂等性的步骤,step是最小且无法在分割的单元
     4. 在上面是task层，task层通过组装step，将一组step封装为原子性和幂等性的小任务，不同的step组合可以形成不同的task。
     5. 在task层之上是module层，module层通过组装task，将一组task组装成module， 不同的task组合可以形成不同的module，
     6. 在上面是pipeline，pipeline层通过组装module，将一组module组装成pipeline，不同的module组合可以形成不同的pipeline
     7. 项目不同的step、task、module之间共享数据需要怎么共享？我现在设计了pipelinecache、modulecache、taskcache、modulecache，但是感觉不好用
     8. 项目使用有向无环图执行
     9. apis层是我所有的types定义
     10. 我希望所有工具函数都放在utils下面，下面我的各种工具函数散落在各处，很乱，为啥散乱各处呢？是因为经常报错循环导入cycle import，我没办法了，你看能解决吗
     11. 我的plan包和engine包你看看有啥问题吗
     12 cmd包和rest包是用来封装命令行和restful接口的

## 2. 项目功能
     9. 项目支持以下功能
        9.1 ./kubexm download --cluster=mycluster能将相关的所有文件下载到build/packages放好,要支持单架构单系统、多架构多系统的离线
        9.2 ./kubexm build --cluster=mycluster能生成相应的配置放到build目录相应的位置，配置属于节点，所以配置不应该脱离节点存在，必须放在节点之下，比如mycluster/node1/ca.pem
        9.3 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubeadm，单master集群，这种情况下loadbalancer的enable必须为false
        9.4 要能部署集群的部署类型为kubexm，etcd的部署类型为kubexm，单master集群，这种情况下loadbalancer的enable必须为false
        9.5 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubexm，单master集群，这种情况下loadbalancer的enable必须为false
        9.6 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubeadm，多master集群，且loadbalancer的enable为false
        9.7 要能部署集群的部署类型为kubexm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为false
        9.8 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为false
        9.9 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubeadm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为internal，type为haproxy，此时为静态pod启动
        9.10 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为internal，type为haproxy，此时为静态pod启动
        9.11 要能部署集群的部署类型为kubexm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为internal，type为haproxy，此时为二进制启动
        9.12 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubeadm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为internal，type为nginx，此时为静态pod启动
        9.13 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为internal，type为nginx，此时为静态pod启动
        9.14 要能部署集群的部署类型为kubexm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为internal，type为ngnx，此时为二进制启动
        9.15 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubeadm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为kube-vip
        9.16 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为kube-vip
        9.17 要能部署集群的部署类型为kubexm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为kube-vip
        9.18 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubeadm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为external, type为kubexm-kh
        9.19 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubeadm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为external, type为kubexm-kn
        9.20 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为external, type为kubexm-kh
        9.21 要能部署集群的部署类型为kubeadm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为external, type为kubexm-kn
        9.22 要能部署集群的部署类型为kubexm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为external, type为kubexm-kh
        9.23 要能部署集群的部署类型为kubexm，etcd的部署类型为kubexm，多master集群，且loadbalancer的enable为true,且loadbalancer的mode为external, type为kubexm-kn

## 3. 核心架构设计原则

### 3.1 配置与逻辑分离
- **conf/**：所有配置集中管理，支持多环境隔离
- **lib/**：纯逻辑库，无状态，原子函数
- **templates/**：配置模板渲染

### 3.2 离线优先设计
- **packages/**：包含所有离线资源
- **kubexm-dist/**：完整的离线部署包
- **三阶段交付**：Stage 0(下载) → Stage 1(构建) → Stage 2(部署)
- **使用Skopeo下载镜像**

### 3.3 模块化架构
- **lib/os/**：操作系统适配层
- **lib/runtime/**：容器运行时适配
- **lib/loadbalancer/**：多种LB方案支持
- **lib/components/**：K8s组件管理

### 3.4 企业级工程标准
- **防御性编程**：所有操作都有错误检查
- **幂等性**：重复执行结果一致
- **模块化**：功能解耦，易于维护

## 4. 核心功能特性

### 4.1 双模式部署
- **kubexm(Binary)模式**：最小化依赖，极致性能
- **Kubeadm模式**：标准化流程，官方兼容

### 4.2 负载均衡方案
- **External LB**：Keepalived + HAProxy/Nginx
- **Internal LB**：Worker节点轻量级代理
- **Kube-VIP**：现代化VIP方案

### 4.3 网络插件支持
- **CNI插件**：Calico, Flannel, Cilium, Kube-Router
- **NodeLocal DNSCache**：DNS性能优化

### 4.4 存储方案
- **Local Path Provisioner**：本地存储
- **OpenEBS Local PV**：生产级本地存储
- **Rook Ceph**：分布式存储
- **Longhorn**：云原生存储

### 4.5 安全体系
- **PKI管理**：完整的证书生命周期
- **自动续期**：systemd timer自动续期
- **备份恢复**：Etcd自动备份

## 5. 三阶段交付流程

### Stage 0: 资源准备 (kubexm download)
```bash
./kubexm download --os centos7,ubuntu22 --k8s v1.32.4
./kubexm download cluster --name=mycluster
```
- 从互联网下载所需资源
- 生成kubexm-dist目录
- 计算SHA256校验和

### Stage 1: 配置构建 (kubexm build)
```bash
./kubexm build cluster --name=mycluster
```
- 解析inventory文件
- 生成证书和配置文件
- 渲染模板

### Stage 2: 安装集群 (kubexm install)
```bash
./kubexm install cluster --name=mycluster
```
- 并发分发资源
- 执行节点初始化
- 启动服务

### 安装集群 (kubexm create cluster)

```
./kubexm create cluster --name=mycluster
```
### 删除集群
```
kubexm delete cluster --name=mycluster
```

### 升级集群
```
kubexm upgrade cluster --name=mycluster
```

### 升级etcd
```
kubexm upgrade etcd --name=mycluster
```


### kubernetes证书更新
```
kubexm update k8s-cert --name=mycluster
```

### etcd证书更新
```
kubexm update etcd-cert --name=mycluster
```

### kubernetes CA更新
```
kubexm update k8s-ca --name=mycluster
```

### etcd CA更新
```
kubexm update etcd-ca --name=mycluster
```



- 自动执行download下载资源
- 自动读取conf下mycluster相关配置，解析配置文件渲染模板，生成**部署配置文件组织结构**
- 自动部署集群

## 6. 容器运行时

**这是最容易被忽视的痛点。大多数教程教你用** **yum install docker-ce**，离线时是个大坑。

### **Containerd**:

* **下载官方 release 包（包含** **runc**, **ctr**, **containerd**）。

* **解压到** **/usr/local/bin**。

* **手动下发** **config.toml** **和** **containerd.service**。

  

### **Docker**:

* **下载** **docker-24.x.tgz** **静态二进制包。**

* **解压即用，不依赖系统库。**

* **手动配置** **daemon.json** **和** **docker.service**。

### **CRI-O / Podman**:

  * **同样优先寻找静态编译版本。实在不行才使用包管理器部署**

## 7. kubernetes部署模式
###  Kubernetes 部署类型 (Deployment Types)

**kubexm** **支持两种核心的 Kubernetes 部署方式，满足从完全定制化到标准化的所有需求：**

* **kubexm** **(Binary / Hard Way)**:

  * **描述**: 全二进制文件部署，所有组件（apiserver, controller-manager, scheduler, kubelet, proxy）通过 Systemd 托管。
* **适用场景**: 对 K8s 内部机制有极致掌控需求的场景，或者是通过了安全审计需要特定配置的场景。证书有效期默认配置为 10 年，免去频繁续期烦恼。
* **kubeadm** **(Standard)**:

  * **描述**: 使用官方 **kubeadm** **工具进行集群初始化和节点加入。**
* **适用场景**: 标准化生产环境，兼容社区生态。**kubexm** **会自动处理** **kubeadm** **默认证书 1 年过期的问题，通过 Systemd Timer 自动续期。**


## 8. etcd部署模式
###  Etcd 部署模式 (Etcd Topology)

**Etcd 是集群的心脏，**kubexm **提供了灵活的部署选项：**

* **部署类型 (Type)**:

  * **kubexm** **(Binary)**: 推荐模式。Etcd 进程通过 Systemd 独立管理，不依赖容器运行时，更稳定，升级更可控。
* **kubeadm** **(Static Pod)**: 由 Kubelet 托管的静态 Pod 运行 Etcd。维护简单，适合标准 Kubeadm 用户。

## 9. 负载均衡器部署模式
###  负载均衡方案 (Load Balancer Strategies)

**为了保证控制平面（Control Plane）的高可用，**kubexm **提供了业内最全的 LB 方案，包括对现代化的** **kube-vip** **的支持：**

* **类型 A: External (外部 LB)**

  * **描述**: 使用独立于 K8s 节点的机器作为 LB 节点，通过 VIP (虚拟 IP) 暴露服务。
  * **模式 1:** **kubexm_kh**: Keepalived (VIP漂移) + HAProxy (高性能四层代理)。
  * **模式 2:** **kubexm_kn**: Keepalived (VIP漂移) + Nginx (Stream模块四层代理)。
* **类型 B: Internal (内部 LB)**

  * **描述**: 没有独立 LB 节点。在每个 Worker 节点上运行一个轻量级代理，监听 **127.0.0.1:6443**，转发流量到所有 Master 节点。
  * **模式 1:** **haproxy** **(Binary/StaticPod)**: 在 Worker 上运行 HAProxy。
  * **模式 2:** **nginx** **(Binary/StaticPod)**: 在 Worker 上运行 Nginx。
* **类型 C: Kube-VIP (现代化方案)**

  * **描述**: 使用 **kube-vip** **以 Static Pod 或 DaemonSet 方式运行在 Master 节点上，通过 ARP/BGP 广播 VIP。不需要额外部署 Keepalived/HAProxy。**
  * **模式**: **kube-vip** **(ARP 模式或 BGP 模式)。**

## 10.网络与插件
### 9.1 网络与插件 (Networking & Addons)

* **CNI 插件**: 支持 Calico, Flannel, Cilium, Kube-Router。
* **DNS 优化**: **NodeLocal DNSCache** **(默认支持并推荐开启)，在每个节点上缓存 DNS 记录，大幅降低 CoreDNS 压力并提高解析速度。**
* **CSI 存储**:

  * **Rancher Local Path Provisioner**: 极简本地存储，适合测试。
* **OpenEBS Local PV**: 生产级本地存储。
* **NFS Subdir External Provisioner**: 基于 NFS 的动态供给。
* **Rook Ceph**: 分布式块/文件/对象存储。
* **Longhorn**: 易用的云原生分布式存储。

## 11. 交付模式
###  交付模式 (Delivery Modes)

* **Online (在线)**: 自动检测网络，从配置的上游源（Github/DockerHub/Google）下载最新包。
* **Offline (离线)**:

  * **全量自包含**: **kubexm** **的** **packages/** **目录包含部署所需的一切：K8s/Etcd 二进制、容器镜像、OS 依赖包 (rpm/deb)。**
* **系统适配**: 针对不同 OS (CentOS 7/8, Ubuntu 20/22, Kylin 等) 提供对应的依赖包集合，解决离线环境装不上 **conntrack**, **ipset** **的痛点。**

## 12. 证书与安全
- **PKI证书体系**：CA生成、证书分发、证书轮换
- **零停机证书管理**：不影响生产服务的证书更新
- **安全通信**：SSH密钥管理、TLS证书验证
- **访问控制**：基于角色的权限管理（RBAC）

## 13. 生命周期管理
- **节点扩缩容**：Master节点扩容、Worker节点批量添加
- **集群升级**：支持Kubernetes版本升级和组件更新
- **维护操作**：备份恢复、性能调优、故障诊断

## 14. 高级功能范围
#### 监控与诊断
- **部署状态监控**：实时进度显示和健康检查
- **故障自动诊断**：日志聚合分析、错误根因分析
- **性能监控**：集群资源使用情况监控
- **告警通知**：系统异常和关键事件通知

####  运维自动化
- **自动化运维**：定时任务、周期性检查
- **备份策略**：Etcd数据备份、应用状态备份
- **灾难恢复**：一键恢复、快速回滚机制
- **配置管理**：配置模板、版本控制、批量部署

#### 多云与混合云
- **多云支持**：AWS、Azure、GCP公有云部署
- **混合云管理**：统一管理本地和云上Kubernetes集群
- **多区域部署**：跨地域集群的集中管理
- **云原生集成**：集成Prometheus、Grafana、Istio等

####  高级安全特性
- **网络策略管理**：Kubernetes NetworkPolicy配置
- **安全扫描**：集群安全漏洞扫描和加固
- **合规性检查**：满足行业标准和法规要求
- **审计日志**：完整的操作审计和合规报告

####  明确排除范围
- **应用级管理**：不包含应用部署、服务网格、微服务治理
- **开发工具链**：不包含IDE集成、开发环境搭建
- **培训服务**：不包含用户培训和技术支持服务
- **云服务托管**：不提供SaaS化的Kubernetes托管服务


## 15. 部署架构
**目标**：实现100%在线/离线的三阶段部署流程

**组件架构**：
1. **资源准备引擎**
   - 二进制文件下载器（支持多版本、多架构）
   - 容器镜像拉取器（支持本地镜像仓库）
   - 系统依赖包收集器（解决rpm/deb依赖地狱）
   - 完整性验证器（SHA256校验和验证）

2. **配置构建引擎**
   - PKI证书生成器（CA、Server、Client证书）
   - 配置文件渲染器（Jinja2模板引擎）
   - 多架构适配器（x86_64/ARM64）
   - 配置验证器（参数检查和冲突检测）

3. **分发部署引擎**
   - 并发分发器（高并发文件同步）
   - 进度追踪器（实时部署进度显示）
   - 错误恢复器（自动重试和回滚机制）
   - 状态验证器（健康检查和状态确认）

#### 双模式部署架构
**目标**：提供Binary和Kubeadm双模式智能选择

**Binary模式架构**：
- 组件管理：全组件Systemd托管
- 配置方式：Dropin覆盖式配置
- 证书策略：10年有效期证书，减少维护成本
- 定制化级别：极致定制化控制

**Kubeadm模式架构**：
- 标准支持：完全符合官方kubeadm标准
- 配置管理：完整配置文件支持
- 证书策略：自动续期机制
- 升级支持：原生Kubernetes升级路径

## 16. 高级功能及其配置
### 自动续期证书
**autoRenewCerts: true时**

```
root@tofu-vm1:~# cat /etc/systemd/system/k8s-certs-renew.timer 
[Unit]
Description=Timer to renew K8S control plane certificates
[Timer]
OnCalendar=Mon *-*-* 03:00:00
Unit=k8s-certs-renew.service
[Install]
WantedBy=multi-user.target
root@tofu-vm1:~# cat /etc/systemd/system/k8s-certs-renew.service 
[Unit]
Description=Renew K8S control plane certificates
[Service]
Type=oneshot
ExecStart=/usr/local/bin/kube-scripts/k8s-certs-renew.sh
root@tofu-vm1:~# cat /usr/local/bin/kube-scripts/k8s-certs-renew.sh
#!/bin/bash
kubeadmCerts='/usr/local/bin/kubeadm certs'
getCertValidDays() {
  local earliestExpireDate; earliestExpireDate=$(${kubeadmCerts} check-expiration | grep -o "[A-Za-z]\{3,4\}\s\w\w,\s[0-9]\{4,\}\s\w*:\w*\s\w*\s*" | xargs -I {} date -d {} +%s | sort | head -n 1)
  local today; today="$(date +%s)"
  echo -n $(( ($earliestExpireDate - $today) / (24 * 60 * 60) ))
}
echo "## Expiration before renewal ##"
${kubeadmCerts} check-expiration
if [ $(getCertValidDays) -lt 30 ]; then
  echo "## Renewing certificates managed by kubeadm ##"
  ${kubeadmCerts} renew all
  echo "## Restarting control plane pods managed by kubeadm ##"
  $(which crictl | grep crictl) pods --namespace kube-system --name 'kube-scheduler-*|kube-controller-manager-*|kube-apiserver-*|etcd-*' -q | /usr/bin/xargs $(which crictl | grep crictl) rmp -f
  echo "## Updating /root/.kube/config ##"
  cp /etc/kubernetes/admin.conf /root/.kube/config
fi
echo "## Waiting for apiserver to be up again ##"
until printf "" 2>>/dev/null >>/dev/tcp/127.0.0.1/6443; do sleep 1; done
echo "## Expiration after renewal ##"
${kubeadmCerts} check-expiration
root@tofu-vm1:~# 

```
### 自动备份etcd
**autoBackupEtcd: true时**
```
root@tofu-vm1:~# cat /etc/systemd/system/backup-etcd.timer 
[Unit]
Description=Timer to backup ETCD
[Timer]
OnCalendar=*-*-* 02:00:00
Unit=backup-etcd.service
[Install]
WantedBy=multi-user.target
root@tofu-vm1:~# cat /etc/systemd/system/backup-etcd.service 
[Unit]
Description=Backup ETCD
[Service]
Type=oneshot
ExecStart=/usr/local/bin/kube-scripts/etcd-backup.sh
root@tofu-vm1:~# cat /usr/local/bin/kube-scripts/etcd-backup.sh
#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ETCDCTL_PATH='/usr/local/bin/etcdctl'
ENDPOINTS='https://10.200.200.150:2379'
ETCD_DATA_DIR="/var/lib/etcd"
BACKUP_DIR="/var/backups/kube_etcd/etcd-$(date +%Y-%m-%d-%H-%M-%S)"
KEEPBACKUPNUMBER='6'
ETCDBACKUPSCIPT='/usr/local/bin/kube-scripts'

ETCDCTL_CERT="/etc/ssl/etcd/ssl/admin-tofu-vm1.pem"
ETCDCTL_KEY="/etc/ssl/etcd/ssl/admin-tofu-vm1-key.pem"
ETCDCTL_CA_FILE="/etc/ssl/etcd/ssl/ca.pem"

[ ! -d $BACKUP_DIR ] && mkdir -p $BACKUP_DIR

export ETCDCTL_API=2;$ETCDCTL_PATH backup --data-dir $ETCD_DATA_DIR --backup-dir $BACKUP_DIR

sleep 3

{
export ETCDCTL_API=3;$ETCDCTL_PATH --endpoints="$ENDPOINTS" snapshot save $BACKUP_DIR/snapshot.db \
                                   --cacert="$ETCDCTL_CA_FILE" \
                                   --cert="$ETCDCTL_CERT" \
                                   --key="$ETCDCTL_KEY"
} > /dev/null 

sleep 3

cd $BACKUP_DIR/../ && ls -lt |awk '{if(NR > '$KEEPBACKUPNUMBER'){print "rm -rf "$9}}'|sh

root@tofu-vm1:~# 

```
### kubeadm模式
#### 初始化第一台master的kubeadm-config.yaml
```
root@tofu-vm1:~# cat /etc/kubernetes/kubeadm-config.yaml 
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
etcd:
  external:
    endpoints:
    - https://10.200.200.150:2379
    caFile: /etc/ssl/etcd/ssl/ca.pem
    certFile: /etc/ssl/etcd/ssl/node-tofu-vm1.pem
    keyFile: /etc/ssl/etcd/ssl/node-tofu-vm1-key.pem
dns:
  imageRepository: dockerhub.kubekey.local/kubesphereio
  imageTag: 1.9.3
imageRepository: dockerhub.kubekey.local/kubesphereio
kubernetesVersion: v1.32.4
certificatesDir: /etc/kubernetes/pki
clusterName: cluster.local
controlPlaneEndpoint: lb.cars.local:6443
networking:
  dnsDomain: cluster.local
  podSubnet: 10.233.64.0/18
  serviceSubnet: 10.233.0.0/18
apiServer:
  extraArgs:
    bind-address: 0.0.0.0
    feature-gates: RotateKubeletServerCertificate=true
  certSANs:
    - "kubernetes"
    - "kubernetes.default"
    - "kubernetes.default.svc"
    - "kubernetes.default.svc.cluster.local"
    - "localhost"
    - "127.0.0.1"
    - "lb.cars.local"
    - "10.200.200.150"
    - "tofu-vm12"
    - "tofu-vm12.cluster.local"
    - "10.200.200.190"
    - "tofu-vm1"
    - "tofu-vm1.cluster.local"
    - "tofu-vm2"
    - "tofu-vm2.cluster.local"
    - "10.200.200.143"
    - "tofu-vm3"
    - "tofu-vm3.cluster.local"
    - "10.200.200.103"
    - "10.233.0.1"
controllerManager:
  extraArgs:
    node-cidr-mask-size: "24"
    bind-address: 0.0.0.0
    cluster-signing-duration: 87600h
    feature-gates: RotateKubeletServerCertificate=true
  extraVolumes:
  - name: host-time
    hostPath: /etc/localtime
    mountPath: /etc/localtime
    readOnly: true
scheduler:
  extraArgs:
    bind-address: 0.0.0.0
    feature-gates: RotateKubeletServerCertificate=true

---
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
localAPIEndpoint:
  advertiseAddress: 10.200.200.150
  bindPort: 6443
nodeRegistration:
  criSocket: unix:///run/containerd/containerd.sock
  kubeletExtraArgs:
    cgroup-driver: systemd
---
apiVersion: kubeproxy.config.k8s.io/v1alpha1
kind: KubeProxyConfiguration
clusterCIDR: 10.233.64.0/18
iptables:
    masqueradeAll: false
    masqueradeBit: 14
    minSyncPeriod: 0s
    syncPeriod: 30s
mode: ipvs
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cgroupDriver: systemd
clusterDNS:
    - 169.254.25.10
clusterDomain: cluster.local
containerLogMaxFiles: 3
containerLogMaxSize: 5Mi
evictionHard:
    memory.available: 5%
    pid.available: 10%
evictionMaxPodGracePeriod: 120
evictionPressureTransitionPeriod: 30s
evictionSoft:
    memory.available: 10%
evictionSoftGracePeriod:
    memory.available: 2m
featureGates:
    RotateKubeletServerCertificate: true
kubeReserved:
    cpu: 200m
    memory: 250Mi
maxPods: 110
podPidsLimit: 10000
rotateCertificates: true
systemReserved:
    cpu: 200m
    memory: 250Mi
```
#### 添加master的kubeadm-config.yaml
```
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: JoinConfiguration
discovery:
  bootstrapToken:
    apiServerEndpoint: lb.cars.local:6443
    token: "xo6iyb.aufhy1nmbz0jsf3h"
    unsafeSkipCAVerification: true
  tlsBootstrapToken: "xo6iyb.aufhy1nmbz0jsf3h"
controlPlane:
  localAPIEndpoint:
    advertiseAddress: 10.200.200.174
    bindPort: 6443
  certificateKey: 1f491189941d287c0049bc20996f3b1e828095792e6b5d6785a531082cc2bfc0
nodeRegistration:
  criSocket: unix:///run/containerd/containerd.sock
  kubeletExtraArgs:
    cgroup-driver: systemd

```
#### 添加worker的kubeadm-config.yaml
```
root@tofu-vm8:~# cat /etc/kubernetes/kubeadm-config.yaml 
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: JoinConfiguration
discovery:
  bootstrapToken:
    apiServerEndpoint: lb.cars.local:6443
    token: "xo6iyb.aufhy1nmbz0jsf3h"
    unsafeSkipCAVerification: true
  tlsBootstrapToken: "xo6iyb.aufhy1nmbz0jsf3h"
nodeRegistration:
  criSocket: unix:///run/containerd/containerd.sock
  kubeletExtraArgs:
    cgroup-driver: systemd
root@tofu-vm8:~# 

```
#### 初始化节点的脚本
```
root@tofu-vm1:~# cat /usr/local/bin/kube-scripts/initOS.sh 
#!/usr/bin/env bash

# Copyright 2020 The KubeSphere Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

swapoff -a
sed -i /^[^#]*swap*/s/^/\#/g /etc/fstab

# See https://github.com/kubernetes/website/issues/14457
if [ -f /etc/selinux/config ]; then
  sed -ri 's/SELINUX=enforcing/SELINUX=disabled/' /etc/selinux/config
fi
# for ubuntu: sudo apt install selinux-utils
# for centos: yum install selinux-policy
if command -v setenforce &> /dev/null
then
  setenforce 0
  getenforce
fi

echo 'net.ipv4.ip_forward = 1' >> /etc/sysctl.conf
echo 'net.bridge.bridge-nf-call-arptables = 1' >> /etc/sysctl.conf
echo 'net.bridge.bridge-nf-call-ip6tables = 1' >> /etc/sysctl.conf
echo 'net.bridge.bridge-nf-call-iptables = 1' >> /etc/sysctl.conf
echo 'net.ipv4.ip_local_reserved_ports = 30000-32767' >> /etc/sysctl.conf
echo 'net.core.netdev_max_backlog = 65535' >> /etc/sysctl.conf
echo 'net.core.rmem_max = 33554432' >> /etc/sysctl.conf
echo 'net.core.wmem_max = 33554432' >> /etc/sysctl.conf
echo 'net.core.somaxconn = 32768' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog = 1048576' >> /etc/sysctl.conf
echo 'net.ipv4.neigh.default.gc_thresh1 = 512' >> /etc/sysctl.conf
echo 'net.ipv4.neigh.default.gc_thresh2 = 2048' >> /etc/sysctl.conf
echo 'net.ipv4.neigh.default.gc_thresh3 = 4096' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_retries2 = 15' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_tw_buckets = 1048576' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_orphans = 65535' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_keepalive_time = 600' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_keepalive_intvl = 30' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_keepalive_probes = 10' >> /etc/sysctl.conf
echo 'net.ipv4.udp_rmem_min = 131072' >> /etc/sysctl.conf
echo 'net.ipv4.udp_wmem_min = 131072' >> /etc/sysctl.conf
echo 'net.ipv4.conf.all.rp_filter = 0' >> /etc/sysctl.conf
echo 'net.ipv4.conf.default.rp_filter = 0' >> /etc/sysctl.conf
echo 'net.ipv4.conf.all.arp_accept = 1' >> /etc/sysctl.conf
echo 'net.ipv4.conf.default.arp_accept = 1' >> /etc/sysctl.conf
echo 'net.ipv4.conf.all.arp_ignore = 1' >> /etc/sysctl.conf
echo 'net.ipv4.conf.default.arp_ignore = 1' >> /etc/sysctl.conf
echo 'vm.max_map_count = 262144' >> /etc/sysctl.conf
echo 'vm.swappiness = 0' >> /etc/sysctl.conf
echo 'vm.overcommit_memory = 1' >> /etc/sysctl.conf
echo 'fs.inotify.max_user_instances = 524288' >> /etc/sysctl.conf
echo 'fs.inotify.max_user_watches = 10240001' >> /etc/sysctl.conf
echo 'fs.pipe-max-size = 4194304' >> /etc/sysctl.conf
echo 'fs.aio-max-nr = 262144' >> /etc/sysctl.conf
echo 'kernel.pid_max = 65535' >> /etc/sysctl.conf
echo 'kernel.watchdog_thresh = 5' >> /etc/sysctl.conf
echo 'kernel.hung_task_timeout_secs = 5' >> /etc/sysctl.conf

#See https://help.aliyun.com/document_detail/118806.html#uicontrol-e50-ddj-w0y
sed -r -i "s@#{0,}?net.ipv4.tcp_tw_recycle ?= ?(0|1|2)@net.ipv4.tcp_tw_recycle = 0@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_tw_reuse ?= ?(0|1)@net.ipv4.tcp_tw_reuse = 0@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.conf.all.rp_filter ?= ?(0|1|2)@net.ipv4.conf.all.rp_filter = 0@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.conf.default.rp_filter ?= ?(0|1|2)@net.ipv4.conf.default.rp_filter = 0@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.ip_forward ?= ?(0|1)@net.ipv4.ip_forward = 1@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.bridge.bridge-nf-call-arptables ?= ?(0|1)@net.bridge.bridge-nf-call-arptables = 1@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.bridge.bridge-nf-call-ip6tables ?= ?(0|1)@net.bridge.bridge-nf-call-ip6tables = 1@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.bridge.bridge-nf-call-iptables ?= ?(0|1)@net.bridge.bridge-nf-call-iptables = 1@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.ip_local_reserved_ports ?= ?([0-9]{1,}-{0,1},{0,1}){1,}@net.ipv4.ip_local_reserved_ports = 30000-32767@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?vm.max_map_count ?= ?([0-9]{1,})@vm.max_map_count = 262144@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?vm.swappiness ?= ?([0-9]{1,})@vm.swappiness = 0@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?fs.inotify.max_user_instances ?= ?([0-9]{1,})@fs.inotify.max_user_instances = 524288@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?kernel.pid_max ?= ?([0-9]{1,})@kernel.pid_max = 65535@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?vm.overcommit_memory ?= ?(0|1|2)@vm.overcommit_memory = 0@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?fs.inotify.max_user_watches ?= ?([0-9]{1,})@fs.inotify.max_user_watches = 524288@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?fs.pipe-max-size ?= ?([0-9]{1,})@fs.pipe-max-size = 4194304@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.core.netdev_max_backlog ?= ?([0-9]{1,})@net.core.netdev_max_backlog = 65535@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.core.rmem_max ?= ?([0-9]{1,})@net.core.rmem_max = 33554432@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.core.wmem_max ?= ?([0-9]{1,})@net.core.wmem_max = 33554432@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_max_syn_backlog ?= ?([0-9]{1,})@net.ipv4.tcp_max_syn_backlog = 1048576@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.neigh.default.gc_thresh1 ?= ?([0-9]{1,})@net.ipv4.neigh.default.gc_thresh1 = 512@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.neigh.default.gc_thresh2 ?= ?([0-9]{1,})@net.ipv4.neigh.default.gc_thresh2 = 2048@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.neigh.default.gc_thresh3 ?= ?([0-9]{1,})@net.ipv4.neigh.default.gc_thresh3 = 4096@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.core.somaxconn ?= ?([0-9]{1,})@net.core.somaxconn = 32768@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.conf.eth0.arp_accept ?= ?(0|1)@net.ipv4.conf.eth0.arp_accept = 1@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?fs.aio-max-nr ?= ?([0-9]{1,})@fs.aio-max-nr = 262144@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_retries2 ?= ?([0-9]{1,})@net.ipv4.tcp_retries2 = 15@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_max_tw_buckets ?= ?([0-9]{1,})@net.ipv4.tcp_max_tw_buckets = 1048576@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_max_orphans ?= ?([0-9]{1,})@net.ipv4.tcp_max_orphans = 65535@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_keepalive_time ?= ?([0-9]{1,})@net.ipv4.tcp_keepalive_time = 600@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_keepalive_intvl ?= ?([0-9]{1,})@net.ipv4.tcp_keepalive_intvl = 30@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.tcp_keepalive_probes ?= ?([0-9]{1,})@net.ipv4.tcp_keepalive_probes = 10@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.udp_rmem_min ?= ?([0-9]{1,})@net.ipv4.udp_rmem_min = 131072@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.udp_wmem_min ?= ?([0-9]{1,})@net.ipv4.udp_wmem_min = 131072@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.conf.all.arp_ignore ?= ??(0|1|2)@net.ipv4.conf.all.arp_ignore = 1@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?net.ipv4.conf.default.arp_ignore ?= ??(0|1|2)@net.ipv4.conf.default.arp_ignore = 1@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?kernel.watchdog_thresh ?= ?([0-9]{1,})@kernel.watchdog_thresh = 5@g" /etc/sysctl.conf
sed -r -i "s@#{0,}?kernel.hung_task_timeout_secs ?= ?([0-9]{1,})@kernel.hung_task_timeout_secs = 5@g" /etc/sysctl.conf


tmpfile="$$.tmp"
awk ' !x[$0]++{print > "'$tmpfile'"}' /etc/sysctl.conf
mv $tmpfile /etc/sysctl.conf

# ulimit
echo "* soft nofile 1048576" >> /etc/security/limits.conf
echo "* hard nofile 1048576" >> /etc/security/limits.conf
echo "* soft nproc 65536" >> /etc/security/limits.conf
echo "* hard nproc 65536" >> /etc/security/limits.conf
echo "* soft memlock unlimited" >> /etc/security/limits.conf
echo "* hard memlock unlimited" >> /etc/security/limits.conf

sed -r -i  "s@#{0,}?\* soft nofile ?([0-9]{1,})@\* soft nofile 1048576@g" /etc/security/limits.conf
sed -r -i  "s@#{0,}?\* hard nofile ?([0-9]{1,})@\* hard nofile 1048576@g" /etc/security/limits.conf
sed -r -i  "s@#{0,}?\* soft nproc ?([0-9]{1,})@\* soft nproc 65536@g" /etc/security/limits.conf
sed -r -i  "s@#{0,}?\* hard nproc ?([0-9]{1,})@\* hard nproc 65536@g" /etc/security/limits.conf
sed -r -i  "s@#{0,}?\* soft memlock ?([0-9]{1,}([TGKM]B){0,1}|unlimited)@\* soft memlock unlimited@g" /etc/security/limits.conf
sed -r -i  "s@#{0,}?\* hard memlock ?([0-9]{1,}([TGKM]B){0,1}|unlimited)@\* hard memlock unlimited@g" /etc/security/limits.conf

tmpfile="$$.tmp"
awk ' !x[$0]++{print > "'$tmpfile'"}' /etc/security/limits.conf
mv $tmpfile /etc/security/limits.conf

# Check if firewalld service exists and is running
systemctl status firewalld 1>/dev/null 2>/dev/null
if [ $? -eq 0 ]; then
    # Firewall service exists and is running, stop and disable it
    systemctl stop firewalld 1>/dev/null 2>/dev/null
    systemctl disable firewalld 1>/dev/null 2>/dev/null
fi
# Check if ufw service exists and is running
systemctl status ufw 1>/dev/null 2>/dev/null
if [ $? -eq 0 ]; then
    # ufw service exists and is running, stop and disable it
    systemctl stop ufw 1>/dev/null 2>/dev/null
    systemctl disable ufw 1>/dev/null 2>/dev/null
fi

modinfo br_netfilter > /dev/null 2>&1
if [ $? -eq 0 ]; then
   modprobe br_netfilter
   mkdir -p /etc/modules-load.d
   echo 'br_netfilter' > /etc/modules-load.d/kubekey-br_netfilter.conf
fi

modinfo overlay > /dev/null 2>&1
if [ $? -eq 0 ]; then
   modprobe overlay
   echo 'overlay' >> /etc/modules-load.d/kubekey-br_netfilter.conf
fi

modprobe ip_vs
modprobe ip_vs_rr
modprobe ip_vs_wrr
modprobe ip_vs_sh

cat > /etc/modules-load.d/kube_proxy-ipvs.conf << EOF
ip_vs
ip_vs_rr
ip_vs_wrr
ip_vs_sh
EOF

modprobe nf_conntrack_ipv4 1>/dev/null 2>/dev/null
if [ $? -eq 0 ]; then
   echo 'nf_conntrack_ipv4' > /etc/modules-load.d/kube_proxy-ipvs.conf
else
   modprobe nf_conntrack
   echo 'nf_conntrack' > /etc/modules-load.d/kube_proxy-ipvs.conf
fi
sysctl -p

sed -i ':a;$!{N;ba};s@# kubekey hosts BEGIN.*# kubekey hosts END@@' /etc/hosts
sed -i '/^$/N;/\n$/N;//D' /etc/hosts

cat >>/etc/hosts<<EOF
# kubekey hosts BEGIN
10.200.200.150  tofu-vm1.cluster.local tofu-vm1
10.200.200.190  tofu-vm12.cluster.local tofu-vm12
10.200.200.143  tofu-vm2.cluster.local tofu-vm2
10.200.200.103  tofu-vm3.cluster.local tofu-vm3
10.200.200.190  dockerhub.kubekey.local
10.200.200.150  lb.cars.local
# kubekey hosts END
EOF

sync
# echo 3 > /proc/sys/vm/drop_caches

# Make sure the iptables utility doesn't use the nftables backend.
update-alternatives --set iptables /usr/sbin/iptables-legacy >/dev/null 2>&1 || true
update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy >/dev/null 2>&1 || true
update-alternatives --set arptables /usr/sbin/arptables-legacy >/dev/null 2>&1 || true
update-alternatives --set ebtables /usr/sbin/ebtables-legacy >/dev/null 2>&1 || true

root@tofu-vm1:~# 

```

#### kubelet的dropin
```
root@tofu-vm1:~# cat /etc/systemd/system/kubelet.service.d/10-kubeadm.conf 
# Note: This dropin only works with kubeadm and kubelet v1.11+
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
# This is a file that "kubeadm init" and "kubeadm join" generate at runtime, populating the KUBELET_KUBEADM_ARGS variable dynamically
EnvironmentFile=-/var/lib/kubelet/kubeadm-flags.env
# This is a file that the user can use for overrides of the kubelet args as a last resort. Preferably, the user should use
# the .NodeRegistration.KubeletExtraArgs object in the configuration files instead. KUBELET_EXTRA_ARGS should be sourced from this file.
EnvironmentFile=-/etc/default/kubelet
Environment="KUBELET_EXTRA_ARGS=--node-ip=10.200.200.150 --hostname-override=tofu-vm1  "
ExecStart=
ExecStart=/usr/local/bin/kubelet $KUBELET_KUBECONFIG_ARGS $KUBELET_CONFIG_ARGS $KUBELET_KUBEADM_ARGS $KUBELET_EXTRA_ARGS
root@tofu-vm1:~# 
```
### 二进制模式

### 1. Etcd

**Service 文件**: /usr/lib/systemd/system/etcd.service

codeIni

```
[Unit]
Description=Etcd Server
After=network.target
After=network-online.target
Wants=network-online.target

[Service]
Type=notify
WorkingDirectory=/var/lib/etcd
ExecStart=/usr/local/bin/etcd
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/etcd.service.d/kubexm-etcd.conf

注意，admin证书是给etcdctl使用的，member证书是etcd证书和peer证书，node证书是apiserver访问etcd的证书

```
[Service]
ExecStart=
ExecStart=/usr/local/bin/etcd \
  --name=etcd-node1 \
  --data-dir=/var/lib/etcd/default.etcd \
  --listen-peer-urls=https://192.168.1.10:2380 \
  --listen-client-urls=https://192.168.1.10:2379,http://127.0.0.1:2379 \
  --advertise-client-urls=https://192.168.1.10:2379 \
  --initial-advertise-peer-urls=https://192.168.1.10:2380 \
  --initial-cluster=etcd-node1=https://192.168.1.10:2380,etcd-node2=https://192.168.1.11:2380 \
  --initial-cluster-token=etcd-cluster \
  --initial-cluster-state=new \
  --cert-file=/etc/ssl/etcd/ssl/member-node1.pem \
  --key-file=/etc/ssl/etcd/ssl/member-node1-key.pem \
  --peer-cert-file=/etc/ssl/etcd/ssl/member-node1.pem \
  --peer-key-file=/etc/ssl/etcd/ssl/member-node1-key.pem \
  --trusted-ca-file=/etc/ssl/etcd/ssl/ca.pem \
  --peer-trusted-ca-file=/etc/ssl/etcd/ssl/ca.pem
  
```

------



### 2. Kube-APIServer

**Service 文件**: /usr/lib/systemd/system/kube-apiserver.service

codeIni

```
[Unit]
Description=Kubernetes API Server
Documentation=https://github.com/kubernetes/kubernetes
After=network.target

[Service]
Type=notify
ExecStart=/usr/local/bin/kube-apiserver
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/kube-apiserver.service.d/kubexm-apiserver.conf

codeIni

```
[Service]
ExecStart=
ExecStart=/usr/local/bin/kube-apiserver \
  --advertise-address=192.168.1.10 \
  --allow-privileged=true \
  --authorization-mode=Node,RBAC \
  --client-ca-file=/etc/kubernetes/pki/ca.pem \
  --enable-admission-plugins=NodeRestriction \
  --enable-bootstrap-token-auth=true \
  --etcd-cafile=/etc/kubernetes/pki/etcd/ca.pem \
  --etcd-certfile=/etc/ssl/etcd/ssl/node-node1.pem \
  --etcd-keyfile=/etc/ssl/etcd/ssl/node-node1-key.pem \
  --etcd-servers=https://192.168.1.10:2379,https://192.168.1.11:2379 \
  --kubelet-client-certificate=/etc/kubernetes/pki/apiserver-kubelet-client.pem \
  --kubelet-client-key=/etc/kubernetes/pki/apiserver-kubelet-client-key.pem \
  --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname \
  --proxy-client-cert-file=/etc/kubernetes/pki/front-proxy-client.pem \
  --proxy-client-key-file=/etc/kubernetes/pki/front-proxy-client-key.pem \
  --service-account-key-file=/etc/kubernetes/pki/sa.pub \
  --service-account-signing-key-file=/etc/kubernetes/pki/sa.key \
  --service-account-issuer=https://kubernetes.default.svc.cluster.local \
  --service-cluster-ip-range=10.96.0.0/12 \
  --tls-cert-file=/etc/kubernetes/pki/apiserver.pem \
  --tls-private-key-file=/etc/kubernetes/pki/apiserver-key.pem \
  --requestheader-client-ca-file=/etc/kubernetes/pki/front-proxy-ca.pem \
  --requestheader-allowed-names=front-proxy-client \
  --requestheader-extra-headers-prefix=X-Remote-Extra- \
  --requestheader-group-headers=X-Remote-Group \
  --requestheader-username-headers=X-Remote-User \
  --secure-port=6443
```

------



### 3. Kube-Controller-Manager

**Service 文件**: /usr/lib/systemd/system/kube-controller-manager.service

codeIni

```
[Unit]
Description=Kubernetes Controller Manager
After=network.target

[Service]
ExecStart=/usr/local/bin/kube-controller-manager
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/kube-controller-manager.service.d/kubexm-controller-manager.conf

codeIni

```
[Service]
ExecStart=
ExecStart=/usr/local/bin/kube-controller-manager \
  --bind-address=127.0.0.1 \
  --cluster-cidr=10.244.0.0/16 \
  --cluster-name=kubernetes \
  --cluster-signing-cert-file=/etc/kubernetes/pki/ca.pem \
  --cluster-signing-key-file=/etc/kubernetes/pki/ca-key.pem \
  --kubeconfig=/etc/kubernetes/controller-manager.conf \
  --root-ca-file=/etc/kubernetes/pki/ca.pem \
  --service-account-private-key-file=/etc/kubernetes/pki/sa.key \
  --service-cluster-ip-range=10.96.0.0/12 \
  --use-service-account-credentials=true \
  --leader-elect=true
```

------



### 4. Kube-Scheduler

**Service 文件**: /usr/lib/systemd/system/kube-scheduler.service

codeIni

```
[Unit]
Description=Kubernetes Scheduler
After=network.target

[Service]
ExecStart=/usr/local/bin/kube-scheduler
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/kube-scheduler.service.d/kubexm-scheduler.conf

codeIni

```
[Service]
ExecStart=
ExecStart=/usr/local/bin/kube-scheduler \
  --bind-address=127.0.0.1 \
  --kubeconfig=/etc/kubernetes/scheduler.conf \
  --leader-elect=true
```

------



### 5. Kubelet

**Service 文件**: /usr/lib/systemd/system/kubelet.service

codeIni

```
[Unit]
Description=Kubernetes Kubelet
After=network-online.target firewalld.service containerd.service
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/kubelet
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/kubelet.service.d/kubexm-kubelet.conf

codeIni

```
[Service]
ExecStart=
# 这里的 --config 通常指向一个 yaml 文件，但也可以在这里直接加参数覆盖
ExecStart=/usr/local/bin/kubelet \
  --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \
  --kubeconfig=/etc/kubernetes/kubelet.conf \
  --config=/var/lib/kubelet/config.yaml \
  --container-runtime-endpoint=unix:///run/containerd/containerd.sock \
  --pod-infra-container-image=registry.k8s.io/pause:3.9
```

------



### 6. Kube-Proxy

**Service 文件**: /usr/lib/systemd/system/kube-proxy.service

codeIni

```
[Unit]
Description=Kubernetes Kube-Proxy
After=network.target

[Service]
ExecStart=/usr/local/bin/kube-proxy
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/kube-proxy.service.d/kubexm-proxy.conf

codeIni

```
[Service]
ExecStart=
ExecStart=/usr/local/bin/kube-proxy \
  --config=/var/lib/kube-proxy/config.yaml
```

*(注：Kube-proxy 现代部署建议使用配置文件，如果非要用 Flags，可以替换为 --kubeconfig=... --cluster-cidr=... 等)*

------



### 7. Containerd

**Service 文件**: /usr/lib/systemd/system/containerd.service

codeIni

```
[Unit]
Description=containerd container runtime
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Type=notify
Delegate=yes
KillMode=process
Restart=always
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/containerd.service.d/kubexm-containerd.conf

codeIni

```
[Service]
ExecStart=
# Containerd 强依赖 config.toml，通常不通过命令行传复杂参
ExecStart=/usr/local/bin/containerd --config /etc/containerd/config.toml
```

------



### 8. Docker (如果使用)

**Service 文件**: /usr/lib/systemd/system/docker.service

codeIni

```
[Unit]
Description=Docker Application Container Engine
After=network-online.target firewalld.service containerd.service
Wants=network-online.target

[Service]
Type=notify
ExecStart=/usr/bin/dockerd
ExecReload=/bin/kill -s HUP $MAINPID
Restart=always
LimitNOFILE=infinity
Delegate=yes
KillMode=process

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/docker.service.d/kubexm-docker.conf

codeIni

```
[Service]
ExecStart=
# 在这里指定 cgroup-driver 和镜像源等
ExecStart=/usr/bin/dockerd \
  -H fd:// \
  --containerd=/run/containerd/containerd.sock \
  --exec-opt native.cgroupdriver=systemd \
  --data-root=/var/lib/docker
```

------



### 9. CRI-Docker (配合 Docker 使用)

**Service 文件**: /usr/lib/systemd/system/cri-docker.service

codeIni

```
[Unit]
Description=CRI Interface for Docker Application Container Engine
After=network-online.target firewalld.service docker.service
Wants=network-online.target
Requires=docker.service

[Service]
Type=notify
ExecStart=/usr/local/bin/cri-dockerd
Restart=always
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

**Drop-in 配置**: /usr/lib/systemd/system/cri-docker.service.d/kubexm-cri-docker.conf

codeIni

```
[Service]
ExecStart=
ExecStart=/usr/local/bin/cri-dockerd \
  --container-runtime-endpoint fd:// \
  --network-plugin=cni \
  --pod-infra-container-image=registry.k8s.io/pause:3.9 \
  --cni-bin-dir=/opt/cni/bin \
  --cni-conf-dir=/etc/cni/net.d
```

## 6. 用户场景

### 6.1 企业私有云部署
- **场景**：大型企业在隔离网络环境中部署多个Kubernetes集群
- **解决方案**：完全离线部署，标准化流程

### 6.2 混合云管理
- **场景**：企业在公有云和私有云环境中统一管理Kubernetes
- **解决方案**：云平台无关，统一管理接口

### 6.3 大规模集群运维
- **场景**：管理超过100个节点的大型生产集群
- **解决方案**：支持大规模节点管理，零停机升级

## 7. 竞争优势

### 7.1 差异化优势
- **唯一完全离线**：与Kubespray/Kops的在线依赖形成差异化
- **双模式创新**：满足不同用户的定制化和标准化需求
- **企业级工程**：相比Rancher轻量70%，相比Kubespray高效50%

### 7.2 技术护城河
- **离线优先**：在网络隔离环境中零依赖部署
- **防御性编程**：企业级错误处理和日志
- **模块化设计**：易于扩展和维护

## 8. 总结

基于require2.md目录结构规范的Kubexm产品架构设计，提供了：
- **完整的产品架构**：严格按照目录结构规范设计
- **离线优先**：企业级离线部署能力
- **双模式创新**：Binary + Kubeadm双模式支持
- **模块化设计**：清晰的代码组织和职责分离
- **企业级标准**：防御性编程、幂等性、错误处理

这个架构为Kubexm提供了坚实的技术基础，能够满足企业级Kubernetes集群管理的各种需求。