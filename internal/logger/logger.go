// Package logger provides a flexible and configurable logging solution for applications.
// It supports multiple log levels (including custom ones like SUCCESS and FAIL),
// colored console output, JSON file output, and global/instance-based loggers.
package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Level int8

const (
	DebugLevel Level = iota - 1
	InfoLevel
	SuccessLevel
	WarnLevel
	ErrorLevel
	FailLevel
	PanicLevel
	FatalLevel
)

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
		return "fail"
	case PanicLevel:
		return "panic"
	case FatalLevel:
		return "fatal"
	default:
		return fmt.Sprintf("level(%d)", l)
	}
}

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

func (l Level) ToZapLevel() zapcore.Level {
	switch l {
	case DebugLevel:
		return zapcore.DebugLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case SuccessLevel:
		return zapcore.InfoLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	case FailLevel:
		return zapcore.FatalLevel
	case PanicLevel:
		return zapcore.PanicLevel
	case FatalLevel:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

type Options struct {
	ConsoleLevel Level
	FileLevel Level
	LogFilePath string
	ConsoleOutput bool
	FileOutput bool
	ColorConsole bool
	TimestampFormat string
	LogMaxSizeMB  int
	LogMaxBackups int
	LogMaxAgeDays int
	LogCompress   bool
}

type Logger struct {
	*zap.SugaredLogger
	opts        Options
	atomicLevel zap.AtomicLevel
}

var globalLogger *Logger
var once sync.Once

func Init(opts Options) {
	once.Do(func() {
		var err error
		globalLogger, err = NewLogger(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize global logger: %v. Falling back to basic console logging.\n", err)
			cfg := zap.NewDevelopmentConfig()
			cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			fallbackAtomicLevel := zap.NewAtomicLevelAt(zapcore.InfoLevel)
			l, _ := cfg.Build(zap.AddCallerSkip(1), zap.IncreaseLevel(fallbackAtomicLevel))
			globalLogger = &Logger{
				SugaredLogger: l.Sugar(),
				opts:          Options{ConsoleOutput: true, ConsoleLevel: InfoLevel, ColorConsole: true},
				atomicLevel:   fallbackAtomicLevel,
			}
		}
	})
}

func Get() *Logger {
	if globalLogger == nil {
		Init(DefaultOptions())
	}
	return globalLogger
}

func DefaultOptions() Options {
	return Options{
		ConsoleLevel:    InfoLevel,
		FileLevel:       DebugLevel,
		LogFilePath:     "app.log",
		ConsoleOutput:   true,
		FileOutput:      false,
		ColorConsole:    true,
		TimestampFormat: time.RFC3339,
		LogMaxSizeMB:  100,
		LogMaxBackups: 3,
		LogMaxAgeDays: 28,
		LogCompress:   false,
	}
}

func NewLogger(opts Options) (*Logger, error) {
	var cores []zapcore.Core
	if opts.TimestampFormat == "" {
		opts.TimestampFormat = time.RFC3339
	}

	initialAtomicZapLevel := zapcore.FatalLevel
	if opts.ConsoleOutput {
		effectiveConsoleLevel := opts.ConsoleLevel.ToZapLevel()
		if opts.ConsoleLevel == SuccessLevel { effectiveConsoleLevel = zapcore.InfoLevel }
		if opts.ConsoleLevel == FailLevel { effectiveConsoleLevel = zapcore.FatalLevel }
		if effectiveConsoleLevel < initialAtomicZapLevel {
			initialAtomicZapLevel = effectiveConsoleLevel
		}
	}
	if opts.FileOutput {
		effectiveFileLevel := opts.FileLevel.ToZapLevel()
		if opts.FileLevel == SuccessLevel { effectiveFileLevel = zapcore.InfoLevel }
		if opts.FileLevel == FailLevel { effectiveFileLevel = zapcore.FatalLevel }
		if effectiveFileLevel < initialAtomicZapLevel {
			initialAtomicZapLevel = effectiveFileLevel
		}
	}
	if !opts.ConsoleOutput && !opts.FileOutput {
		initialAtomicZapLevel = zapcore.InfoLevel
	}
	atomicLevel := zap.NewAtomicLevelAt(initialAtomicZapLevel)

	if opts.ConsoleOutput {
		consoleEncoderCfg := zap.NewProductionEncoderConfig()
		consoleEncoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat)
		consoleEncoderCfg.TimeKey = "time"; consoleEncoderCfg.LevelKey = ""; consoleEncoderCfg.CallerKey = "caller"
		consoleEncoderCfg.MessageKey = "msg"; consoleEncoderCfg.NameKey = "logger"; consoleEncoderCfg.StacktraceKey = "stacktrace"
		var consoleEncoder zapcore.Encoder
		if opts.ColorConsole { consoleEncoder = NewColorConsoleEncoder(consoleEncoderCfg, opts)
		} else { consoleEncoder = NewPlainTextConsoleEncoder(consoleEncoderCfg, opts) }

		consoleStaticLevel := opts.ConsoleLevel.ToZapLevel()
		if opts.ConsoleLevel == SuccessLevel { consoleStaticLevel = zapcore.InfoLevel }
		if opts.ConsoleLevel == FailLevel { consoleStaticLevel = zapcore.FatalLevel }
		consoleEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= consoleStaticLevel
		})
		cores = append(cores, zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), consoleEnabler))
	}

	if opts.FileOutput {
		if opts.LogFilePath == "" { return nil, fmt.Errorf("log file path cannot be empty when file output is enabled") }
		fileEncoderCfg := zap.NewProductionEncoderConfig()
		fileEncoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat)
		fileEncoderCfg.TimeKey = "time"; fileEncoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		fileEncoderCfg.CallerKey = "caller"; fileEncoderCfg.MessageKey = "msg"; fileEncoderCfg.NameKey = "logger"; fileEncoderCfg.StacktraceKey = "stacktrace"
		fileEncoder := zapcore.NewJSONEncoder(fileEncoderCfg)
		ljLogger := &lumberjack.Logger{
			Filename: opts.LogFilePath, MaxSize: opts.LogMaxSizeMB, MaxBackups: opts.LogMaxBackups, MaxAge: opts.LogMaxAgeDays, Compress: opts.LogCompress,
		}
		fileWriter := zapcore.AddSync(ljLogger)

		fileStaticLevel := opts.FileLevel.ToZapLevel()
		if opts.FileLevel == SuccessLevel { fileStaticLevel = zapcore.InfoLevel }
		if opts.FileLevel == FailLevel { fileStaticLevel = zapcore.FatalLevel }
		fileEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= fileStaticLevel
		})
		cores = append(cores, zapcore.NewCore(fileEncoder, fileWriter, fileEnabler))
	}

	if len(cores) == 0 {
		fmt.Fprintln(os.Stderr, "Warning: No logger output (console or file) configured. Logger will be a no-op.")
		zapNopLogger := zap.NewNop()
		return &Logger{SugaredLogger: zapNopLogger.Sugar(), opts: opts, atomicLevel: zap.NewAtomicLevelAt(zapcore.InfoLevel)}, nil
	}

	core := zapcore.NewTee(cores...)
	// AddCallerSkip(2) because our public methods (e.g., logger.Infof or instance.Infof)
	// involve two levels of calls before reaching the underlying zap SugaredLogger methods (e.g., Infow):
	// 1. logger.Infof -> Get().logWithCustomLevel -> l.SugaredLogger.Infow
	// 2. instance.Infof -> instance.logWithCustomLevel -> l.SugaredLogger.Infow
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2), zap.IncreaseLevel(atomicLevel))

	return &Logger{SugaredLogger: zapLogger.Sugar(), opts: opts, atomicLevel: atomicLevel}, nil
}

