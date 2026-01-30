package runner

import (
	"time"
)

// ===================================================================
// Runner Result Types (独立于StepResult)
// Runner是工具层，不应该返回StepResult，StepResult是Step层的概念
// ===================================================================

// RunnerStatus 表示Runner操作的状态
type RunnerStatus string

const (
	RunnerStatusSuccess RunnerStatus = "success"
	RunnerStatusFailed  RunnerStatus = "failed"
	RunnerStatusSkipped RunnerStatus = "skipped"
)

// RunnerResult Runner操作的结果类型
// 轻量级、结构简单，只包含操作结果信息
type RunnerResult struct {
	Status    RunnerStatus   `json:"status"`
	StartTime time.Time      `json:"start_time"`
	EndTime   time.Time      `json:"end_time"`
	Duration  time.Duration  `json:"duration"`
	Message   string         `json:"message,omitempty"`
	Error     string         `json:"error,omitempty"`
	Output    string         `json:"output,omitempty"`
	Changed   bool           `json:"changed"` // 标识是否有实际变更
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// NewRunnerResult 创建新的RunnerResult
func NewRunnerResult() *RunnerResult {
	return &RunnerResult{
		Status:    RunnerStatusSuccess,
		StartTime: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// MarkSuccess 标记为成功
func (r *RunnerResult) MarkSuccess(message string) {
	r.Status = RunnerStatusSuccess
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Message = message
	r.Changed = true
}

// MarkFailed 标记为失败
func (r *RunnerResult) MarkFailed(err error, message string) {
	r.Status = RunnerStatusFailed
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	if err != nil {
		r.Error = err.Error()
	}
	r.Message = message
}

// MarkSkipped 标记为跳过
func (r *RunnerResult) MarkSkipped(reason string) {
	r.Status = RunnerStatusSkipped
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Message = reason
}

// SetOutput 设置输出
func (r *RunnerResult) SetOutput(output string) {
	r.Output = output
}

// SetMetadata 设置元数据
func (r *RunnerResult) SetMetadata(key string, value any) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]any)
	}
	r.Metadata[key] = value
}

// GetMetadata 获取元数据
func (r *RunnerResult) GetMetadata(key string) (any, bool) {
	if r.Metadata == nil {
		return nil, false
	}
	v, ok := r.Metadata[key]
	return v, ok
}

// ===================================================================
// CommandResult 命令执行结果
// ===================================================================

// CommandResult 命令执行的结果类型
type CommandResult struct {
	ExitCode int           `json:"exit_code"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration"`
}

// NewCommandResult 创建CommandResult
func NewCommandResult(exitCode int, stdout, stderr string, success bool, duration time.Duration) *CommandResult {
	return &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Success:  success,
		Duration: duration,
	}
}

// ===================================================================
// FileResult 文件操作结果
// ===================================================================

// FileResult 文件操作的结果类型
type FileResult struct {
	Path        string `json:"path"`
	Operation   string `json:"operation"` // read, write, copy, delete, etc.
	Size        int64  `json:"size"`
	Changed     bool   `json:"changed"`
	Permissions string `json:"permissions,omitempty"`
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
}

// NewFileResult 创建FileResult
func NewFileResult(path, operation string) *FileResult {
	return &FileResult{
		Path:      path,
		Operation: operation,
		Changed:   true,
	}
}

// ===================================================================
// ServiceResult 服务操作结果
// ===================================================================

// ServiceResult 服务操作的结果类型
type ServiceResult struct {
	Name      string `json:"name"`
	Operation string `json:"operation"` // start, stop, restart, enable, disable
	Active    bool   `json:"active"`
	Enabled   bool   `json:"enabled"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewServiceResult 创建ServiceResult
func NewServiceResult(name, operation string) *ServiceResult {
	return &ServiceResult{
		Name:      name,
		Operation: operation,
	}
}

// ===================================================================
// PackageResult 包操作结果
// ===================================================================

// PackageResult 包操作的结果类型
type PackageResult struct {
	Name         string   `json:"name"`
	Operation    string   `json:"operation"` // install, remove, update
	Version      string   `json:"version,omitempty"`
	Changed      bool     `json:"changed"`
	Dependencies []string `json:"dependencies,omitempty"`
	Message      string   `json:"message,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// NewPackageResult 创建PackageResult
func NewPackageResult(name, operation string) *PackageResult {
	return &PackageResult{
		Name:      name,
		Operation: operation,
		Changed:   true,
	}
}

// ===================================================================
// DownloadResult 下载操作结果
// ===================================================================

// DownloadResult 下载操作的结果类型
type DownloadResult struct {
	URL          string        `json:"url"`
	DestPath     string        `json:"dest_path"`
	Size         int64         `json:"size"`
	Checksum     string        `json:"checksum,omitempty"`
	ChecksumType string        `json:"checksum_type,omitempty"`
	Cached       bool          `json:"cached"`
	Duration     time.Duration `json:"duration"`
	Message      string        `json:"message,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// NewDownloadResult 创建DownloadResult
func NewDownloadResult(url, destPath string) *DownloadResult {
	return &DownloadResult{
		URL:      url,
		DestPath: destPath,
		Cached:   false,
	}
}

// ===================================================================
// FactsResult 收集Facts的结果
// ===================================================================

// FactsResult 收集Facts的结果类型（包装Facts）
type FactsResult struct {
	Success  bool          `json:"success"`
	Facts    *Facts        `json:"facts,omitempty"`
	Cached   bool          `json:"cached"`
	Duration time.Duration `json:"duration"`
	Message  string        `json:"message,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// NewFactsResult 创建FactsResult
func NewFactsResult(success bool, facts *Facts, cached bool) *FactsResult {
	return &FactsResult{
		Success: success,
		Facts:   facts,
		Cached:  cached,
	}
}
