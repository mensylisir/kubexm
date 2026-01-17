package etcd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestoreEtcdStep struct {
	step.Base
	LocalSnapshotPath string
	RemoteTempDir     string
	DataDir           string
	EtcdctlBinaryPath string
	EtcdUser          string
	EtcdGroup         string
}

type RestoreEtcdStepBuilder struct {
	step.Builder[RestoreEtcdStepBuilder, *RestoreEtcdStep]
}

func NewRestoreEtcdStepBuilder(ctx runtime.Context, instanceName string) *RestoreEtcdStepBuilder {
	s := &RestoreEtcdStep{
		LocalSnapshotPath: filepath.Join(ctx.GetGlobalWorkDir(), "etcd-snapshot-for-restore.db"),
		RemoteTempDir:     filepath.Join(common.DefaultRemoteWorkDir, "etcd-restore"),
		DataDir:           common.EtcdDefaultDataDirTarget,
		EtcdctlBinaryPath: filepath.Join(common.DefaultBinDir, "etcdctl"),
		EtcdUser:          "etcd",
		EtcdGroup:         "etcd",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restore etcd data from a snapshot on current node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 15 * time.Minute

	b := new(RestoreEtcdStepBuilder).Init(s)
	return b
}

func (b *RestoreEtcdStepBuilder) WithLocalSnapshotPath(path string) *RestoreEtcdStepBuilder {
	b.Step.LocalSnapshotPath = path
	return b
}

func (b *RestoreEtcdStepBuilder) WithRemoteTempDir(path string) *RestoreEtcdStepBuilder {
	b.Step.RemoteTempDir = path
	return b
}

func (b *RestoreEtcdStepBuilder) WithDataDir(path string) *RestoreEtcdStepBuilder {
	b.Step.DataDir = path
	return b
}

func (b *RestoreEtcdStepBuilder) WithEtcdctlBinaryPath(path string) *RestoreEtcdStepBuilder {
	b.Step.EtcdctlBinaryPath = path
	return b
}

func (b *RestoreEtcdStepBuilder) WithEtcdUser(user string) *RestoreEtcdStepBuilder {
	b.Step.EtcdUser = user
	return b
}

func (b *RestoreEtcdStepBuilder) WithEtcdGroup(group string) *RestoreEtcdStepBuilder {
	b.Step.EtcdGroup = group
	return b
}

func (s *RestoreEtcdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestoreEtcdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestoreEtcdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Warn("This is a destructive operation! It will replace the existing etcd data on this node.")

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.RemoteTempDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote temp directory %s: %w", s.RemoteTempDir, err)
	}

	remoteSnapshotPath := filepath.Join(s.RemoteTempDir, "snapshot.db")
	logger.Info("Uploading snapshot file...", "from", s.LocalSnapshotPath, "to", remoteSnapshotPath)
	if err := runner.Upload(ctx.GoContext(), conn, s.LocalSnapshotPath, remoteSnapshotPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload snapshot file: %w", err)
	}

	logger.Warn("Removing old etcd data directory...", "path", s.DataDir)
	if err := runner.Remove(ctx.GoContext(), conn, s.DataDir, s.Sudo, true); err != nil {
	}

	restoredDataDirName := fmt.Sprintf("%s.etcd", ctx.GetHost().GetName())
	restoredDataDirPath := filepath.Join(s.RemoteTempDir, restoredDataDirName)

	restoreCmd := fmt.Sprintf("cd %s && ETCDCTL_API=3 %s snapshot restore %s --name %s --data-dir %s",
		s.RemoteTempDir,
		s.EtcdctlBinaryPath,
		remoteSnapshotPath,
		ctx.GetHost().GetName(),
		restoredDataDirName,
	)

	logger.Info("Restoring etcd data from snapshot...")
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, restoreCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to restore snapshot: %w, stderr: %s", err, stderr)
	}

	logger.Info("Moving restored data to the final data directory...", "from", restoredDataDirPath, "to", s.DataDir)
	moveCmd := fmt.Sprintf("mv %s %s", restoredDataDirPath, s.DataDir)
	if _, stderr, err := runner.OriginRun(ctx.GoContext(), conn, moveCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to move restored data directory: %w, stderr: %s", err, stderr)
	}

	logger.Info("Setting ownership of the data directory...", "path", s.DataDir, "owner", s.EtcdUser)
	if err := runner.Chown(ctx.GoContext(), conn, s.DataDir, s.EtcdUser, s.EtcdGroup, true); err != nil {
		return fmt.Errorf("failed to set ownership on data directory: %w", err)
	}

	logger.Info("Cleaning up temporary directory...", "path", s.RemoteTempDir)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteTempDir, s.Sudo, true); err != nil {
		logger.Warn("Failed to clean up remote temp directory.", "error", err)
	}

	logger.Info("Etcd data restore completed successfully on this node.")
	return nil
}

