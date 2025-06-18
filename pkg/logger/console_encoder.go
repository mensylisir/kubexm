package logger

import (
	"bytes" // Import bytes for Buffer
	"fmt"
	"strconv" // For AddXxx methods in tempEncoder and field formatting
	"strings"
	// "sync" // No longer needed directly in this file
	"time"

	"go.uber.org/zap/buffer" // Keep for _bufferPool
	"go.uber.org/zap/zapcore"
)

const (
	// ANSI color codes
	colorRed     = "[31m"
	colorGreen   = "[32m"
	colorYellow  = "[33m"
	colorBlue    = "[34m"
	colorMagenta = "[35m"
	colorCyan    = "[36m"
	colorReset   = "[0m"
)

var _bufferPool = buffer.NewPool() // Keep global pool

// colorConsoleEncoder implements zapcore.Encoder for customized console output.
type colorConsoleEncoder struct {
	zapcore.EncoderConfig
	spaced       bool
	colors       bool
	loggerOpts   Options
	levelStrings map[Level]string
}

// NewColorConsoleEncoder creates a new console encoder that uses colors.
func NewColorConsoleEncoder(cfg zapcore.EncoderConfig, opts Options) zapcore.Encoder {
	return &colorConsoleEncoder{
		EncoderConfig: cfg,
		spaced:        true,
		colors:        true,
		loggerOpts:    opts,
		levelStrings:  cacheLevelStrings(true, opts.ColorConsole),
	}
}

// NewPlainTextConsoleEncoder creates a new console encoder without colors.
func NewPlainTextConsoleEncoder(cfg zapcore.EncoderConfig, opts Options) zapcore.Encoder {
	return &colorConsoleEncoder{
		EncoderConfig: cfg,
		spaced:        true,
		colors:        false,
		loggerOpts:    opts,
		levelStrings:  cacheLevelStrings(false, false),
	}
}

// cacheLevelStrings (as before)
func cacheLevelStrings(color bool, useColor bool) map[Level]string {
	m := make(map[Level]string)
	levels := []Level{DebugLevel, InfoLevel, SuccessLevel, WarnLevel, ErrorLevel, FailLevel, PanicLevel, FatalLevel}
	for _, l := range levels {
		str := fmt.Sprintf("[%s]", l.CapitalString())
		if color && useColor { m[l] = levelToColor(l, str)
		} else { m[l] = str }
	}
	return m
}

// Clone clones the encoder.
func (enc *colorConsoleEncoder) Clone() zapcore.Encoder {
	return &colorConsoleEncoder{
		EncoderConfig: enc.EncoderConfig,
		spaced:        enc.spaced,
		colors:        enc.colors,
		loggerOpts:    enc.loggerOpts,
		levelStrings:  enc.levelStrings,
	}
}

// --- Methods for zapcore.ObjectEncoder interface ---
// These methods are called by Zap when structured fields are added to a log statement
// (e.g., logger.Info("message", zap.String("key", "value"))).
// For this console encoder, we don't accumulate fields in an internal buffer within these AddXxx methods.
// Instead, EncodeEntry will iterate over the `fields []zapcore.Field` argument it receives.
// Thus, most of these AddXxx methods can be minimal or no-op, as their primary role in some encoders
// (like JSON) is to append to an internal buffer that is then finalized. Here, that finalization
// happens directly from the `fields` slice in EncodeEntry.

func (enc *colorConsoleEncoder) OpenNamespace(key string) {}
func (enc *colorConsoleEncoder) AddArray(key string, arr zapcore.ArrayMarshaler) error { return nil } // Processed in EncodeEntry via fields
func (enc *colorConsoleEncoder) AddObject(key string, obj zapcore.ObjectMarshaler) error { return nil } // Processed in EncodeEntry via fields
func (enc *colorConsoleEncoder) AddBinary(key string, val []byte)          {}
func (enc *colorConsoleEncoder) AddByteString(key string, val []byte)    {}
func (enc *colorConsoleEncoder) AddBool(key string, val bool)              {}
func (enc *colorConsoleEncoder) AddComplex128(key string, val complex128)  {}
func (enc *colorConsoleEncoder) AddComplex64(key string, val complex64)    {}
func (enc *colorConsoleEncoder) AddDuration(key string, val time.Duration) {}
func (enc *colorConsoleEncoder) AddFloat64(key string, val float64)        {}
func (enc *colorConsoleEncoder) AddFloat32(key string, val float32)        {}
func (enc *colorConsoleEncoder) AddInt(key string, val int)                {}
func (enc *colorConsoleEncoder) AddInt64(key string, val int64)            {}
func (enc *colorConsoleEncoder) AddInt32(key string, val int32)            {}
func (enc *colorConsoleEncoder) AddInt16(key string, val int16)            {}
func (enc *colorConsoleEncoder) AddInt8(key string, val int8)              {}
func (enc *colorConsoleEncoder) AddString(key, val string)                 {}
func (enc *colorConsoleEncoder) AddTime(key string, val time.Time)         {}
func (enc *colorConsoleEncoder) AddUint(key string, val uint)              {}
func (enc *colorConsoleEncoder) AddUint64(key string, val uint64)          {}
func (enc *colorConsoleEncoder) AddUint32(key string, val uint32)          {}
func (enc *colorConsoleEncoder) AddUint16(key string, val uint16)          {}
func (enc *colorConsoleEncoder) AddUint8(key string, val uint8)            {}
func (enc *colorConsoleEncoder) AddUintptr(key string, val uintptr)        {}
func (enc *colorConsoleEncoder) AddReflected(key string, obj interface{}) error { return nil }

