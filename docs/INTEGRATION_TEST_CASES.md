# kubexm 集成测试用例文档（命令/子命令/分支/参数全覆盖）

> 目标：产出“可执行的集成测试用例清单”，覆盖 CLI 的命令树、参数组合、关键分支（成功/失败/交互/幂等/dry-run）与配置参数边界。本文档只描述测试设计，不包含任何代码实现与测试执行。

---

## 1. 设计范围与约束

### 1.1 覆盖范围

- CLI 根命令与全局参数：`kubexm`、`--verbose/-v`、`--yes/-y`。
- 一级命令组：`create`、`build`、`delete`、`download`、`version`、`completion`、`config`。
- 二级命令组：`cluster`、`node`、`certs`、`registry`、`iso`、`images`。
- 写操作分支：`--dry-run`、交互确认（yes/no）、`--force`、参数缺失、无效参数。
- 读操作分支：默认输出、显式输出格式、资源不存在、路径不存在。

### 1.2 参数基线来源

- CLI 参数来自 `internal/cmd/**` 下 Cobra Flags 定义。
- 配置参数来自 `internal/apis/kubexms/v1alpha1/**` 的类型定义与校验逻辑。
- 机器规模与角色划分以 `ClusterSpec.hosts + roleGroups` 为主，按 HA/etcd/network/storage 组件扩展。

---

## 2. 机器规模（测试床）定义

> 每条测试用例都必须绑定“最小机器数”。为降低成本，采用复用测试床。

### T0：单机控制面（1 台）

- 角色：`master/control-plane + worker` 合并。
- 用于：本地配置类命令、只读类命令、dry-run 分支、参数校验类失败分支。

### T1：标准高可用（3 台）

- 角色：3×`master`（同时 etcd）+ 可选 worker。
- 用于：`create/upgrade/reconfigure/health/certs` 主流程。

### T2：带独立工作节点（5 台）

- 角色：3×control-plane(etcd) + 2×worker。
- 用于：`add-nodes/delete-nodes/scale/node drain|cordon|uncordon|get|list`。

### T3：含独立镜像仓库与 LB（6~7 台）

- 在 T2 基础上增加：
  - 1×registry 节点（`roleGroups.registry`）；
  - 1×loadbalancer 节点（若 external LB 采用托管模式）。
- 用于：`create registry/delete registry/images push/create iso/build iso/download`。

### T4：外部依赖场景（可选）

- 外部 etcd（非集群内）或外部私有仓库。
- 用于：`etcd.type=external`、`registry.auths`、镜像重写等兼容性分支。

---

## 3. API 参数覆盖矩阵（按配置模型）

> 该矩阵规定“配置参数”在集成测试里至少要被一条用例覆盖。

### 3.1 集群基础（ClusterSpec）

| 配置域 | 关键字段 | 覆盖要求 | 建议机器数 |
|---|---|---|---|
| hosts | `hosts[].name/address/internalAddress/port/user/password/privateKey/privateKeyPath/arch/labels/taints` | 正常值 + 重复主机名 + 重复地址 + localhost 非法地址 + 端口越界 | T0/T1 |
| roleGroups | `master/worker/etcd/loadbalancer/storage/registry` | 至少 1 master；未分配角色主机；引用不存在主机；空角色组 | T1/T2 |
| global | `user/port/connectionTimeout/workDir/offlineMode/skipPreflight` | 默认值、绝对路径校验、超时为负、offline 模式 | T0/T1 |
| system | `ntpServers/timezone/packageManager/rpms/debs/sysctlParams` | rpm/deb 互斥与组合边界；非法 NTP；空 sysctl value | T0/T1 |

### 3.2 Kubernetes/Etcd/Network

| 配置域 | 关键字段 | 覆盖要求 | 建议机器数 |
|---|---|---|---|
| kubernetes | `version/type/serviceNodePortRange/audit/*/kubelet*` | 版本合法/非法；nodePort 范围格式错误；audit 开启但缺少 policy | T1 |
| etcd | `type(cluster/external)/external.endpoints/*/backup/performance` | external 与 cluster 配置互斥；endpoint 空值；端口范围 | T1/T4 |
| network | `kubePodsCIDR/kubeServiceCIDR/plugin(calico/cilium/flannel/kubeovn/hybridnet)` | 每个插件至少 1 条成功用例；插件与配置段不匹配失败 | T1/T2 |

### 3.3 HA/Registry/Storage/Addons/Preflight

| 配置域 | 关键字段 | 覆盖要求 | 建议机器数 |
|---|---|---|---|
| highAvailability | `enabled/external/internal/type/loadBalancerHostGroupName` | HA 开启但未选择 internal/external；LB 类型非法；必填字段缺失 | T1/T3 |
| registry | `mirroring.privateRegistry/namespaceRewrite/auths/local.type/dataRoot` | auth 互斥（user/pass vs auth）；base64 非法；rewrite 规则为空 | T3/T4 |
| storage | `openebs/nfs/rookceph/longhorn/defaultStorageClass` | 至少覆盖 3 种存储；引擎全关时报错；NFS 缺 server/path | T2/T3 |
| addons | `name/enabled/sources(chart|yaml)/values/timeout` | source 必填；chart/yaml 互斥；非法 repo URL；values 非 key=value | T1 |
| preflight | `minCPUCores/minMemoryMB/disableSwap/skipChecks` | 负值失败；skipChecks 非法项 | T0/T1 |