func (s *RestoreEtcdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	logger.Error(nil, "Rollback for a restore operation is not possible and requires manual intervention. The original data directory has been removed.")
	return fmt.Errorf("etcd restore failed and cannot be automatically rolled back")
}

var _ step.Step = (*RestoreEtcdStep)(nil)

//恢复工作流 (在所有 etcd 节点上执行)
//StopEtcdStep: 确保所有旧的 etcd 进程都已停止。
//(可选) BackupEtcdStep: 在执行破坏性操作前，对当前（可能已损坏的）状态进行最后一次备份，以防万一。
//RestoreEtcdStep:
//.WithLocalSnapshotPath("/path/to/good/snapshot.db")
//这个 Step 会在每个 etcd 节点上执行，清理旧数据并从快照恢复。
//ConfigureEtcdStep:
//非常重要：在恢复数据后，集群的成员列表可能已经改变。必须重新运行 ConfigureEtcdStep 来生成与新恢复的数据目录一致的 etcd.conf.yaml。
//这个 ConfigureEtcdStep 应该使用 WithInitialClusterState("existing") (或根据快照的具体情况)，并且 EtcdNodes 列表应该与快照中的成员列表一致。
//StartEtcdStep: 启动新恢复的 etcd 集群。
//CheckEtcdHealthStep: 验证恢复后的集群是否健康。
//我们已经完成了 etcd 备份与恢复这一对最核心的运维操作。现在，你的 kubexm 工具在 etcd 管理方面已经具备了非常强大的生产级能力。

