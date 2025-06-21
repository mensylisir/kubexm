# Kubernetes 部署流程的最终架构设计 (The Yggdrasil Blueprint)

本设计旨在将您提供的 Kubernetes 部署说明，精确地、完整地映射到我们最终确定的、基于执行图/DAG的“奥丁”架构上。

## 第一部分：可复用的原子 `Step` 库设计

这是构建所有 `Task` 的基础工具箱。`Task` 的 `Plan` 方法将会创建这些 `Step` 的实例。

*   **`step.CommandStep`**: (如您提供的范例) 执行任意Shell命令，支持 `sudo`, `timeout`, `checkCmd`, `rollbackCmd` 等丰富参数。
*   **`step.PrintMessageStep`**: (本地执行) 在控制台打印静态信息或Logo。
   *   **参数**: `Message string`。
*   **`step.ReportTableStep`**: (本地执行) 将收集到的信息格式化为表格并打印。
   *   **参数**: `Headers []string`, `Rows [][]string`。
   *   **设计**: `Rows` 可以由 `Task` 在规划时静态填充，或者 `Step` 的 `Run` 方法被设计为从 `runtime` 缓存中动态获取数据。
*   **`step.UserInputStep`**: (本地执行) 暂停执行并等待用户输入确认。
   *   **参数**: `Prompt string`, `AssumeYes bool` (用于 `--yes` 标志)。
*   **`step.FileChecksumStep`**: (本地执行) 计算本地文件的校验和。
   *   **参数**: `FilePath string`, `ExpectedChecksum string`, `Algorithm string` (e.g., "md5", "sha256")。
*   **`step.DownloadFileStep`**: (本地执行) 从URL下载文件到本地控制节点。
   *   **参数**: `URL string`, `DestinationPath string`, `Checksum string`, `ChecksumAlgo string`。
*   **`step.UploadFileStep`**: 将本地文件上传到一个或多个远程主机。
   *   **参数**: `SourcePath string`, `DestinationPath string`, `Permissions string`, `Sudo bool`。
*   **`step.ExtractArchiveStep`**: 在目标主机上解压压缩包。
   *   **参数**: `SourcePath string`, `DestinationDir string`, `Sudo bool`。
*   **`step.RenderTemplateStep`**: 将Go模板（或任何模板引擎）渲染成文件，并直接写入目标主机。
   *   **参数**: `TemplateContent string`, `Data interface{}`, `DestinationPath string`, `Permissions string`, `Sudo bool`。
*   **`step.EnableServiceStep`**: 启用并立即启动一个 systemd 服务。
   *   **参数**: `ServiceName string`。
*   **`step.DisableServiceStep`**: 禁用并立即停止一个 systemd 服务。
   *   **参数**: `ServiceName string`。
*   **`step.InstallPackageStep`**: 安装一个或多个软件包。
   *   **参数**: `Packages []string`。
*   **`step.ModprobeStep`**: 加载内核模块 (`modprobe <name>`)。
   *   **参数**: `ModuleName string`。
*   **`step.SysctlStep`**: 配置内核参数 (`sysctl -w <key>=<value>`)。
   *   **参数**: `Key string`, `Value string`。
*   **`step.GenerateCACertStep`**: (本地执行) 生成一个自签名的CA证书。
   *   **参数**: `CommonName string`, `OutputDir string`。
*   **`step.GenerateSignedCertStep`**: (本地执行) 使用一个CA来签发一个新的证书。
   *   **参数**: `CAName string` (引用CA的逻辑名), `CAKeyPath string`, `CACertPath string`, `CommonName string`, `SANs []string`, `OutputDir string`。
*   **`step.KubeadmInitStep`**: 封装 `kubeadm init` 命令，并能捕获其输出。
   *   **参数**: `ConfigPathOnHost string`。
   *   **设计**: `Run` 方法执行 `kubeadm init`。成功后，它会解析输出，并将 `join command`, `token`, `cert-hash` 等关键信息写入到 `runtime` 的缓存中。
