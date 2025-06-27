pkg/runner已存在，已经实现,同时支持调用ssh.go和local.go
# runner的interface.go
```aiignore
package runner

import (
	"context"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Facts, PackageInfo, ServiceInfo etc. structure definitions
type Facts struct {
	OS             *connector.OS
	Hostname       string
	Kernel         string
	TotalMemory    uint64 // in MiB
	TotalCPU       int
	IPv4Default    string
	IPv6Default    string
	PackageManager *PackageInfo
	InitSystem     *ServiceInfo
}
type PackageManagerType string
const (
	PackageManagerUnknown PackageManagerType = "unknown"
	PackageManagerApt     PackageManagerType = "apt"
	PackageManagerYum     PackageManagerType = "yum"
	PackageManagerDnf     PackageManagerType = "dnf"
)
type PackageInfo struct {
	Type          PackageManagerType
	UpdateCmd     string
	InstallCmd    string
	RemoveCmd     string
	PkgQueryCmd   string
	CacheCleanCmd string
}
type InitSystemType string
const (
	InitSystemUnknown InitSystemType = "unknown"
	InitSystemSystemd InitSystemType = "systemd"
	InitSystemSysV    InitSystemType = "sysvinit"
)
type ServiceInfo struct {
	Type            InitSystemType
	StartCmd        string
	StopCmd         string
	EnableCmd       string
	DisableCmd      string
	RestartCmd      string
	IsActiveCmd     string
	DaemonReloadCmd string
}


// Runner interface defines a complete, stateless host operation service library.
type Runner interface {
	GatherFacts(ctx context.Context, conn connector.Connector) (*Facts, error)
	Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error)
	MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string
	Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error)
	RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error)
	Download(ctx context.Context, conn connector.Connector, facts *Facts, url, destPath string, sudo bool) error
	Extract(ctx context.Context, conn connector.Connector, facts *Facts, archivePath, destDir string, sudo bool) error
	DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *Facts, url, destDir string, sudo bool) error
	Exists(ctx context.Context, conn connector.Connector, path string) (bool, error)
	IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error)
	ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error)
	WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error
	Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error
	Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error
	GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error)
	LookPath(ctx context.Context, conn connector.Connector, file string) (string, error)
	IsPortOpen(ctx context.Context, conn connector.Connector, facts *Facts, port int) (bool, error)
	WaitForPort(ctx context.Context, conn connector.Connector, facts *Facts, port int, timeout time.Duration) error
	SetHostname(ctx context.Context, conn connector.Connector, facts *Facts, hostname string) error
	AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error
	InstallPackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error
	RemovePackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error
	UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *Facts) error
	IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *Facts, packageName string) (bool, error)
	AddRepository(ctx context.Context, conn connector.Connector, facts *Facts, repoConfig string, isFilePath bool) error
	StartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	StopService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	RestartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	EnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	DisableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	IsServiceActive(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error)
	DaemonReload(ctx context.Context, conn connector.Connector, facts *Facts) error
	Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error
	UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error)
	GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error)
	AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error
	AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error
}

```


### 整体评价：一个强大而全面的主机操作抽象层

**优点 (Strengths):**

1. **高度抽象和声明式**: 接口方法名（InstallPackages, SetHostname）都是声明式的，它们描述了**“要做什么”**，而不是**“怎么做”**。这使得上层的 Step 或 Task 逻辑变得非常干净和易于理解。例如，一个Step只需要调用 runner.InstallPackages(ctx, conn, facts, "nginx")，而无需关心目标主机是apt系还是yum系。
2. **职责清晰 (SRP)**: Runner 的职责非常明确：它是一个**无状态的函数库**，负责将业务意图转换为在具体主机上的一系列命令。它不管理连接（由Connector负责），也不编排复杂的业务流程（由Pipeline/Task负责）。
3. **依赖注入 (DI)**: 所有方法都接受 connector.Connector 作为参数，这使得Runner可以操作任何实现了Connector接口的主机（无论是远程SSH还是本地执行），完美实现了与底层连接方式的解耦。
4. **基于 Facts 的决策**: 大多数方法都接受 *Facts 作为参数。这是一种非常高效和健壮的设计模式。Facts（主机信息）通常在流程开始时一次性收集（GatherFacts），然后在后续所有操作中被复用。这避免了在每个函数中重复执行命令来判断操作系统类型或包管理器，显著提升了性能。
5. **全面的功能覆盖**: 接口覆盖了从系统配置（主机名、hosts文件）、文件操作、包管理、服务管理、网络检查到用户/组管理的几乎所有常见的运维操作。这是一个非常完备的工具集。
6. **错误处理与便利性**: 提供了 Run, MustRun, Check 三种不同错误处理风格的执行方法，满足了不同场景的需求。RunWithOptions 则为需要精细控制的场景提供了“后门”。

