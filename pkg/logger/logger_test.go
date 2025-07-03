package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

	outC := make(chan string, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		r.Close()
		outC <- buf.String()
	}()

	f()

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

func TestNewLogger_ConsoleOutput(t *testing.T) {
	globalLogger = nil; once = sync.Once{}
	opts := DefaultOptions(); opts.ConsoleLevel = DebugLevel; opts.FileOutput = false; opts.ColorConsole = false
	testMsg := "Test console message"

	// var consoleBuf bytes.Buffer // This was for a previous custom sink approach
	testOpts := opts
	testOpts.ConsoleOutput = true

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger, err := NewLogger(testOpts)
	assert.NoError(t, err, "NewLogger error")
	if err != nil { t.FailNow() }

	logger.Infof("%s", testMsg)
	logger.Sync()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()

	output := buf.String()
	assert.Contains(t, output, testMsg, "Console output missing message.")
	// More detailed check for actual content, level, etc. could be added here
	// For instance, check if [INFO] is present, and the correct caller.
	// Example: 2023-10-27T10:00:00Z [INFO] logger/logger_test.go:XXX: Test console message
	// The customlevel=INFO field should not appear as a regular field because it's used by the console encoder
	// to determine the level string. Other fields (not used by encoder for special purposes) would appear.
	assert.Contains(t, output, "[INFO]", "Console output missing [INFO] level marker")
	assert.NotContains(t, output, "customlevel=INFO", "Console output should not contain customlevel=INFO as a trailing field")
	assert.NotContains(t, output, "customlevel_num=0", "Console output should not contain customlevel_num=0 as a trailing field")


}


func TestNewLogger_FileOutput(t *testing.T) {
	globalLogger = nil; once = sync.Once{}
	tmpDir, errTmp := os.MkdirTemp("", "logtest");
	assert.NoError(t, errTmp, "Failed to create temp dir")
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test.log")
	opts := DefaultOptions(); opts.FileLevel = InfoLevel; opts.LogFilePath = logFilePath; opts.FileOutput = true; opts.ConsoleOutput = false

	logger, errNew := NewLogger(opts);
	assert.NoError(t, errNew, "NewLogger() error")
	if errNew != nil { t.FailNow() }

	logger.Infof("%s", "Test file message"); logger.Debugf("%s", "No debug in file");
	errSync := logger.Sync()
	assert.NoError(t, errSync, "Logger sync error")

	content, errRead := os.ReadFile(logFilePath);
	assert.NoError(t, errRead, "Failed to read log file")
	logContent := string(content)

	assert.Contains(t, logContent, "Test file message", "Log file missing message.")
	assert.NotContains(t, logContent, "No debug in file", "Log file contains debug.")
}


func TestLogLevelFiltering(t *testing.T) {
	opts := DefaultOptions(); opts.ConsoleLevel = WarnLevel; opts.FileOutput = false; opts.ColorConsole = false

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger, err := NewLogger(opts)
	assert.NoError(t, err)
	if err != nil { t.FailNow() }

	logger.Debugf("%s", "debug_test"); logger.Infof("%s", "info_test"); logger.Successf("%s", "success_test");
	logger.Warnf("%s", "warn_test"); logger.Errorf("%s", "error_test")
	logger.Sync()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer // Corrected variable name
	io.Copy(&buf, r)
	r.Close()

	output := buf.String()
	assert.NotContains(t, output, "debug_test", "Output contains DEBUG log")
	assert.NotContains(t, output, "success_test", "Output contains SUCCESS log")
	assert.NotContains(t, output, "info_test", "Output contains INFO log")
	assert.Contains(t, output, "warn_test", "Output missing WARN log")
	assert.Contains(t, output, "error_test", "Output missing ERROR log")
}

