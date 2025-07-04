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

func TestColorConsoleEncoder_EncodeEntry(t *testing.T) {
	opts := DefaultOptions()
	opts.TimestampFormat = "2006-01-02 15:04:05"
	cfg := newTestEncoderConfig(opts)
	// Define a base entry that will be slightly modified by test cases
	baseEntry := zapcore.Entry{
		Time:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Message: "Test message", // Default message
		Caller:  zapcore.EntryCaller{Defined: true, File: "test/file.go", Line: 42},
	}

	tests := []struct {
		name            string
		colors          bool
		level           Level // Our custom level (0 if not using custom level field)
		zapLevel        zapcore.Level // Underlying zap level for the entry
		withFields      []zapcore.Field
		logFields       []zapcore.Field
		messageOverride *string // Pointer to allow nil (no override) vs empty string
		expectedPrefix  string
		expectedLevel   string
		expectedSuffix  string // This should include the leading space if fields are present
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
			expectedSuffix: " data=payload",
		},
		{
			name:           "Success level with color",
			colors:         true,
			level:          SuccessLevel,
			zapLevel:       zapcore.InfoLevel, // Custom levels often map to Info or Error for zap's core
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
			name:     "Fail level with color and all context fields",
			colors:   true,
			level:    FailLevel,
			zapLevel: zapcore.FatalLevel, // FailLevel implies a more severe outcome
			withFields: []zap.Field{
				zap.String("pipeline_name", "deploy"),
				zap.String("module_name", "kubelet"),
				zap.String("task_name", "start"),
				zap.String("hook_event", "pre-start"),
				zap.String("step_name", "check_status"),
				zap.String("hook_step_name", "verify_env"),
				zap.String("host_name", "worker-01"),
			},
			expectedPrefix: "[P:deploy][M:kubelet][T:start][HE:pre-start][S:check_status][HS:verify_env][H:worker-01]",
			expectedLevel:  colorRed + "[FAIL]" + colorReset,
			expectedSuffix: "", // No additional fields in this case
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
			name:           "Zap InfoLevel when customlevel not present",
			colors:         true,
			zapLevel:       zapcore.InfoLevel, // No custom level, just zap's
			expectedPrefix: "",
			expectedLevel:  "[INFO]",
			expectedSuffix: "",
		},
		{
			name:           "Zap WarnLevel with color",
			colors:         true,
			zapLevel:       zapcore.WarnLevel, // No custom level
			expectedPrefix: "",
			expectedLevel:  colorYellow + "[WARN]" + colorReset,
			expectedSuffix: "",
		},
		{
			name:            "Empty message and specific field types",
			colors:          false, // Plain text for simplicity
			level:           InfoLevel,
			zapLevel:        zapcore.InfoLevel,
			logFields: []zapcore.Field{
				zap.String("emptyStr", ""),
				{Key: "nilErrorField", Type: zapcore.ErrorType, Interface: nil},
				zap.String("strWithSpace", "hello world"),
			},
			messageOverride: stringPtr(""), // Override to empty message
			expectedPrefix:  "",
			expectedLevel:   "[INFO]",
			// Encoder behavior: CALLER + " " + " field1=value1" (double space issue if not careful in assertion)
			// Encoder: caller + (":" + msg | " ") + fields_with_leading_space
			// If msg empty: caller + " " + (fields_with_leading_space | "")
			// fields_with_leading_space is like " field=value"
			// So: caller + " " + " field=value" -> "caller  field=value"
			// The expectedSuffix should capture this: " emptyStr=\"\" nilErrorField=nil strWithSpace=\"hello world\"" (single leading space from field loop)
			// The assertion builder will add the other space.
			expectedSuffix: ` emptyStr="" nilErrorField=nil strWithSpace="hello world"`,
		},
		{
			name:            "No message, no fields",
			colors:          false,
			level:           InfoLevel,
			zapLevel:        zapcore.InfoLevel,
			messageOverride: stringPtr(""),
			expectedPrefix:  "",
			expectedLevel:   "[INFO]",
			expectedSuffix:  "", // No fields
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var enc zapcore.Encoder
			currentOpts := opts // Start with base opts
			currentOpts.ColorConsole = tt.colors
			if tt.colors {
				enc = NewColorConsoleEncoder(cfg, currentOpts)
			} else {
				enc = NewPlainTextConsoleEncoder(cfg, currentOpts)
			}

			allTestFields := make([]zapcore.Field, 0, len(tt.withFields)+len(tt.logFields)+1)
			allTestFields = append(allTestFields, tt.withFields...)
			if tt.level != 0 { // If a custom level is specified for the test (0 is not a valid custom level here)
				allTestFields = append(allTestFields, zap.String("customlevel", tt.level.CapitalString()))
			}
			allTestFields = append(allTestFields, tt.logFields...)

			currentEntry := baseEntry // Make a copy of the base entry for this test run
			currentEntry.Level = tt.zapLevel
			if tt.messageOverride != nil {
				currentEntry.Message = *tt.messageOverride
			}
			// else: it keeps the default "Test message" from baseEntry, or whatever was last set.
			// This is important: ensure baseEntry.Message is used if messageOverride is nil.

			buf, err := enc.EncodeEntry(currentEntry, allTestFields)
			assert.NoError(t, err, "EncodeEntry failed for test: %s", tt.name)
			if err != nil {
				return
			}
			logOutput := buf.String()
			buf.Free()

			// Construct the expected string meticulously
			expected := strings.Builder{}
			// Timestamp
			expected.WriteString(baseEntry.Time.Format(currentOpts.TimestampFormat))
			expected.WriteString(" ")

			// Context Prefix
			if tt.expectedPrefix != "" {
				expected.WriteString(tt.expectedPrefix)
				expected.WriteString(" ")
			}

			// Level
			expected.WriteString(tt.expectedLevel)
			expected.WriteString(" ")

			// Caller
			expected.WriteString(baseEntry.Caller.File) // e.g., test/file.go
			expected.WriteString(":")
			expected.WriteString(fmt.Sprintf("%d", baseEntry.Caller.Line)) // e.g., 42

			// Message part (including separator)
			// Encoder logic:
			// if ent.Message != "" { line.AppendString(": ") } else { line.AppendString(" ") }
			// if ent.Message != "" { line.AppendString(ent.Message) }
			if currentEntry.Message != "" {
				expected.WriteString(": ")
				expected.WriteString(currentEntry.Message)
			} else {
				expected.WriteString(" ") // If message is empty, a single space follows the caller
			}

			// Suffix (fields)
			// Encoder logic for fields: for _, f := range remainingFields { line.AppendString(" "); ... }
			// This means each field part in the output is prefixed by a space.
			// tt.expectedSuffix should represent this, e.g., " field1=val1 field2=val2"
			if tt.expectedSuffix != "" {
				// If message was present, the fields follow directly after the message.
				// If message was empty, a space was already added after caller.
				// The tt.expectedSuffix must contain its own leading space.
				if currentEntry.Message != "" { // If there was a message, and suffix exists
					if !strings.HasPrefix(tt.expectedSuffix, " ") && len(tt.expectedSuffix) > 0 {
						expected.WriteString(" ") // Ensure separation if suffix somehow didn't have it
					}
				}
				// If message was empty, the single space after caller is already there.
				// The suffix (e.g. " field1=val1") will correctly follow.
				expected.WriteString(tt.expectedSuffix)
			}

			expected.WriteString(zapcore.DefaultLineEnding)

			assert.Equal(t, expected.String(), logOutput, "Log output mismatch for test: %s", tt.name)
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

// Helper to get a pointer to a string, useful for messageOverride
func stringPtr(s string) *string {
	return &s
}
