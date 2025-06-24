// Package logger provides a flexible and configurable logging solution for applications.
// It supports multiple log levels (including custom ones like SUCCESS and FAIL),
// colored console output, JSON file output, and global/instance-based loggers.
//
// Basic Usage (Global Logger):
//
// import "github.com/mensylisir/kubexm/pkg/logger" // Adjust import path
//
//	func main() {
//	  // Initialize the global logger once at the start of your application.
//	  // Default options log INFO+ to console (colored) and DEBUG+ to "app.log" (JSON, if enabled).
//	  logOpts := logger.DefaultOptions()
//	  logOpts.FileOutput = true // Enable file logging for this example
//	  logOpts.LogFilePath = "my_application.log"
//	  logOpts.ConsoleLevel = logger.DebugLevel // Show debug messages on console
//	  logger.Init(logOpts)
//
//	  defer logger.SyncGlobal() // Flush buffered logs before exiting
//
//	  logger.Debug("This is a debug message.")
//	  logger.Info("Application starting...")
//	  logger.Success("Deployment successful!")
//	  logger.Warn("Configuration value is unusual.")
//	  logger.Error("Failed to connect to database: %s", "connection refused")
//	  // logger.Fail("Critical system failure, exiting.") // This would os.Exit(1)
//	}
//
// Instance-based Logger:
//
//	opts := logger.DefaultOptions()
//	opts.ConsoleLevel = logger.InfoLevel
//	opts.LogFilePath = "module_specific.log"
//	opts.FileOutput = true
//	myModuleLogger, err := logger.NewLogger(opts)
//	if err != nil {
//	  // Handle error
//	}
//	defer myModuleLogger.Sync()
//	myModuleLogger.Infof("Special logs for my module.")
package logger

import (
	"fmt"
	"os"
	// "strings" // Removed as unused
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level defines the log level.
// These levels are used to control verbosity and are mapped to zapcore.Level
// for the underlying Zap logger. Custom levels like SuccessLevel and FailLevel
// are handled by the custom console encoder for special display.
type Level int8

// Log levels constants.
const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production. Useful for detailed troubleshooting.
	DebugLevel Level = iota - 1
	// InfoLevel is the default logging priority. Use for general operational messages.
	InfoLevel
	// SuccessLevel logs indicate successful completion of significant operations.
	// Displayed distinctively in the console (e.g., green).
	SuccessLevel // Custom level
	// WarnLevel logs are more important than Info, but don't need individual
	// human review. Potential issues that are not yet errors.
	WarnLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-level logs. These indicate problems.
	ErrorLevel
	// FailLevel logs indicate a critical failure, and the logger will call os.Exit(1)
	// after logging the message. Use for unrecoverable errors.
	FailLevel // Custom level, maps to zapcore.FatalLevel conceptually
	// PanicLevel logs a message, then panics. Should be used sparingly.
	PanicLevel // Maps to zapcore.PanicLevel
	// FatalLevel logs a message, then calls os.Exit(1).
	// Note: FailLevel is the preferred way to log fatal errors with custom prefix.
	// This level is exposed for direct compatibility if needed.
	FatalLevel // Maps to zapcore.FatalLevel (used by FailLevel)
)

// String returns a lowercase string representation of the Level.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case SuccessLevel:
		return "success"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FailLevel:
		return "fail" // For display, maps to Fatal internally
	case PanicLevel:
		return "panic"
	case FatalLevel: // This is zap's FatalLevel, used by our FailLevel
		return "fatal"
	default:
		return fmt.Sprintf("level(%d)", l)
	}
}

// CapitalString returns a capitalized string representation of the Level.
func (l Level) CapitalString() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case SuccessLevel:
		return "SUCCESS"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FailLevel:
		return "FAIL"
	case PanicLevel:
		return "PANIC"
	case FatalLevel:
		return "FATAL"
	default:
		return fmt.Sprintf("LEVEL(%d)", l)
	}
}

