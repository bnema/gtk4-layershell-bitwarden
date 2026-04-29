// Package logging provides a redacting slog-based Logger adapter.
package logging

import (
	"io"
	"log/slog"
	"strings"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/ports/out"
)

// Keys whose (lowercased) values are redacted.
var redactKeys = []string{"password", "token", "secret", "key", "auth", "ciphertext"}

// Adapter implements out.Logger by delegating to a *slog.Logger after
// redacting sensitive attribute values.
type Adapter struct {
	logger *slog.Logger
}

// New returns a new Adapter. If logger is nil, a discard logger is used.
func New(logger *slog.Logger) *Adapter {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Adapter{logger: logger}
}

func (a *Adapter) Debug(msg string, attrs ...any) { a.logger.Debug(msg, a.redact(attrs...)...) }
func (a *Adapter) Info(msg string, attrs ...any)  { a.logger.Info(msg, a.redact(attrs...)...) }
func (a *Adapter) Warn(msg string, attrs ...any)  { a.logger.Warn(msg, a.redact(attrs...)...) }
func (a *Adapter) Error(msg string, attrs ...any) { a.logger.Error(msg, a.redact(attrs...)...) }

// redact returns a copy of attrs with sensitive values replaced by
// "[REDACTED]". It recurses one level into slog.Group attrs.
func (a *Adapter) redact(args ...any) []any {
	result := make([]any, 0, len(args))
	for i := 0; i < len(args); {
		// Handle pre-formed slog.Attr (e.g. slog.Group).
		if attr, ok := args[i].(slog.Attr); ok {
			result = append(result, redactAttr(attr))
			i++
			continue
		}

		// Standard key-value pair.
		if i+1 >= len(args) {
			// Odd trailing element — pass through.
			result = append(result, args[i])
			i++
			continue
		}
		key, ok := args[i].(string)
		if !ok {
			result = append(result, args[i])
			i++
			continue
		}
		val := args[i+1]
		if shouldRedact(key) {
			result = append(result, key, "[REDACTED]")
		} else {
			result = append(result, key, redactValue(val))
		}
		i += 2
	}
	return result
}

// redactAttr returns attr with its value redacted if attr.Key matches, or
// recursing into groups.
func redactAttr(attr slog.Attr) slog.Attr {
	if shouldRedact(attr.Key) {
		return slog.Attr{Key: attr.Key, Value: slog.StringValue("[REDACTED]")}
	}
	if attr.Value.Kind() == slog.KindGroup {
		return slog.Attr{Key: attr.Key, Value: redactGroupValue(attr.Value)}
	}
	return attr
}

// redactGroupValue returns a copy of v (which must be a group) with sensitive
// child attrs redacted.
func redactGroupValue(v slog.Value) slog.Value {
	attrs := v.Group()
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactAttr(a)
	}
	return slog.GroupValue(redacted...)
}

// redactValue redacts val if it is a group, otherwise passes it through.
func redactValue(val any) any {
	if v, ok := val.(slog.Value); ok && v.Kind() == slog.KindGroup {
		return redactGroupValue(v)
	}
	return val
}

func shouldRedact(key string) bool {
	lower := strings.ToLower(key)
	for _, k := range redactKeys {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// compile-time check
var _ out.Logger = (*Adapter)(nil)
