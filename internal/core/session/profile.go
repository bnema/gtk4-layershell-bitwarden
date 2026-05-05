package session

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	// PINProfileVersion is the current PIN profile format version.
	PINProfileVersion = 1

	// MinPINLength is the minimum allowed PIN length after trimming spaces.
	MinPINLength = 4

	pinVerifierSaltSize = 16
	pinVerifierHashSize = 32

	pinVerifierKDF        = "argon2id"
	pinVerifierKDFTime    = 3
	pinVerifierKDFMemory  = 64 * 1024
	pinVerifierKDFThreads = 4

	// EnvelopeKeySize is the size of the high-entropy envelope wrapping key in bytes.
	EnvelopeKeySize = 32
)

// ErrInvalidPINProfile indicates a structurally invalid or unsupported PIN profile.
var ErrInvalidPINProfile = errors.New("invalid PIN profile")

// PINProfile holds persistent local PIN configuration for an account/server.
// It stores a verifier (not the raw PIN) and a high-entropy envelope wrapping key.
type PINProfile struct {
	Version int `json:"version"`

	AccountID string `json:"accountId,omitempty"`
	Email     string `json:"email"`
	ServerURL string `json:"serverUrl"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// KDF metadata for Argon2id verifier.
	KDF        string `json:"kdf"`
	KDFTime    uint32 `json:"kdfTime"`
	KDFMemory  uint32 `json:"kdfMemory"`
	KDFThreads uint8  `json:"kdfThreads"`

	// VerifierSalt is the per-profile random salt for Argon2id.
	VerifierSalt []byte `json:"verifierSalt"`
	// VerifierHash is the Argon2id-derived verifier hash of the PIN.
	VerifierHash []byte `json:"verifierHash"`

	// EnvelopeKey is a random high-entropy 32-byte key stored in Secret Service
	// and used to wrap/unwrap unlock envelopes. Never derived from the raw PIN.
	EnvelopeKey []byte `json:"envelopeKey"`
}

// NewPINProfile creates a new PINProfile for the given account reference, PIN,
// and current time. It generates a random verifier salt, derives the Argon2id
// verifier hash, and generates a random envelope wrapping key.
func NewPINProfile(ref AccountRef, accountID, pin string, now time.Time) (PINProfile, error) {
	pin = strings.TrimSpace(pin)
	if len(pin) < MinPINLength {
		return PINProfile{}, fmt.Errorf("PIN must be at least %d characters", MinPINLength)
	}

	norm := normalizeAccountRef(ref)

	salt := make([]byte, pinVerifierSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return PINProfile{}, fmt.Errorf("generating verifier salt: %w", err)
	}

	envKey := make([]byte, EnvelopeKeySize)
	if _, err := rand.Read(envKey); err != nil {
		return PINProfile{}, fmt.Errorf("generating envelope key: %w", err)
	}

	nowUTC := now.UTC()

	p := PINProfile{
		Version:   PINProfileVersion,
		AccountID: accountID,
		Email:     norm.Email,
		ServerURL: norm.ServerURL,

		CreatedAt: nowUTC,
		UpdatedAt: nowUTC,

		KDF:        pinVerifierKDF,
		KDFTime:    pinVerifierKDFTime,
		KDFMemory:  pinVerifierKDFMemory,
		KDFThreads: pinVerifierKDFThreads,

		VerifierSalt: salt,
		EnvelopeKey:  envKey,
	}

	// Derive verifier hash after setting up the profile so derive() can use
	// the KDF parameters and salt. Since VerifierHash is not assigned yet,
	// derive() uses pinVerifierHashSize.
	p.VerifierHash = p.derive(pin)

	return p, nil
}

// Clone returns a deep copy of the PINProfile with all byte slices independently
// allocated so mutations to the original do not affect the clone.
func (p PINProfile) Clone() PINProfile {
	c := p
	c.VerifierSalt = append([]byte(nil), p.VerifierSalt...)
	c.VerifierHash = append([]byte(nil), p.VerifierHash...)
	c.EnvelopeKey = append([]byte(nil), p.EnvelopeKey...)
	return c
}

// Close zeroes the backing arrays of secret slices and nils them.
func (p *PINProfile) Close() {
	clear(p.VerifierSalt)
	clear(p.VerifierHash)
	clear(p.EnvelopeKey)
	p.VerifierSalt = nil
	p.VerifierHash = nil
	p.EnvelopeKey = nil
}

// Validate checks structural validity of the profile and verifies that the
// account reference matches the profile's normalized email and server URL.
func (p PINProfile) Validate(ref AccountRef) error {
	if !p.validMaterial() || strings.TrimSpace(p.Email) == "" || strings.TrimSpace(p.ServerURL) == "" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return ErrInvalidPINProfile
	}

	norm := normalizeAccountRef(ref)
	if normalizeEmail(p.Email) != norm.Email || normalizeServerURL(p.ServerURL) != norm.ServerURL {
		return ErrAccountMismatch
	}

	return nil
}

// VerifyPIN returns true if the given PIN derives to the stored verifier hash
// using Argon2id. It returns false for profiles in an invalid state or short
// PINs without exposing the stored hash through timing side channels.
func (p PINProfile) VerifyPIN(pin string) bool {
	pin = strings.TrimSpace(pin)
	if len(pin) < MinPINLength || !p.validMaterial() {
		return false
	}
	candidate := p.derive(pin)
	return subtle.ConstantTimeCompare(candidate, p.VerifierHash) == 1
}

func (p PINProfile) validMaterial() bool {
	return p.Version == PINProfileVersion &&
		p.KDF == pinVerifierKDF &&
		p.KDFTime == pinVerifierKDFTime &&
		p.KDFMemory == pinVerifierKDFMemory &&
		p.KDFThreads == pinVerifierKDFThreads &&
		len(p.VerifierSalt) == pinVerifierSaltSize &&
		len(p.VerifierHash) == pinVerifierHashSize &&
		len(p.EnvelopeKey) == EnvelopeKeySize
}

// derive runs Argon2id key derivation with the profile's parameters and salt.
// The output length is determined by the stored VerifierHash length (or the
// default hash size if not yet set during profile creation).
func (p PINProfile) derive(pin string) []byte {
	keyLen := uint32(pinVerifierHashSize)
	if len(p.VerifierHash) > 0 {
		keyLen = uint32(len(p.VerifierHash))
	}
	return argon2.IDKey([]byte(pin), p.VerifierSalt, p.KDFTime, p.KDFMemory, p.KDFThreads, keyLen)
}

// ---------------------------------------------------------------------------
// Normalization helpers (session-local; adapters have their own copies)
// ---------------------------------------------------------------------------

// normalizeEmail lowercases and trims whitespace.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// normalizeServerURL trims whitespace and removes a trailing slash.
func normalizeServerURL(serverURL string) string {
	return strings.TrimRight(strings.TrimSpace(serverURL), "/")
}

// normalizeAccountRef returns a copy of ref with normalized email and server URL.
func normalizeAccountRef(ref AccountRef) AccountRef {
	return AccountRef{
		Email:     normalizeEmail(ref.Email),
		ServerURL: normalizeServerURL(ref.ServerURL),
	}
}