// ToZapLevel converts our custom Level to zapcore.Level.
func (l Level) ToZapLevel() zapcore.Level {
	switch l {
	case DebugLevel:
		return zapcore.DebugLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case SuccessLevel: // Success will be logged at InfoLevel in Zap, but with custom prefix/color
		return zapcore.InfoLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	case FailLevel: // Fail will use Zap's FatalLevel
		return zapcore.FatalLevel
	case PanicLevel:
		return zapcore.PanicLevel
	case FatalLevel:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel // Default to Info
	}
}

// Options holds configuration for the logger.
type Options struct {
	// ConsoleLevel sets the minimum log level for console output.
	ConsoleLevel Level
	// FileLevel sets the minimum log level for file output.
	FileLevel Level
	// LogFilePath specifies the path to the log file. Required if FileOutput is true.
	LogFilePath string
	// ConsoleOutput enables or disables logging to the console (os.Stdout).
	ConsoleOutput bool
	// FileOutput enables or disables logging to a file.
	FileOutput bool
	// ColorConsole enables or disables ANSI color codes for console output.
	ColorConsole bool
	// TimestampFormat defines the format for timestamps in logs (e.g., time.RFC3339 or "2006-01-02T15:04:05Z07:00").
	TimestampFormat string
}

// Logger is a wrapper around zap.SugaredLogger, providing custom level handling
// and easier configuration for console and file outputs.
type Logger struct {
	*zap.SugaredLogger
	opts Options
	mu   sync.Mutex
}

var globalLogger *Logger
var once sync.Once

// Init initializes the global logger instance with the provided options.
// This function should be called only once, typically at the beginning of the
// application (e.g., in the main function). Subsequent calls to Init are no-ops due to sync.Once.
// If initialization fails (e.g., file path not writable), it falls back to a basic Zap development logger
// printing to stderr, ensuring that logging capabilities are always available in some form.
func Init(opts Options) {
	once.Do(func() {
		var err error
		globalLogger, err = NewLogger(opts)
		if err != nil {
			// Fallback to a basic console logger if initialization fails
			fmt.Fprintf(os.Stderr, "Failed to initialize global logger: %v. Falling back to basic console logging.\n", err)
			cfg := zap.NewDevelopmentConfig()
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // Basic colored output
			l, _ := cfg.Build(zap.AddCallerSkip(1))                          // Skip this Init frame
			globalLogger = &Logger{SugaredLogger: l.Sugar(), opts: Options{ConsoleOutput: true, ConsoleLevel: InfoLevel, ColorConsole: true}}
		}
	})
}

// Get returns the global logger instance.
// If Init() has not been called before Get(), Get will implicitly call Init() with DefaultOptions()
// to ensure a usable logger is always returned. For predictable configuration, it's recommended
// to explicitly call Init() once at application startup.
func Get() *Logger {
	if globalLogger == nil { // If Init was never called
		Init(DefaultOptions()) // Initialize with defaults
	}
	return globalLogger
}

// DefaultOptions returns a default logger configuration:
// - ConsoleLevel: InfoLevel
// - FileLevel: DebugLevel (logs more verbosely to file than console by default)
// - LogFilePath: "app.log" (default filename if file output is enabled)
// - ConsoleOutput: true (logging to console is enabled by default)
// - FileOutput: false (logging to file is disabled by default to prevent accidental file creation)
// - ColorConsole: true (console output will be colored by default)
// - TimestampFormat: time.RFC3339 (standard timestamp format)
func DefaultOptions() Options {
	return Options{
		ConsoleLevel:    InfoLevel,
		FileLevel:       DebugLevel,
		LogFilePath:     "app.log",
		ConsoleOutput:   true,
		FileOutput:      false,
		ColorConsole:    true,
		TimestampFormat: time.RFC3339,
	}
}

