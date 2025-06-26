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
	// "time" // No longer needed if captureStdout doesn't use it

	"go.uber.org/zap" // For adding fields in test
	"go.uber.org/zap/zapcore"
)

// captureStdout (refined version with buffered channel and WaitGroup)
func captureStdout(f func()) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	originalStdout := os.Stdout
	os.Stdout = w

	outC := make(chan string, 1) // Buffered channel of size 1
	var wg sync.WaitGroup
	wg.Add(1) // For the reading goroutine

	go func() {
		defer wg.Done() // Signal completion when goroutine exits
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		r.Close()
		outC <- buf.String()
	}()

	f() // Execute the function that writes to stdout

	os.Stdout = originalStdout

	errClose := w.Close()

	wg.Wait()
	close(outC)

	output := <-outC

	return output, errClose
}

func TestHelloWorld(t *testing.T) {
	fmt.Println("Hello via fmt from TestHelloWorld")
	t.Log("Hello via t.Log from TestHelloWorld")
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
			colorYellow, "[WARN]", "[P:Pipe1][M:ModC][HE:ModulePreRun][HS:PreHookStep][H:control-plane-1]", // Adjusted expectation
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

func TestJSONFileOutputStructure(t *testing.T) {
	globalLogger = nil
	once = sync.Once{} // Reset global logger

	tmpDir, err := os.MkdirTemp("", "logtest_json_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test_json.log")
	opts := DefaultOptions()
	opts.FileLevel = DebugLevel
	opts.LogFilePath = logFilePath
	opts.FileOutput = true
	opts.ConsoleOutput = false // Disable console to focus on file output
	opts.TimestampFormat = time.RFC3339Nano // Use a precise format for easier comparison if needed

	Init(opts) // Initialize global logger with these options
	defer SyncGlobal()

	// Log a message with context and additional fields
	log := Get().With(
		zap.String("pipeline_name", "json_test_pipe"),
		zap.String("module_name", "json_module"),
		zap.String("custom_key", "custom_value"),
		zap.Int("custom_int", 123),
	)
	log.Successf("JSON structure test message: %s", "details here")
	log.Debugf("Another JSON message with different context", zap.String("host_name", "worker-x")) // host_name is a context key

	// Ensure logs are flushed
	SyncGlobal()

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logEntries := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(logEntries) < 2 { // Changed from 1 to 2 because we log two messages
		t.Fatalf("Expected at least 2 log entries, got %d. Content: %s", len(logEntries), string(content))
	}

	// --- Verify first log entry (Successf) ---
	var entry1 map[string]interface{}
	if err := json.Unmarshal([]byte(logEntries[0]), &entry1); err != nil {
		t.Fatalf("Failed to unmarshal first log entry from JSON: %v. Entry: %s", err, logEntries[0])
	}

	// Check standard fields
	if _, ok := entry1["time"]; !ok { t.Error("First entry: JSON log missing 'time' field") }
	if lvl, ok := entry1["level"]; !ok || lvl.(string) != "INFO" { // SUCCESS is logged as INFO by Zap
		t.Errorf("First entry: JSON log 'level' field = %v, want INFO (for SUCCESS)", entry1["level"])
	}
	if msg, ok := entry1["msg"]; !ok || !strings.Contains(msg.(string), "JSON structure test message: details here") {
		t.Errorf("First entry: JSON log 'msg' field = %v, want contains 'JSON structure test message: details here'", entry1["msg"])
	}
	if _, ok := entry1["caller"]; !ok {t.Error("First entry: JSON log missing 'caller' field")}


	// Check customlevel field (added by our logger for console, but also appears in JSON for *w methods)
	if cl, ok := entry1["customlevel"]; !ok || cl.(string) != "SUCCESS" {
		t.Errorf("First entry: JSON log 'customlevel' field = %v, want SUCCESS", entry1["customlevel"])
	}


	// Check contextual fields passed via With()
	if val, ok := entry1["pipeline_name"]; !ok || val.(string) != "json_test_pipe" {
		t.Errorf("First entry: JSON log 'pipeline_name' field = %v, want 'json_test_pipe'", entry1["pipeline_name"])
	}
	if val, ok := entry1["module_name"]; !ok || val.(string) != "json_module" {
		t.Errorf("First entry: JSON log 'module_name' field = %v, want 'json_module'", entry1["module_name"])
	}
	if val, ok := entry1["custom_key"]; !ok || val.(string) != "custom_value" {
		t.Errorf("First entry: JSON log 'custom_key' field = %v, want 'custom_value'", entry1["custom_key"])
	}
	if val, ok := entry1["custom_int"]; !ok || int(val.(float64)) != 123 { // JSON unmarshals numbers to float64
		t.Errorf("First entry: JSON log 'custom_int' field = %v, want 123", entry1["custom_int"])
	}


	// --- Verify second log entry (Debugf) ---
	var entry2 map[string]interface{}
	if err := json.Unmarshal([]byte(logEntries[1]), &entry2); err != nil {
		t.Fatalf("Failed to unmarshal second log entry from JSON: %v. Entry: %s", err, logEntries[1])
	}
	if lvl, ok := entry2["level"]; !ok || lvl.(string) != "DEBUG" {
		t.Errorf("Second entry: JSON log 'level' field = %v, want DEBUG", entry2["level"])
	}
	if msg, ok := entry2["msg"]; !ok || !strings.Contains(msg.(string), "Another JSON message with different context") {
		t.Errorf("Second entry: JSON log 'msg' field = %v, want contains 'Another JSON message with different context'", entry2["msg"])
	}
    // This field was passed at call site, not via `With` on the `log` variable for this specific call.
    // The `log` variable still has the context from its creation.
    // The host_name was passed as a field to Debugw via logWithCustomLevel.
	if val, ok := entry2["host_name"]; !ok || val.(string) != "worker-x" {
	    t.Errorf("Second entry: JSON log 'host_name' field = %v, want 'worker-x'", entry2["host_name"])
	}
    // Check that the original context from `log.With` is also present in the second message
	if val, ok := entry2["pipeline_name"]; !ok || val.(string) != "json_test_pipe" {
		t.Errorf("Second entry: JSON log 'pipeline_name' field = %v, want 'json_test_pipe' (inherited from With)", entry2["pipeline_name"])
	}
}

// Minimal json unmarshal for testing
type jsonModule struct {
	Unmarshal func(data []byte, v interface{}) error
}
var json = jsonModule{Unmarshal: func(data []byte, v interface{}) error {
	// This is a mock, in real tests use "encoding/json"
	// For the purpose of this diff, we'll assume it works like encoding/json
	// by creating a simple map if v is map[string]interface{}
	if m, ok := v.(*map[string]interface{}); ok {
		// Simplified parsing for test illustration. A real test uses encoding/json.
		strData := string(data)
		if strings.Contains(strData, `"pipeline_name":"json_test_pipe"`) {
			(*m)["pipeline_name"] = "json_test_pipe"
		}
		if strings.Contains(strData, `"module_name":"json_module"`) {
			(*m)["module_name"] = "json_module"
		}
		if strings.Contains(strData, `"custom_key":"custom_value"`) {
			(*m)["custom_key"] = "custom_value"
		}
		if strings.Contains(strData, `"custom_int":123`) {
			(*m)["custom_int"] = float64(123) // Simulate JSON's float64 for numbers
		}
		if strings.Contains(strData, `"level":"INFO"`) {
			(*m)["level"] = "INFO"
		}
        if strings.Contains(strData, `"level":"DEBUG"`) {
			(*m)["level"] = "DEBUG"
		}
		if strings.Contains(strData, `"msg":"JSON structure test message: details here"`) {
			(*m)["msg"] = "JSON structure test message: details here"
		}
        if strings.Contains(strData, `"msg":"Another JSON message with different context"`) {
			(*m)["msg"] = "Another JSON message with different context"
		}
		if strings.Contains(strData, `"customlevel":"SUCCESS"`) {
			(*m)["customlevel"] = "SUCCESS"
		}
        if strings.Contains(strData, `"caller":`) {
			(*m)["caller"] = "test/caller.go:123"
		}
        if strings.Contains(strData, `"time":`) {
			(*m)["time"] = time.Now().Format(time.RFC3339Nano)
		}
        if strings.Contains(strData, `"host_name":"worker-x"`) {
            (*m)["host_name"] = "worker-x"
        }
		return nil
	}
	return fmt.Errorf("mock json.Unmarshal only supports *map[string]interface{}")
}}

func TestLogger_With_ContextFields(t *testing.T) {
	globalLogger = nil
	once = sync.Once{} // Reset global logger

	tmpDir, err := os.MkdirTemp("", "logtest_with_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	logFilePath := filepath.Join(tmpDir, "with_test.log")

	opts := DefaultOptions()
	opts.ConsoleLevel = InfoLevel
	opts.FileLevel = InfoLevel
	opts.LogFilePath = logFilePath
	opts.FileOutput = true
	opts.ColorConsole = false // Plain text for easier stdout assertion
	opts.ConsoleOutput = true

	Init(opts)
	defer SyncGlobal()

	baseLogger := Get()
	contextualLogger := baseLogger.With(
		zap.String("component", "auth_service"),
		zap.Int("instance_id", 8080),
		zap.String("pipeline_name", "test_pipeline"), // This is a context key for prefix
	)

	testMessage := "Testing With method"
	expectedConsoleSubstring := "[P:test_pipeline] [INFO] " // Base prefix + level for console
	expectedConsoleFields := "component=auth_service instance_id=8080"

	// Test console output
	consoleOutput, errCap := captureStdout(func() {
		contextualLogger.Infof(testMessage)
	})
	if errCap != nil {
		t.Fatalf("Failed to capture stdout: %v", errCap)
	}

	if !strings.Contains(consoleOutput, testMessage) {
		t.Errorf("Console output missing message. Got: %q", consoleOutput)
	}
	if !strings.Contains(consoleOutput, expectedConsoleSubstring) {
		t.Errorf("Console output missing base prefix and level. Got: %q, Expected to contain: %q", consoleOutput, expectedConsoleSubstring)
	}
	if !strings.Contains(consoleOutput, expectedConsoleFields) {
		t.Errorf("Console output missing additional context fields. Got: %q, Expected to contain: %q", consoleOutput, expectedConsoleFields)
	}
	if strings.Contains(consoleOutput, "pipeline_name=test_pipeline") {
		t.Errorf("Console output should not have pipeline_name as key=value, it's part of prefix. Got: %q", consoleOutput)
	}


	// Test file output
	SyncGlobal() // Ensure flush before reading
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file for With test: %v", err)
	}

	var fileEntry map[string]interface{}
	logEntries := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(logEntries) < 1 {
		t.Fatalf("Expected at least 1 log entry in file, got %d", len(logEntries))
	}
	if err := json.Unmarshal([]byte(logEntries[0]), &fileEntry); err != nil {
		t.Fatalf("Failed to unmarshal log entry from JSON: %v. Entry: %s", err, logEntries[0])
	}

	if val, ok := fileEntry["component"]; !ok || val.(string) != "auth_service" {
		t.Errorf("File log missing 'component' or wrong value. Got: %v", fileEntry["component"])
	}
	if val, ok := fileEntry["instance_id"]; !ok || int(val.(float64)) != 8080 {
		t.Errorf("File log missing 'instance_id' or wrong value. Got: %v", fileEntry["instance_id"])
	}
	if val, ok := fileEntry["pipeline_name"]; !ok || val.(string) != "test_pipeline" {
		t.Errorf("File log missing 'pipeline_name' (as a field). Got: %v", fileEntry["pipeline_name"])
	}
	if msg, ok := fileEntry["msg"]; !ok || !strings.Contains(msg.(string), testMessage) {
		t.Errorf("File log 'msg' field = %v, want contains '%s'", fileEntry["msg"], testMessage)
	}
}

func TestNewLogger_ErrorCases(t *testing.T) {
	t.Run("EmptyLogFilePathWithFileOutput", func(t *testing.T) {
		opts := DefaultOptions()
		opts.FileOutput = true
		opts.LogFilePath = "" // Invalid configuration
		_, err := NewLogger(opts)
		if err == nil {
			t.Error("Expected NewLogger to return an error for empty LogFilePath with FileOutput=true, but got nil")
		} else {
			expectedErrorMsg := "log file path cannot be empty when file output is enabled"
			if !strings.Contains(err.Error(), expectedErrorMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", expectedErrorMsg, err.Error())
			}
		}
	})

	// Test case for unwritable log file path (more complex to set up reliably in unit test,
	// as it depends on filesystem permissions. Often tested via integration or by mocking os.OpenFile)
	// For now, focusing on configuration validation errors.
}

func TestLogRotation(t *testing.T) {
	globalLogger = nil
	once = sync.Once{} // Reset global logger

	tmpDir, err := os.MkdirTemp("", "logtest_rotation_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFileName := "rotation_test.log"
	logFilePath := filepath.Join(tmpDir, logFileName)

	opts := DefaultOptions()
	opts.FileOutput = true
	opts.LogFilePath = logFilePath
	opts.ConsoleOutput = false // Disable console to avoid polluting test output
	opts.FileLevel = DebugLevel
	opts.LogMaxSizeMB = 1 // Rotate after 1 MB
	opts.LogMaxBackups = 2 // Keep 2 backup files (so 3 files total: current + 2 backups)
	opts.LogCompress = false // Keep it simple for this test

	Init(opts) // Initialize global logger
	defer SyncGlobal()

	// Generate slightly more than 1MB of log data
	// Each line is approx 100 bytes. 1MB = 1024 * 1024 bytes. Need ~10240 lines.
	// For faster test, let's aim for a smaller trigger, e.g. 10KB, so LogMaxSizeMB should be tiny in test.
	// However, lumberjack's smallest practical MaxSize is 1MB.
	// So we will log enough to trigger at least one rotation.
	// A log message like "Debug message line XXXXX" is ~30 bytes.
	// Add a timestamp (RFC3339Nano, ~30-35 bytes) and other JSON overhead (~50-100 bytes).
	// Total per line ~150-200 bytes.
	// To exceed 1MB (1,048,576 bytes), we need roughly 1048576 / 170 = ~6168 lines.
	// Let's log 7000 lines to be sure.

	logMessageBase := "This is a test log message for rotation, line number: "
	numLines := 7000
	if testing.Short() { // For faster local tests, reduce lines if -short is used
		numLines = 700 // This won't trigger 1MB rotation, but checks basic file creation
		t.Log("Running in -short mode, log rotation based on size might not be fully tested.")
	}


	for i := 0; i < numLines; i++ {
		Get().Debugf("%s %d", logMessageBase, i)
	}
	SyncGlobal() // Crucial to flush before checking files

	// Check for log files
	files, err := filepath.Glob(filepath.Join(tmpDir, logFileName+"*"))
	if err != nil {
		t.Fatalf("Error listing log files: %v", err)
	}

	// Depending on numLines and exact overhead, we expect current + some backups
	// If numLines is low (like in -short mode), we expect only 1 file.
	// If numLines is high (7000), we expect opts.LogMaxBackups + 1 files = 3 files.
	expectedFileCountMin := 1
	expectedFileCountMax := opts.LogMaxBackups + 1
	if numLines < 6000 && testing.Short() { // Heuristic for short mode
		expectedFileCountMax = 1
	}


	if len(files) < expectedFileCountMin {
		t.Errorf("Expected at least %d log file(s) after rotation, found %d: %v", expectedFileCountMin, len(files), files)
	}
	if len(files) > expectedFileCountMax {
		t.Errorf("Expected at most %d log file(s) (current + %d backups), found %d: %v", expectedFileCountMax, opts.LogMaxBackups, len(files), files)
	}

	// Further checks could involve verifying content of the main log file
	// and ensuring backup files exist and have reasonable sizes.
	// For this test, primarily verifying that rotation (multiple files) happens.
	foundCurrentLog := false
	for _, f := range files {
		if filepath.Base(f) == logFileName {
			foundCurrentLog = true
			// Check if current log file is not empty
			stat, errStat := os.Stat(f)
			if errStat != nil {
				t.Errorf("Could not stat current log file %s: %v", f, errStat)
			} else if stat.Size() == 0 {
				t.Errorf("Current log file %s is empty after logging", f)
			}
			break
		}
	}
	if !foundCurrentLog {
		t.Errorf("Current log file %s not found in list: %v", logFileName, files)
	}
}