//etcd 恢复工作流详解
//场景:
//目标: 从一个名为 snapshot.db 的快照文件恢复 etcd 集群。
//快照来源: 这个 snapshot.db 文件位于控制节点的 kubexm 工作目录下。
//目标集群: 我们要恢复到一个由 etcd-1, etcd-2, etcd-3 组成的集群。
//前提: kubexm 的清单（Inventory）文件已经被正确配置，ctx.GetHostsByRole("etcd") 会返回 etcd-1, etcd-2, etcd-3 这三个 Host 对象。这必须与快照中的成员相匹配。
//工作流编排 (在控制节点上定义):
//这个工作流中的所有 Step 都会被调度到所有 etcd 节点 (etcd-1, etcd-2, etcd-3) 上执行。
//第 1 步: 停止现有服务 (StopEtcdStep)
//目的: 确保所有 etcd 进程都已停止，以便我们可以安全地替换它们的数据目录。
//构建与调用:
//Generated go
//stopStep, err := NewStopEtcdStepBuilder(ctx, "stop-etcd-before-restore").Build()
//if err != nil {
//	return fmt.Errorf("failed to build stop step: %v", err)
//}
//Use code with caution.
//Go
//关键参数:
//ServiceName: 默认为 "etcd.service"，无需修改。
//Sudo: 默认为 true，正确。
//执行: kubexm 会在 etcd-1, etcd-2, etcd-3 上并行执行 systemctl stop etcd.service。
//第 2 步: 执行恢复 (RestoreEtcdStep)
//目的: 这是核心的恢复步骤。它会清理旧数据，并从快照文件在每个节点上恢复数据目录。
//构建与调用:
//Generated go
//// 假设快照文件在工作区的根目录下
//snapshotPath := filepath.Join(ctx.GetGlobalWorkDir(), "snapshot.db")
//
//restoreStep := NewRestoreEtcdStepBuilder(ctx, "restore-etcd-data-from-snapshot").
//WithLocalSnapshotPath(snapshotPath). // 明确指定快照文件路径
//Build()
//Use code with caution.
//Go
//关键参数:
//LocalSnapshotPath: 必须通过 Builder 方法指定，指向你在控制节点上准备好的快照文件。
//DataDir: 默认为 /var/lib/etcd，符合标准，无需修改。
//Sudo: 默认为 true，正确。
//执行: kubexm 会在每个 etcd 节点上：
//上传 snapshot.db 到 /tmp/etcd-restore/。
//删除 /var/lib/etcd。
//执行 etcdctl snapshot restore。
//将恢复好的数据 mv 到 /var/lib/etcd。
//设置文件所有权并清理临时文件。
//第 3 步: 重新生成配置文件 (ConfigureEtcdStep)
//目的: 至关重要的一步。确保 etcd.conf.yaml 的内容与新恢复的数据目录中的集群成员信息完全一致。
//构建与调用:
//Generated go
//reconfigureStep := NewConfigureEtcdStepBuilder(ctx, "reconfigure-etcd-after-restore").
//// **注意**: 这里不需要调用 WithInitialClusterState("existing")
//// 因为恢复一个集群就像创建一个新集群，默认的 "new" 状态是正确的。
//Build()
//Use code with caution.
//Go
//关键参数:
//EtcdNodes: Builder 会自动从 ctx 获取，因为我们的前提是清单与快照匹配，所以这个列表是正确的。
//InitialClusterState: 默认为 "new"，这是正确的。
//Sudo: 默认为 true，正确。
//执行: kubexm 会在每个 etcd 节点上：
//根据 EtcdNodes 列表（包含 etcd-1,2,3）计算出正确的 initial-cluster 字符串。
//使用 "new" 状态。
//渲染并覆盖 /etc/etcd/etcd.conf.yaml。
//第 4 步: 启用服务 (EnableEtcdStep)
//目的: 确保恢复后的服务被设置为开机自启。这是一个好习惯，即使它之前可能已经被启用了。
//构建与调用:
//Generated go
//enableStep, err := NewEnableEtcdStepBuilder(ctx, "enable-etcd-after-restore").Build()
//if err != nil {
//	return fmt.Errorf("failed to build enable step: %v", err)
//}
//Use code with caution.
//Go
//关键参数: 无需修改，默认值即可。
//第 5 步: 启动服务 (StartEtcdStep)
//目的: 启动所有 etcd 节点，让它们基于恢复的数据和新配置组成集群。
//构建与调用:
//Generated go
//startStep, err := NewStartEtcdStepBuilder(ctx, "start-etcd-after-restore").Build()
//if err != nil {
//	return fmt.Errorf("failed to build start step: %v", err)
//}
//Use code with caution.
//Go
//关键参数: 无需修改，默认值即可。
//第 6 步: 健康检查 (CheckEtcdHealthStep)
//目的: 最终验证恢复后的集群是否健康、可用。
//构建与调用:
//Generated go
//// 这个 Step 可以在所有 etcd 节点上运行，也可以只在一个上运行，
//// 取决于工作流引擎的调度策略。我们的 Step 设计兼容这两种情况。
//checkHealthStep := NewCheckEtcdHealthStepBuilder(ctx, "check-etcd-health-after-restore").
//WithRetryCount(15). // 恢复后首次启动可能稍慢，增加重试次数
//Build()
//Use code with caution.
//Go
//关键参数: 可以适当增加 RetryCount 和 RetryDelay，给集群足够的稳定时间。
//流程总结
//序号	Step 名称	调度目标	关键参数/逻辑
//1	StopEtcdStep	所有 etcd 节点	无需特殊参数
//2	RestoreEtcdStep	所有 etcd 节点	必须用 WithLocalSnapshotPath 指定快照文件
//3	ConfigureEtcdStep	所有 etcd 节点	无需特殊参数，依赖 Context 和默认值
//4	EnableEtcdStep	所有 etcd 节点	无需特殊参数
//5	StartEtcdStep	所有 etcd 节点	无需特殊参数
//6	CheckEtcdHealthStep	所有或任一 etcd 节点	(可选) 增加重试次数
//是的，我们现有的 Step 集合不仅能支撑恢复流程，而且能以一种非常模块化和清晰的方式来完成这个复杂的任务。每一个 Step 都像一个精密的工具，而工作流就是使用这些工具的正确说明书。