// NewLogger creates a new Logger instance based on the provided options.
// This is useful if you need multiple logger instances with different configurations
// (e.g., separate log files or console formats for different parts of an application),
// though typically the global logger (configured via Init and accessed via Get) is sufficient.
func NewLogger(opts Options) (*Logger, error) {
	var cores []zapcore.Core

	if opts.TimestampFormat == "" {
		opts.TimestampFormat = time.RFC3339 // Default timestamp format if not provided
	}

	// Console Core
	if opts.ConsoleOutput {
		consoleEncoderCfg := zap.NewProductionEncoderConfig()
		consoleEncoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat)
		consoleEncoderCfg.TimeKey = "time"
		// LevelKey is intentionally left empty for console as our custom encoder handles level prefix.
		consoleEncoderCfg.LevelKey = ""
		consoleEncoderCfg.CallerKey = "caller"
		consoleEncoderCfg.MessageKey = "msg"
		consoleEncoderCfg.NameKey = "logger"
		consoleEncoderCfg.StacktraceKey = "stacktrace"

		var consoleEncoder zapcore.Encoder
		if opts.ColorConsole {
			// Use our custom console encoder for level prefix, color, and custom level handling
			consoleEncoder = NewColorConsoleEncoder(consoleEncoderCfg, opts)
		} else {
			consoleEncoder = NewPlainTextConsoleEncoder(consoleEncoderCfg, opts)
		}

		consoleLevelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			// This logic ensures that if a user sets ConsoleLevel to e.g. InfoLevel,
			// they still see SUCCESS (which is InfoLevel in Zap) and FAIL (FatalLevel in Zap).
			if opts.ConsoleLevel == SuccessLevel { // User wants to see SUCCESS logs
				return lvl >= zapcore.InfoLevel // Success logs are InfoLevel in Zap
			}
			if opts.ConsoleLevel == FailLevel { // User wants to see FAIL logs
				return lvl >= zapcore.FatalLevel // Fail logs are FatalLevel in Zap
			}
			// For other levels, compare directly with their Zap equivalent.
			return lvl >= opts.ConsoleLevel.ToZapLevel()
		})
		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), consoleLevelEnabler)
		cores = append(cores, consoleCore)
	}

	// File Core
	if opts.FileOutput {
		if opts.LogFilePath == "" {
			return nil, fmt.Errorf("log file path cannot be empty when file output is enabled")
		}
		fileEncoderCfg := zap.NewProductionEncoderConfig()
		fileEncoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat)
		// For file output, use Zap's standard level encoding (e.g., "INFO", "ERROR").
		fileEncoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

		// JSON format is generally preferred for file logs for easier parsing.
		fileEncoder := zapcore.NewJSONEncoder(fileEncoderCfg)

		file, err := os.OpenFile(opts.LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", opts.LogFilePath, err)
		}
		fileWriter := zapcore.AddSync(file)

		fileLevelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			// Similar logic as console for custom levels, ensuring they are included if their
			// underlying Zap level meets the FileLevel threshold.
			if opts.FileLevel == SuccessLevel {
				return lvl >= zapcore.InfoLevel
			}
			if opts.FileLevel == FailLevel {
				return lvl >= zapcore.FatalLevel
			}
			return lvl >= opts.FileLevel.ToZapLevel()
		})
		fileCore := zapcore.NewCore(fileEncoder, fileWriter, fileLevelEnabler)
		cores = append(cores, fileCore)
	}

	if len(cores) == 0 {
		// If no outputs are configured (e.g. both ConsoleOutput and FileOutput are false),
		// Zap's default is to use a no-op logger. We can explicitly return a no-op logger
		// or rely on the caller to handle this. The Init function provides a fallback,
		// so this path might not be hit if NewLogger is only called via Init or with valid options.
		// However, to be robust, if NewLogger is called directly with no outputs,
		// we return a logger that essentially does nothing to prevent nil pointer issues.
		fmt.Fprintln(os.Stderr, "Warning: No logger output (console or file) configured. Logger will be a no-op.")
		zapNopLogger := zap.NewNop()
		return &Logger{SugaredLogger: zapNopLogger.Sugar(), opts: opts}, nil
	}

	core := zapcore.NewTee(cores...)
	// AddCallerSkip(1) ensures that the caller information in logs points to the user's code
	// that called our logger methods (e.g., logger.Infof), rather than the methods within this package.
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{SugaredLogger: zapLogger.Sugar(), opts: opts}, nil
}