*   **`step.KubectlApplyStep`**: 封装 `kubectl apply -f` 命令。
   *   **参数**: `ManifestPathOnHost string`。
*   **`step.HelmInstallStep`**: (如果支持Helm) 封装 `helm install` 命令。

---

## 第二部分：`Task` 的详细设计与图构建

以下是将您的部署说明拆分成的 `Task` 设计。

### **阶段一：准备与预检**

#### 1. `pkg/task/greeting/GreetingTask`
*   **职责**: 显示欢迎 Logo。
*   **`Plan()`**: 构建一个 `ExecutionFragment`，其中包含一个 `ExecutionNode`：
   *   **Node**: `print-logo`
   *   **Step**: `step.PrintMessageStep` (配置了你的Logo字符串)。
   *   **Hosts**: 本地控制节点。

#### 2. `pkg/task/pre/PreTask`
*   **职责**: 执行前置检查。
*   **`Plan()`**:
   1.  为 **检查项 2.1 - 2.10** 的每一项创建一个 `step.CommandStep`，用于在**所有节点**上执行检查命令 (e.g., `rpm -q docker`, `cat /proc/swaps`, `sestatus`)。
   2.  创建多个 `ExecutionNode`，每个节点对应一项检查。**这些节点之间没有依赖关系**，因此可以完全并发执行。
   3.  创建一个最终的 `report-pre-checks` 节点，使用 `step.ReportTableStep`。
   4.  **【依赖】**: `report-pre-checks` 节点**依赖于所有**上述检查节点。
   5.  `ReportTableStep` 的 `Run` 方法会从缓存中读取之前所有检查步骤的结果，并格式化输出。

#### 3. `pkg/task/pre/ConfirmTask`
*   **职责**: 获取用户安装确认。
*   **`Plan()`**: 构建一个 `ExecutionNode`：
   *   **Node**: `confirm-installation`
   *   **Step**: `step.UserInputStep` (配置了确认信息，并从命令行参数读取 `AssumeYes` 的值)。
   *   **Hosts**: 本地控制节点。

#### 4. `pkg/task/pre/VerifyArtifactsTask`
*   **职责**: (离线) 校验离线包。
*   **`Plan()`**: 构建一个 `ExecutionNode`：
   *   **Node**: `verify-offline-pack`
   *   **Step**: `step.FileChecksumStep` (路径和期望的 checksum 从 `config` 读取)。
   *   **Hosts**: 本地控制节点。

#### 5. `pkg/task/pre/CreateRepositoryTask`
*   **职责**: (离线) 在所有节点创建临时本地仓库。
*   **`Plan()`**: 构建一个**链式依赖**的 `ExecutionFragment`：
   1.  **Node `upload-iso`**: `step.UploadFileStep` (上传 ISO 到所有节点)。
   2.  **Node `mount-iso`**: `step.CommandStep` (`mount ...`)。**依赖**: `upload-iso`。
   3.  **Node `backup-repo`**: `step.CommandStep` (`mv ...`)。**依赖**: `mount-iso`。
   4.  **Node `create-repo-file`**: `step.RenderTemplateStep`。**依赖**: `backup-repo`。
   5.  **Node `install-deps`**: `step.InstallPackageStep`。**依赖**: `create-repo-file`。
   *   *清理步骤（umount, restore repo）可以设计为单独的 `Task`，在流水线末尾执行。*

#### 6. `pkg/task/preflight/PreCheckTask`
*   **职责**: 执行 `kubeadm` 风格的系统预检和配置。
*   **`Plan()`**: 为 **检查项 1-10** 的每一项创建一个 `ExecutionNode`。
   *   **Node `check-os`**: `step.CommandStep`。
   *   **Node `check-resources`**: `step.CommandStep`。
   *   **Node `disable-firewall`**: `step.DisableServiceStep`。
   *   **Node `disable-selinux`**: `step.CommandStep` (`setenforce 0 && sed ...`)。
   *   **Node `disable-swap`**: `step.CommandStep` (`swapoff -a && sed ...`)。
   *   **Node `load-modules`**: `step.ModprobeStep`。
   *   **Node `config-sysctl`**: `step.SysctlStep`。
   *   **【并发】**: 这些节点大部分可以并发执行，在**所有节点**上运行。

