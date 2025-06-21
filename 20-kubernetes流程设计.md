这是为您的 Kubernetes 部署流程量身定制的、详细的 Task 和 Step 设计，完全遵循执行图/DAG模型和您的项目结构。
核心设计原则
Task 是业务逻辑单元: 一个 Task 对应您描述中的一个大块业务逻辑（例如 InstallETCDTask）。
Step 是原子操作: Task 中的每一步（例如“下载文件”、“分发文件”、“执行命令”）都应该被拆分成一个独立的、可复用的 Step。
依赖驱动: Task 负责构建其内部 Step 对应的 ExecutionNode，并定义它们之间的依赖关系。
配置驱动: 所有 Step 的参数都由 Task 从 runtime.TaskContext 中获取的 config 来填充。
第一部分：需要预先实现的、可复用的原子 Step
在开始设计 Task 之前，我们需要一个强大的、可复用的 Step 库。根据你的部署说明，以下是一些必须存在的、高度可配置的原子 Step Spec：
step.CommandStep: 执行任意shell命令 (你已提供完美范例)。
step.DownloadFileStep: 从URL下载文件到本地控制节点。
参数: URL, DestinationPath, SHA256Checksum。
step.UploadFileStep: 将本地控制节点的文件上传到一个或多个远程主机。
参数: SourcePath, DestinationPath, Permissions, Sudo。
step.ExtractArchiveStep: 在指定主机上解压一个压缩包 (tar.gz, zip)。
参数: ArchivePath, DestinationDir, Sudo。
step.RenderTemplateStep: 将Go模板渲染成文件并上传到目标主机。
参数: TemplateContent, TemplateData, DestinationPath, Permissions, Sudo。
step.EnableServiceStep: systemctl enable --now <service>。
step/DisableServiceStep: systemctl disable --now <service>。
step.InstallPackageStep: 使用runner安装软件包。
step.CheckPortStep: 检查端口是否被占用。
step.ModprobeStep: 加载内核模块。
step.SysctlStep: 配置内核参数。
step.GenerateCertStep: (本地执行) 生成证书。
参数: CAName (可选，用于签名), CommonName, SANs, OutputDir。
step.UserInputStep: (本地执行) 向用户请求确认。
参数: PromptMessage, DefaultYes。
step.ReportTableStep: (本地执行) 打印一个表格到控制台。
参数: Headers []string, Rows [][]string。
第二部分：Task 的详细设计 (按执行顺序)
以下是将您的部署说明拆分成的 Task 设计。每个 Task 的 Plan() 方法都将构建一个 ExecutionFragment（子图）。
1. pkg/task/greeting/
   greeting.GreetingTask:
   职责: 显示欢迎信息。
   IsRequired(): 始终返回 true。
   Plan():
   创建一个 step.PrintMessageStep，内容是你的LOGO。
   创建一个 ExecutionNode，在本地控制节点上执行此 Step。
   返回一个只包含这一个节点的 ExecutionFragment。
2. pkg/task/pre/
   pre.PreflightCheckTask:
   职责: 在所有节点上执行系统环境检查。
   IsRequired(): 始终返回 true。
   Plan():
   为您的检查项2.1-2.10中的每一项，创建一个独立的 step.CommandStep（或更专门的Step）。
   【并发】: 为每个检查项创建一个ExecutionNode，这些节点没有内部依赖关系，可以在所有目标主机上完全并发执行。
   结果聚合: 创建一个依赖于所有上述检查节点的ExecutionNode，它使用一个特殊的 step.ReportTableStep。这个Step需要设计成能从runtime.Context的缓存中读取其他Step的执行结果，并格式化成表格。
   返回一个包含这些检查节点和最终报告节点的ExecutionFragment。
   pre.ConfirmTask:
   职责: 获取用户最终确认。
   IsRequired(): 始终返回 true。
   Plan():
   创建一个 step.UserInputStep。
   在本地控制节点上执行。
   pre.VerifyArtifactsTask:
   职责: （离线部署场景）校验离线资源包。
   IsRequired(): 检查 config 中是否为离线模式。
   Plan():
   创建一个 step.FileChecksumStep，在本地控制节点上计算离线包的哈希值，并与config中提供的值进行比对。
   pre.CreateRepositoryTask:
   职责: （离线部署场景）在所有节点上创建临时本地YUM/APT仓库。
   IsRequired(): 检查 config 中是否为离线模式且需要创建本地仓库。
   Plan() (构建一个依赖链):
   upload-iso (Node): step.UploadFileStep，将ISO从本地上传到所有节点的 /tmp/kubexm。
   mount-iso (Node): step.CommandStep (mount -o loop ...)，依赖于 upload-iso。
   backup-repo (Node): step.CommandStep (mv /etc/yum.repos.d ...)，依赖于 mount-iso。
   create-local-repo (Node): step.RenderTemplateStep (生成.repo文件)，依赖于 backup-repo。
   install-pkgs (Node): step.InstallPackageStep，依赖于 create-local-repo。
3. pkg/task/container_runtime/
   container_runtime.InstallTask:
   职责: 下载、分发并安装容器运行时。
   IsRequired(): 检查 config 中是否定义了容器运行时。
   Plan():
   决策: 从 config 读取运行时类型 (containerd, docker) 和版本。构造下载URL和本地路径。
   download-runtime (Node): step.DownloadFileStep，在本地控制节点执行。
   upload-runtime (Node): step.UploadFileStep，将下载的包分发到所有节点。依赖于 download-runtime。
   extract-runtime (Node): step.ExtractArchiveStep，在所有节点上解压。依赖于 upload-runtime。
   configure-runtime (Node): step.RenderTemplateStep (生成配置文件如 daemon.json)。依赖于 extract-runtime。
   start-runtime (Node): step.EnableServiceStep。依赖于 configure-runtime。
