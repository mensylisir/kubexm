### pkg/common已存在，存放常量

# Kubexm 公共常量 (`pkg/common`)

本文档描述了 Kubexm 项目中 `pkg/common` 包内定义的全局常量。这些常量主要用于规范默认的目录结构、特殊标识符以及在整个项目代码中可能出现的“魔法字符串”，以提高可维护性和一致性。

## 1. 设计目标

`pkg/common` 包的目标是：

*   集中管理项目中使用的通用常量。
*   避免在代码中硬编码字符串和数值，方便后续修改和维护。
*   为核心组件和操作提供标准的命名和路径约定。

## 2. 主要常量定义

以下是 `pkg/common/constants.go` 中定义的主要常量及其用途：

### 2.1. 默认目录结构常量

这些常量定义了 Kubexm 在执行操作时使用的标准目录名称：

*   **`KUBEXM`**: `".kubexm"`
    *   Kubexm 操作的默认根目录名称。通常在用户主目录或指定的工作目录下创建。
*   **`DefaultLogsDir`**: `"logs"`
    *   在 `KUBEXM` 目录下，用于存放日志文件的默认目录名称。
*   **`DefaultCertsDir`**: `"certs"`
    *   用于存放证书文件的默认目录名称（例如，在集群的产物目录下）。
*   **`DefaultContainerRuntimeDir`**: `"container_runtime"`
    *   用于存放容器运行时相关产物（如二进制文件、配置文件）的默认目录名称。
*   **`DefaultKubernetesDir`**: `"kubernetes"`
    *   用于存放 Kubernetes 组件相关产物（如二进制文件、配置文件）的默认目录名称。
*   **`DefaultEtcdDir`**: `"etcd"`
    *   用于存放 Etcd 相关产物（如二进制文件、配置文件、数据备份）的默认目录名称。

### 2.2. 控制节点标识符

这些常量用于标识在控制机器本地执行的操作或节点：

*   **`ControlNodeHostName`**: `"kubexm-control-node"`
    *   一个特殊的、逻辑上的主机名，代表执行 Kubexm 命令的控制机器本身。当操作需要在本地进行而非远程主机时使用。
*   **`ControlNodeRole`**: `"control-node"`
    *   分配给 `ControlNodeHostName` 的特殊角色名。

## 3. 使用方式

项目中的其他包（如 `pkg/runtime`, `pkg/resource`, `pkg/step` 等）在需要引用这些标准路径或标识符时，应导入 `pkg/common` 包并使用这些已定义的常量。

例如，`runtime.Context` 在构建各种产物下载路径或日志文件路径时，会依赖这些常量来确保一致性。

---

本文档反映了当前 `pkg/common/constants.go` 的主要内容。未来如果添加新的通用常量，