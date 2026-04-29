package out

// SecretBox provides authenticated symmetric encryption.
type SecretBox interface {
	Seal(plaintext, key []byte) ([]byte, error)
	Open(ciphertext, key []byte) ([]byte, error)
}
