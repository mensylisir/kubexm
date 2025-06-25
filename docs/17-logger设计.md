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
```
(注意: 上述示例中的 With(...) 方法直接使用 zap.Field 可能不会被 console_encoder.go 中当前的上下文前缀提取逻辑直接识别，该逻辑似乎依赖于在调用 logWithCustomLevel 时传递的特定键。实际使用中，上下文信息通常由 runtime.Context 传递并由更上层的日志封装注入)

4. 结构化日志与字段
推荐使用格式化字符串（如 Infof, Errorf）或 zap.Field（当与底层的 zap.SugaredLogger 或 zap.Logger 交互时）来提供结构化的键值对信息，这对于文件日志尤其有用。

colorConsoleEncoder 会将非上下文的 zap.Field 附加在日志消息之后，格式为 key=value。

本文档描述了 pkg/logger 的主要设计和功能。


-

### 整体评价：信息丰富、高度可定制的日志中心

**优点 (Strengths):**

1. **结构化日志为核心**: 采用结构化日志（JSON文件输出）是现代应用开发的最佳实践。它极大地提升了日志的可读性（对机器而言），使得日志的采集、解析、索引和告警变得异常简单和高效。
2. **人性化的控制台输出**:
    - **彩色高亮**: colorConsoleEncoder 通过对不同级别的日志（尤其是自定义的Success, Fail）应用不同颜色，极大地增强了人工阅读日志时的体验，能够让人一眼就抓住关键信息。
    - **上下文前缀**: 自动提取并格式化上下文信息（[P:my-pipe][M:etcd]...）是一个**绝妙的设计**。它在保持单行日志简洁性的同时，提供了丰富的、层次化的执行上下文，对于调试并发执行的复杂流程来说，这是无价之宝。
3. **自定义日志级别 (Success, Fail)**:
    - 这是一个非常贴近业务需求的创新。标准的日志级别（Info, Warn, Error）有时无法精确表达“一个重要的里程碑成功了”或“一个可恢复的失败发生了”。Success 和 Fail 提供了更强的语义，让日志的意图更加清晰。
    - Failf 直接绑定 os.Exit(1)，为处理致命错误提供了一个统一、方便的出口。
4. **灵活的配置与双输出**:
    - 通过Options结构体，用户可以精细地控制日志的行为（级别、颜色、文件路径等）。
    - 同时支持**控制台**和**文件**输出，并可以为两者设置不同的日志级别，这是一个非常实用的功能。例如，控制台可以只显示Info及以上级别的信息以保持整洁，而文件则记录Debug级别的所有细节以备排查问题。
5. **易用的API**:
    - 提供了全局函数（logger.Info(...)）和实例方法（logger.Get().With(...).Info(...)）两种使用方式，兼顾了简单场景的便利性和复杂场景（如需要携带上下文）的灵活性。
    - API命名风格（Infof, Errorf等）与Go标准库的fmt和许多其他日志库保持一致，学习成本低。

### 与整体架构的契合度

pkg/logger 是**第二层：基础服务**中最基础、最核心的组件之一，它被几乎所有其他模块所依赖。

- **与 pkg/runtime 的集成**: Runtime 在初始化时会创建并持有一个全局的Logger实例。
- **通过 Context 传递**: 每一层的Context（PipelineContext, ModuleContext, TaskContext, StepContext）都会提供一个GetLogger()方法。
- **上下文注入**: 关键在于，当Runtime或Engine创建下一层的上下文时（例如，从ModuleContext创建TaskContext），它应该使用logger.With(...)方法，将当前层级的上下文信息（如module_name）注入到新的Logger实例中，然后再放入新的Context。这样，当Task或Step中的代码调用ctx.GetLogger().Info(...)时，日志就自动携带了完整的上下文信息，从而被consoleConsoleEncoder正确地格式化。

### 可改进和完善之处

这个设计已经非常完善，改进点主要在于一些细微的体验和集成方面。

1. **日志轮转 (Log Rotation)**:

    - **问题**: 当前设计只指定了一个日志文件路径。如果应用长时间运行，这个文件会无限增大，需要手动或通过外部工具（如logrotate）来管理。
    - **完善方案**: 可以集成一个日志轮转库（如 gopkg.in/natefinch/lumberjack.v2）。在logger.Init中，当FileOutput为true时，不是直接打开一个文件，而是创建一个lumberjack.Logger实例，并将其作为zapcore.WriteSyncer。Options中可以增加相应的配置字段，如LogMaxSizeMB, LogMaxBackups, LogMaxAgeDays。

2. **更智能的上下文管理**:

    - **问题**: 如您在示例代码注释中指出的，开发者需要手动地、正确地使用With(...)来添加上下文信息。

    - **完善方案**: 可以创建一个ContextualLogger的包装器。Runtime在创建各层Context时，不仅仅是简单地With(...)，而是返回一个已经预设好上下文的ContextualLogger实例。

      Generated go

      ```
      // 在StepContext中
      func (ctx *myStepContext) GetLogger() *logger.ContextualLogger {
          // 这个logger在创建时就已经被注入了pipeline, module, task, step, host等所有信息
          return ctx.pre-configuredLogger 
      }
      ```

      content_copydownload

      Use code [with caution](https://support.google.com/legal/answer/13505487).Go

      这样，Step的开发者调用ctx.GetLogger().Info(...)时，无需再关心如何附加上下文，体验会更流畅。

3. **动态级别调整**:

    - **问题**: 日志级别在初始化后是固定的。有时我们希望在程序运行时，动态地调整日志级别来进行在线调试，而无需重启服务。
    - **完善方案**: zap本身支持AtomicLevel。logger.Init可以使用zap.NewAtomicLevelAt(...)。Logger可以暴露一个SetLevel(level Level)的方法。然后，可以通过一个API端点或信号来触发这个方法，从而动态地改变日志级别。

### 总结：架构的“眼睛”和“嘴巴”

pkg/logger 是整个“世界树”项目的**眼睛（观察系统状态）和嘴巴（报告执行情况）**。

- 它**信息丰富**，通过结构化和上下文前缀，提供了调试和监控所需的一切信息。
- 它**用户友好**，通过彩色和自定义级别，让开发者和运维人员都能轻松地理解执行过程。
- 它**性能卓越**，基于zap保证了在高并发下也不会成为系统瓶颈。

这是一个顶级水准的日志模块设计，它不仅仅是一个日志记录工具，更是一个集成了项目业务领域知识（如Success/Fail级别、上下文前缀）的、深度定制的**可观测性解决方案**。它将为kubexm项目的开发、调试和长期运维提供巨大的价值。

