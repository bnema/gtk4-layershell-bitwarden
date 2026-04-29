package logging

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

// capture returns a *slog.Logger that writes structured JSON to buf.
func capture(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		// Remove time and level for deterministic comparison.
		if a.Key == slog.TimeKey || a.Key == slog.LevelKey {
			return slog.Attr{}
		}
		return a
	}}))
}

func TestLogsNonSensitiveAttr(t *testing.T) {
	var buf bytes.Buffer
	logger := capture(&buf)
	adapter := New(logger)

	adapter.Info("hello", "user", "alice", "action", "login")

	output := buf.String()
	require.Contains(t, output, "hello")
	require.Contains(t, output, "alice")
	require.Contains(t, output, "login")
}

func TestRedactsSensitiveKeys(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"password", "hunter2"},
		{"token", "my-token"},
		{"secret", "my-secret"},
		{"api_key", "key-123"},
		{"auth", "basic-auth"},
		{"ciphertext", "encrypted-data"},
		{"access_token", "tok-xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			var buf bytes.Buffer
			logger := capture(&buf)
			adapter := New(logger)

			adapter.Info("test", tt.key, tt.value)

			output := buf.String()
			require.Contains(t, output, "[REDACTED]", "value should be redacted for key %q", tt.key)
			require.NotContains(t, output, tt.value, "raw value should not appear for key %q", tt.key)
		})
	}
}

func TestRedactsGroupAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := capture(&buf)
	adapter := New(logger)

	adapter.Info("group-test",
		slog.Group("credentials",
			"password", "hunter2",
			"username", "admin",
		),
	)

	output := buf.String()
	require.Contains(t, output, "[REDACTED]", "group password should be redacted")
	require.Contains(t, output, "admin", "non-sensitive group attr should remain")
	require.NotContains(t, output, "hunter2", "password value should be redacted in group")
}

func TestNilLoggerDoesNotPanic(t *testing.T) {
	adapter := New(nil)
	require.NotNil(t, adapter)

	require.NotPanics(t, func() {
		adapter.Debug("debug", "key", "val")
		adapter.Info("info", "key", "val")
		adapter.Warn("warn", "key", "val")
		adapter.Error("error", "key", "val")
	})
}