---

## 4. CLI 集成测试用例（按命令树）

> 说明：每条用例都包含“主分支 + 失败分支 + 参数边界分支”。

## 4.1 根命令与公共行为

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| ROOT-001 | `kubexm --help` | 基础帮助分支 | 返回命令树与全局参数 | T0 |
| ROOT-002 | `kubexm -v version` | `--verbose` 继承 | 输出版本且日志级别可提升 | T0 |
| ROOT-003 | `kubexm -y delete cluster ...` | `--yes` 交互旁路 | 不出现交互确认 | T1 |
| ROOT-004 | `kubexm unknown` | 未知命令失败分支 | 返回错误与帮助提示 | T0 |

## 4.2 `create` 命令组

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| CRT-CL-001 | `kubexm create cluster -f valid.yaml` | 主成功路径 | 集群创建完成 | T1 |
| CRT-CL-002 | 同上 + `--dry-run` | dry-run 分支 | 不落地变更，仅输出计划 | T1 |
| CRT-CL-003 | 同上 + `--skip-preflight` | 跳过预检分支 | 预检阶段被绕过 | T1 |
| CRT-CL-004 | 缺少 `-f` | 必填参数缺失 | 命令失败并提示 required | T0 |
| CRT-ISO-001 | `kubexm create iso -f valid.yaml -i base.iso` | 主成功路径 | 生成离线 ISO | T3 |
| CRT-ISO-002 | 同上 `--dry-run` | dry-run 分支 | 不生成文件，仅清单输出 | T0 |
| CRT-ISO-003 | `-i` 不存在路径 | 输入文件不存在分支 | 明确报错 | T0 |
| CRT-REG-001 | `kubexm create registry -f valid.yaml` | 主成功路径 | 仓库组件部署成功 | T3 |
| CRT-REG-002 | 同上 `--type/--port` | 参数覆盖分支 | 使用指定类型和端口 | T3 |
| CRT-REG-003 | 同上 `--dry-run` | dry-run | 无实际部署 | T0 |

## 4.3 `delete` 命令组

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| DEL-CL-001 | `kubexm delete cluster -n c1` | 主成功路径 | 集群资源被清理 | T1 |
| DEL-CL-002 | 同上（不加 `-y`）输入 `no` | 交互拒绝分支 | 操作中止 | T1 |
| DEL-CL-003 | 同上 `--force` | 强制删除分支 | 无确认直接执行 | T1 |
| DEL-CL-004 | 同上 `--dry-run` | dry-run | 不删除 | T1 |
| DEL-ND-001 | `kubexm delete delete-nodes -f scale-in.yaml` | 删除节点主流程 | 指定节点下线 | T2 |
| DEL-ND-002 | 同上 `--force` | 强制确认旁路 | 直接执行 | T2 |
| DEL-ND-003 | 同上 `--dry-run` | dry-run | 不落地 | T2 |
| DEL-REG-001 | `kubexm delete registry -f valid.yaml` | 主成功路径 | registry 卸载 | T3 |
| DEL-REG-002 | 同上 `--delete-images` | 镜像数据清理分支 | 镜像数据一并删除 | T3 |
| DEL-REG-003 | 同上 `--force --dry-run` | 组合分支 | 无确认且不落地 | T0 |

## 4.4 `build` 与 `download`

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| BLD-ISO-001 | `kubexm build iso -f valid.yaml` | host 模式主流程 | 产出 ISO 包 | T3 |
| BLD-ISO-002 | `--mode docker --os ubuntu --version 22.04` | docker 分支 | 跨平台构建成功 | T3 |
| BLD-ISO-003 | `--multi-arch --mode docker` | 多架构分支 | 产出多架构制品 | T3 |
| BLD-ISO-004 | `--runtime/--cni/--lb/--storage/--extra-pkgs` | 组件覆盖分支 | 依赖解析与打包正确 | T3 |
| BLD-ISO-005 | `--dry-run` | dry-run | 无制品写入 | T0 |
| DLD-001 | `kubexm download -f valid.yaml -o bundle.tar.gz` | 主流程 | 离线资产打包成功 | T3 |
| DLD-002 | 同上 `--dry-run` | dry-run | 不下载 | T0 |
| DLD-003 | 缺少 `-f` | 必填参数分支 | 返回 required 错误 | T0 |