### **阶段二：核心组件安装**

#### 7. `pkg/task/container_runtime/InstallTask`
*   **职责**: 部署容器运行时。
*   **`Plan()`**:
   1.  `Task` 从 `config` 决定运行时类型和版本，并构造资源句柄 `resource.RemoteBinaryHandle`。
   2.  **Node `download-runtime`**: 调用 `resource.Handle.EnsurePlan()` 生成，在**本地控制节点**执行。
   3.  **Node `upload-runtime`**: `step.UploadFileStep`。**依赖**: `download-runtime` 的出口节点。
   4.  **Node `install-runtime`**: `step.CommandStep` (解压、移动文件)。**依赖**: `upload-runtime`。
   5.  **Node `configure-runtime`**: `step.RenderTemplateStep` (生成 `daemon.json` 等)。**依赖**: `install-runtime`。
   6.  **Node `start-runtime`**: `step.EnableServiceStep`。**依赖**: `configure-runtime`。

#### 8. `pkg/task/etcd/InstallETCDTask`
*   **职责**: 部署 ETCD 集群。
*   **`Plan()`**: `Task` 从 `config` 读取 `etcd.type`。
   *   **如果 type 是 `kubexm` (二进制部署)**:
      1.  **并行分支 1 (证书)**:
         *   `gen-ca`: `step.GenerateCACertStep` (本地)。
         *   `gen-etcd-certs`: `step.GenerateSignedCertStep` (本地)。**依赖**: `gen-ca`。
         *   `upload-etcd-certs`: `step.UploadFileStep`。**依赖**: `gen-etcd-certs`。
      2.  **并行分支 2 (二进制)**:
         *   `download-etcd`: `step.DownloadFileStep` (本地)。
         *   `upload-etcd-binary`: `step.UploadFileStep`。**依赖**: `download-etcd`。
      3.  **合并点**:
         *   `configure-etcd`: `step.RenderTemplateStep` (生成etcd配置文件)。**依赖**: `upload-etcd-certs` 和 `upload-etcd-binary`。
         *   `start-etcd`: `step.RenderTemplateStep` (生成systemd文件) + `step.EnableServiceStep`。**依赖**: `configure-etcd`。
   *   **如果 type 是 `stack` (静态Pod)**:
      *   `render-etcd-pod-manifest`: `step.RenderTemplateStep` (生成 `etcd.yaml`)。
      *   `upload-etcd-pod-manifest`: `step.UploadFileStep` (上传到 master 节点的 `/etc/kubernetes/manifests`)。**依赖**: `render-etcd-pod-manifest`。
   *   **如果 type 是 `external`**: `Plan()` 返回一个空的 `ExecutionFragment`。

#### 9. `pkg/task/kubernetes/InstallBinariesTask`
*   **职责**: 分发 K8s 二进制文件。
*   **`Plan()`**: 构建一个依赖链：
   *   `download-kube-bins` (本地) -> `upload-kube-bins` (分发到所有节点) -> `chmod-kube-bins`。

#### 10. `pkg/task/kubernetes/PullImagesTask`
*   **职责**: 提前拉取 K8s 核心镜像。
*   **`Plan()`**:
   *   `Task` 从 `config` 获取镜像列表和私有仓库地址。
   *   为**所有节点**创建并发的 `ExecutionNode`，每个都执行 `step.CommandStep` (`crictl pull ...`)。**这些节点没有内部依赖**。

### **阶段三：Kubernetes 集群引导**

