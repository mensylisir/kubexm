# pkg/resource 的职责与设计思想

## 1. 它要解决什么问题？(The Problem)

在我们当前的 Task 设计中，Task 的 `Plan` 方法直接负责：

-   **计算资源的来源**: 例如，根据 config 中的版本号，拼装出 etcd 二进制包的下载URL。
-   **计算资源的本地路径**: 例如，决定下载的 etcd 压缩包应该放在 `workdir/.../etcd/v3.5.4/etcd.tar.gz`。
-   **规划获取资源的步骤**: 创建 `DownloadFileStep`、`ExtractArchiveStep` 等 Step 来完成资源的本地准备工作。

这种方式在多数情况下是可行的。但当系统变得复杂时，会遇到一些问题：

-   **重复逻辑**: 多个 Task 可能需要同一个资源。例如，`InstallETCDTask` 需要 etcd 二进制，而一个 `EtcdHealthCheckTask` 可能需要 `etcdctl`，它们都来自同一个 `etcd.tar.gz` 包。这两个 Task 可能都会重复编写下载和解压的规划逻辑。
-   **职责过载**: Task 不仅要规划自己的核心业务逻辑（如配置和启动etcd服务），还要关心这些依赖的二进制文件或证书从哪里来、如何获取。这违反了单一职责原则。
-   **缓存和去重困难**: 如果没有一个集中的地方来管理“资源获取”这个动作，很难实现高效的缓存和去重。我们不希望在一次执行中，因为两个 Task 都需要，就下载同一个文件两次。

## 2. pkg/resource 是如何解决的？(The Solution)

`pkg/resource` 层通过引入**“资源句柄 (Resource Handle)”**的概念来解决上述问题。

-   **核心职责**: 将“资源的定义”与“获取资源的过程”解耦。
-   **目标**: Task 不再关心如何获取一个资源，它只需要声明它需要哪个资源。

## 3. 详细设计

### `pkg/resource/interface.go`

```go
package resource

import (
    "github.com/mensylisir/kubexm/pkg/runtime"
    "github.com/mensylisir/kubexm/pkg/task" // To return an ExecutionFragment
)

// Handle 是对一个“资源”的抽象引用。它代表“某个东西”，但不关心这个东西如何被获取。
type Handle interface {
    // ID 返回该资源的唯一标识符，用于缓存和去重。
    // 例如："etcd-binary-v3.5.4-amd64"
    ID() string

    // Path 返回该资源在成功获取后，在本地控制节点上的最终路径。
    // 例如："/path/to/workdir/.../etcd/v3.5.4/bin/etcd"
    Path(ctx runtime.TaskContext) string
    
    // EnsurePlan 生成一个ExecutionFragment，其中包含了获取此资源所需的所有步骤。
    // 它的实现可以包含缓存逻辑：如果检查到资源已存在，则返回一个空的Fragment。
    EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error)
}
```

### `pkg/resource/remote_binary.go` (一个具体的实现)

```go
package resource

import (
    "fmt"
    "path/filepath"
    // ...
)

// RemoteBinaryHandle 代表一个需要从URL下载、解压并获取的二进制文件资源。
type RemoteBinaryHandle struct {
    Name                string
    Version             string
    Arch                string
    URLTemplate         string // e.g., "https://.../etcd-v%s-linux-%s.tar.gz"
    BinaryPathInArchive string // e.g., "etcd-v3.5.4-linux-amd64/etcd"
}

func (h *RemoteBinaryHandle) ID() string {
    return fmt.Sprintf("%s-binary-%s-%s", h.Name, h.Version, h.Arch)
}

func (h *RemoteBinaryHandle) Path(ctx runtime.TaskContext) string {
    // 根据上下文计算出最终二进制文件应该在的本地路径
    return filepath.Join(ctx.GetGlobalWorkDir(), /* ... */, h.Name, h.Version, "bin", h.Name)
}

func (h *RemoteBinaryHandle) EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
    // 1. 检查本地路径 h.Path(ctx) 是否已存在且有效（例如通过checksum）。
    //    如果已存在，直接返回一个空的 ExecutionFragment 和 nil error。

    // 2. 如果不存在，则构建获取资源的图：
    downloadURL := fmt.Sprintf(h.URLTemplate, h.Version, h.Arch)
    localArchivePath := /* ... 计算压缩包的本地路径 ... */

    // 创建下载和解压的Step
    downloadStep := &step.DownloadFileStep{URL: downloadURL, DestPath: localArchivePath}
    extractStep := &step.ExtractArchiveStep{SourcePath: localArchivePath, DestDir: /* ... */}

    // 构建包含这两个节点的图 (download -> extract)
    fragment := &task.ExecutionFragment{
        // ... nodes, entry, exit ...
    }
    
    return fragment, nil
}
```

## 4. Task 如何使用它？

Task 的 `Plan` 方法现在变得非常简洁和聚焦：

```go
// in pkg/task/etcd/install.go
func (t *InstallETCDTask) Plan(ctx runtime.TaskContext) (*task.ExecutionFragment, error) {
    // 1. 声明需要的资源
    etcdBinaryHandle := resource.NewRemoteBinaryHandle("etcd", "v3.5.4", ...)

    // 2. 生成“确保资源就绪”的规划
    ensureEtcdBinPlan, err := etcdBinaryHandle.EnsurePlan(ctx)
    if err != nil { 
        return nil, err 
    }

    // 3. 获取资源在本地的最终路径
    localEtcdPath := etcdBinaryHandle.Path(ctx)

    // 4. 规划核心业务逻辑的步骤
    uploadStep := &step.UploadFileStep{SourcePath: localEtcdPath, ...}
    configureStep := &step.RenderTemplateStep{...}

    // ... 将 uploadStep 和 configureStep 等节点打包成另一个业务逻辑的 fragment ...

    // 5. 将“资源准备”的图和“业务逻辑”的图链接起来
    //    例如，让 upload 节点依赖于 ensureEtcdBinPlan 的出口节点。
    //    最终返回一个合并后的大图。

    return finalMergedFragment, nil
}
```

## 总结：pkg/resource 的价值

-   **封装复杂性**: 将“如何获取一个资源”的复杂逻辑（下载、解压、校验、缓存）从所有 Task 中抽离出来，封装到一个专门的地方。
-   **提升复用性**: `etcdBinaryHandle` 可以在任何需要它的 Task 中被实例化和使用，而获取逻辑是完全一致和可复用的。
-   **简化 Task**: Task 的职责回归到其核心——编排业务逻辑，而不是关心底层资源的来源。
-   **中央缓存**: 缓存逻辑可以被统一实现在 `EnsurePlan` 方法中，所有资源获取都能自动受益。

总而言之，`pkg/resource` 是一个可选的、优雅的抽象层，它位于 Task 和 Step 之间，专门负责将对“资源”的声明式需求，转化为具体的、可执行的“获取步骤”规划。对于一个大型、复杂的部署系统，引入这一层是非常有价值的。