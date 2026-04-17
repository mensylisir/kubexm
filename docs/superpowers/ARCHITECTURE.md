# kubexm 六层执行架构规范

## 一、六层执行架构

```
┌──────────────────────────────────────────────────────────────┐
│  Pipeline (入口层)                                           │
│  create_cluster / delete / scale / upgrade / renew / download│
│  编排 Module，执行参数校验，全局流程控制                     │
└─────────────────────────────┬────────────────────────────────┘
                              │ 调用
┌─────────────────────────────▼────────────────────────────────┐
│  Module (组件层)                                              │
│  kubeadm / kubexm / runtime / cni / lb / addons / etcd      │
│  按功能组件封装，管理相关 Task，负责编排多个 Task              │
└─────────────────────────────┬────────────────────────────────┘
                              │ 编排
┌─────────────────────────────▼────────────────────────────────┐
│  Task (动作层)                                                │
│  cluster/ / kubernetes/ / network/ / runtime/ / addons/ / hosts│
│  原子动作集，只做组件级操作，不编排                            │
└─────────────────────────────┬────────────────────────────────┘
                              │ 调用
┌─────────────────────────────▼────────────────────────────────┐
│  Step (步骤层)                                                │
│  最小不可分割单位，原子化 + 幂等                              │
│  step/cluster/ / kubernetes/ / runtime/ / network/ / cni/  │
│  step/addons/ / certs/ / etcd/ / registry/ / images/ / os/   │
└─────────────────────────────┬────────────────────────────────┘
                              │ runner.Run()
┌─────────────────────────────▼────────────────────────────────┐
│  Runner (执行层)                                              │
│  runner.go — 封装 start/stop/restart/copy/fetch 等远程操作   │
│  屏蔽 Connector 细节，供 Step 层调用                           │
└─────────────────────────────┬────────────────────────────────┘
                              │ connector.Run()
┌─────────────────────────────▼────────────────────────────────┐
│  Connector (传输层)                                          │
│  connector.go + ssh.go — SSH 连接封装                       │
│  统一处理 SSH 凭据、超时、重试、host 验证                      │
└──────────────────────────────────────────────────────────────┘
```

## 二、调用约束

| 约束 | 说明 |
|------|------|
| Pipeline → Module → Task → Step → Runner → Connector | 单向调用，禁止逆流 |
| Step 严禁直接调用 Connector | 必须通过 Runner 间接调用 |
| Task 层只做组件级原子操作，不编排 | 只组合 Step，返回 ExecutionFragment |
| Module 层负责编排 Task | 调用 `t.Plan()`，合并 fragment |
| Pipeline 层编排 Module | 调用 `m.PlanTasks()` |

### Precheck 幂等性约定

每个 Step 必须实现 `Precheck()` 方法：

```go
// Precheck 返回值约定：
// (true, nil)   — 已完成，跳过执行
// (false, nil)  — 需要执行
// (false, err)  — 检查本身失败，返回错误
func (s *SomeStep) Precheck(ctx runtime.ExecutionContext) (bool, error)
```

## 三、支撑体系

| 组件 | 职责 |
|------|------|
| Context | 集群隔离状态、运行时状态、取消信号传递 |
| Logger | 分级日志（Debug/Info/Warn/Error），JSON + 彩色 Console |
| Cache | PipelineCache / ModuleCache / TaskCache / StepCache 分层缓存 |
| Parser | 解析 config.yaml / host.yaml |
| Config | 多源配置管理（YAML / Environment / CLI Flags） |
| Validator | 配置校验（schema + consistency） |

## 四、Pipeline 完整实现

| Pipeline | 模块流程 |
|----------|----------|
| CreateClusterPipeline | PreflightConnectivity → Preflight → OsModule → Etcd → Runtime → LoadBalancer → ControlPlane → Network → Worker → Addons |
| DeleteClusterPipeline | PreflightConnectivity → Preflight → WorkerCleanup → ControlPlaneCleanup → CNICleanup → RuntimeCleanup → EtcdCleanup → LoadBalancerCleanup → OsCleanup |
| AddNodesPipeline | PreflightConnectivity → Preflight → OsModule → Etcd → Runtime → Worker |
| UpgradeClusterPipeline | PreflightConnectivity → Preflight → ControlPlaneUpgrade → WorkerUpgrade → NetworkUpgrade |
| UpgradeEtcdPipeline | PreflightConnectivity → EtcdModule |
| RenewPKIPipeline | PreflightConnectivity → PKIModule |
| BackupPipeline | PreflightConnectivity → BackupModule |
| RestorePipeline | PreflightConnectivity → RestoreModule |

## 五、关键实现模式

### Step Builder 模式

所有 Step 通过 Builder 创建：

```go
// 链式调用
step, err := NewSomeStepBuilder(ctx, "StepName").
    WithOption(value).
    Build()
```

### Task Plan 模式

Task 组合多个 Step，返回执行片段：

```go
func (t *MyTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
    fragment := plan.NewExecutionFragment(t.Name())
    runtimeCtx := ctx.ForTask(t.Name())

    step, err := somestep.NewMyStepBuilder(runtimeCtx, "StepNodeName").Build()
    node := &plan.ExecutionNode{Name: "StepNodeName", Step: step, Hosts: hosts}
    fragment.AddNode(node)
    fragment.CalculateEntryAndExitNodes()
    return fragment, nil
}
```

### Module PlanTasks 模式

Module 遍历 Task，执行 IsRequired 检查后调用 Plan：

```go
func (b *BaseModule) PlanTasks(ctx runtime.ModuleContext) (*plan.ExecutionFragment, map[string]interface{}, error) {
    taskCtx, ok := ctx.(runtime.TaskContext)
    for _, t := range b.ModuleTasks {
        required, err := t.IsRequired(taskCtx)
        if !required { continue }
        taskFragment, err := t.Plan(taskCtx)
        fragment.MergeFragment(taskFragment)
    }
    fragment.CalculateEntryAndExitNodes()
    return fragment, nil, nil
}
```

## 六、层间数据传递

- **Pipeline → Module**：通过 `runtime.Context` 携带 ClusterConfig、ConnectionPool、Logger
- **Module → Task**：通过 `ctx.ForTask()` 派生子上下文，携带 taskName
- **Task → Step**：通过 Builder 注入配置和 context
- **Step → Runner**：通过 `ctx.GetRunner()` 获取 runner 实例
- **状态共享**：GlobalState / PipelineState / ModuleState / TaskState 四层 StateBag

## 七、已知约束

- [x] Step 禁止直接调用 Connector（已通过 Runner 接口隔离）
- [x] Base.Precheck() 有默认实现（return false, nil）
- [x] Precheck 错误传播（err != nil 时必须返回 false, err）
- [x] hostInfoMu 指针化（避免 Context 派生时锁复制）
- [x] BaseModule.PlanTasks() 调用 IsRequired()
