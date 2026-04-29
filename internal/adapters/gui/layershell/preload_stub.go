//go:build !linux || nogtk

package layershell

// EnsurePreloaded is a no-op outside Linux GTK builds.
func EnsurePreloaded() {}