## 4.5 `cluster` 命令组

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| CLS-001 | `kubexm cluster list` | 列表成功 | 输出集群列表 | T0 |
| CLS-002 | `kubexm cluster get c1` | 详情成功 | 输出指定集群摘要 | T0 |
| CLS-003 | `kubexm cluster kubeconfig c1` | 输出到 stdout | 返回 kubeconfig 文本 | T0 |
| CLS-004 | `kubexm cluster kubeconfig c1 -o ./k` | 写文件分支 | 文件存在且可用 | T0 |
| CLS-005 | `kubexm cluster manifests -f valid.yaml` | 主流程 | 渲染核心 manifests | T0/T1 |
| CLS-006 | 同上 `--dry-run` | dry-run | 仅展示渲染计划 | T0 |
| CLS-007 | `kubexm cluster health -f valid.yaml -c all` | 健康检查主流程 | 返回健康状态 | T1 |
| CLS-008 | 同上 `-c apiserver/scheduler/controller-manager/kubelet/cluster` | 组件分支全覆盖 | 各组件检查执行 | T1 |
| CLS-009 | 同上 `--wait 10m` | 超时参数分支 | 超时行为可控 | T1 |
| CLS-010 | `kubexm cluster add-nodes -f scale-out.yaml` | 扩容主流程 | 新节点加入 | T2 |
| CLS-011 | 同上 `--skip-preflight --dry-run` | 组合分支 | 跳过预检且不落地 | T2 |
| CLS-012 | `kubexm cluster scale -f scale.yaml --direction out` | scale-out 分支 | 扩容执行 | T2 |
| CLS-013 | 同上 `--direction in` | scale-in 分支 | 缩容执行 | T2 |
| CLS-014 | `kubexm cluster backup -f valid.yaml --type all` | 备份主流程 | 备份产物生成 | T1 |
| CLS-015 | 同上 `--type pki/etcd/kubernetes` | 类型分支 | 按类型备份 | T1 |
| CLS-016 | `kubexm cluster restore -f valid.yaml -b backup.tar --type all` | 恢复主流程 | 恢复成功 | T1 |
| CLS-017 | 同上 `--type etcd -s snapshot.db` | etcd 专有分支 | snapshot 恢复成功 | T1 |
| CLS-018 | `kubexm cluster reconfigure -f old.yaml -n new.yaml -c all` | 全量重配置 | 配置变更落地 | T1 |
| CLS-019 | 同上 `-c apiserver/scheduler/controller-manager/kubelet/proxy` | 组件分支 | 指定组件生效 | T1 |
| CLS-020 | 同上 `--restart=false --backup=false` | 开关参数分支 | 不重启、不备份路径 | T1 |
| CLS-021 | `kubexm cluster upgrade -f valid.yaml -t vX.Y.Z` | K8s 升级主流程 | 升级完成 | T1 |
| CLS-022 | 同上 `--dry-run` | dry-run | 无实际升级 | T1 |
| CLS-023 | `kubexm cluster upgrade etcd -f valid.yaml -t v3.5.9` | etcd 升级分支 | etcd 升级成功 | T1 |
| CLS-024 | 同上 `--dry-run` | dry-run | 不改动 etcd | T1 |

## 4.6 `node` 命令组

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| NOD-001 | `kubexm node list -c c1` | 主流程 | 节点表格输出 | T2 |
| NOD-002 | `kubexm node get node1 -c c1 -o yaml` | yaml 分支 | 输出 YAML | T2 |
| NOD-003 | 同上 `-o summary` | summary 分支 | 输出摘要字段 | T2 |
| NOD-004 | 同上 `-o json` | 未实现分支 | 返回明确错误 | T2 |
| NOD-005 | `kubexm node cordon node1 -c c1` | 主流程 | 节点不可调度 | T2 |
| NOD-006 | 重复 cordon | 幂等分支 | 返回 already cordoned | T2 |
| NOD-007 | `kubexm node uncordon node1 -c c1` | 主流程 | 节点恢复调度 | T2 |
| NOD-008 | 重复 uncordon | 幂等分支 | 返回 already schedulable | T2 |
| NOD-009 | `kubexm node drain node1 -c c1 --ignore-daemonsets` | 有 DaemonSet 分支 | 跳过 DS Pod | T2 |
| NOD-010 | 同上不加 `--force` 且有裸 Pod | 失败分支 | 以未托管 Pod 错误退出 | T2 |
| NOD-011 | 同上 `--force --grace-period 0 --timeout 2m` | 强制清空分支 | 节点排空完成 | T2 |

## 4.7 `certs` 命令组

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| CRTS-001 | `kubexm certs check-expiration -c c1` | 主流程 | 证书过期表输出 | T1 |
| CRTS-002 | 同上 `--warn-within 7` | 告警窗口分支 | 临期高亮阈值生效 | T1 |
| CRTS-003 | `kubexm certs renew -f valid.yaml -t all` | 主流程 | 证书续期完成 | T1 |
| CRTS-004 | 同上 `-t kubernetes-ca/etcd-ca/kubernetes-certs/etcd-certs` | 类型分支全覆盖 | 各管线可执行 | T1 |
| CRTS-005 | 同上 `--dry-run` | dry-run | 无证书改动 | T1 |
| CRTS-006 | `kubexm certs update -f valid.yaml -t all` | 主流程 | 证书更新完成 | T1 |
| CRTS-007 | 同上使用位置参数 `update kubernetes-ca` | 参数优先级分支 | 位置参数覆盖 type | T1 |
| CRTS-008 | `kubexm certs rotate -f valid.yaml -t all --service apiserver` | 轮转分支 | 目标服务轮转成功 | T1 |
| CRTS-009 | `renew/update/rotate` + `--force` | 交互旁路 | 无确认直接执行 | T1 |

