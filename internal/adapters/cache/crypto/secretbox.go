package crypto

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// Box implements ports/out.SecretBox using XChaCha20-Poly1305.
type Box struct{}

// NewBox returns a new Box ready for use.
func NewBox() Box {
	return Box{}
}

var (
	ErrInvalidKey      = errors.New("secretbox: invalid key size")
	ErrDecryptFailed   = errors.New("secretbox: decryption failed")
	ErrCiphertextShort = errors.New("secretbox: ciphertext too short")
)

// Seal encrypts and authenticates plaintext with key using XChaCha20-Poly1305.
// Returns nonce || ciphertext.
func (Box) Seal(plaintext, key []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, ErrInvalidKey
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, ErrInvalidKey
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.New("secretbox: failed to generate nonce")
	}

	// Seal appends encrypted+auth data after the nonce, giving nonce || ciphertext.
	sealed := aead.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}

// Open decrypts and authenticates ciphertext (nonce || sealed) with key.
func (Box) Open(ciphertext, key []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, ErrInvalidKey
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, ErrInvalidKey
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextShort
	}

	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	return plaintext, nil
}
