// Package clipboard defines the Policy type for clipboard auto-clear behaviour.
package clipboard

import "time"

// Policy controls when the clipboard is automatically cleared and whether the
// vault overlay should close after a copy action.
type Policy struct {
	// ClearAfter is the duration after which the clipboard is automatically
	// cleared. A zero value means no auto-clear.
	ClearAfter time.Duration

	// CloseAfterCopy, when true, closes the vault overlay after the user
	// copies an item to the clipboard.
	CloseAfterCopy bool
}