4. pkg/task/etcd/
   etcd.InstallTask:
   职责: 部署ETCD集群。
   IsRequired(): 检查 config 中 etcd 的类型不是 external。
   Plan():
   决策:
   证书:
   gen-ca (Node): step.GenerateCertStep (本地执行)。
   gen-etcd-certs (Node): step.GenerateCertStep (本地执行)，依赖于 gen-ca。
   二进制:
   download-etcd (Node): step.DownloadFileStep (本地执行)。
   分发:
   upload-certs (Node): step.UploadFileStep，依赖于 gen-etcd-certs。
   upload-etcd-bin (Node): step.UploadFileStep，依赖于 download-etcd。
   配置与启动:
   configure-etcd (Node): step.RenderTemplateStep (生成 etcd.conf.yml)，依赖于 upload-certs 和 upload-etcd-bin。
   start-etcd (Node): step.RenderTemplateStep (生成 etcd.service) + step.EnableServiceStep，依赖于 configure-etcd。
5. pkg/task/kubernetes/
   kubernetes.InstallBinariesTask: (与 InstallContainerRuntimeTask 类似)
   download-kube-bins -> upload-kube-bins -> chmod-bins。
   kubernetes.PullImagesTask:
   职责: 在所有节点上提前拉取镜像。
   Plan():
   决策: Task 从 config 获取镜像仓库地址和镜像列表。
   为每个节点创建一个 step.CommandStep，执行 crictl pull ... 或 nerdctl pull ...。这些节点可以完全并发执行。
   kubernetes.InitMasterTask:
   职责: 初始化第一个Master节点。
   Plan():
   决策: Task 读取所有相关配置，使用Go的 kubeadm API (kubeadmapiv1beta3) 在内存中构建 InitConfiguration 和 ClusterConfiguration 对象。
   render-kubeadm-config (Node): step.RenderTemplateStep，将内存中的配置对象序列化为YAML，并上传到第一个Master节点。
   kubeadm-init (Node): step.CommandStep (kubeadm init --config ...)，依赖于 render-kubeadm-config。
   capture-join-info (Node): 一个特殊的Step，它执行 kubeadm token create --print-join-command，并将输出写入到 runtime.Context 的缓存中，供后续 Task 使用。依赖于 kubeadm-init。
   kubernetes.JoinMastersTask:
   职责: 将其他Master节点加入集群。
   Plan():
   决策: Task 从runtime.Context的缓存中读取 capture-join-info 保存的join命令。
   创建一个step.CommandStep，在所有其他Master节点上执行 kubeadm join ... --control-plane 命令。
   kubernetes.JoinWorkersTask: (与 JoinMastersTask 类似，但不带 --control-plane 参数)
   kubernetes.PostInstallTask:
   职责: 执行所有安装后的配置。
   Plan():
   为您的列表中的每一项（移除污点、打标签、拷贝kubeconfig等）创建一个step.CommandStep (使用 kubectl) 或 step.UploadFileStep。
   这些节点大部分可以并发执行，但有些需要依赖关系（例如，必须在kubeadm init之后才能执行kubectl命令）。
6. pkg/task/network/
   network.InstallPluginTask:
   职责: 部署CNI网络插件。
   Plan():
   决策: Task 从 config 读取CNI插件类型和参数。
   render-cni-manifest (Node): step.RenderTemplateStep，将CNI的YAML模板（可以内置在程序中）用config中的参数（如Pod CIDR）进行渲染，并保存到本地控制节点的工作目录。
   apply-cni (Node): step.CommandStep (kubectl apply -f ...)，在Master节点上执行。这个 Step 的命令会引用一个文件路径，因此需要先用UploadFileStep将渲染好的manifest上传到Master节点。所以它依赖于一个 upload-cni-manifest 节点，而 upload-cni-manifest 依赖于 render-cni-manifest。
7. pkg/task/addon/
   addon.InstallTask:
   职责: 部署各类可选插件。
   Plan():
   Task 遍历config中的addons列表。
   对每个addon，它会创建一个类似于InstallNetworkPluginTask的依赖链：render-addon-manifest -> upload-addon-manifest -> apply-addon。
   所有不同addon的部署链之间可以并发执行。
   第三部分：Module 和 Pipeline 的设计
   pkg/module/preflight.Module:
   Tasks: GreetingTask, PreflightCheckTask, ConfirmTask, VerifyArtifactsTask。
   Plan(): 将这些 Task 的Fragment按顺序链接起来。
   pkg/module/cluster_install.Module:
   Tasks: CreateRepositoryTask, InstallContainerRuntimeTask, InstallETCDTask, InstallKubeBinariesTask, PullImagesTask, InitMasterTask, JoinMastersTask, JoinWorkersTask, InstallNetworkPluginTask。
   Plan(): 这是最复杂的链接逻辑。
   InitMasterTask 依赖于 PullImagesTask 和 InstallKubeBinariesTask 等。
   JoinMastersTask 和 JoinWorkersTask 依赖于 InitMasterTask。
   InstallNetworkPluginTask 依赖于 InitMasterTask。
   pkg/module/post_install.Module:
   Tasks: PostInstallTask, InstallAddonsTask。
   Plan(): 将这些 Task 的Fragment按顺序链接起来。
   pkg/pipeline/create_cluster.Pipeline:
   Modules: preflight.Module, cluster_install.Module, post_install.Module。
   Plan(): 将这三个 Module 的Fragment按顺序链接，最终生成完整的 ExecutionGraph。