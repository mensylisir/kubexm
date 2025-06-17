package logger

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	// ANSI color codes for console output
	colorRed     = "[31m" // Red for Error, Fail, Fatal
	colorGreen   = "[32m" // Green for Success
	colorYellow  = "[33m" // Yellow for Warn
	colorBlue    = "[34m" // Blue (can be used for Info if desired, currently Info is plain)
	colorMagenta = "[35m" // Magenta for Debug
	colorCyan    = "[36m" // Cyan for Panic
	colorReset   = "[0m"  // Resets color to default
)

// _bufferPool is a pool of zap.Buffer objects to reduce allocations.
var _bufferPool = buffer.NewPool()
// _encoderPool is a pool for colorConsoleEncoder instances to reduce allocations.
// Note: Proper Reset method for pooled objects is crucial if they have complex state.
// For this encoder, the critical parts are re-initialized or passed in (like EncoderConfig).
var _encoderPool = sync.Pool{New: func() interface{} {
	return &colorConsoleEncoder{}
}}


// getEncoder retrieves an encoder from the pool.
// Currently not used directly by NewColorConsoleEncoder/NewPlainTextConsoleEncoder,
// but could be integrated if encoder instances themselves were pooled more aggressively.
/*
func getEncoder() *colorConsoleEncoder {
	return _encoderPool.Get().(*colorConsoleEncoder)
}

func putEncoder(enc *colorConsoleEncoder) {
	// Reset fields to defaults before putting back to pool
	enc.EncoderConfig = zapcore.EncoderConfig{}
	enc.buf = nil // Buffer should be obtained from _bufferPool when needed
	enc.spaced = false
	enc.colors = false
	// loggerOpts and levelStrings are typically configured once and might not need resetting
	// if the encoder is reconfigured upon Get, or if they are shared/immutable.
	_encoderPool.Put(enc)
}
*/


// colorConsoleEncoder implements zapcore.Encoder. It's designed to produce
// human-readable, colored (optional), level-prefixed log output for the console.
// It handles custom log levels like SUCCESS and FAIL by checking for a "customlevel"
// field in the log entry, which is expected to be added by the wrapper Logger's
// logWithCustomLevel method.
type colorConsoleEncoder struct {
	zapcore.EncoderConfig                  // Embeds Zap's encoder configuration.
	buf                   *buffer.Buffer   // A temporary buffer used for various encoding tasks.
	spaced                bool             // Whether to add spaces between log elements (e.g., timestamp, level, message).
	colors                bool             // Whether to use ANSI color codes in the output.
	loggerOpts            Options          // Reference to logger options for things like TimestampFormat.
	levelStrings          map[Level]string // Cache for pre-formatted and potentially colored [LEVEL] strings.
}

// NewColorConsoleEncoder creates a console encoder that formats log entries with
// ANSI color codes for different log levels, making console output more readable.
func NewColorConsoleEncoder(cfg zapcore.EncoderConfig, opts Options) zapcore.Encoder {
	return &colorConsoleEncoder{
		EncoderConfig: cfg,
		buf:           _bufferPool.Get(), // Get a buffer from the pool for temporary use.
		spaced:        true,              // Default to spaced formatting.
		colors:        true,              // Enable colors.
		loggerOpts:    opts,
		levelStrings:  cacheLevelStrings(true, opts.ColorConsole), // Pre-cache colored level strings.
	}
}

// NewPlainTextConsoleEncoder creates a console encoder that formats log entries
// without ANSI color codes. This is suitable for environments that don't support
// ANSI colors or when plain text logs are preferred (e.g., piping to a file).
func NewPlainTextConsoleEncoder(cfg zapcore.EncoderConfig, opts Options) zapcore.Encoder {
	return &colorConsoleEncoder{
		EncoderConfig: cfg,
		buf:           _bufferPool.Get(),
		spaced:        true,
		colors:        false, // Disable colors.
		loggerOpts:    opts,
		levelStrings:  cacheLevelStrings(false, false), // Pre-cache plain level strings.
	}
}

