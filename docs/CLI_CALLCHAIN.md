# kubexm CLI 调用链梳理（从 bin/kubexm 开始）

本文件基于当前代码扫描整理，便于后续重构与完善。

## 总入口

```
bin/kubexm/main.go
  -> internal/cmd.Execute()
      -> internal/cmd/root.go (cobra root)
```

## 全局 Flags
- `--verbose/-v`：全局日志级别
- `--yes/-y`：全局自动确认

## 命令与调用链

### `kubexm download`
```
internal/cmd/download.go
  -> config.ParseFromFileWithOptions(... SkipHostValidation=true)
  -> runtime.NewBuilderFromConfig(...).WithSkipHostConnect(true).WithSkipConfigValidation(true)
  -> pipeline/assets.DownloadAssetsPipeline
  -> module/assets.AssetsDownloadModule
  -> task/step/runner/connector
```
备注：符合“download 不校验 host.yaml”的要求。

### `kubexm cluster create`
```
internal/cmd/cluster/create.go
  -> config.ParseFromFile
  -> runtime.NewBuilderFromConfig
  -> pipeline/cluster.CreateClusterPipeline
     -> module/preflight
     -> module/infrastructure
     -> module/loadbalancer
     -> module/kubernetes
     -> module/network
     -> module/addon
```

### `kubexm cluster delete`
```
internal/cmd/cluster/delete.go
  -> config.ParseFromFile
  -> runtime.NewBuilderFromConfig
  -> pipeline/cluster.DeleteClusterPipeline
```

### `kubexm cluster upgrade`
```
internal/cmd/cluster/upgrade.go
  -> config.ParseFromFile
  -> runtime.NewBuilderFromConfig
  -> pipeline/cluster.UpgradeClusterPipeline
```

### `kubexm cluster list/get/kubeconfig`
```
internal/cmd/cluster/list.go
internal/cmd/cluster/get.go
internal/cmd/cluster/kubeconfig.go
```
当前仅读取本地 artifacts 路径，未连集群。

### `kubexm cluster add-nodes/delete-nodes/scale`
```
internal/cmd/cluster/add_nodes.go
internal/cmd/cluster/delete_nodes.go
internal/cmd/cluster/scale.go
```
**状态：占位实现（TODO）**

### `kubexm node list/get/cordon/drain`
```
internal/cmd/node/*
```
使用 kubeconfig 调用 Kubernetes API。

### `kubexm certs check-expiration/rotate/update`
```
internal/cmd/certs/*
```
`check-expiration` 有实现；`rotate/update` 为占位（TODO）。

### `kubexm config view/set/use-context`
```
internal/cmd/config/*
```
当前均为占位（TODO）。

### `kubexm completion/version`
```
internal/cmd/completion.go
internal/cmd/version.go
```

## 重要实现注意点（与重构目标相关）
- 真实执行链路由 `pipeline -> module -> task -> step -> runner -> connector` 驱动。
- `loadbalancer` 的任务选择逻辑在 `internal/task/loadbalancer/loadbalancer_tasks.go`。
- `offline` 相关逻辑由 `ctx.IsOfflineMode()` 控制下载步骤。

## 当前缺口（待完善）
- `cluster add-nodes / delete-nodes / scale` 未实现
- `config` 子命令未实现
- `certs rotate / update` 未实现
