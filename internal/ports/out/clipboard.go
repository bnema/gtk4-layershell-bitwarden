package out

import (
	"context"
	"time"
)

// Clipboard provides system clipboard access with automatic clearing.
type Clipboard interface {
	Set(ctx context.Context, text string, ttl time.Duration) error
	Clear(ctx context.Context) error
}