#### 11. `pkg/task/kubernetes/InitMasterTask`
*   **职责**: 初始化第一个 Master 节点。
*   **`Plan()`**:
   1.  `Task` 从 `config` 构建 `kubeadm` 配置。
   2.  **Node `render-kubeadm-config`**: `step.RenderTemplateStep` (上传 `kubeadm-config.yaml` 到第一个 master)。
   3.  **Node `kubeadm-init`**: `step.KubeadmInitStep`。**依赖**: `render-kubeadm-config`。
   4.  **Node `capture-join-info`**: `step.CommandStep` (`kubeadm token create --print-join-command`)，**依赖于** `kubeadm-init`。此 `Step` 会将 join 命令写入 `runtime` 缓存。

#### 12. `pkg/task/kubernetes/JoinMastersTask`
*   **职责**: 其他 Master 节点加入集群。
*   **`Plan()`**:
   *   `Task` 的 `Plan()` 方法**首先从 `runtime` 缓存中读取 join 命令**。
   *   为所有**其他 Master 节点**创建并发的 `ExecutionNode`，执行 `step.CommandStep` (使用缓存中的 join 命令 + `--control-plane`)。

#### 13. `pkg/task/kubernetes.JoinWorkerNodesTask`
*   **职责**: Worker 节点加入集群。
*   **`Plan()`**:
   *   与 `JoinMastersTask` 类似，但 join 命令不带 `--control-plane` 参数，并在所有 **Worker 节点**上执行。

### **阶段四：集群配置与收尾**

#### 14. `pkg/task/network/InstallNetworkPluginTask`
*   **职责**: 部署 CNI 网络插件。
*   **`Plan()`**:
   1.  `Task` 从 `config` 读取 CNI 类型和参数。
   2.  **Node `render-cni-yaml`**: `step.RenderTemplateStep` (渲染 CNI manifest 到本地)。
   3.  **Node `upload-cni-yaml`**: `step.UploadFileStep` (上传 manifest 到一个 master 节点)。**依赖**: `render-cni-yaml`。
   4.  **Node `apply-cni`**: `step.KubectlApplyStep`。**依赖**: `upload-cni-yaml`。

#### 15. `pkg/task/kubernetes/PostScriptTask`
*   **职责**: 执行所有安装后的脚本和配置。
*   **`Plan()`**:
   *   为“移除污点”、“打标签”、“拷贝kubeconfig”、“生成证书续订脚本”等每一个动作创建一个 `step.CommandStep` 或 `step.RenderTemplateStep` 节点。
   *   定义它们之间必要的依赖关系（例如，必须先拷贝 kubeconfig 才能执行 `kubectl` 命令）。

#### 16. `pkg/task/addon/InstallAddonsTask`
*   **职责**: 部署可选的 Addons。
*   **`Plan()`**:
   *   `Task` 遍历 `config` 中的 `addons` 列表。
   *   对每个 addon，都创建一个类似于 `InstallNetworkPluginTask` 的依赖链 (`render -> upload -> apply`)。
   *   **【并发】**: **不同 addon 的部署链之间没有依赖关系**，可以并发执行。

---

### **`Module` 和 `Pipeline` 组装**

*   **`PreflightModule`**: 包含 `GreetingsTask`, `PreTask`, `ConfirmTask`, `VerifyArtifactsTask`, `CreateRepositoryTask`, `PreCheckTask`。
*   **`CoreComponentsModule`**: 包含 `InstallContainerRuntimeTask`, `InstallETCDTask`, `InstallKubeBinariesTask`, `PullImagesTask`。
*   **`ClusterBootstrapModule`**: 包含 `InitMasterTask`, `JoinMastersTask`, `JoinWorkerNodesTask`。
*   **`ClusterReadyModule`**: 包含 `InstallNetworkPluginTask`, `PostScriptTask`, `InstallAddonsTask`。

*   **`CreateClusterPipeline`**:
   *   **`Plan()`**: 按顺序链接以上四个 `Module` 的 `ExecutionFragment`，生成最终的 `ExecutionGraph`。例如，`CoreComponentsModule` 的入口节点依赖于 `PreflightModule` 的出口节点。