func TestGlobalLogger(t *testing.T) {
	originalGlobalLogger := globalLogger; originalOnce := once
	defer func() { globalLogger = originalGlobalLogger; once = originalOnce }()

	opts1 := DefaultOptions(); opts1.ConsoleLevel = InfoLevel; opts1.FileOutput = false; opts1.ColorConsole = true

	oldStdout := os.Stdout
	r, wPipe, _ := os.Pipe()
	os.Stdout = wPipe

	globalLogger = nil; once = sync.Once{}
	Init(opts1)

	Info("Global logger test"); Success("Global success test")
	SyncGlobal()

	wPipe.Close()
	os.Stdout = oldStdout
	var buf1 bytes.Buffer
	io.Copy(&buf1, r)
	r.Close()
	output1 := buf1.String()

	assert.Contains(t, output1, "[INFO]", "Global Info() log incorrect (level).")
	assert.Contains(t, output1, "Global logger test", "Global Info() log incorrect (message).")
	assert.Contains(t, output1, colorGreen+"[SUCCESS]"+colorReset, "Global Success() log incorrect (level/color).")
	assert.Contains(t, output1, "Global success test", "Global Success() log incorrect (message).")

	globalLogger = nil; once = sync.Once{}
	opts2 := DefaultOptions(); opts2.ConsoleLevel = DebugLevel; opts2.FileOutput = false; opts2.ColorConsole = false

	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	Init(opts2)

	Debug("Global debug, should appear now")
	SyncGlobal()

	w2.Close()
	os.Stdout = oldStdout
	var buf2 bytes.Buffer
	io.Copy(&buf2, r2)
	r2.Close()
	output2 := buf2.String()
	assert.Contains(t, output2, "Global debug, should appear now", "Global Debug() did not appear after re-initializing with DebugLevel.")
}

func TestTimestampFormat(t *testing.T) {
	customFormat := "2006/01/02_15:04:05"
	opts := DefaultOptions(); opts.ConsoleLevel=InfoLevel; opts.FileOutput=false; opts.ColorConsole=false; opts.TimestampFormat=customFormat

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger, err := NewLogger(opts)
	assert.NoError(t, err, "NewLogger err in TestTimestampFormat")
	if err != nil { return }

	logger.Infof("%s", "Timestamp test")
	logger.Sync()

	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	output := buf.String()

	re := regexp.MustCompile(`\d{4}/\d{2}/\d{2}_\d{2}:\d{2}:\d{2}`);
	assert.Regexp(t, re, output, "Timestamp wrong format")
}

func TestJSONFileOutputStructure(t *testing.T) {
	globalLogger = nil
	once = sync.Once{}

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
	opts.ConsoleOutput = false
	opts.TimestampFormat = time.RFC3339Nano

	logger, err := NewLogger(opts)
	assert.NoError(t, err, "Failed to create logger for JSON test")
	if err != nil { t.FailNow() }

	logWithCtx := logger.With(
		"pipeline_name", "json_test_pipe",
		"module_name", "json_module",
		"custom_key", "custom_value",
		"custom_int", 123,
	)
	logWithCtx.Successf("%s", "JSON structure test message: details here")
	logger.Debugf("%s", "Plain debug message to file")


	err = logger.Sync()
	assert.NoError(t, err, "Error syncing logger for JSON test")

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logEntriesStr := strings.TrimSpace(string(content))
	if logEntriesStr == "" {
		t.Fatalf("Log file is empty")
	}
	logEntries := strings.Split(logEntriesStr, "\n")

	if len(logEntries) < 2 {
		t.Fatalf("Expected at least 2 log entries, got %d. Content: %s", len(logEntries), string(content))
	}

	var entry1 map[string]interface{}
	err = json.Unmarshal([]byte(logEntries[0]), &entry1);
	assert.NoError(t, err, "Failed to unmarshal first log entry from JSON. Entry: %s", logEntries[0])

	assert.Contains(t, entry1, "time", "First entry: JSON log missing 'time' field")
	assert.Equal(t, "INFO", entry1["level"], "First entry: JSON log 'level' field mismatch (for SUCCESS)")
	assert.Contains(t, entry1["msg"], "JSON structure test message: details here", "First entry: JSON log 'msg' field mismatch")
	assert.Contains(t, entry1, "caller", "First entry: JSON log missing 'caller' field")
	assert.Equal(t, "SUCCESS", entry1["customlevel"], "First entry: JSON log 'customlevel' field mismatch")
	assert.EqualValues(t, float64(SuccessLevel), entry1["customlevel_num"], "First entry: JSON log 'customlevel_num' field mismatch")

	assert.Equal(t, "json_test_pipe", entry1["pipeline_name"], "First entry: JSON log 'pipeline_name' field mismatch")
	assert.Equal(t, "json_module", entry1["module_name"], "First entry: JSON log 'module_name' field mismatch")
	assert.Equal(t, "custom_value", entry1["custom_key"], "First entry: JSON log 'custom_key' field mismatch")
	assert.EqualValues(t, float64(123), entry1["custom_int"], "First entry: JSON log 'custom_int' field mismatch")


	var entry2 map[string]interface{}
	err = json.Unmarshal([]byte(logEntries[1]), &entry2);
	assert.NoError(t, err, "Failed to unmarshal second log entry from JSON. Entry: %s", logEntries[1])

	assert.Equal(t, "DEBUG", entry2["level"], "Second entry: JSON log 'level' field mismatch")
	assert.Equal(t, "Plain debug message to file", entry2["msg"], "Second entry: JSON log 'msg' field mismatch")
	assert.Equal(t, "DEBUG", entry2["customlevel"], "Second entry: JSON log 'customlevel' field mismatch")
	assert.EqualValues(t, float64(DebugLevel), entry2["customlevel_num"], "Second entry: JSON log 'customlevel_num' field mismatch")
}


