# 重构后清理候选目录/脚本

以下为扫描得到的“迁移期/工具型/可能已废弃”的候选项。**不在当前阶段删除**，待新架构稳定后再统一清理。

## 顶层脚本
- `fix_steps.sh`：历史签名批量修复脚本
- `update_helpers.sh`：空文件
- `update_helpers_v2.sh`：空文件

## cmd 目录（非主入口）
- `cmd/fix_steps`：历史 AST 批量修复工具
- `cmd/fixsteps`：历史 AST 批量修复工具（含 `v2/`、`v3/` 目录）

## script 目录
- `script/make_packages.sh`：离线包打包脚本（若后续已由 pipeline 取代，可清理）

## 冗余任务文件（未接入调用链）
- （已清理）`internal/task/kubernetes/kubexm/prepare_components.go`：下载型任务，现已由 Preflight 统一下载
- （已清理）`internal/task/packages/install_packages.go`：旧包安装任务，已由 `internal/task/os/install_prerequisites.go` 统一处理

## 备注
- 若这些工具仍在 CI 或运维流程中使用，需要先迁移/替代再删除。
