创建一个 pkg/asset 或 pkg/resource 包。
定义资源接口和具体资源类型：
```aiignore
package resource

// Handle 是对一个可获取资源的引用。
type Handle interface {
    ID() string // 唯一标识，如 "etcd-v3.5.4-amd64"
    Ensure(ctx runtime.ControlNodeContext) (pathOnControlNode string, err error)
}

// RemoteBinary a代表一个需要从URL下载的二进制文件资源。
type RemoteBinary struct {
    Version string
    Arch    string
    URL     string
    SHA256  string
}

func (rb *RemoteBinary) ID() string { /* ... */ }

// Ensure 负责规划下载和验证的步骤，并返回最终在本地的路径。
// 它可以内部使用缓存，如果已下载，则直接返回路径。
func (rb *RemoteBinary) Ensure(ctx) (string, error) {
    // ... 内部逻辑：检查缓存 -> 规划下载Step -> 规划解压Step -> 执行这些Steps -> 返回路径 ...
}
```
带来的好处:
资源抽象与复用: Task 不再关心资源的具体来源（是下载还是本地构建），它只关心“我需要这个资源”。资源的获取逻辑被封装在资源自身中。
去重和缓存: asset.Handle 的实现可以自动处理资源的去重下载和缓存，简化了所有 Task 的逻辑。
可扩展性: 可以轻松添加新的资源类型，如 GitRepo, LocalDirectory, ContainerImage 等，它们都实现 Handle 接口。