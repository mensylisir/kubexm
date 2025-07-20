package task

import (
	"github.com/mensylisir/kubexm/pkg/spec"
	"time"
)

type Base struct {
	Meta        spec.TaskMeta
	Timeout     time.Duration
	IgnoreError bool
}

func (b *Base) GetBase() *Base {
	return b
}

//import "github.com/mensylisir/kubexm/pkg/common" // 存放常量 Key
//
//// ...
// Node 1: Download Etcd
//downloadNode := &plan.ExecutionNode{
//Name: "DownloadEtcd",
//Step: common.NewDownloadFileStepBuilder(...).Build(),
//RuntimeConfig: map[string]interface{}{
//// 配置行为：告诉 DownloadStep，你的输出应该用这个 Key 存起来
//"outputCacheKey": common.CacheKeyEtcdArchivePath,
//},
//}
//
//// Node 2: Extract Etcd
//extractNode := &plan.ExecutionNode{
//Name: "ExtractEtcd",
//Step: common.NewExtractArchiveStepBuilder(...).Build(),
//RuntimeConfig: map[string]interface{}{
//// 配置行为：告诉 ExtractStep，你的输入来自这个 Key...
//"inputCacheKey": common.CacheKeyEtcdArchivePath,
//// ...你的输出应该存到这个 Key
//"outputCacheKey": common.CacheKeyEtcdExtractedDir,
//},
//}
//// ...

//package task
//
//import (
//"fmt"
//"github.com.mensylisir/kubexm/pkg/common"
//"github.com.mensylisir/kubexm/pkg/plan"
//"github.com/mensylisir/kubexm/pkg/runtime" // 导入 runtime
//// ...
//)

// CreateKubeadmInitTask 创建一个只在第一个 Master 节点上运行的 kubeadm init 任务。
//func CreateKubeadmInitTask(ctx *runtime.Context) (*plan.ExecutionFragment, error) {
//	fragment := plan.NewExecutionFragment("task-kubeadm-init")
//
//	// --- 1. 从 Context 中查询主机 ---
//	masters := ctx.GetHostsByRole("master")
//	if len(masters) == 0 {
//		return nil, fmt.Errorf("no hosts with 'master' role found to run kubeadm init")
//	}
//	firstMaster := masters[0] // <-- 获取第一个 Master
//
//	// --- 2. 创建 Step ---
//	// 这个命令很长，只是个例子
//	initCmdStep := common.NewCommandStepBuilder("KubeadmInit", "kubeadm init --config ...").
//		WithSudo(true).
//		Build()
//
//	// --- 3. 创建 Node，并精确设置 Hosts 列表 ---
//	initNode := &plan.ExecutionNode{
//		Name:  "KubeadmInit-On-First-Master",
//		Step:  initCmdStep,
//		Hosts: []connector.Host{firstMaster}, // <-- 精确地只设置一个主机！
//	}
//
//	fragment.AddNode(initNode)
//	return fragment, nil
//}
//
//// CreateKubeletRestartTask 创建一个在所有节点（master 和 worker）上运行的任务。
//func CreateKubeletRestartTask(ctx *runtime.Context) (*plan.ExecutionFragment, error) {
//	fragment := plan.NewExecutionFragment("task-restart-kubelet")
//
//	// --- 1. 获取所有 Master 和 Worker 节点 ---
//	masters := ctx.GetHostsByRole("master")
//	workers := ctx.GetHostsByRole("worker")
//	allNodes := append(masters, workers...)
//
//	if len(allNodes) == 0 {
//		return nil, fmt.Errorf("no master or worker hosts found to restart kubelet")
//	}
//
//	// --- 2. 创建 Step ---
//	restartCmdStep := common.NewCommandStepBuilder("RestartKubelet", "systemctl restart kubelet").
//		WithSudo(true).
//		Build()
//
//	// --- 3. 创建 Node，并将所有主机都放进去 ---
//	restartNode := &plan.ExecutionNode{
//		Name:  "RestartKubelet-On-All-Nodes",
//		Step:  restartCmdStep,
//		Hosts: allNodes, // <-- 这里是多个主机！
//	}
//
//	fragment.AddNode(restartNode)
//	return fragment, nil
//}