### 与整体架构的契合度

pkg/runner 完美地坐落在**第二层：基础服务**中，并起到了至关重要的作用：

- **对下**: 它消费 pkg/connector 提供的原子能力。Runner的所有实现，其内部逻辑都是在拼装命令字符串，然后调用 conn.Exec() 或 conn.CopyContent()。
- **对上**: 它服务于**第三层：执行与决策**。所有的 pkg/step 都会直接或间接地调用 Runner 中的方法来完成其实际工作。Runner 是 Step 实现的“弹药库”。
    - 例如，一个 InstallNginxStep 的 Run 方法，其核心逻辑可能就是：
        1. runner.AddRepository(...)
        2. runner.UpdatePackageCache(...)
        3. runner.InstallPackages(..., "nginx")
        4. runner.EnableService(..., "nginx")
        5. runner.StartService(..., "nginx")
- **与 pkg/runtime 的关系**: Runtime 会持有一个 Runner 的实例，并将其注入到需要它的 Step 或 Task 的上下文中。

### 设计细节的分析与潜在的完善点

这个接口设计已经非常成熟，几乎没有明显的“缺陷”。我们可以探讨的是一些可以让它更完美的“锦上添花”的细节。

1. **Facts 结构的演进**:
    - Facts 中已经包含了 PackageManager 和 InitSystem 的详细信息（如具体的命令），这是一个非常好的实践。
    - **可考虑增加**:
        - SELinux/AppArmor 状态：对于需要处理安全策略的部署很有用。
        - Firewall 信息：当前激活的防火墙类型（firewalld, iptables, ufw）。
        - 更详细的网络信息：所有网络接口及其IP地址，而不仅仅是默认路由的IP。
2. **模板渲染 (Render) 方法**:
    - 接口中包含了Render方法，这是一个非常强大的功能，可以将模板渲染后直接写入远程文件。
    - **建议**: 考虑提供一个 RenderToString 的变体，它只返回渲染后的字符串，而不写入文件。这在需要动态生成配置文件内容，但后续还需进一步处理的场景中会很有用。
3. **操作的幂等性**:
    - 接口的设计天然地倾向于幂等性，但真正的幂等性保证需要在**实现**中完成。例如，AddHostEntry 在实现时，应该先检查 /etc/hosts 中是否已存在该条目，如果存在则不进行任何操作。
    - **文档约定**: 可以在接口的注释中明确强调，所有 Runner 的实现都**必须**保证幂等性。这是整个“世界树”架构幂等执行哲学的基石。
4. **对 sudo -S 的支持**:
    - 这是我们之前在 connector 层面讨论过的问题。当需要通过sudo -S传递密码时，这个密码信息如何传递到Runner，再到Connector？
    - **方案**:
        1. Runner 的方法签名**不应该**改变。Runner 应该保持无状态。
        2. 密码信息应该存储在传递给Runner方法的 context.Context 中，或者由 Runtime 在调用 Connector 前动态构建 ExecOptions。
        3. Runner 的实现中，当需要 sudo 时，它会创建一个 ExecOptions 对象，设置 Sudo: true，然后调用 conn.RunWithOptions(...)。
        4. 真正的密码填充逻辑发生在 Runner 调用 Connector 的**适配层**，这个适配层通常在 Runtime 中。Runtime 从 secrets.Provider 获取密码，填充到 ExecOptions.SudoPassword 中，然后再调用 Connector。这样 Runner 本身依然对密码无感知。

### 总结：架构的“肌肉”

如果说 pkg/connector 是架构的“骨骼和神经”，那么 pkg/runner 就是架构的**“肌肉”**。它将底层的“脉冲信号”（Exec）转化为了有力量、有目的的动作（InstallPackages）。

这份接口设计非常成熟和全面，它为上层 Step 的编写提供了极大的便利和抽象，同时保持了自身的无状态和可测试性。它是连接底层协议和上层业务逻辑的完美桥梁，是整个“世界树”项目中最高度可复用的业务逻辑库。这是一个不需要大改，可以直接投入实现的出色设计



