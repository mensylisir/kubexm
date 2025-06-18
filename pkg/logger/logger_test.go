package logger

import (
	"bytes"
	// "context" // Not directly used by these logger tests
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap" // For adding fields in test
	"go.uber.org/zap/zapcore"
)

// captureStdout (as previously defined)
func captureStdout(f func()) (string, error) {
	r, w, err := os.Pipe(); if err != nil { return "", err };
	originalStdout := os.Stdout
	os.Stdout = w

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		r.Close() // Close reader a bit earlier
		outC <- buf.String()
	}()

	f()
	os.Stdout = originalStdout // Restore stdout
	errClose := w.Close() // Close writer to signal EOF to reader goroutine.

	output := <-outC
	return output, errClose // Return error from closing the writer if any
}


// TestNewLogger_ConsoleOutput (as previously defined, ensure it still passes or adapt)
func TestNewLogger_ConsoleOutput(t *testing.T) {
	// Reset global logger for this test
	globalLogger = nil; once = sync.Once{}
	opts := DefaultOptions(); opts.ConsoleLevel = DebugLevel; opts.FileOutput = false; opts.ColorConsole = false
	logger, err := NewLogger(opts); if err != nil { t.Fatalf("NewLogger() error = %v", err) }; defer logger.Sync()
	testMsg := "Test console message"; expectedLevelPrefix := "[INFO]"
	output, errCap := captureStdout(func() { logger.Infof(testMsg) })
	if errCap != nil { t.Fatalf("Failed to capture stdout: %v", errCap) }
	if !strings.Contains(output, expectedLevelPrefix) { t.Errorf("Console output missing level prefix. Got: %q", output) }
	if !strings.Contains(output, testMsg) { t.Errorf("Console output missing message. Got: %q", output) }
}

// TestNewLogger_FileOutput (as previously defined, ensure it still passes or adapt)
func TestNewLogger_FileOutput(t *testing.T) {
	// Reset global logger for this test
	globalLogger = nil; once = sync.Once{}
	tmpDir, _ := os.MkdirTemp("", "logtest"); defer os.RemoveAll(tmpDir)
	logFilePath := filepath.Join(tmpDir, "test.log")
	opts := DefaultOptions(); opts.FileLevel = InfoLevel; opts.LogFilePath = logFilePath; opts.FileOutput = true; opts.ConsoleOutput = false
	logger, err := NewLogger(opts); if err != nil { t.Fatalf("NewLogger() error = %v", err) };
	testMsg := "Test file message"; logger.Infof(testMsg); logger.Debugf("No debug in file"); logger.Sync()
	content, _ := os.ReadFile(logFilePath); logContent := string(content)
	if !strings.Contains(logContent, testMsg) { t.Errorf("Log file missing message. Got: %q", logContent) }
	if strings.Contains(logContent, "No debug in file") {t.Errorf("Log file contains debug. Content: %q", logContent)}
}


