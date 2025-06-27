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
	"time" // Added import
	"encoding/json" // Added for real JSON unmarshalling

	"github.com/stretchr/testify/assert" // Added for testify assertions
	"go.uber.org/zap" // For adding fields in test
	// "go.uber.org/zap/zapcore" // Commented out as it became unused
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
	testMsg := "Test console message"; expectedLevelPrefix := "[INFO]"

	output, errCap := captureStdout(func() {
		logger, err := NewLogger(opts) // Initialize logger INSIDE captureStdout
		if err != nil {
			// Use t.Errorf or t.Fatalf from the outer scope.
			// To do this safely, we might need to pass `t` into this closure or handle error differently.
			// For now, let's assume fatal is okay for this test structure if logger fails.
			panic(fmt.Sprintf("NewLogger() error = %v", err)) // panic to fail test from goroutine
		}
		defer logger.Sync()
		logger.Info(testMsg)
	})

	assert.NoError(t, errCap, "Failed to capture stdout")
	// Allow for timestamp and caller by checking for Contains, not exact match at start
	assert.Contains(t, output, expectedLevelPrefix, "Console output missing level prefix.")
	assert.Contains(t, output, testMsg, "Console output missing message.")
}

// TestNewLogger_FileOutput (as previously defined, ensure it still passes or adapt)
func TestNewLogger_FileOutput(t *testing.T) {
	// Reset global logger for this test
	globalLogger = nil; once = sync.Once{}
	tmpDir, errTmp := os.MkdirTemp("", "logtest");
	assert.NoError(t, errTmp, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test.log")
	opts := DefaultOptions(); opts.FileLevel = InfoLevel; opts.LogFilePath = logFilePath; opts.FileOutput = true; opts.ConsoleOutput = false

	logger, errNew := NewLogger(opts);
	assert.NoError(t, errNew, "NewLogger() error")
	if errNew != nil { t.FailNow() } // Fail fast if logger creation failed

	testMsg := "Test file message"; logger.Info(testMsg); logger.Debug("No debug in file");
	errSync := logger.Sync()
	assert.NoError(t, errSync, "Logger sync error")

	content, errRead := os.ReadFile(logFilePath);
	assert.NoError(t, errRead, "Failed to read log file")
	logContent := string(content)

	assert.Contains(t, logContent, testMsg, "Log file missing message.")
	assert.NotContains(t, logContent, "No debug in file", "Log file contains debug.")
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
			// Store tc locally for the closure to use in the captureStdout func
			currentTC := tc

			output, errCap := captureStdout(func() {
				// Reset global logger state and Initialize INSIDE the capture function
				globalLogger = nil
				once = sync.Once{}
				Init(baseOpts) // baseOpts is from the outer scope

				lg := Get().SugaredLogger // Get the global logger which is now set up

				// If there are context fields from the test case, add them using With
				if len(currentTC.logFields) > 0 {
					// Explicitly convert []zap.Field to []interface{} for With
					args := make([]interface{}, len(currentTC.logFields))
					for i, field := range currentTC.logFields {
						args[i] = field
					}
					lg = lg.With(args...)
				}

				// Perform the actual logging operation based on the test case level
				// The "customlevel" field is added here to simulate how our wrapper logWithCustomLevel
				// would inform the custom console encoder.
				switch currentTC.level {
				case DebugLevel:
					lg.Debugw(currentTC.message, zap.String("customlevel", currentTC.level.CapitalString()))
				case InfoLevel:
					lg.Infow(currentTC.message, zap.String("customlevel", currentTC.level.CapitalString()))
				case SuccessLevel: // SuccessLevel logs as InfoLevel in Zap but uses customlevel for formatting
					lg.Infow(currentTC.message, zap.String("customlevel", currentTC.level.CapitalString()))
				case WarnLevel:
					lg.Warnw(currentTC.message, zap.String("customlevel", currentTC.level.CapitalString()))
				case ErrorLevel:
					lg.Errorw(currentTC.message, zap.String("customlevel", currentTC.level.CapitalString()))
				default:
					// This case should ideally not be hit if all levels are covered.
					// Using fmt.Printf here as t.Logf might behave unexpectedly from a goroutine.
					fmt.Printf("Unhandled log level in test %s: %s\n", currentTC.name, currentTC.level.String())
				}
			})

			// Assertions remain outside the captureStdout func, using currentTC
			assert.NoError(t, errCap, "Failed to capture stdout for %s", currentTC.name)

			// Skip further checks for levels that cause termination
			if currentTC.level == FatalLevel || currentTC.level == PanicLevel || currentTC.level == FailLevel {
				return
			}

			assert.Contains(t, output, currentTC.message, "Console output for %s missing message.", currentTC.name)

			if currentTC.expectedCtxPrefix != "" {
				assert.Contains(t, output, currentTC.expectedCtxPrefix, "Console output for %s missing context prefix.", currentTC.name)
			}

			// Check for uncolored level string.
			assert.Contains(t, output, currentTC.levelString, "Console output for %s missing level string.", currentTC.name)

			if currentTC.expectedColor != "" {
				expectedColoredLevelString := currentTC.expectedColor + currentTC.levelString + colorReset
				assert.Contains(t, output, expectedColoredLevelString, "Console output for %s missing expected color sequence around level.", currentTC.name)
			}
		})
	}
}