## 4.8 `config` / `version` / `completion` / `images`

| 用例ID | 命令 | 分支/参数 | 预期 | 机器数 |
|---|---|---|---|---|
| CFG-001 | `kubexm config view` | 默认输出 | 输出 YAML 配置 | T0 |
| CFG-002 | `kubexm config set default-package-dir /opt/x` | 主流程 | 配置落盘成功 | T0 |
| CFG-003 | `kubexm config set verbose true/false` | bool 分支 | 值生效 | T0 |
| CFG-004 | `kubexm config set unknown v` | 非法 key 分支 | 返回 unknown key | T0 |
| CFG-005 | `kubexm config use-context c1` | 上下文切换成功 | currentContext 更新 | T0 |
| CFG-006 | `kubexm config use-context noexist` | 不存在上下文分支 | 返回不存在错误 | T0 |
| VER-001 | `kubexm version` | 主流程 | 输出 version/commit/date | T0 |
| CMP-001 | `kubexm completion bash|zsh|fish|powershell` | shell 分支全覆盖 | 输出补全脚本 | T0 |
| CMP-002 | `kubexm completion invalid` | 非法参数分支 | 参数校验失败 | T0 |
| IMG-001 | `kubexm images push -l images.txt -r reg:5000` | 主流程 | 全量 push 成功 | T3 |
| IMG-002 | 同上 `--dry-run` | dry-run | 仅打印映射 | T0 |
| IMG-003 | 同上 `--concurrency 1/10` | 并发分支 | 并发稳定性 | T3 |
| IMG-004 | 同上 `--auth-file` + `--skip-tls-verify` | 鉴权+TLS 分支 | 透传到 skopeo | T3/T4 |
| IMG-005 | 缺少 `-l` 或 `-r` | 必填参数缺失分支 | 失败并提示 | T0 |

---

## 5. 分支覆盖清单（执行前核对）

- [ ] 每个写命令至少 1 条 `--dry-run`。
- [ ] 每个有交互确认的命令至少覆盖：`yes`、`no`、`--yes`、`--force`。
- [ ] 每个必填参数至少覆盖：缺失时失败。
- [ ] 每个枚举参数至少覆盖：合法最小集 + 非法值。
- [ ] 每个输出参数至少覆盖：默认输出 + 非法输出格式。
- [ ] 每个“路径类参数”至少覆盖：存在路径 + 不存在路径。
- [ ] 节点命令至少覆盖：幂等分支（already cordoned/uncordoned）。

---

## 6. 数据准备与回收规范（仅文档要求）

- 测试配置文件至少准备 6 套：
  - `cluster-minimal.yaml`（T0）
  - `cluster-ha.yaml`（T1）
  - `cluster-scale.yaml`（T2）
  - `cluster-registry-lb.yaml`（T3）
  - `cluster-external-etcd.yaml`（T4）
  - `cluster-invalid-*.yaml`（参数负例）
- 每条“破坏性命令”必须定义回收动作（如 restore、recreate）。
- 镜像推送测试应使用隔离命名空间/仓库前缀，避免污染正式仓库。

---

## 7. 通过准则

- 命令覆盖：100%（文档中列出的命令/子命令全部有用例）。
- 参数覆盖：100%（必填、可选、枚举、布尔、路径、时长、并发全部覆盖）。
- 分支覆盖：核心分支 100%（成功、失败、交互、dry-run、幂等）。
- 机器维度：每条用例均明确最小机器数，且可映射到 T0~T4 测试床。

---

## 8. 命令参数“逐项覆盖清单”（执行人员打勾用）

> 本节用于补足“所有命令/子命令/参数”的逐项核对，避免遗漏。

### 8.1 Root 级参数

| 命令路径 | 参数 | 必填 | 取值/样例 | 最少用例数 | 机器数 |
|---|---|---|---|---|---|
| `kubexm` | `--verbose,-v` | 否 | true/false | 2（开/关） | T0 |
| `kubexm` | `--yes,-y` | 否 | true/false | 2（开/关） | T1 |

### 8.2 create 组

