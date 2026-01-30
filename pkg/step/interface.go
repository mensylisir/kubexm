package step

import (
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
)

// Step 是最小的原子操作单元，具有幂等性
// 所有Step都应该实现这个接口
type Step interface {
	// Meta 返回Step的元数据信息
	Meta() *spec.StepMeta

	// Precheck 检查，判断是否需要执行
	// 返回 (true, nil) 表示已经完成，无需执行
	// 返回 (false, nil) 表示需要执行
	// 返回 (_, error) 表示检查失败
	Precheck(ctx runtime.ExecutionContext) (bool, error)

	// Validate 验证Step的配置是否正确
	// 在Run之前调用，用于验证参数和环境
	Validate(ctx runtime.ExecutionContext) error

	// Run 执行Step的核心逻辑
	// 返回nil表示成功
	// 返回error表示失败
	Run(ctx runtime.ExecutionContext) error

	// Cleanup 清理资源，无论成功还是失败都调用
	// 用于清理临时文件、释放资源等
	Cleanup(ctx runtime.ExecutionContext) error

	// Rollback 回滚操作，在Run失败时调用
	// 将系统恢复到Run之前的状态
	Rollback(ctx runtime.ExecutionContext) error

	// GetStatus 获取Step的当前状态
	// 用于在执行过程中查询状态
	GetStatus(ctx runtime.ExecutionContext) (StepStatus, error)

	// GetBase 返回Step的基础信息
	GetBase() *Base
}

// Timeout 包装time.Duration，提供更好的类型安全
type Timeout struct {
	time.Duration
}

func (t Timeout) IsZero() bool {
	return t.Duration == 0
}

func (t Timeout) Default() Timeout {
	if t.IsZero() {
		return Timeout{Duration: 5 * 60 * 1000000000} // 默认5分钟
	}
	return t
}