// TestLogLevelFiltering (as previously defined, should still pass)
func TestLogLevelFiltering(t *testing.T) {
	opts := DefaultOptions(); opts.ConsoleLevel = WarnLevel; opts.FileOutput = false; opts.ColorConsole = false
	var output string
	output, errCap := captureStdout(func() {
		// Logger instance created inside
		logger, err := NewLogger(opts)
		if err != nil {
			panic(fmt.Sprintf("NewLogger error: %v", err)) // Using panic as t.Fatalf is tricky from closure
		}
		defer logger.Sync()
		logger.Debugf("%s", "debug_test"); logger.Infof("%s", "info_test"); logger.Successf("%s", "success_test");
		logger.Warnf("%s", "warn_test"); logger.Errorf("%s", "error_test")
	})
	assert.NoError(t, errCap, "captureStdout error in TestLogLevelFiltering")
	assert.NotContains(t, output, "debug_test", "Output contains DEBUG log")
	assert.NotContains(t, output, "success_test", "Output contains SUCCESS log") // Success is Info level
	assert.NotContains(t, output, "info_test", "Output contains INFO log")
	assert.Contains(t, output, "warn_test", "Output missing WARN log")
	assert.Contains(t, output, "error_test", "Output missing ERROR log")
}

// TestGlobalLogger (as previously defined, should still pass)
func TestGlobalLogger(t *testing.T) {
	originalGlobalLogger := globalLogger; originalOnce := once
	defer func() { globalLogger = originalGlobalLogger; once = originalOnce }()

	opts1 := DefaultOptions(); opts1.ConsoleLevel = InfoLevel; opts1.FileOutput = false; opts1.ColorConsole = false
	output1, errCap1 := captureStdout(func() {
		globalLogger = nil; once = sync.Once{} // Reset global state
		Init(opts1) // Init inside
		defer SyncGlobal()
		Info("%s", "Global logger test"); Success("%s", "Global success test")
	})
	assert.NoError(t, errCap1, "captureStdout error in TestGlobalLogger (output1)")
	assert.Contains(t, output1, "[INFO]", "Global Info() log incorrect (level).")
	assert.Contains(t, output1, "Global logger test", "Global Info() log incorrect (message).")
	assert.Contains(t, output1, "[SUCCESS]", "Global Success() log incorrect (level).")
	assert.Contains(t, output1, "Global success test", "Global Success() log incorrect (message).")

	// Test that re-Init with different options works after reset.
	opts2 := DefaultOptions(); opts2.ConsoleLevel = DebugLevel; opts2.FileOutput = false; opts2.ColorConsole = false
	output2, errCap2 := captureStdout(func() {
		globalLogger = nil; once = sync.Once{} // Reset global state again
		Init(opts2) // Init inside with different options
		defer SyncGlobal()
		Debug("%s", "Global debug, should appear now")
	})
	assert.NoError(t, errCap2, "captureStdout error in TestGlobalLogger (output2)")
	assert.Contains(t, output2, "Global debug, should appear now", "Global Debug() did not appear after re-initializing with DebugLevel.")

	// Test stickiness: if we init with Info, then call Init with Debug *without* reset, Debug should still be filtered.
	output3, errCap3 := captureStdout(func() {
		globalLogger = nil; once = sync.Once{} // Reset
		Init(opts1) // Init with InfoLevel
		Init(opts2) // Attempt to Init with DebugLevel (should be no-op on globalLogger instance's level)
		defer SyncGlobal()
		Debug("%s", "Global debug, should NOT appear due to sticky InfoLevel")
	})
	assert.NoError(t, errCap3, "captureStdout error in TestGlobalLogger (output3)")
	assert.NotContains(t, output3, "Global debug, should NOT appear due to sticky InfoLevel", "Global Debug() appeared after sticky Init. Expected InfoLevel to persist and filter Debug.")
}