// Append methods are for array elements, not used by our simple field formatting in EncodeEntry.
func (enc *colorConsoleEncoder) AppendArray(zapcore.ArrayMarshaler) error { return nil }
func (enc *colorConsoleEncoder) AppendObject(zapcore.ObjectMarshaler) error { return nil }
func (enc *colorConsoleEncoder) AppendBool(bool) {}
func (enc *colorConsoleEncoder) AppendByteString([]byte) {}
func (enc *colorConsoleEncoder) AppendBinary([]byte) {} // Zap's console encoder does base64, we can skip or do simple string
func (enc *colorConsoleEncoder) AppendComplex128(complex128) {}
func (enc *colorConsoleEncoder) AppendComplex64(complex64) {}
func (enc *colorConsoleEncoder) AppendDuration(time.Duration) {}
func (enc *colorConsoleEncoder) AppendFloat64(float64) {}
func (enc *colorConsoleEncoder) AppendFloat32(float32) {}
func (enc *colorConsoleEncoder) AppendInt(int) {}
func (enc *colorConsoleEncoder) AppendInt64(int64) {}
func (enc *colorConsoleEncoder) AppendInt32(int32) {}
func (enc *colorConsoleEncoder) AppendInt16(int16) {}
func (enc *colorConsoleEncoder) AppendInt8(int8) {}
func (enc *colorConsoleEncoder) AppendString(string) {}
func (enc *colorConsoleEncoder) AppendTime(time.Time) {}
func (enc *colorConsoleEncoder) AppendUint(uint) {}
func (enc *colorConsoleEncoder) AppendUint64(uint64) {}
func (enc *colorConsoleEncoder) AppendUint32(uint32) {}
func (enc *colorConsoleEncoder) AppendUint16(uint16) {}
func (enc *colorConsoleEncoder) AppendUint8(uint8) {}
func (enc *colorConsoleEncoder) AppendUintptr(uintptr) {}


