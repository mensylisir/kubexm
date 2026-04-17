# Step 层重构计划

## 目录结构规范（新）

```
internal/step/
├── common/           # 公共辅助函数（原 lib/）
├── addons/           # Addon 安装/删除
├── certs/            # 证书相关
├── cluster/          # 集群操作（drain/cordon/uncordon 等）
├── cni/              # CNI 步骤入口 → 迁移到 network/cni/
├── download/         # 下载步骤
├── etcd/             # etcd 安装/配置
├── images/           # 镜像推送
├── iso/              # ISO 制作步骤
├── kubernetes/       # K8s 组件（按组件分）
│   ├── apiserver/
│   ├── controller-manager/
│   ├── scheduler/
│   ├── kubelet/
│   ├── kube-proxy/
│   ├── kubeadm/      # kubeadm 操作
│   ├── kubeconfig/
│   ├── kubectl/
│   ├── certs/
│   ├── health/
│   ├── labels/
│   ├── perform/      # cleanup/cordon/drain/uncordon
│   ├── rbac/
│   ├── backup/
│   └── common/
├── loadbalancer/     # LB 步骤（对外）
│   ├── common/       # keepalived（VIP 故障转移）
│   ├── haproxy/
│   ├── nginx/
│   └── kube-vip/
├── manifests/        # 清单生成
├── network/          # 网络（内部组织）
│   ├── cni/          # CNI 实现
│   ├── calico/
│   ├── cilium/
│   ├── flannel/
│   ├── hybridnet/
│   ├── kubeovn/
│   ├── multus/
│   └── common/
├── os/               # OS 配置（hosts/swap/firewall 等）
├── registry/         # Registry 操作
├── runtime/          # 容器运行时
└── storage/          # 存储相关
```

## 迁移规则

1. **kubernetes/ 目录下**：已有 kube-apiserver/ 等目录，需要重命名为 apiserver/（去掉 kube- 前缀）
2. **loadbalancer/ 目录下**：已有，结构合理
3. **network/ 目录下**：calico/ 等需要迁移
4. **step/common/**：保留公共函数
5. **删除的目录**：helpers/（移动到 tool/ 或 common/）

## Task 层规范

- Task 按组件分类：kubernetes/kubeadm/, kubernetes/kubelet/, etcd/, loadbalancer/ 等
- Task 只做组件级完整操作，不编排其他 Task

## Module 层规范

- Module 编排多个 Task
- 通过配置或注册的方式组装 Task，避免硬编码

## Pipeline 层规范

- Pipeline 编排 Module
- 每个 Pipeline 前先执行连通性检测 Module
- 参数验证（backupType/component/certType 等）