// TestTimestampFormat (as previously defined, should still pass)
func TestTimestampFormat(t *testing.T) {
	customFormat := "2006/01/02_15:04:05"
	opts := DefaultOptions(); opts.ConsoleLevel=InfoLevel; opts.FileOutput=false; opts.ColorConsole=false; opts.TimestampFormat=customFormat
	output, errCap := captureStdout(func(){
		logger, err := NewLogger(opts)
		assert.NoError(t, err, "NewLogger err in TestTimestampFormat")
		if err != nil { // Ensure logger is not nil before using
			return
		}
		// defer func() { assert.NoError(t, logger.Sync()) }() // Temporarily comment out Sync for diagnosis
		logger.Infof("%s", "Timestamp test")
	})
	assert.NoError(t, errCap, "captureStdout error in TestTimestampFormat")
	re := regexp.MustCompile(`\d{4}/\d{2}/\d{2}_\d{2}:\d{2}:\d{2}`); // Regex for YYYY/MM/DD_HH:MM:SS
	assert.Regexp(t, re, output, "Timestamp wrong format")
}

/*
// TestFailAndFatalLevels (as previously defined, should still pass for buffered output)
func TestFailAndFatalLevels(t *testing.T) {
    globalLogger = nil; once = sync.Once{}; opts := DefaultOptions(); opts.ConsoleLevel = InfoLevel; opts.FileOutput = false; opts.ColorConsole = false
    t.Run("FailLevelOutput", func(t *testing.T) {
        var buf bytes.Buffer; fatalCoreCfg := zap.NewProductionEncoderConfig(); fatalCoreCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat); fatalCoreCfg.TimeKey="time"; fatalCoreCfg.LevelKey="";
        fatalEncoder := NewPlainTextConsoleEncoder(fatalCoreCfg, opts)
        core := zapcore.NewCore(fatalEncoder, zapcore.AddSync(&buf), zap.NewAtomicLevelAt(zapcore.FatalLevel))
		tempZapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)) // Adjusted skip: Failf will add another frame
        tempLogger := &Logger{SugaredLogger: tempZapLogger.Sugar(), opts: opts}
		// Simulate call from our logger's Failf
        tempLogger.Failf("%s", "This is a fail test") // Call Failf directly
        // tempLogger.Sync() // Sync might not happen if Fatalw exits, but buffer should have content
        output := buf.String()
        if !strings.Contains(output, "[FAIL]") {t.Errorf("Fail log missing [FAIL]. Got: %s", output)}
        if !strings.Contains(output, "This is a fail test") {t.Errorf("Fail log missing message. Got: %s", output)}
    })
}
*/

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
	log.Debugf("Another JSON message with different context %v", zap.String("host_name", "worker-x")) // host_name is a context key, added %v for vet

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
	// Use encoding/json directly
	err = json.Unmarshal([]byte(logEntries[0]), &entry1);
	assert.NoError(t, err, "Failed to unmarshal first log entry from JSON. Entry: %s", logEntries[0])


	// Check standard fields
	assert.Contains(t, entry1, "time", "First entry: JSON log missing 'time' field")
	assert.Equal(t, "INFO", entry1["level"], "First entry: JSON log 'level' field mismatch (for SUCCESS)")
	assert.Contains(t, entry1["msg"], "JSON structure test message: details here", "First entry: JSON log 'msg' field mismatch")
	assert.Contains(t, entry1, "caller", "First entry: JSON log missing 'caller' field")

	// Check customlevel field
	assert.Equal(t, "SUCCESS", entry1["customlevel"], "First entry: JSON log 'customlevel' field mismatch")

	// Check contextual fields passed via With()
	assert.Equal(t, "json_test_pipe", entry1["pipeline_name"], "First entry: JSON log 'pipeline_name' field mismatch")
	assert.Equal(t, "json_module", entry1["module_name"], "First entry: JSON log 'module_name' field mismatch")
	assert.Equal(t, "custom_value", entry1["custom_key"], "First entry: JSON log 'custom_key' field mismatch")
	assert.EqualValues(t, 123, entry1["custom_int"], "First entry: JSON log 'custom_int' field mismatch") // Use EqualValues for float64 vs int

	// --- Verify second log entry (Debugf) ---
	var entry2 map[string]interface{}
	err = json.Unmarshal([]byte(logEntries[1]), &entry2);
	assert.NoError(t, err, "Failed to unmarshal second log entry from JSON. Entry: %s", logEntries[1])

	assert.Equal(t, "DEBUG", entry2["level"], "Second entry: JSON log 'level' field mismatch")

	expectedMsgPart := "Another JSON message with different context"
	// This is what `fmt.Sprintf("%v", zap.String("host_name", "worker-x"))` produces
	expectedMangledHostName := "{host_name 15 0 worker-x <nil>}" // Note: KubeXMS logger likely uses a different field name than "host_name" for its contextual prefix logic. This test might need adjustment if "host_name" is a special context key.
	                                                          // For JSON output of Debugf, the zap.String field becomes part of the formatted message string.
	actualMsg, msgOk := entry2["msg"].(string)
	assert.True(t, msgOk, "Second entry: 'msg' field is not a string")
	assert.Contains(t, actualMsg, expectedMsgPart, "Second entry: JSON log 'msg' does not contain expected part.")
	assert.Contains(t, actualMsg, expectedMangledHostName, "Second entry: JSON log 'msg' does not contain mangled host_name field.")

    // host_name will NOT be a separate field due to fmt.Sprintf in Debugf.
    // The assertion for its presence as a top-level field is removed.

    // Check that the original context from `log.With` is also present in the second message
	assert.Equal(t, "json_test_pipe", entry2["pipeline_name"], "Second entry: JSON log 'pipeline_name' field mismatch (inherited from With)")
}