func TestNewLogger_ErrorCases(t *testing.T) {
	t.Run("EmptyLogFilePathWithFileOutput", func(t *testing.T) {
		opts := DefaultOptions()
		opts.FileOutput = true
		opts.LogFilePath = ""
		_, err := NewLogger(opts)
		assert.Error(t, err, "Expected NewLogger to return an error for empty LogFilePath with FileOutput=true")
		if err != nil {
			expectedErrorMsg := "log file path cannot be empty when file output is enabled"
			assert.Contains(t, err.Error(), expectedErrorMsg, "Error message mismatch.")
		}
	})
}

func TestLogRotation(t *testing.T) {
	globalLogger = nil
	once = sync.Once{}

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
	opts.ConsoleOutput = false
	opts.FileLevel = DebugLevel
	opts.LogMaxSizeMB = 1
	opts.LogMaxBackups = 2
	opts.LogCompress = false

	logger, err := NewLogger(opts)
	assert.NoError(t, err, "Failed to create logger for rotation test")
	if err != nil {t.FailNow()}
	defer logger.Sync()

	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("log line content to make it reasonably long ")
	}
	logLine := sb.String() // approx 4KB

	numLinesToWrite := (opts.LogMaxSizeMB * 1024 * 1024) / len(logLine) * (opts.LogMaxBackups + 1) + 1000


	for i := 0; i < numLinesToWrite; i++ {
		logger.Debugf("%s %d", logLine, i)
	}
	logger.Sync()

	files, err := filepath.Glob(filepath.Join(tmpDir, logFileName+"*"))
	if err != nil {
		t.Fatalf("Error listing log files: %v", err)
	}

	assert.True(t, len(files) >= 1, "Expected at least one log file, found %d: %v", len(files), files)
	// Allow for some flexibility in exact number of files due to rotation timing
	assert.True(t, len(files) <= opts.LogMaxBackups+1+1, "Expected at most %d files (+1 margin), found %d: %v", opts.LogMaxBackups+1, len(files), files)

	for _, fPath := range files {
		stat, errStat := os.Stat(fPath)
		assert.NoError(t, errStat, "Could not stat log file %s", fPath)
		if errStat == nil {
			if filepath.Base(fPath) == logFileName {
				assert.LessOrEqual(t, stat.Size(), int64(opts.LogMaxSizeMB*1024*1024)+int64(len(logLine)), "Current log file %s exceeds MaxSizeMB (with tolerance)", fPath)
			}
		}
	}
}

