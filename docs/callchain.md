# kubexm CLI 调用链条（从 `bin/kubexm` 出发）

## 全局入口
- 入口：`bin/kubexm/main.go`
- Root CLI：`internal/cmd/root.go`
- 全局参数：
  - `-v, --verbose`：开启调试日志
  - `-y, --yes`：非交互确认

## 命令与调用链

### 1) `kubexm download -f <config> [-o <bundle>] [--dry-run]`
调用链：
1. `internal/cmd/download.go`
2. `config.ParseFromFileWithOptions(...SkipHostValidation=true...)`
3. `runtime.NewBuilderFromConfig(...).WithSkipHostConnect(true).WithSkipConfigValidation(true)`
4. `pipeline/assets.DownloadAssetsPipeline`
5. `module/assets.AssetsDownloadModule`
6. Tasks:
   - `PrepareAssets`（下载二进制、镜像、Helm Charts）
   - `VerifyArtifacts`
   - `PackageAssets`（生成离线包，默认 `./kubexm-bundle.tar.gz`）

说明：
- `download` 不校验 `host.yaml`，仅在堡垒机本地执行下载与打包。
- 输出离线包后，用户可复制 `packages/` 与离线包进入内网。

### 2) `kubexm create -f <config> [--skip-preflight] [--dry-run]`
调用链：
1. `internal/cmd/cluster/create.go`
2. `config.ParseFromFile(...)`
3. `runtime.NewBuilderFromConfig(...)`
4. `pipeline/cluster.CreateClusterPipeline`
5. Modules（顺序执行）：
   - `Preflight`（离线包解压/在线下载/工具安装/校验）
   - `Infrastructure`（Etcd/Runtime 等）
   - `LoadBalancer`（external/internal/kube-vip）
   - `ControlPlane`
   - `Network`
   - `Worker`
   - `Addons`

说明：
- 在线模式：`PrepareAssets` 自动下载所需资源。
- 离线模式：`ExtractBundle` 只在控制节点执行解压；后续所有节点资源均由堡垒机分发。
- `--skip-preflight` 会跳过预检任务（`PreflightChecks`）。

### 3) `kubexm delete -f <config>`
调用链：
1. `internal/cmd/cluster/delete.go`
2. `config.ParseFromFile(...)`
3. `runtime.NewBuilderFromConfig(...)`
4. `pipeline/cluster.DeleteClusterPipeline`

### 4) `kubexm upgrade -f <config> -t <version> [--dry-run]`
调用链：
1. `internal/cmd/cluster/upgrade.go`
2. `config.ParseFromFile(...)`
3. `runtime.NewBuilderFromConfig(...)`
4. `pipeline/cluster.UpgradeClusterPipeline`

### 5) 其它命令
- `kubexm node ...` → `internal/cmd/node/*`
- `kubexm certs ...` → `internal/cmd/certs/*`
- `kubexm config ...` → `internal/cmd/config/*`

## 关键约束（实现已对齐）
- 禁止 `localhost` / `127.0.0.1` 出现在 `host.yaml`。
- 未指定主机时，自动探测本机公网/大网地址，并通过 SSH 访问本机。
- 生产路径禁止 LocalConnector，连接统一走 SSH。
- `loadbalancer_mode=exists` 映射为 `external`，直接跳过部署。
- internal 模式在 worker 节点部署本地 LB，kubelet 连接本地代理。
