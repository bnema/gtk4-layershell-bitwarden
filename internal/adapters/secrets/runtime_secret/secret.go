// Package runtime_secret wraps runtime/secret.Do to provide a testable runner.
package runtime_secret

import "runtime/secret"

// Runner is a stateless wrapper around runtime/secret.Do.
type Runner struct{}

// NewRunner returns a new Runner.
func NewRunner() Runner { return Runner{} }

// Do invokes fn inside secret.Do.
func (Runner) Do(fn func()) { secret.Do(fn) }

// Bytes copies value into an owned buffer, runs fn inside secret.Do with the
// copy, then zeroes the buffer inside the secret.Do scope via defer so a panic
// inside fn cannot skip wiping. Mutating the slice passed to fn does not affect
// the original input.
func (Runner) Bytes(value []byte, fn func([]byte)) {
	buf := make([]byte, len(value))
	copy(buf, value)
	secret.Do(func() {
		defer func() {
			for i := range buf {
				buf[i] = 0
			}
		}()
		fn(buf)
	})
}

// String runs fn(value) inside secret.Do.
func (Runner) String(value string, fn func(string)) {
	secret.Do(func() {
		fn(value)
	})
}