| 命令路径 | 参数 | 必填 | 分支要求 | 机器数 |
|---|---|---|---|---|
| `create cluster` | `--config,-f` | 是 | 存在/不存在/格式非法 | T0/T1 |
| `create cluster` | `--skip-preflight` | 否 | true/false | T1 |
| `create cluster` | `--dry-run` | 否 | true/false | T0/T1 |
| `create iso` | `--config,-f` | 是 | 存在/不存在 | T0/T3 |
| `create iso` | `--iso,-i` | 是 | 存在/不存在/非 ISO 文件 | T0/T3 |
| `create iso` | `--output,-o` | 否 | 默认路径/自定义路径无权限 | T0/T3 |
| `create iso` | `--os` | 否 | ubuntu/非法值 | T0/T3 |
| `create iso` | `--os-version` | 否 | 默认值/自定义值 | T0/T3 |
| `create iso` | `--arch,-a` | 否 | amd64/arm64/非法值 | T0/T3 |
| `create iso` | `--packages,-p` | 否 | 存在/不存在 | T0/T3 |
| `create iso` | `--skip-verification` | 否 | true/false | T0/T3 |
| `create iso` | `--dry-run` | 否 | true/false | T0 |
| `create registry` | `--config,-f` | 是 | 存在/不存在 | T0/T3 |
| `create registry` | `--type` | 否 | registry/非法值 | T3 |
| `create registry` | `--port` | 否 | 5000/自定义端口/越界值 | T3 |
| `create registry` | `--dry-run` | 否 | true/false | T0/T3 |

### 8.3 delete 组

| 命令路径 | 参数 | 必填 | 分支要求 | 机器数 |
|---|---|---|---|---|
| `delete cluster` | `--name,-n` | 是 | 存在/不存在集群名 | T0/T1 |
| `delete cluster` | `--force` | 否 | true/false（含交互 yes/no） | T1 |
| `delete cluster` | `--dry-run` | 否 | true/false | T1 |
| `delete delete-nodes` | `--config,-f` | 是 | 存在/不存在 | T0/T2 |
| `delete delete-nodes` | `--force` | 否 | true/false | T2 |
| `delete delete-nodes` | `--dry-run` | 否 | true/false | T2 |
| `delete registry` | `--config,-f` | 是 | 存在/不存在 | T0/T3 |
| `delete registry` | `--force` | 否 | true/false（含交互） | T3 |
| `delete registry` | `--delete-images` | 否 | true/false | T3 |
| `delete registry` | `--dry-run` | 否 | true/false | T0/T3 |

### 8.4 build/download 组

| 命令路径 | 参数 | 必填 | 分支要求 | 机器数 |
|---|---|---|---|---|
| `build iso` | `--os` | 条件必填（docker 模式） | ubuntu/centos/rocky/debian/非法值 | T3 |
| `build iso` | `--version,-v` | 条件必填（docker 模式） | 合法/缺失 | T3 |
| `build iso` | `--arch,-a` | 否 | 自动探测/指定 amd64/arm64 | T3 |
| `build iso` | `--config,-f` | 否 | 用配置推导/不用配置纯 flag | T0/T3 |
| `build iso` | `--mode,-m` | 否 | host/docker/非法值 | T3 |
| `build iso` | `--multi-arch` | 否 | true/false | T3 |
| `build iso` | `--registry` | 否 | 有/无 | T3 |
| `build iso` | `--output,-o` | 否 | 默认路径/自定义路径 | T3 |
| `build iso` | `--packages,-p` | 否 | 有/无 | T3 |
| `build iso` | `--include-kubexm` | 否 | true/false | T3 |
| `build iso` | `--runtime` | 否 | containerd/docker/cri-o/非法值 | T3 |
| `build iso` | `--cni` | 否 | calico/cilium/flannel/kubeovn/hybridnet/非法值 | T3 |
| `build iso` | `--lb` | 否 | kubexm_kh/kubexm_kn/haproxy/nginx/非法值 | T3 |
| `build iso` | `--storage` | 否 | nfs/longhorn/openebs/非法值 | T3 |
| `build iso` | `--extra-pkgs` | 否 | 空/多值 | T3 |
| `build iso` | `--dry-run` | 否 | true/false | T0/T3 |
| `download` | `--config,-f` | 是 | 存在/不存在 | T0/T3 |
| `download` | `--output,-o` | 否 | 默认路径/自定义路径 | T0/T3 |
| `download` | `--dry-run` | 否 | true/false | T0 |

### 8.5 cluster 组