// cacheLevelStrings pre-formats and caches the string representation of each log level (e.g., "[INFO]", "[SUCCESS]").
// If coloring is enabled via the `useColor` parameter (derived from `opts.ColorConsole`),
// it applies the appropriate ANSI color codes to the level strings.
// This caching is performed once at encoder creation to improve logging performance.
func cacheLevelStrings(color bool, useColor bool) map[Level]string {
	m := make(map[Level]string)
	levels := []Level{DebugLevel, InfoLevel, SuccessLevel, WarnLevel, ErrorLevel, FailLevel, PanicLevel, FatalLevel}
	for _, l := range levels {
		str := fmt.Sprintf("[%s]", l.CapitalString()) // Format as [LEVEL]
		if color && useColor { // Check both generic color flag and specific useColor for this cache
			m[l] = levelToColor(l, str)
		} else {
			m[l] = str
		}
	}
	return m
}

// --- Implementation of zapcore.Encoder interface ---
// Most AddXxx methods are for structured logging fields. For simple console output,
// they append key=value to the internal buffer `enc.buf`.
// The main formatting logic is in EncodeEntry.

func (enc *colorConsoleEncoder) AddArray(key string, arr zapcore.ArrayMarshaler) error {
	enc.addKey(key) // Adds "key="
	return enc.AppendArray(arr)
}
func (enc *colorConsoleEncoder) AddObject(key string, obj zapcore.ObjectMarshaler) error {
	enc.addKey(key)
	return enc.AppendObject(obj)
}
func (enc *colorConsoleEncoder) AddBinary(key string, val []byte)          { enc.AddByteString(key, val) } // Treat binary as string for console
func (enc *colorConsoleEncoder) AddByteString(key string, val []byte)    { enc.addKeyVal(key, string(val)) }
func (enc *colorConsoleEncoder) AddBool(key string, val bool)              { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddComplex128(key string, val complex128)  { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddComplex64(key string, val complex64)    { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddDuration(key string, val time.Duration) { enc.addKeyVal(key, val.String()) }
func (enc *colorConsoleEncoder) AddFloat64(key string, val float64)        { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddFloat32(key string, val float32)        { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddInt(key string, val int)                { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddInt64(key string, val int64)            { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddInt32(key string, val int32)            { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddInt16(key string, val int16)            { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddInt8(key string, val int8)              { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddString(key, val string)                 { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddTime(key string, val time.Time)         { enc.addKeyVal(key, val.Format(enc.loggerOpts.TimestampFormat)) }
func (enc *colorConsoleEncoder) AddUint(key string, val uint)              { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddUint64(key string, val uint64)          { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddUint32(key string, val uint32)          { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddUint16(key string, val uint16)          { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddUint8(key string, val uint8)            { enc.addKeyVal(key, val) }
func (enc *colorConsoleEncoder) AddUintptr(key string, val uintptr)        { /* No-op for console, or hex: enc.addKeyVal(key, fmt.Sprintf("0x%x", val)) */ }
func (enc *colorConsoleEncoder) OpenNamespace(key string)                  { /* TODO: Handle namespaces if needed for console, e.g., prefixing keys. */ }


// AppendXxx methods are used when encoding arrays or objects.
func (enc *colorConsoleEncoder) AppendFloat64(val float64)                 { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendFloat32(val float32)                 { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendInt(val int)                         { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendInt64(val int64)                     { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendInt32(val int32)                     { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendInt16(val int16)                     { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendInt8(val int8)                       { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendString(val string)                   { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendUint(val uint)                       { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendUint64(val uint64)                   { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendUint32(val uint32)                   { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendUint16(val uint16)                   { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendUint8(val uint8)                     { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendUintptr(val uintptr)                 { /* No-op */ }
func (enc *colorConsoleEncoder) AppendBool(val bool)                       { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendComplex128(val complex128)           { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendComplex64(val complex64)             { enc.appendVal(val) }
func (enc *colorConsoleEncoder) AppendDuration(val time.Duration)          { enc.appendVal(val.String())}
func (enc *colorConsoleEncoder) AppendTime(val time.Time)                  { enc.appendVal(val.Format(enc.loggerOpts.TimestampFormat))}
func (enc *colorConsoleEncoder) AppendByteString(val []byte)               { enc.AppendString(string(val)) } // Treat as string for console
func (enc *colorConsoleEncoder) AppendBinary(val []byte)                   { enc.AppendString(string(val)) } // Treat as string for console
func (enc *colorConsoleEncoder) AppendArray(arr zapcore.ArrayMarshaler) error {
	enc.buf.AppendByte('[')
	err := arr.MarshalLogArray(enc) // Marshaler calls AppendXxx methods of this encoder
	enc.buf.AppendByte(']')
	return err
}
func (enc *colorConsoleEncoder) AppendObject(obj zapcore.ObjectMarshaler) error {
	enc.buf.AppendByte('{')
	err := obj.MarshalLogObject(enc) // Marshaler calls AddXxx methods of this encoder
	enc.buf.AppendByte('}')
	return err
}

// addKey adds a key to the internal buffer, followed by "=".
func (enc *colorConsoleEncoder) addKey(key string) {
	enc.addElementSeparator() // Add a space if needed before this key-value pair
	enc.buf.AppendString(key)
	enc.buf.AppendByte('=')
}

// addKeyVal adds a key-value pair to the internal buffer.
func (enc *colorConsoleEncoder) addKeyVal(key string, val interface{}) {
	enc.addKey(key)
	enc.appendVal(val)
}

// appendVal appends a value to the internal buffer using simple formatting.
func (enc *colorConsoleEncoder) appendVal(val interface{}) {
	switch v := val.(type) {
	case string:
		// Quote strings if they contain spaces or are empty, for clarity in console.
		if strings.Contains(v, " ") || v == "" {
			fmt.Fprintf(enc.buf, "%q", v)
		} else {
			enc.buf.AppendString(v)
		}
	case bool:
		enc.buf.AppendBool(v)
	// Handle various integer types consistently.
	case int: fmt.Fprintf(enc.buf, "%d", v)
	case int8: fmt.Fprintf(enc.buf, "%d", v)
	case int16: fmt.Fprintf(enc.buf, "%d", v)
	case int32: fmt.Fprintf(enc.buf, "%d", v)
	case int64: fmt.Fprintf(enc.buf, "%d", v)
	case uint: fmt.Fprintf(enc.buf, "%d", v)
	case uint8: fmt.Fprintf(enc.buf, "%d", v)
	case uint16: fmt.Fprintf(enc.buf, "%d", v)
	case uint32: fmt.Fprintf(enc.buf, "%d", v)
	case uint64: fmt.Fprintf(enc.buf, "%d", v)
	case float32:
		enc.buf.AppendFloat(float64(v), 32)
	case float64:
		enc.buf.AppendFloat(v, 64)
	default:
		// For other types, use standard Go formatting.
		fmt.Fprintf(enc.buf, "%v", v)
	}
}

// addElementSeparator adds a space to the buffer if `spaced` is true
// and the buffer already contains content. This is used to separate log elements.
func (enc *colorConsoleEncoder) addElementSeparator() {
	if enc.spaced && enc.buf.Len() > 0 {
		enc.buf.AppendByte(' ')
	}
}

// Clone creates a copy of the encoder.
func (enc *colorConsoleEncoder) Clone() zapcore.Encoder {
	// Create a new encoder instance for the clone.
	// Get a new buffer from the pool for the clone.
	clone := &colorConsoleEncoder{
		EncoderConfig: enc.EncoderConfig,
		buf:           _bufferPool.Get(),
		spaced:        enc.spaced,
		colors:        enc.colors,
		loggerOpts:    enc.loggerOpts,
		levelStrings:  enc.levelStrings, // levelStrings map is read-only after creation, safe to share.
	}
	// If the original encoder's buffer has content (e.g., from `With` fields), copy it.
	if enc.buf != nil {
		clone.buf.Write(enc.buf.Bytes())
	}
	return clone
}


// EncodeEntry is the primary method responsible for formatting a single log entry.
// It constructs the log line by appending timestamp, level (with color/prefix),
// caller (if enabled), message, and any structured fields.
//
// For custom levels (SUCCESS, FAIL), it expects a "customlevel" field (added by
// the main Logger's logWithCustomLevel method) containing the original custom level string.
// This allows it to apply specific formatting (e.g., green for "[SUCCESS]").
// If "customlevel" is not found, it falls back to standard Zap level formatting.
func (enc *colorConsoleEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	// Get a new buffer from the pool for the final log line.
	// This buffer will be returned and eventually freed by Zap.
	line := _bufferPool.Get()

	// Timestamp
	if enc.TimeKey != "" && enc.EncodeTime != nil { // Check TimeKey as per Zap's convention
		line.AppendString(ent.Time.Format(enc.loggerOpts.TimestampFormat))
		line.AppendString(" ") // Separator after timestamp
	}

	// Level string (custom handling for color and prefix)
	customLevelStr := ""
	ourLevel := InfoLevel // Default, will be overridden if "customlevel" field exists.

	// Check for our "customlevel" field first. This field is added by the
	// Logger's logWithCustomLevel method and contains the original custom Level string
	// (e.g., "SUCCESS", "FAIL") before it was mapped to a zapcore.Level.
	for _, f := range fields {
		if f.Key == "customlevel" && f.Type == zapcore.StringType {
			levelStrValue := f.String // e.g., "SUCCESS"
			// Convert the string back to our Level type to use the cached colored string
			// This mapping ensures we get the correct entry from `enc.levelStrings`.
			switch strings.ToUpper(levelStrValue) {
			case "DEBUG": ourLevel = DebugLevel
			case "INFO": ourLevel = InfoLevel
			case "SUCCESS": ourLevel = SuccessLevel
			case "WARN": ourLevel = WarnLevel
			case "ERROR": ourLevel = ErrorLevel
			case "FAIL": ourLevel = FailLevel
			case "PANIC": ourLevel = PanicLevel
			case "FATAL": ourLevel = FatalLevel
			// No default needed here, `ourLevel` remains as initialized if no match.
			}
			customLevelStr = enc.levelStrings[ourLevel] // Use cached, potentially colored string.
			break
		}
	}

	// Fallback to Zap's level if customlevel field isn't present or doesn't match.
	if customLevelStr == "" {
		levelText := fmt.Sprintf("[%s]", strings.ToUpper(ent.Level.String()))
		if enc.colors {
			customLevelStr = levelToColorZap(ent.Level, levelText)
		} else {
			customLevelStr = levelText
		}
	}

	line.AppendString(customLevelStr)
	line.AppendString(" ") // Separator after level

	// Caller information (e.g., file:line)
	if ent.Caller.Defined && enc.CallerKey != "" { // Use configured CallerKey
		// Use a temporary buffer for caller encoding, as EncodeCaller might write complex data.
		tempCallerBuf := _bufferPool.Get()
		// zapcore.PrimitiveArrayEncoder is a simple encoder that can be used for caller.
		// It doesn't add extra quotes or formatting that NewReflectedEncoder might.
		enc.EncodeCaller(ent.Caller, zapcore.NewPrimitiveArrayEncoder(tempCallerBuf))
		if tempCallerBuf.Len() > 0 {
			line.AppendString(tempCallerBuf.String())
			line.AppendString(" ") // Separator after caller
		}
		tempCallerBuf.Free()
	}

	// Main log message
	if enc.MessageKey != "" { // Check MessageKey as per Zap's convention
		// Typically, no "msg=" prefix for console logs, just the message.
	}
	line.AppendString(ent.Message)

	// Handle structured fields from `With` (already in enc.buf) and `Info/Errorw` (in `fields` argument)

    // Fields from `With(...)` are accumulated in `enc.buf` by calls to AddXxx methods.
    // This buffer should be appended and then reset for the next log entry.
    if enc.buf != nil && enc.buf.Len() > 0 {
        line.AppendString(" ") // Separator for these fields
        line.Write(enc.buf.Bytes())
        enc.buf.Reset() // Reset the persistent buffer for the next entry
    }

	// Fields passed directly to the logging method (e.g., Infow, Errorw)
	// These are distinct from fields added via `With`.
	tempFieldsBuf := _bufferPool.Get() // Use a temporary buffer for these fields
	defer tempFieldsBuf.Free()

	for _, f := range fields {
		// Skip "customlevel" and "customlevel_num" as they are already handled or internal.
		if f.Key == "customlevel" || f.Key == "customlevel_num" {
			continue
		}
		if tempFieldsBuf.Len() > 0 {
			tempFieldsBuf.AppendString(" ") // Separator between key-value pairs
		}
		tempFieldsBuf.AppendString(f.Key)
		tempFieldsBuf.AppendString("=")
		// Simple value formatting for console.
		switch f.Type {
		case zapcore.StringType:
			// Quote strings if they contain spaces or are empty for clarity.
			if strings.Contains(f.String, " ") || f.String == "" {
				fmt.Fprintf(tempFieldsBuf, "%q", f.String)
			} else {
				tempFieldsBuf.AppendString(f.String)
			}
		case zapcore.ErrorType:
			// Format error using its Error() method, and quote it.
			if err, ok := f.Interface.(error); ok {
				fmt.Fprintf(tempFieldsBuf, "%q", err.Error())
			} else {
				// Should not happen if type is ErrorType
				fmt.Fprintf(tempFieldsBuf, "%q", f.Interface)
			}
        case zapcore.BoolType:
             tempFieldsBuf.AppendBool(f.Integer == 1)
		default:
			// For other types, use simple %v formatting.
			// Note: f.Integer and f.Interface hold the value for different types.
			// A more robust handling would switch on f.Type for all cases.
			fmt.Fprintf(tempFieldsBuf, "%v", f.Interface)
		}
	}
	if tempFieldsBuf.Len() > 0 {
		line.AppendString(" ") // Separator before appending these fields
		line.Write(tempFieldsBuf.Bytes())
	}

	line.AppendString(enc.LineEnding) // Add newline character.
	return line, nil
}

// levelToColor applies ANSI color codes to a message string based on the custom Level.
// This is used for formatting the [LEVEL] prefix.
func levelToColor(level Level, message string) string {
	switch level {
	case DebugLevel:
		return colorMagenta + message + colorReset
	case InfoLevel:
		return message // No color for INFO, or use blue: colorBlue + message + colorReset
	case SuccessLevel:
		return colorGreen + message + colorReset
	case WarnLevel:
		return colorYellow + message + colorReset
	case ErrorLevel:
		return colorRed + message + colorReset
	case FailLevel, FatalLevel: // FailLevel uses FatalLevel's representation for color
		return colorRed + message + colorReset
	case PanicLevel:
		return colorCyan + message + colorReset
	default:
		return message // Default to no color if level is unknown
	}
}

// levelToColorZap applies ANSI color codes based on zapcore.Level.
// This is used as a fallback if the "customlevel" field (our custom level system)
// is not present in the log entry, ensuring standard Zap levels are still colored.
func levelToColorZap(level zapcore.Level, message string) string {
	switch level {
	case zapcore.DebugLevel:
		return colorMagenta + message + colorReset
	case zapcore.InfoLevel:
		return message // Or blue: colorBlue + message + colorReset
	case zapcore.WarnLevel:
		return colorYellow + message + colorReset
	case zapcore.ErrorLevel:
		return colorRed + message + colorReset
	// DPanicLevel, PanicLevel, and FatalLevel are all critical.
	case zapcore.DPanicLevel:
		return colorCyan + message + colorReset // Often associated with Panic
	case zapcore.PanicLevel:
		return colorCyan + message + colorReset
	case zapcore.FatalLevel:
		return colorRed + message + colorReset
	default:
		return message // Default to no color
	}
}
