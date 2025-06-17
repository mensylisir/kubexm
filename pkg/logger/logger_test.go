package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

// captureStdout captures everything written to os.Stdout during the execution of the given function.
func captureStdout(f func()) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close() // Close reader when done

	stdout := os.Stdout
	os.Stdout = w // Redirect stdout to pipe writer
	defer func() {
		os.Stdout = stdout // Restore stdout
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	var outputBuffer bytes.Buffer
	go func() {
		defer wg.Done()
		// Use io.Copy until the writer is closed.
		// This ensures all output is captured even if f() exits before all writes complete.
		_, _ = io.Copy(&outputBuffer, r)
	}()

	f() // Execute the function that writes to stdout

	// Close the writer to signal the reading goroutine that there's no more input.
	// This is crucial for io.Copy to return.
	if err := w.Close(); err != nil {
		// If the function f caused a panic that was recovered, w might already be closed.
		// Or if there's an issue with the pipe.
		// We return the captured output along with this error.
		return outputBuffer.String(), fmt.Errorf("failed to close pipe writer: %w", err)
	}

	wg.Wait() // Wait for the reading goroutine to finish

	return outputBuffer.String(), nil
}


func TestNewLogger_ConsoleOutput(t *testing.T) {
	opts := DefaultOptions()
	opts.ConsoleLevel = DebugLevel
	opts.FileOutput = false // Ensure only console for this test
	opts.ColorConsole = false // Simplify assertion by disabling color

	logger, err := NewLogger(opts)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Sync()

	testMsg := "Test console message"
	expectedPrefix := "[INFO]" // Default for InfoLevel

	output, err := captureStdout(func() {
		logger.Infof(testMsg)
	})
	if err != nil {
		t.Fatalf("Failed to capture stdout: %v", err)
	}

	if !strings.Contains(output, expectedPrefix) {
		t.Errorf("Console output missing prefix. Got: %q, Expected to contain: %q", output, expectedPrefix)
	}
	if !strings.Contains(output, testMsg) {
		t.Errorf("Console output missing message. Got: %q, Expected to contain: %q", output, testMsg)
	}
}

func TestNewLogger_FileOutput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test_app.log")
	opts := DefaultOptions()
	opts.FileLevel = InfoLevel
	opts.LogFilePath = logFilePath
	opts.FileOutput = true
	opts.ConsoleOutput = false // Ensure only file for this test

	logger, err := NewLogger(opts)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}

	testMsg := "Test file message for InfoLevel"
	logger.Infof(testMsg)
	logger.Debugf("This debug message should not be in file") // FileLevel is Info
	logger.Sync() // Ensure logs are flushed

	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	logContent := string(content)
	if !strings.Contains(logContent, testMsg) {
		t.Errorf("Log file content missing message. Got: %q, Expected to contain: %q", logContent, testMsg)
	}
	if strings.Contains(logContent, "This debug message should not be in file") {
		t.Errorf("Log file contains debug message when FileLevel is Info. Content: %q", logContent)
	}
	// JSON logs are harder to assert for exact format without parsing,
	// but check for key elements.
	if !strings.Contains(logContent, `"level":"INFO"`) {
		t.Errorf("Log file JSON missing level. Got: %q", logContent)
	}
	if !strings.Contains(logContent, `"msg":"Test file message for InfoLevel"`) {
		t.Errorf("Log file JSON missing message field. Got: %q", logContent)
	}
}