| 命令路径 | 参数 | 必填 | 分支要求 | 机器数 |
|---|---|---|---|---|
| `cluster get` | 位置参数 `clusterName` | 是 | 存在/不存在 | T0 |
| `cluster list` | 无 | - | 空列表/非空列表 | T0 |
| `cluster kubeconfig` | 位置参数 `clusterName` | 是 | stdout/文件输出 | T0 |
| `cluster kubeconfig` | `--output,-o` | 否 | 指定路径/不可写路径 | T0 |
| `cluster kubeconfig` | `--raw` | 否 | true/false | T0 |
| `cluster manifests` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `cluster manifests` | `--output,-o` | 否 | 默认目录/自定义目录 | T0/T1 |
| `cluster manifests` | `--dry-run` | 否 | true/false | T0 |
| `cluster health` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `cluster health` | `--component,-c` | 否 | all/apiserver/scheduler/controller-manager/kubelet/cluster/非法值 | T1 |
| `cluster health` | `--wait` | 否 | 默认/短超时/长超时 | T1 |
| `cluster add-nodes` | `--config,-f` | 是 | 存在/不存在 | T0/T2 |
| `cluster add-nodes` | `--skip-preflight` | 否 | true/false | T2 |
| `cluster add-nodes` | `--dry-run` | 否 | true/false | T2 |
| `cluster scale` | `--config,-f` | 是 | 存在/不存在 | T0/T2 |
| `cluster scale` | `--direction` | 是 | in/out/非法值 | T2 |
| `cluster scale` | `--skip-preflight` | 否 | true/false | T2 |
| `cluster scale` | `--dry-run` | 否 | true/false | T2 |
| `cluster backup` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `cluster backup` | `--type,-t` | 否 | all/pki/etcd/kubernetes/非法值 | T1 |
| `cluster backup` | `--output,-o` | 否 | 默认/自定义路径 | T1 |
| `cluster backup` | `--dry-run` | 否 | true/false | T1 |
| `cluster restore` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `cluster restore` | `--backup,-b` | 是 | 存在/不存在 | T0/T1 |
| `cluster restore` | `--type,-t` | 否 | all/pki/etcd/kubernetes/非法值 | T1 |
| `cluster restore` | `--snapshot,-s` | 条件必填（etcd） | 提供/缺失 | T1 |
| `cluster restore` | `--dry-run` | 否 | true/false | T1 |
| `cluster reconfigure` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `cluster reconfigure` | `--new-config,-n` | 条件必填（全量） | 提供/缺失 | T1 |
| `cluster reconfigure` | `--component,-c` | 否 | all/apiserver/scheduler/controller-manager/kubelet/proxy/非法值 | T1 |
| `cluster reconfigure` | `--restart` | 否 | true/false | T1 |
| `cluster reconfigure` | `--backup` | 否 | true/false | T1 |
| `cluster reconfigure` | `--dry-run` | 否 | true/false | T1 |
| `cluster upgrade` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `cluster upgrade` | `--version,-t` | 是 | 合法/非法/缺失 | T1 |
| `cluster upgrade` | `--dry-run` | 否 | true/false | T1 |
| `cluster upgrade etcd` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `cluster upgrade etcd` | `--to-version,-t` | 是 | 合法/非法/缺失 | T1 |
| `cluster upgrade etcd` | `--dry-run` | 否 | true/false | T1 |

### 8.6 node/certs/config/images/completion/version

| 命令路径 | 参数 | 必填 | 分支要求 | 机器数 |
|---|---|---|---|---|
| `node list` | `--cluster,-c` | 是 | 存在/不存在 | T0/T2 |
| `node list` | `--kubeconfig` | 否 | 默认路径/显式路径/不存在路径 | T0/T2 |
| `node get` | `NODE_NAME` | 是 | 存在/不存在 | T2 |
| `node get` | `--output,-o` | 否 | yaml/summary/json(未实现)/非法值 | T2 |
| `node cordon` | `NODE_NAME` + `--cluster` | 是 | 首次/重复执行幂等 | T2 |
| `node uncordon` | `NODE_NAME` + `--cluster` | 是 | 首次/重复执行幂等 | T2 |
| `node drain` | `NODE_NAME` + `--cluster` | 是 | 正常/超时/未托管 pod + force 分支 | T2 |
| `node drain` | `--ignore-daemonsets` | 否 | true/false | T2 |
| `node drain` | `--grace-period` | 否 | -1/0/正值 | T2 |
| `node drain` | `--timeout` | 否 | 默认/短超时 | T2 |
| `node drain` | `--wait-for-delete-timeout` | 否 | 默认/短超时 | T2 |
| `certs check-expiration` | `--cluster,-c` | 是 | 存在/不存在 | T0/T1 |
| `certs check-expiration` | `--warn-within` | 否 | 0/7/30/负值 | T1 |
| `certs renew/update/rotate` | `--config,-f` | 是 | 存在/不存在 | T0/T1 |
| `certs renew/update/rotate` | `--type,-t` | 否 | all + 各子类型 + 非法值 | T1 |
| `certs renew/update/rotate` | `--force` | 否 | true/false（交互） | T1 |
| `certs renew/update/rotate` | `--dry-run` | 否 | true/false | T1 |
| `certs rotate` | `--service` | 否 | apiserver/etcd/kubelet/非法值 | T1 |
| `config view` | `--output,-o` | 否 | yaml/非法值 | T0 |
| `config set` | `<key> <value>` | 是 | known key/unknown key/非法 value | T0 |
| `config use-context` | `<context-name>` | 是 | 存在/不存在 | T0 |
| `images push` | `--list,-l` | 是 | 存在/不存在/空文件 | T0/T3 |
| `images push` | `--registry,-r` | 是 | host:port/非法值 | T3 |
| `images push` | `--concurrency` | 否 | 1/5/10/0/负值 | T3 |
| `images push` | `--auth-file` | 否 | 存在/不存在 | T3/T4 |
| `images push` | `--skip-tls-verify` | 否 | true/false | T3/T4 |
| `images push` | `--dry-run` | 否 | true/false | T0/T3 |
| `completion` | 位置参数 shell | 是 | bash/zsh/fish/powershell/非法值 | T0 |
| `version` | 无 | - | 常规输出 | T0 |

