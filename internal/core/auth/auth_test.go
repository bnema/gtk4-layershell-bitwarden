package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLockStateValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  LockState
		want string
	}{
		{name: "locked", got: LockStateLocked, want: "locked"},
		{name: "unlocking", got: LockStateUnlocking, want: "unlocking"},
		{name: "unlocked", got: LockStateUnlocked, want: "unlocked"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, string(tc.got))
		})
	}
}

func TestRelockPolicyExplicitDurations(t *testing.T) {
	t.Parallel()

	policy := RelockPolicy{
		IdleTimeout:     15 * time.Minute,
		ResidentTimeout: 30 * time.Minute,
	}

	require.Equal(t, 15*time.Minute, policy.IdleTimeout)
	require.Equal(t, 30*time.Minute, policy.ResidentTimeout)
}

func TestUnlockSessionExplicitValues(t *testing.T) {
	t.Parallel()

	unlockedAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	session := UnlockSession{
		AccountID:  "account-1",
		Email:      "user@example.com",
		UnlockedAt: unlockedAt,
	}

	require.Equal(t, "account-1", session.AccountID)
	require.Equal(t, "user@example.com", session.Email)
	require.True(t, session.UnlockedAt.Equal(unlockedAt))
}
