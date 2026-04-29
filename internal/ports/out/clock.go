package out

import "time"

// Clock provides the current time. Useful for testability.
type Clock interface {
	Now() time.Time
}
