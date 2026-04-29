//go:build nogtk

// Package layershell provides a stub implementation for builds without GTK.
// When compiled with the nogtk tag, no real layer-shell functionality is
// available and all operations are no-ops or return zero values.
package layershell
