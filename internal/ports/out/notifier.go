package out

import "context"

// Notifier sends desktop notifications to the user.
type Notifier interface {
	Notify(ctx context.Context, summary, body string) error
}
