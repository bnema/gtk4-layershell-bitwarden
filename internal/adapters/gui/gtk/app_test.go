package gtk

import (
	"context"
	"testing"
)

func TestNewOverlay_NilServiceRunError(t *testing.T) {
	o := NewOverlay(nil, Options{})
	err := o.Run(context.Background())
	if err == nil {
		t.Error("expected error when running overlay with nil service")
	}
}

func TestGTKAvailable_ReturnsBool(t *testing.T) {
	available := GTKAvailable()
	// Must not panic; must return a bool. The value depends on platform.
	_ = available
}
