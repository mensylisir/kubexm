### pkg/logger已经实现，直接用就行

# Kubexm 日志系统设计 (`pkg/logger`)

本文档描述了 Kubexm 项目中 `pkg/logger` 包的设计与主要功能。该日志系统基于 `go.uber.org/zap` 构建，提供了灵活的、可配置的结构化日志记录能力，支持多种日志级别、彩色控制台输出和 JSON 文件输出。

## 1. 设计目标

`pkg/logger` 的主要目标是：

*   提供一个易于使用且功能丰富的日志记录接口。
*   支持不同级别的日志（Debug, Info, Warn, Error），并引入自定义的 `Success` 和 `Fail` 级别以增强可读性。
*   实现结构化日志记录，方便日志的后续处理和分析。
*   支持可配置的控制台输出（包括彩色和普通文本）和文件输出（JSON 格式）。
*   允许通过全局 logger 或实例化的 logger 进行日志记录。
*   确保日志记录的性能。
*   需要同时输出到文件和标准输出

## 2. 主要特性与组件

### 2.1. 日志级别 (`logger.Level`)

定义了以下日志级别：

*   `DebugLevel`: 用于详细的调试信息。
*   `InfoLevel`: 常规操作信息。
*   `SuccessLevel`: (自定义级别) 标记重要操作的成功完成，控制台输出通常为绿色。
*   `WarnLevel`: 潜在问题或警告。
*   `ErrorLevel`: 发生了错误，但不一定导致程序中止。
*   `FailLevel`: (自定义级别) 标记严重错误，记录日志后程序将通过 `os.Exit(1)` 退出。控制台输出通常为红色。
*   `PanicLevel`: 记录日志后会调用 `panic()`。
*   `FatalLevel`: 记录日志后会调用 `os.Exit(1)` (主要由 `FailLevel` 内部使用)。

每个级别都有对应的 `String()` 和 `CapitalString()` 方法，以及转换为 `zapcore.Level` 的 `ToZapLevel()` 方法。

### 2.2. 配置选项 (`logger.Options`)

通过 `Options` 结构体可以配置 logger 的行为：

*   `ConsoleLevel` (Level): 控制台输出的最低日志级别。
*   `FileLevel` (Level): 文件输出的最低日志级别。
*   `LogFilePath` (string): 日志文件的路径（当 `FileOutput` 为 `true` 时必需）。
*   `ConsoleOutput` (bool): 是否启用控制台输出 (默认 `true`)。
*   `FileOutput` (bool): 是否启用文件输出 (默认 `false`)。
*   `ColorConsole` (bool): 控制台输出是否使用 ANSI 颜色 (默认 `true`)。
*   `TimestampFormat` (string): 时间戳格式 (默认 `time.RFC3339`)。

`DefaultOptions()` 函数提供了一套默认配置。

### 2.3. Logger 核心 (`logger.Logger`)

`Logger` 结构体是对 `zap.SugaredLogger` 的封装。

*   **初始化**:
    *   `Init(opts Options)`: 初始化全局 logger 实例。此函数应在程序启动时调用一次，后续调用无效。如果初始化失败，会回退到一个基本的控制台 logger。
    *   `Get() *Logger`: 获取全局 logger 实例。如果 `Init` 未被调用，则会自动使用 `DefaultOptions()` 初始化。
    *   `NewLogger(opts Options) (*Logger, error)`: 创建并返回一个新的 `Logger` 实例，允许模块或组件拥有独立的日志配置。
*   **日志方法**:
    *   提供了一系列格式化日志方法，如 `Debugf`, `Infof`, `Successf`, `Warnf`, `Errorf`, `Failf`, `Panicf`, `Fatalf`。
    *   同时，包级别也提供了对应的全局函数，如 `logger.Debug(...)`, `logger.Info(...)` 等，它们内部调用全局 logger。
*   **同步**:
    *   `Sync() error`: Logger 实例的方法，用于刷新缓冲的日志条目。
    *   `SyncGlobal() error`: 包级别函数，用于刷新全局 logger 的缓冲。建议在程序退出前调用。

### 2.4. 控制台编码器 (`pkg/logger/console_encoder.go`)

为了实现定制化的控制台输出格式，定义了 `zapcore.Encoder` 的实现：

*   **`colorConsoleEncoder`**:
    *   负责生成带 ANSI 颜色的日志输出。
    *   能够识别通过 `zap.String("customlevel", level.CapitalString())` 传递的自定义级别，并应用特定颜色（如 `SUCCESS` 用绿色，`FAIL` 和 `ERROR` 用红色）。
    *   日志格式通常包含：`时间戳 [上下文前缀] [级别] 调用者: 消息 key1=value1 key2="value with space"`。
    *   **上下文前缀**: `colorConsoleEncoder` 会自动从日志字段中提取特定的键 (如 `pipeline_name`, `module_name`, `task_name`, `step_name`, `host_name` 等)，并将其格式化为日志行开头的上下文信息，例如 `[P:my-pipe][M:etcd][H:node1]`。
*   **`PlainTextConsoleEncoder`**: 提供与 `colorConsoleEncoder` 类似格式但无颜色的输出。

### 2.5. 文件输出

当 `Options.FileOutput` 为 `true` 时：

*   日志将以 **JSON 格式** 写入到 `Options.LogFilePath` 指定的文件中。
*   JSON 格式便于机器解析和日志管理系统（如 ELK, Splunk）的集成。
*   文件日志级别由 `Options.FileLevel` 控制。

## 3. 使用示例

```go
// main.go
import "github.com/mensylisir/kubexm/pkg/logger" // 假设的导入路径
import "go.uber.org/zap" // 如果需要传递 zap.Field

func main() {
    opts := logger.DefaultOptions()
    opts.ConsoleLevel = logger.DebugLevel
    opts.FileOutput = true
    opts.LogFilePath = "kubexm_app.log"
    logger.Init(opts)
    defer logger.SyncGlobal() // 确保日志在程序退出前被刷新

    logger.Info("Kubexm application starting...")
    logger.Debug("This is a detailed debug message.")

    // 模拟在某个模块或步骤中记录带上下文的日志
    // logger.Get().With(
    //    zap.String("pipeline_name", "create_cluster"),
    //    zap.String("module_name", "etcd_setup"),
    //    zap.String("host_name", "node1"),
    // ).Successf("Etcd successfully configured on %s", "node1")

    logger.Warn("A non-critical warning occurred.")
    // logger.Fail("A critical error occurred, exiting.") // 这会导致程序退出
}
(注意: 上述示例中的 With(...) 方法直接使用 zap.Field 可能不会被 console_encoder.go 中当前的上下文前缀提取逻辑直接识别，该逻辑似乎依赖于在调用 logWithCustomLevel 时传递的特定键。实际使用中，上下文信息通常由 runtime.Context 传递并由更上层的日志封装注入)

4. 结构化日志与字段
推荐使用格式化字符串（如 Infof, Errorf）或 zap.Field（当与底层的 zap.SugaredLogger 或 zap.Logger 交互时）来提供结构化的键值对信息，这对于文件日志尤其有用。

colorConsoleEncoder 会将非上下文的 zap.Field 附加在日志消息之后，格式为 key=value。

本文档描述了 pkg/logger 的主要设计和功能。
