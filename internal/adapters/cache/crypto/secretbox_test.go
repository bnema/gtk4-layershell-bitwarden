package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"
)

func TestBox_SealAndOpen_RoundTrip(t *testing.T) {
	b := NewBox()
	key := make([]byte, chacha20poly1305.KeySize)
	for i := range key {
		key[i] = byte(i)
	}
	plaintext := []byte("hello, this is a secret message")

	ciphertext, err := b.Seal(plaintext, key)
	require.NoError(t, err)
	require.NotNil(t, ciphertext)
	require.Greater(t, len(ciphertext), len(plaintext),
		"ciphertext should be larger due to nonce and overhead")
	require.False(t, bytes.Equal(plaintext, ciphertext),
		"ciphertext must differ from plaintext")

	decrypted, err := b.Open(ciphertext, key)
	require.NoError(t, err)
	require.Equal(t, plaintext, decrypted)
}

func TestBox_Open_WrongKey(t *testing.T) {
	b := NewBox()
	key := make([]byte, chacha20poly1305.KeySize)
	for i := range key {
		key[i] = byte(i)
	}
	plaintext := []byte("secret data")

	ciphertext, err := b.Seal(plaintext, key)
	require.NoError(t, err)

	wrongKey := make([]byte, chacha20poly1305.KeySize)
	for i := range wrongKey {
		wrongKey[i] = byte(255 - i)
	}

	_, err = b.Open(ciphertext, wrongKey)
	require.ErrorIs(t, err, ErrDecryptFailed)
}

func TestBox_Seal_InvalidKeySize(t *testing.T) {
	b := NewBox()
	plaintext := []byte("data")

	// Key too short
	shortKey := make([]byte, 16)
	_, err := b.Seal(plaintext, shortKey)
	require.ErrorIs(t, err, ErrInvalidKey)

	// Key too long
	longKey := make([]byte, chacha20poly1305.KeySize+1)
	_, err = b.Seal(plaintext, longKey)
	require.ErrorIs(t, err, ErrInvalidKey)

	// Empty key
	_, err = b.Seal(plaintext, nil)
	require.ErrorIs(t, err, ErrInvalidKey)
}

func TestBox_Open_InvalidKeySize(t *testing.T) {
	b := NewBox()
	ciphertext := make([]byte, 48) // dummy

	// Key too short
	_, err := b.Open(ciphertext, make([]byte, 16))
	require.ErrorIs(t, err, ErrInvalidKey)

	// Key too long
	_, err = b.Open(ciphertext, make([]byte, chacha20poly1305.KeySize+1))
	require.ErrorIs(t, err, ErrInvalidKey)

	// Empty key
	_, err = b.Open(ciphertext, nil)
	require.ErrorIs(t, err, ErrInvalidKey)
}

func TestBox_Open_ShortCiphertext(t *testing.T) {
	b := NewBox()
	key := make([]byte, chacha20poly1305.KeySize)

	// Ciphertext shorter than nonce size (24 bytes for XChaCha20-Poly1305)
	_, err := b.Open([]byte("too short"), key)
	require.ErrorIs(t, err, ErrCiphertextShort)

	// Empty ciphertext
	_, err = b.Open(nil, key)
	require.ErrorIs(t, err, ErrCiphertextShort)
}
