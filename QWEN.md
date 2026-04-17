## Qwen Added Memories
- kubexm 架构核心规则：
1. Kubernetes支持两种安装类型：kubeadm 和 kubexm(二进制)
2. etcd支持三种类型：kubeadm、kubexm(二进制)、exists(已存在etcd,跳过安装直接配置)
3. loadbalancer支持启用和禁用：
   - loadbalancer_mode=external: 在loadbalancer角色机器部署LB
     - loadbalancer_type=kubexm_kh: keepalived+haproxy
     - loadbalancer_type=kubexm_kn: keepalived+nginx
   - loadbalancer_mode=internal: 在所有worker上部署LB代理到master,kubelet连接本地LB
     - type=haproxy+kubernetes_type=kubeadm: worker静态pod部署haproxy
     - type=haproxy+kubernetes_type=kubexm: worker二进制部署haproxy
     - type=nginx+kubernetes_type=kubeadm: worker静态pod部署nginx
     - type=nginx+kubernetes_type=kubexm: worker二进制部署nginx
   - loadbalancer_mode=kube-vip: 使用kube-vip作为LB
   - loadbalancer_mode=exists: 已存在LB,跳过部署直接使用
4. download时不校验host.yaml,离线模式:kubexm download先执行→用户复制packages到离线环境→kubexm create cluster;在线模式:kubexm create cluster自动执行download和create
5. 中心机器(堡垒机)拥有所有包和文件,所有kubernetes机器上的包/配置/文件/证书都通过堡垒机分发
6. 机器配置来自host.yaml,不允许localhost/127.0.0.1,即使本机也要检测大网地址使用SSH操作
7. 所有依赖(jq/yq等)和k8s组件必须支持离线,脚本中用到的各种工具都要离线
8. 重构完成后删除原来的目录
9. --source-registry参数不需要,镜像源地址离线时就知道,不需要指定
10. 安装集群时要添加hosts(ip/hostname/registry域名),删除集群时要删除hosts
11. 每个pipeline前都先执行连通性检测module
12. 层级职责: Pipeline->Module->Task->Step->Runner->Connector,严禁跨层调用,Task只做组件级原子操作不编排,Module编排多个Task
- kubexm 目录结构和路径管理规则：
1. Step目录分类：step/common(公共辅助),step/certs(证书),step/cni,step/download,step/etcd,step/images,step/iso,step/kubernetes(按组件分:apiserver/scheduler/controller-manager/kubelet/kube-proxy),step/loadbalancer(common/keepalived,haproxy,nginx,kube-vip),step/manifests,step/os(hosts/swap/firewall),step/registry,step/runtime,step/addons,step/cluster,step/deployment,step/network
2. Task层:kubeadm相关放在kubernetes/kubeadm下,kubelet相关放在kubernetes/kubelet下,遵循组件目录规则
3. Module层编排Task,Pipeline层编排Module
4. 下载路径规则:${下载位置}/${component_name}/${component_version}/${arch}/${component_name}
   - 例:{下载位置}/kubelet/v1.24.9/amd64/kubelet
   - 例:{下载位置}/helm/v1.3.2/amd64/helm
   - 例:{下载位置}/helm_packages/(helm包自带版本号不区分架构)
   - 例:{下载位置}/manifests/coredns/coredns.yaml
   - 例:{下载位置}/iso/${os_name}/${os_version}/${arch}/${os_name}-${os_version}-${arch}.iso
5. 架构判断:根据spec.arch判断,配置多个架构下载多个架构,host.yaml中机器没配置arch默认x86,安装时根据机器架构分发
6. 多集群防覆盖:使用metadata.name作为集群名称区分路径,如{下载位置}/{cluster_name}/certs/
7. 证书轮转路径:{下载位置}/rotate/kubernetes/old/(旧证书),{下载位置}/rotate/kubernetes/new/(新根证书ca.crt),{下载位置}/rotate/kubernetes/bundle/(bundle后ca.crt),etcd证书类似路径
8. ISO制作:支持本地制作(同版本同架构)和容器制作(各种类型)两种,命名${os_name}-${os_version}-${arch}.iso
9. 重构完成后删除原来的目录
- kubexm 核心架构规范（必须严格遵守）：
1. Kubernetes 安装类型：kubeadm（kubeadm 方式）和 kubexm（二进制方式）
2. etcd 部署类型：kubeadm、kubexm（二进制）、exists（已存在，跳过安装直接配置）
3. LoadBalancer 完整配置矩阵：
   - loadbalancer_mode=external + type=kubexm_kh：loadbalancer 角色机器部署 keepalived+haproxy
   - loadbalancer_mode=external + type=kubexm_kn：loadbalancer 角色机器部署 keepalived+nginx
   - loadbalancer_mode=internal + type=haproxy + k8s_type=kubeadm：worker 静态 pod 部署 haproxy
   - loadbalancer_mode=internal + type=haproxy + k8s_type=kubexm：worker 二进制部署 haproxy
   - loadbalancer_mode=internal + type=nginx + k8s_type=kubeadm：worker 静态 pod 部署 nginx
   - loadbalancer_mode=internal + type=nginx + k8s_type=kubexm：worker 二进制部署 nginx
   - loadbalancer_mode=kube-vip：使用 kube-vip
   - loadbalancer_mode=exists：已存在 LB，跳过部署
4. 离线模式流程：kubexm download（有网）→ 复制 packages 目录到离线环境 → kubexm create cluster（无网）
5. 在线模式流程：kubexm create cluster → 自动 download + create
6. 中心机器（堡垒机）拥有所有包/文件/证书，通过 SSH 分发到所有 kubernetes 机器
7. 下载路径规则：
   - ISO: ${download_location}/iso/${os_name}/${os_version}/${arch}/${os_name}-${os_version}-${arch}.iso
   - 组件: ${download_location}/${component_name}/${component_version}/${arch}/${component_name}
   - Helm: ${download_location}/helm_packages/（自带版本号，不区分架构）
   - Manifests: ${download_location}/manifests/${component_name}/${component}.yaml
8. 多集群防覆盖：使用 metadata.name 作为集群名称区分路径
9. 证书轮转路径：
   - ${download_location}/rotate/kubernetes/old/（旧证书）
   - ${download_location}/rotate/kubernetes/new/（新根证书 ca.crt）
   - ${download_location}/rotate/kubernetes/bundle/（bundle 后 ca.crt）
   - etcd 证书同理在 /rotate/etcd/ 下
10. 架构判断：根据 spec.arch 判断，没配置默认 x86，多架构下载多个
11. 严禁 localhost/127.0.0.1，即使本机也使用 SSH 操作大网地址
12. 所有工具（jq/yq 等）必须支持离线
13. hosts 管理：安装时添加，删除时删除
14. 每个 Pipeline 前必须执行连通性检测 Module
15. 层级调用规则：严禁跨层调用
    - Task：只做组件级原子操作，不编排其他 Task
    - Module：编排多个 Task
    - Pipeline：编排 Module
16. 目录规则：按组件名称分割（如 kubernetes/kubelet/、kubernetes/kubeadm/）
17. Step 严禁直接调用 Connector，必须通过 Runner
18. --source-registry 参数不需要删除，镜像源地址离线时就知道
19. download 时不校验 host.yaml
