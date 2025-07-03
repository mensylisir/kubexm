package logger

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func newTestEncoderConfig(opts Options) zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout(opts.TimestampFormat),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func TestColorConsoleEncoder_Clone(t *testing.T) {
	opts := DefaultOptions()
	opts.ColorConsole = true
	opts.TimestampFormat = "custom-format" // Specific value to check
	cfg := newTestEncoderConfig(opts)
	originalEncoder := NewColorConsoleEncoder(cfg, opts).(*colorConsoleEncoder)

	clonedEncoder := originalEncoder.Clone().(*colorConsoleEncoder)

	assert.NotSame(t, originalEncoder, clonedEncoder, "Clone should return a new instance")

	// Compare individual non-function fields of EncoderConfig
	assert.Equal(t, originalEncoder.EncoderConfig.MessageKey, clonedEncoder.EncoderConfig.MessageKey)
	assert.Equal(t, originalEncoder.EncoderConfig.LevelKey, clonedEncoder.EncoderConfig.LevelKey)
	assert.Equal(t, originalEncoder.EncoderConfig.TimeKey, clonedEncoder.EncoderConfig.TimeKey)
	// Function pointers (EncodeLevel, EncodeTime etc.) are not reliably comparable with DeepEqual.
	// Their correct cloning is implicitly tested by behavior in EncodeEntry if they were different.

	assert.Equal(t, originalEncoder.colors, clonedEncoder.colors, "colors field should be copied")
	assert.Equal(t, originalEncoder.loggerOpts, clonedEncoder.loggerOpts, "loggerOpts should be copied (it's a struct, so value copy)")
	assert.Equal(t, originalEncoder.levelStrings, clonedEncoder.levelStrings, "levelStrings map should be same (it's a reference, but points to the same map data)")

	// Verify that changing a field in the clone's options doesn't affect the original
	clonedEncoder.loggerOpts.TimestampFormat = "clone-modified-format"
	assert.Equal(t, "custom-format", originalEncoder.loggerOpts.TimestampFormat, "Modifying clone's loggerOpts.TimestampFormat should not affect original's")
	assert.Equal(t, true, clonedEncoder.colors, "Cloned colors value mismatch")
}

// TestColorConsoleEncoder_AddFields was removed as its premise (testing contextFields) is no longer valid.
// Field handling is now tested via TestColorConsoleEncoder_EncodeEntry.

func TestColorConsoleEncoder_EncodeEntry(t *testing.T) {
	opts := DefaultOptions()
	opts.TimestampFormat = "2006-01-02 15:04:05"
	cfg := newTestEncoderConfig(opts)
	now := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	entry := zapcore.Entry{
		Time:    now,
		Message: "Test message",
		Caller:  zapcore.EntryCaller{Defined: true, File: "test/file.go", Line: 42},
	}

	tests := []struct {
		name           string
		colors         bool
		level          Level
		zapLevel       zapcore.Level
		withFields     []zapcore.Field
		logFields      []zapcore.Field
		expectedPrefix string
		expectedLevel  string
		expectedSuffix string
	}{
		{
			name:           "Info level with color and context",
			colors:         true,
			level:          InfoLevel,
			zapLevel:       zapcore.InfoLevel,
			withFields:     []zap.Field{zap.String("pipeline_name", "p1"), zap.String("host_name", "h1")},
			logFields:      []zap.Field{zap.String("data", "payload")},
			expectedPrefix: "[P:p1][H:h1]",
			expectedLevel:  "[INFO]",
			expectedSuffix: ` data=payload`,
		},
		{
			name:           "Success level with color",
			colors:         true,
			level:          SuccessLevel,
			zapLevel:       zapcore.InfoLevel,
			logFields:      []zap.Field{zap.Int("count", 5)},
			expectedPrefix: "",
			expectedLevel:  colorGreen + "[SUCCESS]" + colorReset,
			expectedSuffix: " count=5",
		},
		{
			name:           "Error level plain text",
			colors:         false,
			level:          ErrorLevel,
			zapLevel:       zapcore.ErrorLevel,
			logFields:      []zap.Field{zap.Error(fmt.Errorf("test error"))},
			expectedPrefix: "",
			expectedLevel:  "[ERROR]",
			expectedSuffix: ` error="test error"`,
		},
		{
			name:           "Fail level with color and multiple context fields",
			colors:         true,
			level:          FailLevel,
			zapLevel:       zapcore.FatalLevel,
			withFields: []zap.Field{
				zap.String("pipeline_name", "deploy"),
				zap.String("module_name", "kubelet"),
				zap.String("task_name", "start"),
				zap.String("step_name", "check_status"),
				zap.String("host_name", "worker-01"),
			},
			expectedPrefix: "[P:deploy][M:kubelet][T:start][S:check_status][H:worker-01]",
			expectedLevel:  colorRed + "[FAIL]" + colorReset,
			expectedSuffix: "",
		},
		{
			name:           "Debug with no color",
			colors:         false,
			level:          DebugLevel,
			zapLevel:       zapcore.DebugLevel,
			logFields:      []zap.Field{zap.String("detail", "more info")},
			expectedPrefix: "",
			expectedLevel:  "[DEBUG]",
			expectedSuffix: ` detail="more info"`,
		},
		{
			name:     "Zap InfoLevel when customlevel not present",
			colors:   true,
			zapLevel: zapcore.InfoLevel,
			expectedPrefix: "",
			expectedLevel:  "[INFO]",
			expectedSuffix: "",
		},
		{
			name:     "Zap WarnLevel with color",
			colors:   true,
			zapLevel: zapcore.WarnLevel,
			expectedPrefix: "",
			expectedLevel:  colorYellow + "[WARN]" + colorReset,
			expectedSuffix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var enc zapcore.Encoder
			currentOpts := opts
			currentOpts.ColorConsole = tt.colors
			if tt.colors {
				enc = NewColorConsoleEncoder(cfg, currentOpts)
			} else {
				enc = NewPlainTextConsoleEncoder(cfg, currentOpts)
			}

			allTestFields := append([]zapcore.Field{}, tt.withFields...)
			if tt.level != 0 {
				allTestFields = append(allTestFields, zap.String("customlevel", tt.level.CapitalString()))
			}
			allTestFields = append(allTestFields, tt.logFields...)

			currentEntry := entry
			currentEntry.Level = tt.zapLevel

			buf, err := enc.EncodeEntry(currentEntry, allTestFields)
			assert.NoError(t, err)
			if err != nil {
				return
			}
			logOutput := buf.String()
			buf.Free()

			var parts []string
			parts = append(parts, now.Format(opts.TimestampFormat))
			if tt.expectedPrefix != "" {
				parts = append(parts, tt.expectedPrefix)
			}
			parts = append(parts, tt.expectedLevel)
			parts = append(parts, "test/file.go:42:")
			parts = append(parts, entry.Message)

			var expected strings.Builder
			for i, p := range parts {
				expected.WriteString(p)
				if i < len(parts)-1 {
					expected.WriteString(" ")
				}
			}
			if tt.expectedSuffix != "" {
				expected.WriteString(tt.expectedSuffix)
			}
			expected.WriteString(zapcore.DefaultLineEnding)

			assert.Equal(t, expected.String(), logOutput)
		})
	}
}

func TestTempEncoder(t *testing.T) {
	opts := DefaultOptions()
	cfg := newTestEncoderConfig(opts)
	buf := _bufferPool.Get()
	defer buf.Free()

	enc := &tempEncoder{buf: buf, EncoderConfig: cfg}
	enc.AddString("", "test/caller.go:123")
	assert.Equal(t, "test/caller.go:123", buf.String())
}
