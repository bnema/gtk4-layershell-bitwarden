package errors

import (
	"context"
	stderrors "errors"
	"strings"
)

const (
	ShortGenericError    = "Something went wrong"
	ShortSyncFailed      = "Sync failed"
	ShortDecryptFailed   = "Vault could not be decrypted"
	ShortNetworkFailed   = "Network unavailable"
	ShortAuthFailed      = "Please sign in again"
	ShortLocked          = "Vault is locked"
	ShortNotFound        = "Item not found"
	ShortConflict        = "Conflict detected"
	ShortUnsupported     = "Unsupported vault item"
	ShortTemporaryFailed = "Temporary service issue"
)

// ShortMessage returns a brief, frontend-safe message for err.
// It never returns raw backend error text, because errors may contain detailed
// operation context that belongs in logs, not in constrained UI surfaces.
func ShortMessage(err error) string {
	if err == nil {
		return ""
	}
	if stderrors.Is(err, context.Canceled) {
		return "Canceled"
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return ShortTemporaryFailed
	}

	var appErr *Error
	if stderrors.As(err, &appErr) {
		switch appErr.Kind {
		case KindCrypto:
			return ShortDecryptFailed
		case KindUnauthenticated:
			return ShortAuthFailed
		case KindLocked:
			return ShortLocked
		case KindNotFound:
			return ShortNotFound
		case KindConflict:
			return ShortConflict
		case KindNetwork:
			return ShortNetworkFailed
		case KindTemporary:
			return ShortTemporaryFailed
		case KindUnsupported:
			return ShortUnsupported
		}
	}

	return ShortErrorText(err.Error())
}

// ShortErrorText classifies a backend error string defensively for legacy
// event paths that only carry text. Prefer ShortMessage when the typed error is
// available.
func ShortErrorText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ShortGenericError
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "decrypt") || strings.Contains(lower, "decryption_failed") || strings.Contains(lower, "decryption failed"):
		return ShortDecryptFailed
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "unauthenticated") || strings.Contains(lower, "invalid_grant"):
		return ShortAuthFailed
	case strings.Contains(lower, "network") || strings.Contains(lower, "connection") || strings.Contains(lower, "timeout"):
		return ShortNetworkFailed
	case strings.Contains(lower, "locked"):
		return ShortLocked
	case strings.Contains(lower, "not found"):
		return ShortNotFound
	case strings.Contains(lower, "conflict"):
		return ShortConflict
	case strings.Contains(lower, "unsupported"):
		return ShortUnsupported
	case strings.Contains(lower, "sync failed") || strings.Contains(lower, "remote sync failed"):
		return ShortSyncFailed
	default:
		return ShortGenericError
	}
}
