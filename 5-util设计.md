### pkg/util已存在，存放工具函数

# Kubexm 通用工具库 (`pkg/util`)

本文档描述了 Kubexm 项目中 `pkg/util` 包的设计与主要功能。该包旨在提供一系列与项目业务逻辑相对解耦的、可在多处复用的通用工具函数。

## 1. 设计目标

`pkg/util` 包的目标是：

*   集中存放项目中通用的辅助函数和数据结构。
*   提高代码的复用性，避免在不同模块中重复实现相同的基础功能。
*   为特定类型的通用任务（如获取二进制文件信息、文件系统操作、模板渲染）提供标准化的实现。

## 2. 主要功能模块与函数

`pkg/util` 包内的功能主要围绕以下几个方面：

### 2.1. 二进制文件信息管理 (`binary_info.go`)

这是 `pkg/util` 中最核心的部分，用于管理和获取项目所依赖的各种外部二进制组件的下载信息和本地存储路径。

*   **`BinaryType` (枚举类型)**: 定义了支持的二进制文件类型，如 `ETCD`, `KUBE` (kubeadm, kubelet, kubectl 等), `CNI`, `HELM`, `DOCKER`, `CONTAINERD`, `RUNC`, `CRICTL`, `CRIDOCKERD`, `CALICOCTL` 等。
*   **`BinaryInfo` (结构体)**: 存储一个特定二进制文件的详细信息，包括：
    *   `Component`: 组件名称 (如 "kubeadm")。
    *   `Type`: `BinaryType`。
    *   `Version`, `Arch`, `OS`: 版本、架构、操作系统。
    *   `Zone`: 下载区域 (如 "cn" 或默认)。
    *   `FileName`: 预期的下载文件名。
    *   `URL`: 构造好的下载链接。
    *   `IsArchive`: 指示文件是否为归档文件 (如 `.tar.gz`)。
    *   `BaseDir`, `ComponentDir`, `FilePath`: 用于在本地存储和定位该二进制文件的标准路径。
*   **`knownBinaryDetails` (内部变量)**: 一个映射表，存储了各种已知组件的下载 URL 模板、文件名模板、默认操作系统、是否为归档等元数据。这是 `GetBinaryInfo` 函数数据的主要来源。
*   **`GetBinaryInfo(componentName, version, arch, zone, workDir, clusterName string) (*BinaryInfo, error)`**:
    *   核心函数，根据输入的参数和 `knownBinaryDetails` 中的模板，动态生成特定二进制组件的 `BinaryInfo` 实例。
    *   它会处理不同下载区域 (通过 `GetZone()`) 和架构别名 (通过 `ArchAlias()`) 的逻辑。
*   **`GetZone() string`**: 从环境变量 `KXZONE` 获取下载区域设置 (如 "cn")。
*   **`ArchAlias(arch string) string`**: 将标准架构名 (如 "amd64") 转换为某些下载链接中可能使用的别名 (如 "x86_64")。

### 2.2. 文件系统操作 (`filesystem.go`)

提供与文件系统交互相关的工具函数。

*   **`CreateDir(path string) error`**:
    *   创建一个目录。如果目录已存在，则不执行任何操作并返回 `nil`。
    *   如果创建过程中发生错误，则返回错误。
    *   创建目录时使用的权限是 `0755`。

### 2.3. 模板渲染 (`template.go`)

提供基于 Go `text/template` 的通用模板渲染功能。

*   **`RenderTemplate(tmplStr string, data interface{}) (string, error)`**:
    *   接收一个模板字符串和任意类型的数据。
    *   使用数据填充模板，并返回渲染后的字符串。
    *   如果模板解析或执行出错，则返回错误。此函数被 `binary_info.go` 用于生成动态的 URL 和文件名。

### 2.4. 文件下载 (`download.go`) (占位符)

包含与文件下载相关的函数。

*   **`DownloadFileWithConnector(...)`**:
    *   **注意**: 当前版本中，此函数是一个**占位符实现**，并未真正执行下载操作。它仅记录日志并模拟成功返回。
    *   注释表明，实际的下载功能更可能由 `pkg/runner` 中的下载工具或 `pkg/resource` 中的资源句柄来实现和管理。

## 3. 使用场景

*   `binary_info.go` 主要被 `pkg/resource`（如下载步骤）和 `pkg/runtime`（如构建上下文时确定二进制文件路径）使用，以确保所有组件都能从正确的位置下载和引用。
*   `filesystem.go` 中的 `CreateDir` 被项目中需要在执行前确保目录存在的各个模块（如 `pkg/step` 中的某些步骤）广泛使用。
*   `template.go` 中的 `RenderTemplate` 是一个通用工具，主要被 `binary_info.go` 使用，但也可以被其他需要简单模板替换功能的模块使用。