// EncodeEntry is the core method that formats the log entry.
func (enc *colorConsoleEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	line := _bufferPool.Get()

	// Timestamp
	if enc.TimeKey != "" { // TimeKey is usually "ts" in productionEncoderConfig
		line.AppendString(ent.Time.Format(enc.loggerOpts.TimestampFormat))
		line.AppendString(" ")
	}

	// Contextual Prefix (Pipeline, Module, Task, Step/Hook)
	var contextPrefix strings.Builder
	// Define the order of contextual keys for the prefix
	orderedContextKeys := []string{
		"pipeline_name", "module_name", "task_name",
		"hook_event", "step_name", "hook_step_name",
		"host_name", // host_name is now part of the prefix
	}
	logContextValues := make(map[string]string)
	remainingFields := make([]zapcore.Field, 0, len(fields))

	for _, f := range fields {
		isContextField := false
		for _, ctxKey := range orderedContextKeys {
			if f.Key == ctxKey {
				logContextValues[f.Key] = f.String
				isContextField = true
				break
			}
		}
		if !isContextField && f.Key != "customlevel" && f.Key != "customlevel_num" {
			remainingFields = append(remainingFields, f)
		}
	}

	for _, key := range orderedContextKeys {
		if val, ok := logContextValues[key]; ok && val != "" {
			// Use short prefixes for context keys
			shortKey := key
			switch key {
			case "pipeline_name": shortKey = "P"
			case "module_name": shortKey = "M"
			case "task_name": shortKey = "T"
			case "step_name": shortKey = "S"
			case "hook_event": shortKey = "HE" // E.g. PipelinePreRun
			case "hook_step_name": shortKey = "HS"
			case "host_name": shortKey = "H"
			}
			contextPrefix.WriteString(fmt.Sprintf("[%s:%s]", shortKey, val))
		}
	}

	if contextPrefix.Len() > 0 {
		line.AppendString(contextPrefix.String())
		line.AppendString(" ")
	}

	// Level (custom logic as before)
	customLevelStr := ""; ourLevel := InfoLevel // Default
	for _, f := range fields {
		if f.Key == "customlevel" && f.Type == zapcore.StringType {
			levelStr := f.String
			switch strings.ToUpper(levelStr) {
			case "DEBUG": ourLevel = DebugLevel; case "INFO": ourLevel = InfoLevel
			case "SUCCESS": ourLevel = SuccessLevel; case "WARN": ourLevel = WarnLevel
			case "ERROR": ourLevel = ErrorLevel; case "FAIL": ourLevel = FailLevel
			case "PANIC": ourLevel = PanicLevel; case "FATAL": ourLevel = FatalLevel
			}
			customLevelStr = enc.levelStrings[ourLevel]; break
		}
	}
	if customLevelStr == "" {
		levelText := fmt.Sprintf("[%s]", strings.ToUpper(ent.Level.String()))
		if enc.colors { customLevelStr = levelToColorZap(ent.Level, levelText)
		} else { customLevelStr = levelText }
	}
	line.AppendString(customLevelStr)
	line.AppendString(" ")

	// Caller (optional)
	if ent.Caller.Defined && enc.CallerKey != "" && enc.EncodeCaller != nil {
		callerBuf := _bufferPool.Get()
		// Use a temporary encoder that writes to callerBuf for EncodeCaller
		tempEnc := &tempEncoder{buf: callerBuf, EncoderConfig: enc.EncoderConfig}
		enc.EncodeCaller(ent.Caller, tempEnc)
		if callerBuf.Len() > 0 {
			// No "caller=" prefix for console, just the path.
			// line.AppendString(enc.CallerKey); line.AppendString("=")
			line.Write(callerBuf.Bytes())
			line.AppendString(" ")
		}
		callerBuf.Free()
	}

	// Message
	if enc.MessageKey != "" {} // No "msg=" prefix for console usually
	line.AppendString(ent.Message)

	// Remaining structured fields
	for _, f := range remainingFields {
		line.AppendString(" ")
		line.AppendString(f.Key)
		line.AppendString("=")
		// Simple value formatting for console.
		switch f.Type {
		case zapcore.StringType:
			if strings.Contains(f.String, " ") || f.String == "" { fmt.Fprintf(line, "%q", f.String)
			} else { line.AppendString(f.String) }
		case zapcore.ErrorType:
			if f.Interface != nil { fmt.Fprintf(line, "%q", f.Interface.(error).Error())
			} else { line.AppendString("nil")}
		case zapcore.BoolType:
			line.AppendBool(f.Integer == 1)
		case zapcore.Int8Type, zapcore.Int16Type, zapcore.Int32Type, zapcore.Int64Type:
			line.AppendInt(f.Integer)
		case zapcore.Uint8Type, zapcore.Uint16Type, zapcore.Uint32Type, zapcore.Uint64Type, zapcore.UintptrType:
			line.AppendUint(f.Integer) // f.Integer holds uint values as well for these types
		case zapcore.Float32Type:
			line.AppendFloat(float64(f.Interface.(float32)), 32) // Use Interface for float
		case zapcore.Float64Type:
			line.AppendFloat(f.Interface.(float64), 64) // Use Interface for float
		default:
			// For other types, use standard Go formatting.
			fmt.Fprintf(line, "%v", f.Interface)
		}
	}

	line.AppendString(enc.LineEnding)
	return line, nil
}