// logWithCustomLevel is an internal helper to manage logging with our custom Level type.
// It translates our Level to the appropriate zap.SugaredLogger method and ensures
// the "customlevel" field is passed for the console encoder to use.
// It also centralizes the AddCallerSkip logic for wrapper methods.
func (l *Logger) logWithCustomLevel(level Level, template string, args ...interface{}) {
	if l == nil || l.SugaredLogger == nil {
		// This fallback is for cases where the logger instance itself is nil,
		// which Get() and Init() try to prevent for the global logger.
		// For instance-based loggers, the user should check the error from NewLogger.
		fmt.Fprintf(os.Stderr, "Logger not initialized. Message: %s "+template+"\n", level.CapitalString(), fmt.Sprintf(template, args...))
		if level == FailLevel || level == FatalLevel {
			os.Exit(1)
		}
		if level == PanicLevel {
			panic(fmt.Sprintf(template, args...))
		}
		return
	}

	msg := fmt.Sprintf(template, args...)
	// Pass the original custom level string as a field for the encoder to use.
	// This field ("customlevel") is specifically looked for by colorConsoleEncoder.
	customLevelField := zap.String("customlevel", level.CapitalString())
	// Add a hidden field for the original numeric level for sorting/filtering if needed by encoder later
	customNumericLevelField := zap.Int8("customlevel_num", int8(level))

	// Use WithOptions to adjust caller skip for each log call from these wrappers.
	// AddCallerSkip(1) here means the original call site (e.g., user calling logger.Infof)
	// will be reported, not the logWithCustomLevel function itself.
	loggerWithSkip := l.SugaredLogger.WithOptions(zap.AddCallerSkip(1))

	switch level {
	case DebugLevel:
		loggerWithSkip.Debugw(msg, customLevelField, customNumericLevelField)
	case InfoLevel:
		loggerWithSkip.Infow(msg, customLevelField, customNumericLevelField)
	case SuccessLevel:
		// SuccessLevel is logged at Zap's InfoLevel, but our custom encoder
		// will use the "customlevel":"SUCCESS" field to format it distinctively.
		loggerWithSkip.Infow(msg, customLevelField, customNumericLevelField)
	case WarnLevel:
		loggerWithSkip.Warnw(msg, customLevelField, customNumericLevelField)
	case ErrorLevel:
		loggerWithSkip.Errorw(msg, customLevelField, customNumericLevelField)
	case FailLevel:
		// FailLevel is logged at Zap's FatalLevel, causing an os.Exit(1) after logging.
		// The "customlevel":"FAIL" field allows custom formatting.
		loggerWithSkip.Fatalw(msg, customLevelField, customNumericLevelField)
	case PanicLevel:
		loggerWithSkip.Panicw(msg, customLevelField, customNumericLevelField)
	case FatalLevel:
		// Direct Zap FatalLevel, also exits.
		loggerWithSkip.Fatalw(msg, customLevelField, customNumericLevelField)
	default: // Should not happen with defined levels
		loggerWithSkip.Infow(msg, customLevelField, customNumericLevelField, zap.String("unknownlevel", level.String()))
	}
}

// Debugf logs a message at DebugLevel.
func (l *Logger) Debugf(template string, args ...interface{}) {
	l.logWithCustomLevel(DebugLevel, template, args...)
}

// Infof logs a message at InfoLevel.
func (l *Logger) Infof(template string, args ...interface{}) {
	l.logWithCustomLevel(InfoLevel, template, args...)
}