---

## 9. API 负例清单（按字段簇拆分）

> 本节把 API 定义中的“易错输入”显式转成负例测试输入，便于测试数据团队直接造数。

### 9.1 ClusterSpec/hosts/roleGroups

- `hosts` 为空数组。
- `hosts[].name` 重复。
- `hosts[].address` 重复。
- `hosts[].address` 使用 `127.0.0.1` / `localhost`。
- `hosts[].internalAddress` 使用 `127.0.0.1` / `localhost`。
- `roleGroups.master` 为空。
- `roleGroups` 引用了不存在主机名。
- `hosts` 中存在未被任何 roleGroups 引用的主机。

### 9.2 Global/System

- `global.port` 超出 1~65535。
- `global.connectionTimeout <= 0`。
- `global.workDir` 非绝对路径。
- `system.ntpServers` 含空字符串或非法域名/IP。
- `system.packageManager=apt` 但设置了 `rpms`。
- 同时设置 `rpms` 与 `debs` 但未显式 packageManager。
- `system.sysctlParams` 中 value 为空字符串。

### 9.3 Kubernetes/Etcd/Network

- `kubernetes.serviceNodePortRange` 非 `start-end` 格式。
- `kubernetes.audit.enabled=true` 且缺失 `policyFile/policyFileContent`。
- `etcd.type=external` 但仍配置 `cluster/backup/performance`。
- `etcd.external.endpoints` 为空或含空值。
- `etcd.external` 配置 mTLS 时仅给 `cert` 或仅给 `key`。
- `network.plugin` 为未知值。
- `network.plugin=calico` 但 `network.calico` 配置段缺失（其他插件同理）。

### 9.4 HA/Registry/Storage/Addons/Preflight

- `highAvailability.enabled=true` 但 internal/external 均未启用。
- `highAvailability.external.type` 非法。
- 托管 external LB 未设置 `loadBalancerHostGroupName`。
- `registry.auths[addr]` 中同时提供 `username/password` 与 `auth`。
- `registry.auths[addr].auth` 非法 base64。
- `registry.namespaceRewrite.enabled=true` 但 rules 为空。
- `storage.defaultStorageClass` 已设但无任何 provider enabled。
- `storage.nfs.enabled=true` 且缺失 `server/path`。
- `storage.openebs.enabled=true` 且 engines 全部未启用。
- `addons.enabled=true` 但 sources 为空。
- add-on 单个 source 同时定义 chart 与 yaml。
- `preflight.minCPUCores<=0` 或 `minMemoryMB<=0`。
- `preflight.skipChecks` 包含不支持值。

---

## 10. 回归执行顺序建议（降低环境成本）

1. **P0（T0）**：root/config/version/completion + 参数缺失/非法值快速失败。
2. **P1（T1）**：create/health/backup/restore/reconfigure/upgrade/certs 主流程 + dry-run。
3. **P2（T2）**：node 命令与扩缩容、删节点。
4. **P3（T3）**：registry/images/build/create-iso/download 离线链路。
5. **P4（T4）**：external etcd / external registry / TLS 兼容。

> 通过门禁建议：P0+P1 必须 100% 通过方可进入 P2~P4。

---

## 11. 用例模板（统一格式，便于转测试平台）

> 以下模板用于把本文件中的矩阵用例“落表”到 TestRail/禅道/Jira Xray 等系统。

### 11.1 标准模板

```text
用例ID:
标题:
命令:
前置条件:
输入数据:
执行步骤:
预期结果:
后置清理:
最小机器数:
优先级(P0/P1/P2):
标签(命令/参数/分支/API域):
```

### 11.2 示例（写操作 + 交互 + dry-run）

```text
用例ID: DEL-CL-004
标题: delete cluster 在 dry-run 模式下不产生真实删除
命令: kubexm delete cluster -n c1 --dry-run
前置条件: c1 集群已存在，包含至少 1 个 control-plane
输入数据: clusterName=c1
执行步骤:
  1) 执行命令
  2) 检查输出中包含 dry-run 语义
  3) 二次查询 cluster get c1
预期结果:
  - 命令返回成功
  - 集群 c1 仍存在
后置清理: 无
最小机器数: T1
优先级: P1
标签: delete-cluster,dry-run,safety
```

---

## 12. 参数组合策略（避免组合爆炸）

### 12.1 组合优先级

1. **必须组合**：`必填参数 + 核心业务参数 + 分支开关(dry-run/force/yes)`。  
2. **高风险组合**：路径参数 + 权限/不存在路径；网络参数 + TLS/鉴权。  
3. **低风险组合**：多个非关键可选参数并行开启（用 pairwise 覆盖即可）。

### 12.2 Pairwise 建议维度