// tempEncoder is a minimal encoder for capturing parts like caller.
// It's used to pass to Zap's EncodeCaller function.
type tempEncoder struct {
    buf *buffer.Buffer
    zapcore.EncoderConfig
}
func (t *tempEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error { return nil }
func (t *tempEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error { return nil }
func (t *tempEncoder) AddBinary(key string, value []byte) {}
func (t *tempEncoder) AddByteString(key string, value []byte) { t.AppendByteString(value) } // Key is ignored for Append
func (t *tempEncoder) AddBool(key string, value bool) { t.AppendBool(value) }
func (t *tempEncoder) AddComplex128(key string, value complex128) { t.AppendComplex128(value) }
func (t *tempEncoder) AddComplex64(key string, value complex64) { t.AppendComplex64(value) }
func (t *tempEncoder) AddDuration(key string, value time.Duration) { t.AppendDuration(value) }
func (t *tempEncoder) AddFloat64(key string, value float64) { t.AppendFloat64(value) }
func (t *tempEncoder) AddFloat32(key string, value float32) { t.AppendFloat32(value) }
func (t *tempEncoder) AddInt(key string, value int) { t.AppendInt(value) }
func (t *tempEncoder) AddInt64(key string, value int64) { t.AppendInt64(value) }
func (t *tempEncoder) AddInt32(key string, value int32) { t.AppendInt32(value) }
func (t *tempEncoder) AddInt16(key string, value int16) { t.AppendInt16(value) }
func (t *tempEncoder) AddInt8(key string, value int8) { t.AppendInt8(value) }
func (t *tempEncoder) AddString(key, val string) { if key != "" {t.buf.AppendString(key); t.buf.AppendString("=")}; t.buf.AppendString(val)} // Used by EncodeCaller directly
func (t *tempEncoder) AddTime(key string, value time.Time) { t.AppendTime(value) }
func (t *tempEncoder) AddUint(key string, value uint) { t.AppendUint(value) }
func (t *tempEncoder) AddUint64(key string, value uint64){ t.AppendUint64(value) }
func (t *tempEncoder) AddUint32(key string, value uint32){ t.AppendUint32(value) }
func (t *tempEncoder) AddUint16(key string, value uint16){ t.AppendUint16(value) }
func (t *tempEncoder) AddUint8(key string, value uint8){ t.AppendUint8(value) }
func (t *tempEncoder) AddUintptr(key string, v uintptr){}
func (t *tempEncoder) AddReflected(k string, i interface{})error{return nil}
func (t *tempEncoder) OpenNamespace(key string) {}
func (t *tempEncoder) Clone() zapcore.Encoder { return t } // Simplified
func (t *tempEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) { return t.buf, nil }
func (t *tempEncoder) AppendArray(zapcore.ArrayMarshaler) error { return nil }
func (t *tempEncoder) AppendObject(zapcore.ObjectMarshaler) error { return nil }
func (t *tempEncoder) AppendBool(v bool) { t.buf.AppendBool(v) }
func (t *tempEncoder) AppendByteString(v []byte) { t.buf.AppendString(string(v)) }
func (t *tempEncoder) AppendBinary(v []byte) { t.buf.AppendString(string(v)) }
func (t *tempEncoder) AppendComplex128(v complex128) { t.buf.AppendString(fmt.Sprintf("%v", v)) }
func (t *tempEncoder) AppendComplex64(v complex64) { t.buf.AppendString(fmt.Sprintf("%v", v)) }
func (t *tempEncoder) AppendDuration(v time.Duration) { t.buf.AppendString(v.String()) }
func (t *tempEncoder) AppendFloat64(v float64) { t.buf.AppendFloat(v, 64) }
func (t *tempEncoder) AppendFloat32(v float32) { t.buf.AppendFloat(float64(v), 32) }
func (t *tempEncoder) AppendInt(v int) { t.buf.AppendInt(int64(v)) }
func (t *tempEncoder) AppendInt64(v int64) { t.buf.AppendInt(v) }
func (t *tempEncoder) AppendInt32(v int32) { t.buf.AppendInt(int64(v)) }
func (t *tempEncoder) AppendInt16(v int16) { t.buf.AppendInt(int64(v)) }
func (t *tempEncoder) AppendInt8(v int8) { t.buf.AppendInt(int64(v)) }
func (t *tempEncoder) AppendString(v string) { t.buf.AppendString(v) }
func (t *tempEncoder) AppendTime(v time.Time) { t.buf.AppendTime(v, t.EncoderConfig.EncodeTime.Layout()) } // Use layout from config
func (t *tempEncoder) AppendUint(v uint) { t.buf.AppendUint(uint64(v)) }
func (t *tempEncoder) AppendUint64(v uint64) { t.buf.AppendUint(v) }
func (t *tempEncoder) AppendUint32(v uint32) { t.buf.AppendUint(uint64(v)) }
func (t *tempEncoder) AppendUint16(v uint16) { t.buf.AppendUint(uint64(v)) }
func (t *tempEncoder) AppendUint8(v uint8) { t.buf.AppendUint(uint64(v)) }
func (t *tempEncoder) AppendUintptr(v uintptr) {}


// levelToColor and levelToColorZap (implement actual color logic)
func levelToColor(level Level, message string) string {
	switch level {
	case DebugLevel: return colorMagenta + message + colorReset
	case InfoLevel: return message // Or colorBlue + message + colorReset
	case SuccessLevel: return colorGreen + message + colorReset
	case WarnLevel: return colorYellow + message + colorReset
	case ErrorLevel: return colorRed + message + colorReset
	case FailLevel, FatalLevel: return colorRed + message + colorReset
	case PanicLevel: return colorCyan + message + colorReset
	default: return message
	}
}
func levelToColorZap(level zapcore.Level, message string) string {
	switch level {
	case zapcore.DebugLevel: return colorMagenta + message + colorReset
	case zapcore.InfoLevel: return message
	case zapcore.WarnLevel: return colorYellow + message + colorReset
	case zapcore.ErrorLevel: return colorRed + message + colorReset
	case zapcore.DPanicLevel, zapcore.PanicLevel: return colorCyan + message + colorReset
	case zapcore.FatalLevel: return colorRed + message + colorReset
	default: return message
	}
}