func (l *Logger) logWithCustomLevel(level Level, template string, args ...interface{}) {
	if l == nil || l.SugaredLogger == nil {
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

	// Prepare keys and values for the *w methods
	keysAndValues := []interface{}{
		"customlevel", level.CapitalString(),
		"customlevel_num", int(level),
	}

	// No AddCallerSkip via WithOptions here, as it's handled in NewLogger's AddCallerSkip.
	switch level {
	case DebugLevel:
		l.SugaredLogger.Debugw(msg, keysAndValues...)
	case InfoLevel:
		l.SugaredLogger.Infow(msg, keysAndValues...)
	case SuccessLevel: // Success maps to Info for zap's core level
		l.SugaredLogger.Infow(msg, keysAndValues...)
	case WarnLevel:
		l.SugaredLogger.Warnw(msg, keysAndValues...)
	case ErrorLevel:
		l.SugaredLogger.Errorw(msg, keysAndValues...)
	case FailLevel, FatalLevel: // Fail and Fatal map to Fatal for zap's core level
		// For FailLevel, we want the os.Exit(1) behavior, which Fatalw provides.
		l.SugaredLogger.Fatalw(msg, keysAndValues...)
	case PanicLevel:
		l.SugaredLogger.Panicw(msg, keysAndValues...)
	default: // Should not happen with defined levels
		l.SugaredLogger.Infow(msg, keysAndValues...)
	}
}

func (l *Logger) Debugf(template string, args ...interface{}) {
	l.logWithCustomLevel(DebugLevel, template, args...)
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.logWithCustomLevel(InfoLevel, template, args...)
}

func (l *Logger) Successf(template string, args ...interface{}) {
	l.logWithCustomLevel(SuccessLevel, template, args...)
}

func (l *Logger) Warnf(template string, args ...interface{}) {
	l.logWithCustomLevel(WarnLevel, template, args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.logWithCustomLevel(ErrorLevel, template, args...)
}

func (l *Logger) Failf(template string, args ...interface{}) {
	l.logWithCustomLevel(FailLevel, template, args...)
}

func (l *Logger) Panicf(template string, args ...interface{}) {
	l.logWithCustomLevel(PanicLevel, template, args...)
}

func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.logWithCustomLevel(FatalLevel, template, args...)
}

func (l *Logger) Sync() error {
	if l == nil || l.SugaredLogger == nil {
		return nil
	}
	return l.SugaredLogger.Sync()
}

func (l *Logger) With(args ...interface{}) *Logger {
	newSugaredLogger := l.SugaredLogger.With(args...)
	return &Logger{
		SugaredLogger: newSugaredLogger,
		opts:          l.opts,
		atomicLevel:   l.atomicLevel,
	}
}

func Debug(template string, args ...interface{}) {
	Get().logWithCustomLevel(DebugLevel, template, args...)
}

func Info(template string, args ...interface{}) {
	Get().logWithCustomLevel(InfoLevel, template, args...)
}

func Success(template string, args ...interface{}) {
	Get().logWithCustomLevel(SuccessLevel, template, args...)
}

func Warn(template string, args ...interface{}) {
	Get().logWithCustomLevel(WarnLevel, template, args...)
}

func Error(template string, args ...interface{}) {
	Get().logWithCustomLevel(ErrorLevel, template, args...)
}

func Fail(template string, args ...interface{}) {
	Get().logWithCustomLevel(FailLevel, template, args...)
}

func Panic(template string, args ...interface{}) {
	Get().logWithCustomLevel(PanicLevel, template, args...)
}

func Fatal(template string, args ...interface{}) {
	Get().logWithCustomLevel(FatalLevel, template, args...)
}

func SyncGlobal() error {
	return Get().Sync()
}

func (l *Logger) SetLevel(level Level) {
	if l != nil && l.atomicLevel != (zap.AtomicLevel{}) {
		l.atomicLevel.SetLevel(level.ToZapLevel())
	}
}

func SetGlobalLevel(level Level) {
	loggerInstance := Get()
	if loggerInstance != nil && loggerInstance.atomicLevel != (zap.AtomicLevel{}) {
		loggerInstance.atomicLevel.SetLevel(level.ToZapLevel())
	}
}
