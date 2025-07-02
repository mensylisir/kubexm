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
		LevelKey:       "L", // Will be overridden by customlevel logic or Zap's level string
		NameKey:        "N",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder, // Fallback if customlevel not found
		EncodeTime:     zapcore.TimeEncoderOfLayout(opts.TimestampFormat),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

func TestColorConsoleEncoder_Clone(t *testing.T) {
	opts := DefaultOptions()
	cfg := newTestEncoderConfig(opts)
	enc := NewColorConsoleEncoder(cfg, opts).(*colorConsoleEncoder)

	enc.AddString("key1", "value1")
	enc.AddInt("key2", 123)

	clone := enc.Clone().(*colorConsoleEncoder)

	// Modify original after clone
	enc.AddString("key3", "value3")

	// Assert that clone has original fields but not the new one
	assert.Len(t, clone.contextFields, 2, "Clone should have fields from original before original was modified post-clone")
	cloneFieldMap := make(map[string]interface{})
	for _, f := range clone.contextFields {
		switch f.Type {
		case zapcore.StringType:
			cloneFieldMap[f.Key] = f.String
		case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type, zapcore.UintptrType, zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type: // Add other integer types as needed
			cloneFieldMap[f.Key] = f.Integer
		default:
			cloneFieldMap[f.Key] = f.Interface // Fallback, though might be nil for some types
		}
	}
	assert.Equal(t, "value1", cloneFieldMap["key1"])
	assert.Equal(t, int64(123), cloneFieldMap["key2"])

	// Assert that original has all fields
	originalFieldMap := make(map[string]interface{})
	for _, f := range enc.contextFields {
		switch f.Type {
		case zapcore.StringType:
			originalFieldMap[f.Key] = f.String
		case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type, zapcore.UintptrType, zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
			originalFieldMap[f.Key] = f.Integer
		default:
			originalFieldMap[f.Key] = f.Interface
		}
	}
	assert.Len(t, enc.contextFields, 3)
	assert.Equal(t, "value3", originalFieldMap["key3"])

	// Test that modifying clone does not affect original
	clone.AddString("key4", "value4")
	assert.Len(t, enc.contextFields, 3, "Original should not be affected by clone modification")
}

func TestColorConsoleEncoder_AddFields(t *testing.T) {
	opts := DefaultOptions()
	cfg := newTestEncoderConfig(opts)
	enc := NewColorConsoleEncoder(cfg, opts).(*colorConsoleEncoder)

	enc.AddString("stringKey", "stringValue")
	enc.AddInt("intKey", 100)
	enc.AddBool("boolKey", true)
	enc.AddDuration("durationKey", time.Second)

	assert.Len(t, enc.contextFields, 4)
	// Basic check, detailed field checking done in EncodeEntry tests
}

func TestColorConsoleEncoder_EncodeEntry(t *testing.T) {
	opts := DefaultOptions()
	opts.TimestampFormat = "2006-01-02 15:04:05" // Use a fixed format for predictable output
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
		level          Level // Our custom level
		zapLevel       zapcore.Level // Underlying zap level if customlevel not set
		withFields     []zapcore.Field
		logFields      []zapcore.Field
		expectedPrefix string // Expected context prefix part
		expectedLevel  string // Expected level string (with or without color)
		expectedSuffix string // Expected key=value suffix
	}{
		{
			name:           "Info level with color and context",
			colors:         true,
			level:          InfoLevel,
			zapLevel:       zapcore.InfoLevel,
			withFields:     []zap.Field{zap.String("pipeline_name", "p1"), zap.String("host_name", "h1")},
			logFields:      []zap.Field{zap.String("data", "payload")},
			expectedPrefix: "[P:p1][H:h1]",
			expectedLevel:  "[INFO]", // Info is not colored by default in levelToColor
			expectedSuffix: ` data=payload`, // "payload" does not contain spaces, should not be quoted
		},
		{
			name:           "Success level with color",
			colors:         true,
			level:          SuccessLevel,
			zapLevel:       zapcore.InfoLevel, // Success maps to Info in zap
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
			expectedSuffix: ` error="test error"`, // Errors are always quoted by fmt.Fprintf %q on string
		},
		{
			name:           "Fail level with color and multiple context fields",
			colors:         true,
			level:          FailLevel,
			zapLevel:       zapcore.FatalLevel, // Fail maps to Fatal
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
			logFields:      []zap.Field{zap.String("detail", "more info")}, // Contains space, should be quoted
			expectedPrefix: "",
			expectedLevel:  "[DEBUG]",
			expectedSuffix: ` detail="more info"`,
		},
		{
			name:     "Zap InfoLevel when customlevel not present",
			colors:   true,
			zapLevel: zapcore.InfoLevel, // No customlevel field
			// logFields should not contain "customlevel" for this test
			expectedPrefix: "",
			expectedLevel:  "[INFO]", // Default zap info, no color by our current levelToColorZap
			expectedSuffix: "",
		},
		{
			name:     "Zap WarnLevel with color",
			colors:   true,
			zapLevel: zapcore.WarnLevel, // No customlevel field
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

			// Apply 'With' fields
			if len(tt.withFields) > 0 {
				enc = enc.Clone() // Clone to simulate With creating a new logger instance
				for _, f := range tt.withFields {
					f.AddTo(enc)
				}
			}

			// Add customlevel field if our custom level is set
			currentLogFields := make([]zapcore.Field, len(tt.logFields))
			copy(currentLogFields, tt.logFields)
			if tt.level != 0 { // 0 would be an invalid/unset Level
				currentLogFields = append(currentLogFields, zap.String("customlevel", tt.level.CapitalString()))
			}
			entry.Level = tt.zapLevel // Set the underlying zap level

			buf, err := enc.EncodeEntry(entry, currentLogFields)
			assert.NoError(t, err)
			if err != nil {
				return
			}
			logOutput := buf.String()
			buf.Free()

			// Expected format: Time [Context] [Level] Caller: Message Fields
			// Example: 2023-01-01 12:00:00 [P:p1][H:h1] [INFO] test/file.go:42: Test message data="payload"

			var parts []string
			parts = append(parts, now.Format(opts.TimestampFormat))
			if tt.expectedPrefix != "" {
				parts = append(parts, tt.expectedPrefix)
			}
			parts = append(parts, tt.expectedLevel)
			parts = append(parts, "test/file.go:42:") // Caller
			parts = append(parts, entry.Message)

			// Construct expected string carefully
			var expected strings.Builder
			for i, p := range parts {
				expected.WriteString(p)
				if i < len(parts)-1 { // Add space between main parts
					expected.WriteString(" ")
				}
			}
			if tt.expectedSuffix != "" {
				// Suffix already has leading space in definition
				expected.WriteString(tt.expectedSuffix)
			}
			expected.WriteString(zapcore.DefaultLineEnding)

			assert.Equal(t, expected.String(), logOutput)
		})
	}
}


// Test tempEncoder minimally, as it's an internal detail for EncodeCaller
func TestTempEncoder(t *testing.T) {
	opts := DefaultOptions()
	cfg := newTestEncoderConfig(opts)
	buf := _bufferPool.Get()
	defer buf.Free()

	enc := &tempEncoder{buf: buf, EncoderConfig: cfg}
	enc.AddString("", "test/caller.go:123") // EncodeCaller often passes empty key
	assert.Equal(t, "test/caller.go:123", buf.String())
}
