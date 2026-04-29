package clipboard

import (
	"testing"
	"time"
)

func TestPolicy(t *testing.T) {
	tests := []struct {
		name          string
		policy        Policy
		wantClear     time.Duration
		wantCloseCopy bool
	}{
		{
			name:          "defaults",
			policy:        Policy{ClearAfter: 45 * time.Second, CloseAfterCopy: true},
			wantClear:     45 * time.Second,
			wantCloseCopy: true,
		},
		{
			name:          "zero value",
			policy:        Policy{},
			wantClear:     0,
			wantCloseCopy: false,
		},
		{
			name:          "custom",
			policy:        Policy{ClearAfter: 30 * time.Second, CloseAfterCopy: false},
			wantClear:     30 * time.Second,
			wantCloseCopy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.policy.ClearAfter != tt.wantClear {
				t.Errorf("ClearAfter = %v, want %v", tt.policy.ClearAfter, tt.wantClear)
			}
			if tt.policy.CloseAfterCopy != tt.wantCloseCopy {
				t.Errorf("CloseAfterCopy = %v, want %v", tt.policy.CloseAfterCopy, tt.wantCloseCopy)
			}
		})
	}
}