- `dry-run` × `force` × `yes`
- `output`（默认/自定义）× `config`（有效/无效）
- `component`（all/单项）× `timeout`（默认/短）
- `images push`: `auth-file`（有/无）× `skip-tls-verify`（true/false）× `concurrency`（1/默认/高值）

### 12.3 边界值策略

- 端口：`0,1,65535,65536`
- 超时：`0,1s,默认值,极大值`
- 并发：`0,1,默认值,高值(10/20)`
- 枚举：`每个合法值各 1` + `1 个非法值`

---

## 13. 逐命令“最小可执行数据集”定义

> 本节明确每个命令至少要准备哪些输入文件，避免执行时临时拼凑。

| 命令族 | 最小输入文件 | 必备内容 | 复用建议 |
|---|---|---|---|
| create/upgrade/reconfigure/backup/restore | `cluster-*.yaml` | hosts、roleGroups、kubernetes、network、etcd 基线 | `cluster-ha.yaml` 作为主复用 |
| create iso/build iso/download | `cluster-offline.yaml` + `base.iso` | offline 相关组件、registry/storage/cni/runtime | 与 T3/T4 共用 |
| delete cluster | 无（按 name） | 已存在的集群目录/状态 | 由 create 用例产出 |
| delete-nodes/add-nodes/scale | `cluster-scale-*.yaml` | 目标新增/删除节点定义 | 与 node 命令共用 |
| node * | kubeconfig 文件 | 可访问 API Server | 从 `cluster kubeconfig` 产出 |
| certs * | cluster config + pki 目录 | 可读证书文件（check-expiration） | 由 create 后产出 |
| images push | `images.txt` | 至少 3 条有效镜像 + 1 条注释 + 空行 | 一套文件覆盖所有分支 |
| config * | `~/.kubexm/config.yaml` | contexts/defaultPackageDir | 使用临时 HOME 隔离 |

---

## 14. 命令级验收点（DoD）补充

### 14.1 create / delete / scale / upgrade（破坏性/变更性）

- 必须证明“变更前后状态差异”：
  - 资源数量变化（节点数、Pod 数、服务状态）；
  - 关键文件变化（kubeconfig、备份包、manifest 输出目录）；
  - 命令返回码（0/非0）和关键日志语句。
- dry-run 必须证明“状态不变”。

### 14.2 node（运维性）

- `cordon/uncordon` 必须校验 `spec.unschedulable`。
- `drain` 必须校验：
  - DaemonSet pod 分支；
  - unmanaged pod + force 分支；
  - 超时分支返回码与错误信息。

### 14.3 certs（证书生命周期）

- `check-expiration` 必须覆盖：
  - 正常证书；
  - 即将过期证书（warn-within）；
  - 已过期证书；
  - 证书文件损坏/不可解析分支。
- renew/update/rotate 必须覆盖：
  - type 枚举；
  - 交互与 `--force`；
  - dry-run 不落地。

### 14.4 images / registry / iso / download（离线链路）

- 必须校验制品完整性：
  - ISO 文件存在、大小 > 0；
  - download 包存在且可解压；
  - 镜像 push 后目标仓库可查询；
  - registry 删除后不可访问（或服务停止）。

---

## 15. 失败注入建议（提升分支真实性）

### 15.1 环境失败注入

- SSH 不可达（单节点端口阻断）。
- 磁盘空间不足（ISO 输出目录/备份目录）。
- kubeconfig 文件缺失或权限错误。
- registry TLS 证书不受信或域名不匹配。

### 15.2 数据失败注入

- cluster config YAML 语法错误。
- API 字段类型错误（字符串填到数字字段）。
- 枚举字段非法值（plugin/runtime/storage/lb/type）。
- 关键文件路径存在但内容无效（空 ISO、空 images.txt、损坏证书）。

### 15.3 流程失败注入

- 交互流程输入 `no`。
- 先执行 `node get` 再删除节点，验证资源消失分支。
- 先 `backup` 再 `restore`，验证完整闭环。

---

## 16. 可追踪性矩阵（需求 → 用例）

| 需求点 | 对应章节 | 关键用例ID 区间 |
|---|---|---|
| 覆盖所有命令与子命令 | §4, §8 | ROOT/CRT/DEL/BLD/DLD/CLS/NOD/CRTS/CFG/IMG/CMP/VER |
| 覆盖参数与分支 | §4, §5, §8, §12 | 全部区间 |
| 参数参考 apis 定义 | §3, §9 | API-* 负例清单 |
| 每条用例给出机器规模 | §2, §4, §8 | T0~T4 全覆盖 |
| 不写代码，仅文档 | 全文 | 文档性输出 |

---

## 17. 执行记录建议字段（测试报告模板）

```text
执行批次:
执行日期:
执行环境(T0~T4):
执行人:
用例ID:
结果(PASS/FAIL/BLOCKED):
失败类别(环境/数据/产品缺陷/脚本问题):
日志链接:
截图/制品路径:
缺陷单号:
备注:
```

> 建议：每次回归至少输出“按命令组聚合通过率 + 按分支类型聚合通过率 + 按机器拓扑聚合通过率”三份统计。