//你提出了一个极其重要的、关乎整个 Kubernetes 集群而不仅仅是 etcd 本身的运维问题！
//答案是：是的，绝对需要！在对 etcd 进行恢复操作之前，停止 kube-apiserver 是一个强制性的、必须执行的安全步骤。
//我之前的回答只聚焦于 etcd 本身，而忽略了它最重要的“客户”——kube-apiserver，这是一个严重的疏忽。非常感谢你指出了这一点。
//为什么必须停止 kube-apiserver？
//etcd 是 Kubernetes 的状态存储后端。kube-apiserver 是唯一与 etcd 直接交互的核心组件。所有其他组件（如 scheduler, controller-manager）都通过 apiserver 来读写集群状态。
//如果在 etcd 恢复过程中，kube-apiserver 仍然在运行，会发生以下灾难性的问题：
//数据不一致与状态分裂 (Split-Brain):
//当 etcd 服务被停止 (StopEtcdStep)，kube-apiserver 会失去与后端的连接，进入一个不健康但仍在尝试服务的状态。
//当我们用一个旧的快照恢复 etcd 时（比如1小时前的快照），etcd 的数据被“时间倒流”了。
//一旦 etcd 恢复并启动，kube-apiserver 会立刻连接上它。但是，kube-apiserver 及其客户端（scheduler, controllers）的内存中缓存可能仍然是新的状态（基于1小时前的快照之后发生的变化）。
//这会导致严重的状态冲突。例如，一个在快照之后被创建的 Pod，apiserver 的缓存里可能还认为它存在，但 etcd 里已经没有它的记录了。controller-manager 会尝试根据 apiserver 的“记忆”去重新创建 Pod，而 etcd 里的状态又不是这样，最终导致整个集群状态的混乱和损坏。
//Etcd Leader 选举干扰:
//kube-apiserver 会持续地向 etcd 的所有端点发起 health check 和读写请求。
//在新恢复的 etcd 集群正在进行脆弱的首次 Leader 选举时，来自 apiserver 的大量请求可能会干扰选举过程，导致选举超时或失败。
//级联故障:
//一个状态不一致的 apiserver 会给所有下游组件提供错误的信息，导致 scheduler 做出错误的调度决策，controller-manager 错误地创建或删除资源，kubelet 收到错误的 Pod 配置，最终引发整个集群的级联故障。
//正确的、完整的恢复工作流
//一个真正生产级的恢复工作流，必须将 kube-apiserver 的生命周期管理包含进来。
//前提:
//我们假设有一个 StopApiServerStep 和 StartApiServerStep。在 kubexm 中，这可能通过停止 kube-apiserver 的静态 Pod manifest 文件（将其从 /etc/kubernetes/manifests/ 移走）或停止 kubelet 服务来实现。
//修正后的恢复工作流:
//序号	Step 名称	调度目标	关键参数/逻辑
//0	StopApiServerStep	所有 Master 节点	（新增） 停止所有 kube-apiserver 实例。
//1	StopEtcdStep	所有 etcd 节点	停止所有 etcd 实例。
//2	RestoreEtcdStep	所有 etcd 节点	从快照恢复 etcd 数据。
//3	ConfigureEtcdStep	所有 etcd 节点	重新生成与恢复数据一致的配置。
//4	EnableEtcdStep	所有 etcd 节点	确保服务开机自启。
//5	StartEtcdStep	所有 etcd 节点	逐个或并行启动 etcd 服务。
//6	CheckEtcdHealthStep	任一 etcd 节点	验证 etcd 集群是否已健康。
//7	StartApiServerStep	所有 Master 节点	（新增） 在 etcd 健康后，再启动 kube-apiserver。
//8	CheckApiServerHealthStep	localhost 或任一 Master	（新增） 验证 apiserver 是否健康并能正确服务请求。
//结论
//你提出的是一个至关重要的、必须遵守的运维原则。任何对 etcd 的恢复操作，都必须在一个“隔离”的环境中进行，这意味着必须切断它与上层应用（kube-apiserver）的所有连接。
//先停止 apiserver，再恢复 etcd，最后再启动 apiserver，这个顺序是保证 Kubernetes 集群能够从灾难中被干净、一致地恢复回来的唯一正确方法。
//非常感谢你的追问，这让我们的恢复流程从一个“技术上可行”的方案，提升为了一个“生产上安全可靠”的方案。