func TestNewLogger_ColoredConsoleOutput(t *testing.T) {
	opts := DefaultOptions()
	opts.ConsoleLevel = DebugLevel // Ensure all levels are logged for this test
	opts.FileOutput = false
	opts.ColorConsole = true

	logger, err := NewLogger(opts)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer logger.Sync()

	testCases := []struct {
		level         Level
		logFunc       func(template string, args ...interface{})
		message       string
		expectedColor string
		levelString   string
	}{
		{SuccessLevel, logger.Successf, "A success message", colorGreen, "[SUCCESS]"},
		{ErrorLevel, logger.Errorf, "An error message", colorRed, "[ERROR]"},
		{WarnLevel, logger.Warnf, "A warning message", colorYellow, "[WARN]"},
		{InfoLevel, logger.Infof, "An info message", "", "[INFO]"},
		{DebugLevel, logger.Debugf, "A debug message", colorMagenta, "[DEBUG]"},
	}


	for _, tc := range testCases {
		t.Run(tc.level.String(), func(t *testing.T) {
			output, errCap := captureStdout(func() {
				tc.logFunc(tc.message)
			})
			if errCap != nil {
				t.Fatalf("Failed to capture stdout for %s: %v", tc.level.String(), errCap)
			}

			if !strings.Contains(output, tc.message) {
				t.Errorf("Console output for %s missing message. Got: %q", tc.level.String(), output)
			}

			if !strings.Contains(output, tc.levelString) {
				t.Errorf("Console output for %s missing level string %s. Got: %q", tc.level.String(), tc.levelString, output)
			}

			if tc.expectedColor != "" {
				// Check if the colored level string is present
				expectedColoredLevel := tc.expectedColor + tc.levelString + colorReset
				if !strings.Contains(output, expectedColoredLevel) {
                    // It's possible timestamp or other parts have color, so a direct prefix check on output is hard.
                    // We check if the expected colored level string sequence exists.
					t.Errorf("Console output for %s missing expected color sequence %q. Got: %q", tc.level.String(), expectedColoredLevel, output)
				}
			} else {
				// For Info (no color), check that specific color codes are NOT framing the level string directly
				colorsToAvoid := []string{colorRed, colorGreen, colorYellow, colorMagenta, colorCyan}
				for _, c := range colorsToAvoid {
					if strings.Contains(output, c+tc.levelString) {
						t.Errorf("Console output for %s (no color expected for level string) contains color code %q before level. Got: %q", tc.level.String(), c, output)
					}
				}
			}
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	opts := DefaultOptions()
	opts.ConsoleLevel = WarnLevel // Only Warn and above for console
	opts.FileOutput = false
	opts.ColorConsole = false

	logger, err := NewLogger(opts)
	if err != nil {
		t.Fatalf("NewLogger error: %v", err)
	}
	defer logger.Sync()

	var output string
	output, err = captureStdout(func() {
		logger.Debugf("debug_test")
		logger.Infof("info_test")
		logger.Successf("success_test")
		logger.Warnf("warn_test")
		logger.Errorf("error_test")
	})
	if err != nil {
		t.Fatalf("captureStdout error: %v", err)
	}

	if strings.Contains(output, "debug_test") {
		t.Error("Console output contains DEBUG message when level is WARN")
	}
	if strings.Contains(output, "success_test") {
		t.Error("Console output contains SUCCESS message when level is WARN (Success is Info by Zap)")
	}
	if strings.Contains(output, "info_test") {
		t.Error("Console output contains INFO message when level is WARN")
	}


	if !strings.Contains(output, "warn_test") {
		t.Error("Console output missing WARN message when level is WARN")
	}
	if !strings.Contains(output, "error_test") {
		t.Error("Console output missing ERROR message when level is WARN")
	}
}

func TestGlobalLogger(t *testing.T) {
	originalGlobalLogger := globalLogger
	originalOnce := once
	defer func() { // Restore global state
		globalLogger = originalGlobalLogger
		once = originalOnce
	}()

	globalLogger = nil
	once = sync.Once{}


	opts := DefaultOptions()
	opts.ConsoleLevel = InfoLevel
	opts.FileOutput = false
	opts.ColorConsole = false
	Init(opts)

	defer func() {
		if err := SyncGlobal(); err != nil {
			t.Errorf("SyncGlobal failed: %v", err)
		}
	}()


	output, err := captureStdout(func() {
		Info("Global logger test")
		Success("Global success test")
	})
	if err != nil {
		t.Fatalf("captureStdout error: %v", err)
	}

	if !strings.Contains(output, "[INFO] Global logger test") {
		t.Errorf("Global Info() didn't work as expected. Got: %s", output)
	}
	if !strings.Contains(output, "[SUCCESS] Global success test") {
		t.Errorf("Global Success() didn't work as expected. Got: %s", output)
	}

	secondOpts := DefaultOptions()
	secondOpts.ConsoleLevel = DebugLevel
	Init(secondOpts) // This call should be a no-op

	output2, err2 := captureStdout(func() {
		Debug("Global debug, should not appear if Init was no-op")
	})
	if err2 != nil {t.Fatalf("captureStdout error: %v", err2)}

	if strings.Contains(output2, "Global debug") {
		t.Error("Global Debug() appeared, meaning Init was called again or level changed unexpectedly.")
	}
}

func TestTimestampFormat(t *testing.T) {
	customFormat := "2006/01/02_15:04:05"
	opts := DefaultOptions()
	opts.ConsoleLevel = InfoLevel
	opts.FileOutput = false
	opts.ColorConsole = false
	opts.TimestampFormat = customFormat

	logger, err := NewLogger(opts)
	if err != nil {t.Fatalf("NewLogger error: %v", err)}
	defer logger.Sync()

	output, errCap := captureStdout(func() {
		logger.Infof("Timestamp test")
	})
	if errCap != nil {t.Fatalf("captureStdout error: %v", errCap)}

	re := regexp.MustCompile(`\d{4}/\d{2}/\d{2}_\d{2}:\d{2}:\d{2}`)
	if !re.MatchString(output) {
		t.Errorf("Console output does not contain timestamp in expected format %s. Got: %q", customFormat, output)
	}
}


func TestFailAndFatalLevels(t *testing.T) {
    opts := DefaultOptions()
    opts.ConsoleLevel = InfoLevel
    opts.FileOutput = false
    opts.ColorConsole = false

    t.Run("FailLevelOutput", func(t *testing.T) {
        var buf bytes.Buffer
        fatalCoreCfg := zap.NewProductionEncoderConfig()
		fatalCoreCfg.EncodeTime = zapcore.TimeEncoderOfLayout(opts.TimestampFormat)
		// Ensure time key is set for custom encoder if it relies on it
		fatalCoreCfg.TimeKey = "time"
        fatalEncoder := NewPlainTextConsoleEncoder(fatalCoreCfg, opts)

        fatalLevelEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool { return lvl >= zapcore.FatalLevel })
        core := zapcore.NewCore(fatalEncoder, zapcore.AddSync(&buf), fatalLevelEnabler)

		// AddCallerSkip(2) because we are calling logWithCustomLevel from this test function,
		// and logWithCustomLevel calls SugaredLogger.WithOptions(zap.AddCallerSkip(1)).
		// This means the original call site is 2 levels up from the zap core.
		tempZapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(2))
        tempLogger := &Logger{SugaredLogger: tempZapLogger.Sugar(), opts: opts}

        // We are not calling os.Exit here, just testing the log formatting.
		// The actual Failf would call os.Exit via zap's Fatal.
        tempLogger.logWithCustomLevel(FailLevel, "This is a fail test")
        tempLogger.Sync()
        output := buf.String()

        if !strings.Contains(output, "[FAIL]") {
            t.Errorf("FailLevel log output missing [FAIL]. Got: %s", output)
        }
        if !strings.Contains(output, "This is a fail test") {
            t.Errorf("FailLevel log output missing message. Got: %s", output)
        }
    })
}