func TestDynamicLogLevelChange_GlobalLogger(t *testing.T) {
	originalGlobalLogger, originalOnce := globalLogger, once
	originalOsStdout := os.Stdout // Save the original os.Stdout at the beginning of the test
	defer func() {
		globalLogger, originalOnce = originalGlobalLogger, originalOnce
		os.Stdout = originalOsStdout // Restore the very original os.Stdout at the end
	}()

	captureGlobalLogs := func(initLevel Level, logFunc func()) string {
		globalLogger = nil; once = sync.Once{} // Ensure Init runs for this capture segment

		r, w, _ := os.Pipe()
		os.Stdout = w // Redirect os.Stdout: global logger created by Init will use this pipe

		// Initialize the global logger with the specified console level for this segment
		opts := DefaultOptions()
		opts.ConsoleLevel = initLevel // This sets the static level for the console core
		opts.FileOutput = false
		opts.ColorConsole = false
		Init(opts) // globalLogger is now initialized and configured to write to the pipe `w`

		logFunc() // Execute the logging actions

		w.Close()          // Close the writer end of the pipe
		os.Stdout = originalOsStdout // Restore os.Stdout immediately after capture FOR SAFETY, though defer handles test end
									 // This is important if other non-logging actions use os.Stdout between captures.

		var buf bytes.Buffer
		io.Copy(&buf, r) // Read everything from the pipe
		r.Close()        // Close the reader end
		return buf.String()
	}

	// Segment 1: Initial static level is Debug, Atomic level (after Init) is also Debug.
	output1 := captureGlobalLogs(DebugLevel, func() {
		Debug("global_debug_initial_debug_level") // Atomic=Debug, StaticCore=Debug -> PASS
		Info("global_info_initial_debug_level")   // Atomic=Debug, StaticCore=Debug -> PASS
		SyncGlobal()
	})

	assert.Contains(t, output1, "global_debug_initial_debug_level", "Output 1: Debug should be present")
	assert.Contains(t, output1, "global_info_initial_debug_level", "Output 1: Info should be present")

	// Segment 2: Change atomic level to Warn. Initial static level for console core is still Debug (from Init in this capture).
	output2 := captureGlobalLogs(DebugLevel, func() { // Init will set static console to Debug, atomic to Debug
		SetGlobalLevel(WarnLevel) // Change atomic level to Warn. Static console core is still Debug.

		Debug("global_debug_after_warn_set") // Atomic=Warn (filters Debug), StaticCore=Debug -> FILTERED by Atomic
		Info("global_info_after_warn_set")   // Atomic=Warn (filters Info), StaticCore=Debug -> FILTERED by Atomic
		Warn("global_warn_after_warn_set")   // Atomic=Warn (passes), StaticCore=Debug (passes Warn) -> PASS
		SyncGlobal()
	})

	assert.NotContains(t, output2, "global_debug_after_warn_set", "Output 2: Debug should be filtered by atomic Warn")
	assert.NotContains(t, output2, "global_info_after_warn_set", "Output 2: Info should be filtered by atomic Warn")
	assert.Contains(t, output2, "global_warn_after_warn_set", "Output 2: Warn should appear")
}


