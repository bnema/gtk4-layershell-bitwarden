//go:build !linux

package layershell

import "github.com/bnema/puregotk/v4/gtk"

// OverlayConfig configures a layer-shell overlay window.
type OverlayConfig struct {
	Namespace         string
	ExclusiveKeyboard bool
}

// InitOverlay is a no-op on non-Linux platforms.
func InitOverlay(_ *gtk.Window, _ OverlayConfig) bool {
	return false
}

// SetKeyboardExclusive is a no-op on non-Linux platforms.
func SetKeyboardExclusive(_ *gtk.Window) {}
