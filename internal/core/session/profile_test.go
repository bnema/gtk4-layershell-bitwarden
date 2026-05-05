package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPINProfileCreateAndVerify(t *testing.T) {
	ref := AccountRef{Email: "User@Example.com", ServerURL: "https://vault.bitwarden.com/"}
	profile, err := NewPINProfile(ref, "acct-1", "1234", time.Now())
	require.NoError(t, err)

	require.Equal(t, PINProfileVersion, profile.Version)
	require.Equal(t, "acct-1", profile.AccountID)
	require.Equal(t, "user@example.com", profile.Email)
	require.Equal(t, "https://vault.bitwarden.com", profile.ServerURL)
	require.Len(t, profile.VerifierSalt, pinVerifierSaltSize)
	require.Len(t, profile.VerifierHash, pinVerifierHashSize)
	require.Len(t, profile.EnvelopeKey, EnvelopeKeySize)
	require.NoError(t, profile.Validate(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}))
	require.True(t, profile.VerifyPIN("1234"))
	require.True(t, profile.VerifyPIN(" 1234 "))
	require.False(t, profile.VerifyPIN("9999"))
}

func TestPINProfileRejectsShortPIN(t *testing.T) {
	_, err := NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "123", time.Now())
	require.Error(t, err)
	require.Contains(t, err.Error(), "PIN must be at least")

	// Whitespace-only PIN should also be rejected
	_, err = NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "  ", time.Now())
	require.Error(t, err)
	require.Contains(t, err.Error(), "PIN must be at least")
}

func TestPINProfileValidateAccountMismatch(t *testing.T) {
	profile, err := NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "1234", time.Now())
	require.NoError(t, err)
	require.ErrorIs(t, profile.Validate(AccountRef{Email: "other@example.com", ServerURL: "https://vault.bitwarden.com"}), ErrAccountMismatch)
}

func TestPINProfileValidateServerURLMismatch(t *testing.T) {
	profile, err := NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "1234", time.Now())
	require.NoError(t, err)
	require.ErrorIs(t, profile.Validate(AccountRef{Email: "user@example.com", ServerURL: "https://other.example.com"}), ErrAccountMismatch)
}

func TestPINProfileCloneDoesNotAliasVerifierSlices(t *testing.T) {
	profile, err := NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "1234", time.Now())
	require.NoError(t, err)
	clone := profile.Clone()

	// Mutate original slices
	profile.VerifierSalt[0] ^= 0xff
	profile.VerifierHash[0] ^= 0xff
	profile.EnvelopeKey[0] ^= 0xff

	// Clone should still verify correctly
	require.True(t, clone.VerifyPIN("1234"))
	require.False(t, clone.VerifyPIN("9999"))
}

func TestPINProfileCloneEnvelopeKeyNotAliased(t *testing.T) {
	profile, err := NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "1234", time.Now())
	require.NoError(t, err)
	clone := profile.Clone()

	originalKey := make([]byte, len(profile.EnvelopeKey))
	copy(originalKey, profile.EnvelopeKey)

	// Mutate original EnvelopeKey
	profile.EnvelopeKey[0] ^= 0xff

	// Clone's EnvelopeKey should be unchanged
	require.Equal(t, originalKey, clone.EnvelopeKey)
}

func TestPINProfileVerifyFailsForInvalidProfile(t *testing.T) {
	// Empty profile should return false, not panic.
	var p PINProfile
	require.False(t, p.VerifyPIN("1234"))
}

func TestPINProfileValidateRejectsInvalidMaterial(t *testing.T) {
	ref := AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}
	profile, err := NewPINProfile(ref, "acct-1", "1234", time.Now())
	require.NoError(t, err)

	materialTests := map[string]func(PINProfile) PINProfile{
		"version":           func(p PINProfile) PINProfile { p.Version = 0; return p },
		"kdf":               func(p PINProfile) PINProfile { p.KDF = "pbkdf2"; return p },
		"kdf time":          func(p PINProfile) PINProfile { p.KDFTime = 1; return p },
		"kdf memory":        func(p PINProfile) PINProfile { p.KDFMemory = 1024; return p },
		"kdf threads":       func(p PINProfile) PINProfile { p.KDFThreads = 1; return p },
		"salt size":         func(p PINProfile) PINProfile { p.VerifierSalt = p.VerifierSalt[:1]; return p },
		"hash size":         func(p PINProfile) PINProfile { p.VerifierHash = p.VerifierHash[:1]; return p },
		"envelope key size": func(p PINProfile) PINProfile { p.EnvelopeKey = p.EnvelopeKey[:1]; return p },
	}

	for name, mutate := range materialTests {
		t.Run(name, func(t *testing.T) {
			invalid := mutate(profile.Clone())
			require.ErrorIs(t, invalid.Validate(ref), ErrInvalidPINProfile)
			require.False(t, invalid.VerifyPIN("1234"))
		})
	}

	metadataTests := map[string]func(PINProfile) PINProfile{
		"created at": func(p PINProfile) PINProfile { p.CreatedAt = time.Time{}; return p },
		"updated at": func(p PINProfile) PINProfile { p.UpdatedAt = time.Time{}; return p },
	}

	for name, mutate := range metadataTests {
		t.Run(name, func(t *testing.T) {
			invalid := mutate(profile.Clone())
			require.ErrorIs(t, invalid.Validate(ref), ErrInvalidPINProfile)
		})
	}
}

func TestPINProfileCloseClearsSecretSlices(t *testing.T) {
	profile, err := NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "1234", time.Now())
	require.NoError(t, err)

	profile.Close()

	require.Nil(t, profile.VerifierSalt)
	require.Nil(t, profile.VerifierHash)
	require.Nil(t, profile.EnvelopeKey)
}

func TestPINProfileVerifyFailsForShortPIN(t *testing.T) {
	profile, err := NewPINProfile(AccountRef{Email: "user@example.com", ServerURL: "https://vault.bitwarden.com"}, "acct-1", "1234", time.Now())
	require.NoError(t, err)

	require.False(t, profile.VerifyPIN("12"))
	require.False(t, profile.VerifyPIN(""))
	require.False(t, profile.VerifyPIN("   "))
}