func TestDynamicLogLevelChange_InstanceLogger(t *testing.T) {
	createLoggerWithBuffer := func(level Level) (*Logger, *bytes.Buffer, error) {
		var buf bytes.Buffer
		opts := DefaultOptions()
		opts.ConsoleLevel = level
		opts.FileOutput = false
		opts.ColorConsole = false

		consoleEncoderCfg := zap.NewProductionEncoderConfig()
		consoleEncoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat)
		consoleEncoderCfg.TimeKey = "time"; consoleEncoderCfg.LevelKey = ""; consoleEncoderCfg.CallerKey = "caller"
		consoleEncoderCfg.MessageKey = "msg"; consoleEncoderCfg.NameKey = "logger"; consoleEncoderCfg.StacktraceKey = "stacktrace"
		consoleEncoder := NewPlainTextConsoleEncoder(consoleEncoderCfg, opts)

		initialAtomicLevelVal := level.ToZapLevel()
		if level == SuccessLevel { initialAtomicLevelVal = zapcore.InfoLevel}
		if level == FailLevel { initialAtomicLevelVal = zapcore.FatalLevel}
		atomicLvl := zap.NewAtomicLevelAt(initialAtomicLevelVal)

		coreStaticLevelVal := level.ToZapLevel()
		if level == SuccessLevel { coreStaticLevelVal = zapcore.InfoLevel}
		if level == FailLevel { coreStaticLevelVal = zapcore.FatalLevel}
		levelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= coreStaticLevelVal
		})

		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(&buf), levelEnabler)
		zapL := zap.New(consoleCore, zap.AddCaller(), zap.AddCallerSkip(2), zap.IncreaseLevel(atomicLvl))

		lgInstance := &Logger{SugaredLogger: zapL.Sugar(), opts: opts, atomicLevel: atomicLvl}
		return lgInstance, &buf, nil
	}

	logger1, buf1, err1 := createLoggerWithBuffer(InfoLevel) // Static L1 console core is INFO
	assert.NoError(t, err1)
	defer logger1.Sync()

	logger2, buf2, err2 := createLoggerWithBuffer(DebugLevel) // Static L2 console core is DEBUG
	assert.NoError(t, err2)
	defer logger2.Sync()

	var output1, output2 string

	// L1: Atomic=INFO, StaticCore=INFO
	logger1.Debugf("%s", "l1_debug_initial"); logger1.Infof("%s", "l1_info_initial"); logger1.Sync()
	output1 = buf1.String(); buf1.Reset()
	assert.NotContains(t, output1, "l1_debug_initial", "L1: Debug should be filtered by Info static/atomic level")
	assert.Contains(t, output1, "l1_info_initial", "L1: Info should be present")

	// L2: Atomic=DEBUG, StaticCore=DEBUG
	logger2.Debugf("%s", "l2_debug_initial"); logger2.Infof("%s", "l2_info_initial"); logger2.Sync()
	output2 = buf2.String(); buf2.Reset()
	assert.Contains(t, output2, "l2_debug_initial", "L2: Debug should be present")
	assert.Contains(t, output2, "l2_info_initial", "L2: Info should be present")

	// Change L1's atomic level to DEBUG. Its static core level is still INFO.
	logger1.SetLevel(DebugLevel) // L1: Atomic=DEBUG, StaticCore=INFO
	logger1.Debugf("%s", "l1_debug_after_change"); // Atomic=DEBUG (pass), StaticCore=INFO (fail for Debug) -> FILTERED by StaticCore
	logger1.Infof("%s", "l1_info_after_change");   // Atomic=DEBUG (pass), StaticCore=INFO (pass for Info) -> PASS
	logger1.Sync()
	output1 = buf1.String(); buf1.Reset()
	assert.NotContains(t, output1, "l1_debug_after_change", "L1: Debug should still be filtered by static Info level, even if atomic is Debug")
	assert.Contains(t, output1, "l1_info_after_change", "L1: Info should be present")

	// No change to logger2: Atomic=DEBUG, StaticCore=DEBUG
	logger2.Debugf("%s", "l2_debug_no_change"); logger2.Infof("%s", "l2_info_no_change"); logger2.Sync()
	output2 = buf2.String(); buf2.Reset()
	assert.Contains(t, output2, "l2_debug_no_change")
	assert.Contains(t, output2, "l2_info_no_change")

	// Change L2's atomic level to ERROR. Its static core level is DEBUG.
	logger2.SetLevel(ErrorLevel) // L2: Atomic=ERROR, StaticCore=DEBUG
	logger2.Warnf("%s", "l2_warn_after_error_set"); // Atomic=ERROR (fail for Warn), StaticCore=DEBUG (pass for Warn) -> FILTERED by Atomic
	logger2.Errorf("%s", "l2_error_after_error_set"); // Atomic=ERROR (pass), StaticCore=DEBUG (pass for Error) -> PASS
	logger2.Sync()
	output2 = buf2.String(); buf2.Reset()
	assert.NotContains(t, output2, "l2_warn_after_error_set", "L2: Warn filtered by atomic Error")
	assert.Contains(t, output2, "l2_error_after_error_set", "L2: Error should be present")

	// L1: Atomic=DEBUG, StaticCore=INFO.
	logger1.Debugf("%s", "l1_debug_still_debug_atomic_info_static"); logger1.Sync() // Filtered by StaticCore
	output1 = buf1.String(); buf1.Reset()
	assert.NotContains(t, output1, "l1_debug_still_debug_atomic_info_static")
}
