//go:build !wasip1

package log

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Note: Actual WASM logging requires WASM runtime with host functions.
// These tests focus on slog types and log record handling that SDK consumers will use.
// WasmLogHandler is only available in wasip1 builds.

func TestLogRecord_Attributes(t *testing.T) {
	// Test that log records can contain various attribute types
	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{"string attribute", "message", "test message"},
		{"int attribute", "count", 42},
		{"bool attribute", "success", true},
		{"float attribute", "duration", 1.23},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create attribute
			var attr slog.Attr
			switch v := tt.value.(type) {
			case string:
				attr = slog.String(tt.key, v)
			case int:
				attr = slog.Int(tt.key, v)
			case bool:
				attr = slog.Bool(tt.key, v)
			case float64:
				attr = slog.Float64(tt.key, v)
			}

			// Verify attribute creation
			assert.Equal(t, tt.key, attr.Key)
			assert.NotNil(t, attr.Value)
		})
	}
}

func TestLogLevels(t *testing.T) {
	// Document available log levels
	levels := []struct {
		level slog.Level
		name  string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
	}

	for _, l := range levels {
		t.Run(l.name, func(t *testing.T) {
			// Verify level string representation
			assert.Equal(t, l.name, l.level.String())
		})
	}
}

func TestLogRecord_Construction(t *testing.T) {
	// Test creating log records with various configurations
	now := time.Now()

	tests := []struct {
		name    string
		level   slog.Level
		message string
		attrs   []slog.Attr
	}{
		{
			name:    "simple info log",
			level:   slog.LevelInfo,
			message: "operation completed",
			attrs:   nil,
		},
		{
			name:    "error with attributes",
			level:   slog.LevelError,
			message: "operation failed",
			attrs: []slog.Attr{
				slog.String("error", "connection refused"),
				slog.Int("retry_count", 3),
			},
		},
		{
			name:    "debug with multiple attrs",
			level:   slog.LevelDebug,
			message: "processing request",
			attrs: []slog.Attr{
				slog.String("request_id", "req-123"),
				slog.String("user_id", "user-456"),
				slog.Int("duration_ms", 42),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := slog.NewRecord(now, tt.level, tt.message, 0)
			record.AddAttrs(tt.attrs...)

			assert.Equal(t, tt.level, record.Level)
			assert.Equal(t, tt.message, record.Message)
			assert.Equal(t, len(tt.attrs), record.NumAttrs())
		})
	}
}

// Integration test notes for WASM environment (requires wasip1 build):
//
// WasmLogHandler is only available in WASM builds (//go:build wasip1).
// Integration tests for WASM logging should be in a separate _wasip1_test.go file.
//
// Suggested WASM tests:
// - TestWasmLogHandler_Handle: Test actual log message sending to host
// - TestWasmLogHandler_Enabled: Test that all levels are enabled by default
// - TestWasmLogHandler_Attributes: Test attribute serialization to host
// - TestWasmLogHandler_WithAttrs: Test WithAttrs creates new handler
// - TestWasmLogHandler_WithGroup: Test WithGroup creates new handler
// - TestWasmLogHandler_Context: Test context propagation in logging
// - TestWasmLogHandler_Performance: Test logging performance characteristics
//
// The Handle method (log.go:48-72) requires WASM host function:
// - host_log_message(messagePacked uint64)
