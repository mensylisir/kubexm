package logger

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestLevelToColor(t *testing.T) {
	tests := []struct {
		level    Level
		message  string
		expected string
	}{
		{DebugLevel, "[DEBUG]", colorMagenta + "[DEBUG]" + colorReset},
		{InfoLevel, "[INFO]", "[INFO]"}, // Info level has no color by default in this func
		{SuccessLevel, "[SUCCESS]", colorGreen + "[SUCCESS]" + colorReset},
		{WarnLevel, "[WARN]", colorYellow + "[WARN]" + colorReset},
		{ErrorLevel, "[ERROR]", colorRed + "[ERROR]" + colorReset},
		{FailLevel, "[FAIL]", colorRed + "[FAIL]" + colorReset},
		{FatalLevel, "[FATAL]", colorRed + "[FATAL]" + colorReset}, // Though FailLevel is preferred
		{PanicLevel, "[PANIC]", colorCyan + "[PANIC]" + colorReset},
		{Level(99), "[UNKNOWN]", "[UNKNOWN]"}, // Unknown level
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			actual := levelToColor(tt.level, tt.message)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestLevelToColorZap(t *testing.T) {
	tests := []struct {
		level    zapcore.Level
		message  string
		expected string
	}{
		{zapcore.DebugLevel, "[DEBUG]", colorMagenta + "[DEBUG]" + colorReset},
		{zapcore.InfoLevel, "[INFO]", "[INFO]"}, // Info level has no color
		{zapcore.WarnLevel, "[WARN]", colorYellow + "[WARN]" + colorReset},
		{zapcore.ErrorLevel, "[ERROR]", colorRed + "[ERROR]" + colorReset},
		{zapcore.DPanicLevel, "[DPANIC]", colorCyan + "[DPANIC]" + colorReset},
		{zapcore.PanicLevel, "[PANIC]", colorCyan + "[PANIC]" + colorReset},
		{zapcore.FatalLevel, "[FATAL]", colorRed + "[FATAL]" + colorReset},
		{zapcore.Level(99), "[UNKNOWN]", "[UNKNOWN]"}, // Unknown zap level
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			actual := levelToColorZap(tt.level, tt.message)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestCacheLevelStrings(t *testing.T) {
	expectedNoColor := make(map[Level]string)
	allLevels := []Level{DebugLevel, InfoLevel, SuccessLevel, WarnLevel, ErrorLevel, FailLevel, PanicLevel, FatalLevel}
	for _, l := range allLevels {
		expectedNoColor[l] = fmt.Sprintf("[%s]", l.CapitalString())
	}

	cachedNoColor := cacheLevelStrings(false)
	assert.Equal(t, expectedNoColor, cachedNoColor, "cacheLevelStrings(false) mismatch")

	expectedWithColor := make(map[Level]string)
	for _, l := range allLevels {
		expectedWithColor[l] = levelToColor(l, fmt.Sprintf("[%s]", l.CapitalString()))
	}
	cachedWithColor := cacheLevelStrings(true)
	assert.Equal(t, expectedWithColor, cachedWithColor, "cacheLevelStrings(true) mismatch")

	// Test that InfoLevel with color is indeed not colored by levelToColor
	assert.Equal(t, "[INFO]", cachedWithColor[InfoLevel], "InfoLevel should not be colored by default in cache")
	// Test that SuccessLevel is colored
	assert.True(t, strings.Contains(cachedWithColor[SuccessLevel], colorGreen), "SuccessLevel should be green")
}