func TestNewLogger_ColoredConsoleOutput_WithContextPrefix(t *testing.T) {
	baseOpts := DefaultOptions()
	baseOpts.ConsoleLevel = DebugLevel
	baseOpts.FileOutput = false
	baseOpts.ColorConsole = true

	// Test cases
	testCases := []struct {
		name              string
		level             Level
		message           string
		logFields         []zap.Field
		expectedColor     string
		levelString       string
		expectedCtxPrefix string
	}{
		{
			"SuccessWithFullContext", SuccessLevel, "A success message",
			[]zap.Field{
				zap.String("pipeline_name", "Pipe1"), zap.String("module_name", "ModA"),
				zap.String("task_name", "TaskX"), zap.String("step_name", "Step1"),
				zap.String("host_name", "host-01"),
			},
			colorGreen, "[SUCCESS]", "[P:Pipe1][M:ModA][T:TaskX][S:Step1][H:host-01]",
		},
		{
			"ErrorWithPartialContext", ErrorLevel, "An error message",
			[]zap.Field{
				zap.String("pipeline_name", "Pipe1"), zap.String("module_name", "ModB"),
				// Missing task_name, step_name, host_name
			},
			colorRed, "[ERROR]", "[P:Pipe1][M:ModB]",
		},
		{
			"WarnForHook", WarnLevel, "A warning for a hook",
			[]zap.Field{
				zap.String("pipeline_name", "Pipe1"), zap.String("module_name", "ModC"),
				zap.String("hook_event", "ModulePreRun"), zap.String("hook_step_name", "PreHookStep"),
				zap.String("host_name", "control-plane-1"),
			},
			colorYellow, "[WARN]", "[P:Pipe1][M:ModC][HE:ModulePreRun:PreHookStep][H:control-plane-1]",
		},
		{
			"InfoNoContext", InfoLevel, "An info message with no specific exec context",
			[]zap.Field{},
			"",
			"[INFO]",      "",
		},
		{
			"DebugWithHostOnly", DebugLevel, "Debug for a specific host",
			[]zap.Field{zap.String("host_name", "worker-5")},
			colorMagenta, "[DEBUG]", "[H:worker-5]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			globalLogger = nil; once = sync.Once{}
			Init(baseOpts)

			// The fields from tc.logFields are passed to the ...w methods of zap.
			// Our logWithCustomLevel wrapper passes these along.
			// The "customlevel" field is added by logWithCustomLevel itself.

			// Convert zap.Field to the format expected by our logger's ...f methods (key-value pairs)
			// This is a bit of a workaround for testing the full stack.
			// A direct encoder test would construct zapcore.Entry and fields manually.
			var keyValueArgs []interface{}
			if len(tc.logFields) > 0 {
				// Our logger's ...f methods don't directly take zap.Field.
				// They take a template and variadic args.
				// The underlying ...w methods in logWithCustomLevel *do* take zap.Field.
				// So, we need to call the ...w methods.
				// The global funcs (Info, Debug, etc.) call logWithCustomLevel.
			}


			output, errCap := captureStdout(func() {
				// Get a fresh global logger for each call, or manage state.
				// For this test, global logger is already set up with baseOpts.

				// Construct the fields to pass to the *w methods.
				// Our logWithCustomLevel adds "customlevel" automatically.
				// The tc.logFields simulate context fields added by With.
				// To test the encoder, we need these fields to be part of the `fields`
				// argument to EncodeEntry. Zap's `With` normally achieves this.
				// Here, we simulate it by passing them at the call site to `*w` methods.

				lg := Get().SugaredLogger // Get the underlying SugaredLogger

				// If there are context fields, create a new logger with them.
				if len(tc.logFields) > 0 {
					lg = lg.With(tc.logFields...)
				}

				// Now call the appropriate level method on this (potentially contextualized) logger.
				// logWithCustomLevel adds "customlevel", so we call the Zap methods directly for this test.
				switch tc.level {
				case DebugLevel: lg.Debugw(tc.message, zap.String("customlevel", tc.level.CapitalString()))
				case InfoLevel: lg.Infow(tc.message, zap.String("customlevel", tc.level.CapitalString()))
				case SuccessLevel: lg.Infow(tc.message, zap.String("customlevel", tc.level.CapitalString())) // Logs as Info
				case WarnLevel: lg.Warnw(tc.message, zap.String("customlevel", tc.level.CapitalString()))
				case ErrorLevel: lg.Errorw(tc.message, zap.String("customlevel", tc.level.CapitalString()))
				// Fatal/Panic would exit/panic test, TestFailAndFatalLevels covers their formatting.
				default: t.Logf("Test case for level %s not fully handled for stdout capture.", tc.level)
				}
			})
			if errCap != nil { t.Fatalf("Failed to capture stdout for %s: %v", tc.name, errCap) }
			if tc.level == FatalLevel || tc.level == PanicLevel || tc.level == FailLevel { return }


			if !strings.Contains(output, tc.message) {
				t.Errorf("Console output for %s missing message. Got: %q, Expected: %q", tc.name, output, tc.message)
			}

			// Construct the full expected prefix including context and level
			var expectedPrefixElements []string
			if tc.expectedCtxPrefix != "" {
				expectedPrefixElements = append(expectedPrefixElements, tc.expectedCtxPrefix)
			}
			expectedPrefixElements = append(expectedPrefixElements, tc.levelString)
			fullExpectedPrefix := strings.Join(expectedPrefixElements, " ")

            normalizedOutput := strings.Join(strings.Fields(output), " ")
            normalizedExpectedPrefix := strings.Join(strings.Fields(fullExpectedPrefix), " ")

			// Check if the output *contains* the prefix. Timestamp makes exact match hard.
			if !strings.Contains(normalizedOutput, normalizedExpectedPrefix) {
				t.Errorf("Console output for %s:\nExpected prefix (normalized): %q\nActual output   (normalized): %q\nFull output:\n%s",
				    tc.name, normalizedExpectedPrefix, normalizedOutput, output)
			}

			if tc.expectedColor != "" {
				// Check if the colored level string is present
				expectedColoredLevelString := tc.expectedColor + tc.levelString + colorReset
				if !strings.Contains(output, expectedColoredLevelString) {
                     t.Errorf("Console output for %s missing expected color sequence %q around level. Got: %q", tc.name, expectedColoredLevelString, output)
                }
			}
		})
	}
}