// Successf logs a message at SuccessLevel.
// This will be displayed distinctively by the color console encoder.
func (l *Logger) Successf(template string, args ...interface{}) {
	l.logWithCustomLevel(SuccessLevel, template, args...)
}

// Warnf logs a message at WarnLevel.
func (l *Logger) Warnf(template string, args ...interface{}) {
	l.logWithCustomLevel(WarnLevel, template, args...)
}

// Errorf logs a message at ErrorLevel.
func (l *Logger) Errorf(template string, args ...interface{}) {
	l.logWithCustomLevel(ErrorLevel, template, args...)
}

// Failf logs a message at FailLevel and then calls os.Exit(1) via Zap's FatalLevel.
// This is for critical, unrecoverable errors.
func (l *Logger) Failf(template string, args ...interface{}) {
	l.logWithCustomLevel(FailLevel, template, args...)
}

// Panicf logs a message at PanicLevel then panics.
func (l *Logger) Panicf(template string, args ...interface{}) {
	l.logWithCustomLevel(PanicLevel, template, args...)
}

// Fatalf logs a message at FatalLevel then calls os.Exit(1).
// This is kept for direct Zap Fatal compatibility. For custom "FAIL" prefixing, use Failf.
func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.logWithCustomLevel(FatalLevel, template, args...)
}

// Sync flushes any buffered log entries. It's important to call this
// before application exit to ensure all logs are written.
func (l *Logger) Sync() error {
	if l == nil || l.SugaredLogger == nil {
		return nil
	}
	return l.SugaredLogger.Sync()
}

func (l *Logger) With(args ...interface{}) *Logger {
	// Call the embedded SugaredLogger's With method
	newSugaredLogger := l.SugaredLogger.With(args...)

	// Create a new *Logger instance, wrapping the new SugaredLogger.
	// We also carry over the original options.
	return &Logger{
		SugaredLogger: newSugaredLogger,
		opts:          l.opts,
		// mu is not copied as the new logger is a distinct instance.
	}
}

// Global logging functions that use the globalLogger instance.
// These provide convenient package-level access to logging functionality
// once the global logger is initialized via Init().

// Debug logs a message at DebugLevel using the global logger.
func Debug(template string, args ...interface{}) {
	Get().logWithCustomLevel(DebugLevel, template, args...)
}

// Info logs a message at InfoLevel using the global logger.
func Info(template string, args ...interface{}) {
	Get().logWithCustomLevel(InfoLevel, template, args...)
}

// Success logs a message at SuccessLevel using the global logger.
func Success(template string, args ...interface{}) {
	Get().logWithCustomLevel(SuccessLevel, template, args...)
}

// Warn logs a message at WarnLevel using the global logger.
func Warn(template string, args ...interface{}) {
	Get().logWithCustomLevel(WarnLevel, template, args...)
}

// Error logs a message at ErrorLevel using the global logger.
func Error(template string, args ...interface{}) {
	Get().logWithCustomLevel(ErrorLevel, template, args...)
}

// Fail logs a message at FailLevel then calls os.Exit(1) using the global logger.
func Fail(template string, args ...interface{}) {
	Get().logWithCustomLevel(FailLevel, template, args...)
}

// Panic logs a message at PanicLevel then panics using the global logger.
func Panic(template string, args ...interface{}) {
	Get().logWithCustomLevel(PanicLevel, template, args...)
}

// Fatal logs a message at FatalLevel then calls os.Exit(1) using the global logger.
func Fatal(template string, args ...interface{}) {
	Get().logWithCustomLevel(FatalLevel, template, args...)
}

// SyncGlobal flushes any buffered log entries for the global logger.
// It's good practice to call this before the application exits.
func SyncGlobal() error {
	// Get() ensures globalLogger is initialized if it was nil, then calls Sync on it.
	return Get().Sync()
}