在现有基础上，我们可以从以下几个维度来进一步丰富它，使其在处理更复杂的真实世界场景时更加得心应手：

------



### 一、 系统与内核级操作 (System & Kernel Level)

这部分功能用于更深层次地配置操作系统。

1. **内核模块管理 (Kernel Module Management)**
    - LoadModule(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
        - **作用**: 加载一个内核模块（等同于 modprobe <module> [params]）。这对于配置网络（如 br_netfilter）或存储（如 iscsi_tcp）至关重要。
    - UnloadModule(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string) error
        - **作用**: 卸载内核模块 (rmmod <module>)。
    - IsModuleLoaded(ctx context.Context, conn connector.Connector, moduleName string) (bool, error)
        - **作用**: 检查模块是否已加载 (lsmod | grep <module>)。
    - ConfigureModuleOnBoot(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
        - **作用**: 配置模块在系统启动时自动加载（通常是向 /etc/modules-load.d/ 目录写入配置文件）。
2. **Sysctl参数管理 (Sysctl Parameter Management)**
    - GetSysctl(ctx context.Context, conn connector.Connector, key string) (string, error)
        - **作用**: 读取当前的内核参数值 (sysctl -n <key>)。
    - SetSysctl(ctx context.Context, conn connector.Connector, key, value string, temporary bool) error
        - **作用**: 设置内核参数。temporary=true 时使用 sysctl -w <key>=<value>（重启后失效）；temporary=false 时则会写入到 /etc/sysctl.d/ 下的配置文件并执行 sysctl -p，使其永久生效。
3. **系统时间与时区 (Time & Timezone)**
    - SetTimezone(ctx context.Context, conn connector.Connector, facts *Facts, timezone string) error
        - **作用**: 设置系统的时区 (timedatectl set-timezone <timezone>)。
    - SyncTime(ctx context.Context, conn connector.Connector, facts *Facts, ntpServers ...string) error
        - **作用**: 手动与NTP服务器同步时间，或者配置并重启chronyd/ntp服务。
4. **Swap管理**
    - DisableSwap(ctx context.Context, conn connector.Connector, facts *Facts) error
        - **作用**: 临时禁用所有swap (swapoff -a) 并注释掉 /etc/fstab 中的swap条目使其永久生效。Kubernetes部署的经典步骤。
    - IsSwapEnabled(ctx context.Context, conn connector.Connector) (bool, error)
        - **作用**: 检查当前系统是否启用了任何swap。

### 二、 文件系统与存储操作 (Filesystem & Storage)

这部分功能用于处理磁盘和存储。

1. **挂载点管理 (Mount Point Management)**
    - Mount(ctx context.Context, conn connector.Connector, device, mountPoint, fsType string, options []string) error
        - **作用**: 挂载一个设备 (mount -t <fsType> -o <options> <device> <mountPoint>)。
    - Unmount(ctx context.Context, conn connector.Connector, mountPoint string, force bool) error
        - **作用**: 卸载一个挂载点 (umount [-f] <mountPoint>)。
    - IsMounted(ctx context.Context, conn connector.Connector, path string) (bool, error)
        - **作用**: 检查一个路径是否是挂载点。
    - EnsureMount(ctx context.Context, conn connector.Connector, device, mountPoint, fsType string, options []string, persistent bool) error
        - **作用**: 一个幂等的挂载操作。如果未挂载，则执行挂载。如果 persistent=true，则确保 /etc/fstab 中有对应的条目。
2. **文件系统创建 (Filesystem Creation)**
    - MakeFilesystem(ctx context.Context, conn connector.Connector, device, fsType string, force bool) error
        - **作用**: 在一个块设备上创建文件系统 (mkfs.<fsType> [-f] <device>)。
3. **符号链接管理 (Symbolic Link Management)**
    - CreateSymlink(ctx context.Context, conn connector.Connector, target, linkPath string, sudo bool) error
        - **作用**: 创建一个符号链接 (ln -s <target> <linkPath>)。

### 三、 网络操作 (Networking)

这部分功能用于配置网络接口和服务。

1. **防火墙管理 (Firewall Management)**
    - ConfigureFirewall(ctx context.Context, conn connector.Connector, facts *Facts, rules ...FirewallRule) error
        - **作用**: 一个高级接口，用于添加/删除防火墙规则。FirewallRule 可以是一个结构体，如 { Port int, Protocol string, Action string, Zone string }。底层实现会根据 facts 判断是使用 firewalld, iptables还是ufw。
    - DisableFirewall(ctx context.Context, conn connector.Connector, facts *Facts) error
        - **作用**: 禁用防火墙服务。
2. **获取网络信息 (Network Information Gathering)**
    - GetInterfaceAddresses(ctx context.Context, conn connector.Connector, interfaceName string) (map[string][]string, error)
        - **作用**: 获取指定网络接口的所有IP地址（IPv4和IPv6）。返回 map["ipv4": ["ip1", "ip2"], "ipv6": ["ip3"]}。

### 四、 用户与权限增强 (User & Permissions Enhancement)

1. **修改用户属性 (Modify User Attributes)**
    - ModifyUser(ctx context.Context, conn connector.Connector, username string, modifications UserModifications) error
        - **作用**: 修改用户信息，如 usermod。UserModifications 是一个结构体，包含 NewGroup, AddToGroups, NewShell, NewHomeDir 等可选字段。
    - SetUserPassword(ctx context.Context, conn connector.Connector, username, hashedPassword string) error
        - **作用**: 设置用户的密码（通常是已加密的哈希值）。
2. **Sudoers 配置**
    - ConfigureSudoer(ctx context.Context, conn connector.Connector, sudoerName, content string) error
        - **作用**: 在 /etc/sudoers.d/ 目录下创建一个配置文件，为用户或组授予特定的sudo权限，并进行语法检查 (visudo -c)。

### 五、 复合与高级操作 (Compound & High-Level Operations)

这些是组合了多个原子操作的、更贴近实际任务的接口。

1. **幂等的服务配置与启动 (Idempotent Service Configuration)**
    - DeployAndEnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName, configContent, configPath, permissions string, templateData interface{}) error
        - **作用**: 一个完整的服务部署流程。它会：
            1. 用 templateData 渲染 configContent (如果提供了模板)。
            2. 将渲染后的内容写入到 configPath。
            3. 设置正确的权限。
            4. 执行 DaemonReload。
            5. EnableService 并 RestartService。
            6. 可以加入一个检查，只有当配置文件内容变化时才重启服务，实现幂等性。
2. **Reboot & Wait**
    - Reboot(ctx context.Context, conn connector.Connector, timeout time.Duration) error
        - **作用**: 发送重启命令，然后等待主机重新变为可连接状态，直到超时。这是一个非常实用的功能，因为很多系统配置需要重启才能生效。

### 总结：丰富后的接口

通过增加上述功能，你的 Runner 接口将变得更加强大和完善，几乎能满足绝大多数基础设施自动化场景的需求。

Generated go

```
type Runner interface {
    // ... all existing methods ...

    // --- System & Kernel ---
    LoadModule(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
    IsModuleLoaded(ctx context.Context, conn connector.Connector, moduleName string) (bool, error)
    ConfigureModuleOnBoot(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
    SetSysctl(ctx context.Context, conn connector.Connector, key, value string, persistent bool) error
    SetTimezone(ctx context.Context, conn connector.Connector, facts *Facts, timezone string) error
    DisableSwap(ctx context.Context, conn connector.Connector, facts *Facts) error
    IsSwapEnabled(ctx context.Context, conn connector.Connector) (bool, error)

    // --- Filesystem & Storage ---
    EnsureMount(ctx context.Context, conn connector.Connector, device, mountPoint, fsType string, options []string, persistent bool) error
    IsMounted(ctx context.Context, conn connector.Connector, path string) (bool, error)
    MakeFilesystem(ctx context.Context, conn connector.Connector, device, fsType string, force bool) error
    CreateSymlink(ctx context.Context, conn connector.Connector, target, linkPath string, sudo bool) error
    
    // --- Networking ---
    DisableFirewall(ctx context.Context, conn connector.Connector, facts *Facts) error
    // GetInterfaceAddresses(...)

    // --- User & Permissions ---
    // ModifyUser(...)
    ConfigureSudoer(ctx context.Context, conn connector.Connector, sudoerName, content string) error

    // --- High-Level ---
    DeployAndEnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName, configContent, configPath, permissions string, templateData interface{}) error
    Reboot(ctx context.Context, conn connector.Connector, timeout time.Duration) error
}
```