// TestLogLevelFiltering (as previously defined, should still pass)
func TestLogLevelFiltering(t *testing.T) {
	globalLogger = nil; once = sync.Once{}; opts := DefaultOptions(); opts.ConsoleLevel = WarnLevel; opts.FileOutput = false; opts.ColorConsole = false
	logger, err := NewLogger(opts); if err != nil { t.Fatalf("NewLogger error: %v", err)}; defer logger.Sync()
	var output string; output, _ = captureStdout(func() {
		logger.Debugf("debug_test"); logger.Infof("info_test"); logger.Successf("success_test");
		logger.Warnf("warn_test"); logger.Errorf("error_test")
	})
	if strings.Contains(output, "debug_test") {t.Error("Contains DEBUG")}; if strings.Contains(output, "success_test") {t.Error("Contains SUCCESS")}; if strings.Contains(output, "info_test") {t.Error("Contains INFO")}
	if !strings.Contains(output, "warn_test") {t.Error("Missing WARN")}; if !strings.Contains(output, "error_test") {t.Error("Missing ERROR")}
}

// TestGlobalLogger (as previously defined, should still pass)
func TestGlobalLogger(t *testing.T) {
	originalGlobalLogger := globalLogger; originalOnce := once; defer func() { globalLogger = originalGlobalLogger; once = originalOnce }()
	globalLogger = nil; once = sync.Once{}; opts := DefaultOptions(); opts.ConsoleLevel = InfoLevel; opts.FileOutput = false; opts.ColorConsole = false; Init(opts); defer SyncGlobal()
	output, _ := captureStdout(func() { Info("Global logger test"); Success("Global success test") })
	if !strings.Contains(output, "[INFO] Global logger test") {t.Errorf("Global Info() fail. Got: %s", output)}
	if !strings.Contains(output, "[SUCCESS] Global success test") {t.Errorf("Global Success() fail. Got: %s", output)}
	secondOpts := DefaultOptions(); secondOpts.ConsoleLevel = DebugLevel; Init(secondOpts)
	output2, _ := captureStdout(func() { Debug("Global debug, should not appear") })
	if strings.Contains(output2, "Global debug") {t.Error("Global Debug() appeared, Init called again.")}
}

// TestTimestampFormat (as previously defined, should still pass)
func TestTimestampFormat(t *testing.T) {
	globalLogger = nil; once = sync.Once{}; customFormat := "2006/01/02_15:04:05"; opts := DefaultOptions(); opts.ConsoleLevel=InfoLevel; opts.FileOutput=false; opts.ColorConsole=false; opts.TimestampFormat=customFormat
	logger, err := NewLogger(opts); if err != nil {t.Fatalf("NewLogger err: %v",err)}; defer logger.Sync()
	output, _ := captureStdout(func() { logger.Infof("Timestamp test") })
	re := regexp.MustCompile(`\d{4}/\d{2}/\d{2}_\d{2}:\d{2}:\d{2}`); if !re.MatchString(output) {t.Errorf("Timestamp wrong format %s. Got: %q", customFormat, output)}
}

// TestFailAndFatalLevels (as previously defined, should still pass for buffered output)
func TestFailAndFatalLevels(t *testing.T) {
    globalLogger = nil; once = sync.Once{}; opts := DefaultOptions(); opts.ConsoleLevel = InfoLevel; opts.FileOutput = false; opts.ColorConsole = false
    t.Run("FailLevelOutput", func(t *testing.T) {
        var buf bytes.Buffer; fatalCoreCfg := zap.NewProductionEncoderConfig(); fatalCoreCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat); fatalCoreCfg.TimeKey="time"; fatalCoreCfg.LevelKey="";
        fatalEncoder := NewPlainTextConsoleEncoder(fatalCoreCfg, opts)
        core := zapcore.NewCore(fatalEncoder, zapcore.AddSync(&buf), zap.NewAtomicLevelAt(zapcore.FatalLevel))
		tempZapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2)) // Skip this test func and logWithCustomLevel
        tempLogger := &Logger{SugaredLogger: tempZapLogger.Sugar(), opts: opts}
		// Simulate call from our logger's Failf -> logWithCustomLevel -> zap's Fatalw
        tempLogger.logWithCustomLevel(FailLevel, "This is a fail test")
        // tempLogger.Sync() // Sync might not happen if Fatalw exits, but buffer should have content
        output := buf.String()
        if !strings.Contains(output, "[FAIL]") {t.Errorf("Fail log missing [FAIL]. Got: %s", output)}
        if !strings.Contains(output, "This is a fail test") {t.Errorf("Fail log missing message. Got: %s", output)}
    })
}
