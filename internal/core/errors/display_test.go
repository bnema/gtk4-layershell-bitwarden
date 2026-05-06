package errors

import (
	"fmt"
	"testing"
)

func TestShortMessageClassifiesTypedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "crypto", err: &Error{Kind: KindCrypto, Op: "remote.Sync", Code: "decryption_failed", Message: "vault decrypt failed"}, want: ShortDecryptFailed},
		{name: "auth", err: fmt.Errorf("wrap: %w", ErrUnauthenticated), want: ShortAuthFailed},
		{name: "locked", err: &Error{Kind: KindLocked, Message: "locked"}, want: ShortLocked},
		{name: "network", err: &Error{Kind: KindNetwork, Message: "network"}, want: ShortNetworkFailed},
		{name: "unsupported", err: &Error{Kind: KindUnsupported, Message: "unsupported"}, want: ShortUnsupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShortMessage(tt.err); got != tt.want {
				t.Fatalf("ShortMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShortErrorTextClassifiesRawBackendRegression(t *testing.T) {
	raw := "remote sync failed: bitwarden: decryption failed op=crypto.DecryptCipher code=decryption_failed message=failed to decrypt cipher field"

	if got := ShortErrorText(raw); got != ShortDecryptFailed {
		t.Fatalf("ShortErrorText() = %q, want %q", got, ShortDecryptFailed)
	}
}

func TestShortErrorTextGenericForUnknownLongText(t *testing.T) {
	raw := "some very detailed backend failure with op=remote.Sync code=unknown and implementation details"

	if got := ShortErrorText(raw); got != ShortGenericError {
		t.Fatalf("ShortErrorText() = %q, want %q", got, ShortGenericError)
	}
}