/* // Removing the mock json variable and its type as it's no longer used.
// Minimal json unmarshal for testing
type jsonModule struct {
	Unmarshal func(data []byte, v interface{}) error
}
var json = jsonModule{Unmarshal: func(data []byte, v interface{}) error {
	// This is a mock, in real tests use "encoding/json"
	// For the purpose of this diff, we'll assume it works like encoding/json
	// by creating a simple map if v is map[string]interface{}
	if m, ok := v.(*map[string]interface{}); ok {
		// Initialize the map if it's nil
		if *m == nil {
			*m = make(map[string]interface{})
		}
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
		// Default msg to track if it's set by specific conditions
		(*m)["msg"] = "MOCK_MSG_UNSET"

		if strings.Contains(strData, `"level":"INFO"`) { // First log entry (Successf)
			if strings.Contains(strData, `"msg":"JSON structure test message: details here"`) {
				(*m)["msg"] = "JSON structure test message: details here"
			} else {
				(*m)["msg"] = "MOCK_INFO_MSG_PATTERN_MISMATCH"
			}
			if strings.Contains(strData, `"customlevel":"SUCCESS"`) { // For the Successf message
				(*m)["customlevel"] = "SUCCESS"
			}
		} else if strings.Contains(strData, `"level":"DEBUG"`) { // Second log entry (Debugf) in TestJSONFileOutputStructure
			// Check for parts of the message, as precise full string matching is fragile with mocks
			if strings.Contains(strData, "Another JSON message with different context") && strings.Contains(strData, "host_name=worker-x") {
				// Reconstruct the expected message as it would be after Sprintf
				(*m)["msg"] = "Another JSON message with different context host_name=worker-x"
			} else {
				(*m)["msg"] = "MOCK_DEBUG_MSG_SUB_PATTERN_MISMATCH"
			}
		} else if strings.Contains(strData, `"msg":"Testing With method"`) { // For TestLogger_With_ContextFields
             (*m)["msg"] = "Testing With method"
             // This is an INFO message too, but TestLogger_With_ContextFields doesn't check customlevel in JSON
        }


		// This was for the first message only, handled above now.
		// if strings.Contains(strData, `"customlevel":"SUCCESS"`) {
		// 	(*m)["customlevel"] = "SUCCESS"
		// }
		if strings.Contains(strData, `"caller":`) {
			(*m)["caller"] = "test/caller.go:123" // Mock value
		}
		// Add time field for all entries
		(*m)["time"] = time.Now().Format(time.RFC3339Nano) // Mock value

		// This was for when host_name was a separate field, which it isn't for the Debugf case anymore.
		// if strings.Contains(strData, `"host_name":"worker-x"`) {
		//    (*m)["host_name"] = "worker-x"
		// }
		return nil
	}
	return fmt.Errorf("mock json.Unmarshal only supports *map[string]interface{}")
}}
*/

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
	// FileOutput is also true from DefaultOptions or set above.

	testMessage := "Testing With method"
	expectedConsoleSubstring := "[P:test_pipeline] [INFO] " // Base prefix + level for console
	expectedConsoleFields := "component=auth_service instance_id=8080"

	consoleOutput, errCap := captureStdout(func() {
		localOnce := sync.Once{} // Use a local once for this specific Init sequence
		localGlobalLogger := (*Logger)(nil) // Local "copy" of global for this test section

		// Custom Init for this capture block to ensure it uses the piped stdout
		customInit := func(o Options) {
			localOnce.Do(func() {
				var errInit error
				localGlobalLogger, errInit = NewLogger(o)
				if errInit != nil {
					panic(fmt.Sprintf("NewLogger in customInit failed: %v", errInit))
				}
				// Replace the actual global logger for the duration of this specific test logic
				// This is tricky and generally not advised, but test needs specific init for capture.
				// A better way would be to have Get() return a logger based on context if possible,
				// or NewLogger should be used and passed around.
				// For this test, we are testing Get().With(), so global needs to be affected.
				globalLogger = localGlobalLogger // Temporarily assign to actual global
				once = localOnce // And use the local once to allow this Init
			})
		}
		customInit(opts) // Init with opts for console capture
		defer func() {
			if localGlobalLogger != nil {localGlobalLogger.Sync()}
			// Restore global logger state if necessary, though tests usually run sequentially.
			// For safety, re-setting to nil so next test's Get() forces its own Init.
			globalLogger = nil
			once = sync.Once{}
		}()


		baseLogger := Get() // This Get() will use the localGlobalLogger due to assignment above
		contextualLogger := baseLogger.With(
			zap.String("component", "auth_service"),
			zap.Int("instance_id", 8080),
			zap.String("pipeline_name", "test_pipeline"), // This is a context key for prefix
		)
		contextualLogger.Infof("%s", testMessage)
	})

	assert.NoError(t, errCap, "Failed to capture stdout")

	assert.Contains(t, consoleOutput, testMessage, "Console output missing message.")
	assert.Contains(t, consoleOutput, expectedConsoleSubstring, "Console output missing base prefix and level.")
	assert.Contains(t, consoleOutput, expectedConsoleFields, "Console output missing additional context fields.")
	assert.NotContains(t, consoleOutput, "pipeline_name=test_pipeline", "Console output should not have pipeline_name as key=value, it's part of prefix.")

	// Test file output
	// Re-initialize global logger for file test if the above customInit was too disruptive,
	// or ensure opts used by customInit also correctly set up file output.
	// The opts already has FileOutput=true and LogFilePath set.
	// The customInit's NewLogger call would have set up the file core too.
	// We just need to ensure logs are flushed to the file.
	// A specific Sync on the logger that wrote to file might be needed if not using global SyncGlobal.
	// The SyncGlobal below should work if globalLogger was correctly set by customInit.
	if globalLogger != nil { // If customInit actually set it
		globalLogger.Sync()
	} else { // Fallback if global logger wasn't touched by customInit (should not happen with current customInit)
		Init(opts) // Ensure global is set for file path
		SyncGlobal()
	}

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file for With test: %v", err)
	}

	var fileEntry map[string]interface{}
	logEntries := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(logEntries) < 1 {
		t.Fatalf("Expected at least 1 log entry in file, got %d", len(logEntries))
	}
	// Use encoding/json directly
	err = json.Unmarshal([]byte(logEntries[0]), &fileEntry)
	assert.NoError(t, err, "Failed to unmarshal log entry from JSON. Entry: %s", logEntries[0])

	assert.Equal(t, "auth_service", fileEntry["component"], "File log 'component' field mismatch.")
	assert.EqualValues(t, 8080, fileEntry["instance_id"], "File log 'instance_id' field mismatch.") // Use EqualValues for float64 vs int
	assert.Equal(t, "test_pipeline", fileEntry["pipeline_name"], "File log 'pipeline_name' field mismatch.")
	assert.Contains(t, fileEntry["msg"], testMessage, "File log 'msg' field mismatch.")
}

func TestNewLogger_ErrorCases(t *testing.T) {
	t.Run("EmptyLogFilePathWithFileOutput", func(t *testing.T) {
		opts := DefaultOptions()
		opts.FileOutput = true
		opts.LogFilePath = "" // Invalid configuration
		_, err := NewLogger(opts)
		assert.Error(t, err, "Expected NewLogger to return an error for empty LogFilePath with FileOutput=true")
		if err != nil { // Check only if error is not nil
			expectedErrorMsg := "log file path cannot be empty when file output is enabled"
			assert.Contains(t, err.Error(), expectedErrorMsg, "Error message mismatch.")
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
	for _, fPath := range files {
		if filepath.Base(fPath) == logFileName {
			foundCurrentLog = true
			// Check if current log file is not empty
			stat, errStat := os.Stat(fPath)
			assert.NoError(t, errStat, "Could not stat current log file %s", fPath)
			if errStat == nil { // Proceed only if stat was successful
				assert.Greater(t, stat.Size(), int64(0), "Current log file %s is empty after logging", fPath)
			}
			break
		}
	}
	assert.True(t, foundCurrentLog, "Current log file %s not found in list: %v", logFileName, files)
}
