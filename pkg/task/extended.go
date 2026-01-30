package task

import (
	"time"

	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// ===================================================================
// TaskExecutionOptions - Task 执行选项
// ===================================================================

type TaskExecutionOptions struct {
	Timeout           time.Duration
	RetryPolicy       *RetryPolicy
	ContinueOnFailure bool
	RunBefore         string
	RunAfter          string
}

type RetryPolicy struct {
	MaxRetries        int
	RetryDelay        time.Duration
	BackoffMultiplier float64
}

// ===================================================================
// ExtendedTask - 扩展的 Task 接口
// ===================================================================
//
// 在原有 Task 接口基础上增加了：
// - Validate: 验证 Task 配置
// - GetDependencies: 获取依赖的其他 Tasks
// - GetTimeout: 获取超时时间
// - GetRetryPolicy: 获取重试策略
// - GetOrder: 获取执行顺序
type ExtendedTask interface {
	// 原有方法
	Name() string
	Description() string
	IsRequired(ctx runtime.TaskContext) (bool, error)
	Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error)
	GetBase() *Base

	// 新增方法
	Validate(ctx runtime.TaskContext) error // 验证 Task 配置
	GetDependencies() []string              // 依赖的其他 Tasks
	GetTimeout() time.Duration              // 超时时间
	GetRetryPolicy() *RetryPolicy           // 重试策略
	GetOrder() int                          // 执行顺序
}

// ===================================================================
// TaskWithRetry - 带重试功能的 Task 包装器
// ===================================================================

type TaskWithRetry struct {
	Task        Task
	RetryPolicy *RetryPolicy
}

func NewTaskWithRetry(task Task, policy *RetryPolicy) *TaskWithRetry {
	if policy == nil {
		policy = &RetryPolicy{
			MaxRetries:        3,
			RetryDelay:        time.Second * 10,
			BackoffMultiplier: 2.0,
		}
	}
	return &TaskWithRetry{
		Task:        task,
		RetryPolicy: policy,
	}
}

// Forward basic Task interface methods
func (t *TaskWithRetry) Name() string        { return t.Task.Name() }
func (t *TaskWithRetry) Description() string { return t.Task.Description() }
func (t *TaskWithRetry) GetBase() *Base      { return t.Task.GetBase() }
func (t *TaskWithRetry) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return t.Task.IsRequired(ctx)
}
func (t *TaskWithRetry) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	return t.Task.Plan(ctx)
}

// Forward ExtendedTask methods if the underlying task implements them
func (t *TaskWithRetry) Validate(ctx runtime.TaskContext) error {
	if validateTask, ok := t.Task.(interface {
		Validate(ctx runtime.TaskContext) error
	}); ok {
		return validateTask.Validate(ctx)
	}
	return nil
}

func (t *TaskWithRetry) GetDependencies() []string {
	if deps, ok := t.Task.(interface{ GetDependencies() []string }); ok {
		return deps.GetDependencies()
	}
	return nil
}

func (t *TaskWithRetry) GetTimeout() time.Duration {
	if timeoutTask, ok := t.Task.(interface{ GetTimeout() time.Duration }); ok {
		return timeoutTask.GetTimeout()
	}
	return t.Task.GetBase().Timeout
}

func (t *TaskWithRetry) GetRetryPolicy() *RetryPolicy {
	return t.RetryPolicy
}

func (t *TaskWithRetry) GetOrder() int {
	if orderTask, ok := t.Task.(interface{ GetOrder() int }); ok {
		return orderTask.GetOrder()
	}
	return 0
}
