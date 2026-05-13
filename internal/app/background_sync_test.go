package app

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/auth"
	coreconfig "github.com/bnema/gtk4-layershell-bitwarden/internal/core/config"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/session"
)

func TestSetBackgroundSyncSuspendedUnlocked(t *testing.T) {
	svc := NewService(Deps{Config: coreconfig.Default()})

	svc.mu.Lock()
	svc.state = auth.LockStateUnlocked
	svc.backgroundSyncMode = backgroundSyncCacheOnly
	svc.mu.Unlock()

	require.NoError(t, svc.SetBackgroundSyncSuspended(context.Background(), true))
	svc.mu.Lock()
	require.True(t, svc.backgroundSyncSuspended)
	svc.mu.Unlock()

	require.NoError(t, svc.SetBackgroundSyncSuspended(context.Background(), false))
	svc.mu.Lock()
	require.False(t, svc.backgroundSyncSuspended)
	svc.mu.Unlock()
}

func TestSetBackgroundSyncSuspendedLockedIsNoOp(t *testing.T) {
	svc := NewService(Deps{Config: coreconfig.Default()})

	require.NoError(t, svc.SetBackgroundSyncSuspended(context.Background(), true))
	svc.mu.Lock()
	require.False(t, svc.backgroundSyncSuspended)
	svc.mu.Unlock()
}

func TestUnlockWithPINConfiguresCacheOnlyWorkerStateWhenEnabled(t *testing.T) {
	email := "user@example.com"
	pin := "1234"
	ref := session.AccountRef{Email: email, ServerURL: "https://vault.bitwarden.com"}
	bootID := "boot-abc"

	cfg := coreconfig.Default()
	cfg.Bitwarden.Email = email
	cfg.Security.BackgroundSync.Enabled = true
	cfg.Security.BackgroundSync.Interval = 10 * time.Minute

	envelope := session.UnlockEnvelope{
		Version:        session.UnlockEnvelopeVersion,
		Account:        ref,
		AccountID:      "acct-1",
		BootID:         bootID,
		ExpiresAt:      time.Now().Add(time.Hour),
		PINMaxFailures: 5,
	}
	material := session.UnlockMaterial{CacheKey: []byte("cache-key"), UserKey: []byte("user-key")}

	svc := NewService(Deps{
		Config: cfg,
		Remote: &fakeRemote{},
		Credentials: &fakeCredentialStore{
			tokenBundle: session.TokenBundle{
				AccountID:    "acct-1",
				Email:        ref.Email,
				ServerURL:    ref.ServerURL,
				AccessToken:  []byte("at"),
				RefreshToken: []byte("rt"),
				TokenType:    "Bearer",
				ExpiresAt:    time.Now().Add(time.Hour),
			},
			envelope: envelope,
		},
		BootID: &fakeBootID{id: bootID},
		PINEnvelope: &fakePINEnvelope{
			result:       envelope.Clone(),
			openMaterial: material,
			openUpdated:  envelope.Clone(),
		},
	})

	require.NoError(t, svc.UnlockWithPIN(context.Background(), email, pin))

	svc.mu.Lock()
	require.Equal(t, backgroundSyncCacheOnly, svc.backgroundSyncMode)
	require.NotNil(t, svc.cancelWorkers)
	svc.mu.Unlock()

	require.NoError(t, svc.SoftLock(context.Background()))
}

func TestStartBackgroundSyncWorkerDoesNotMutateStateOnLockedService(t *testing.T) {
	// Regression: startBackgroundSyncWorker must not write backgroundSyncMode
	// or backgroundSyncSuspended on a locked service. If SoftLock/Shutdown
	// runs between the unlock path dropping s.mu and calling
	// startBackgroundSyncWorker, the worker startup would otherwise
	// overwrite the mode on an already-locked service and launch a
	// goroutine with an already-canceled context.
	cfg := coreconfig.Default()
	svc := NewService(Deps{Config: cfg})

	// Service starts locked with backgroundSyncDisabled (zero value).
	svc.mu.Lock()
	require.Equal(t, auth.LockStateLocked, svc.state)
	require.Equal(t, backgroundSyncDisabled, svc.backgroundSyncMode)
	require.False(t, svc.backgroundSyncSuspended)
	svc.mu.Unlock()

	// Simulate the race: a canceled context arrives when SoftLock/Shutdown
	// already ran and canceled the workers.
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	svc.startBackgroundSyncWorker(canceledCtx, backgroundSyncCacheOnly)

	// State must not have been overwritten.
	svc.mu.Lock()
	require.Equal(t, backgroundSyncDisabled, svc.backgroundSyncMode,
		"backgroundSyncMode must stay disabled on a locked service")
	require.False(t, svc.backgroundSyncSuspended,
		"backgroundSyncSuspended must stay false on a locked service")
	svc.mu.Unlock()
}

func TestUnlockWithPINLeavesWorkerDisabledWhenBackgroundSyncDisabled(t *testing.T) {
	email := "user@example.com"
	pin := "1234"
	ref := session.AccountRef{Email: email, ServerURL: "https://vault.bitwarden.com"}
	bootID := "boot-abc"

	cfg := coreconfig.Default()
	cfg.Bitwarden.Email = email
	cfg.Security.BackgroundSync.Enabled = false

	envelope := session.UnlockEnvelope{
		Version:        session.UnlockEnvelopeVersion,
		Account:        ref,
		AccountID:      "acct-1",
		BootID:         bootID,
		ExpiresAt:      time.Now().Add(time.Hour),
		PINMaxFailures: 5,
	}
	material := session.UnlockMaterial{CacheKey: []byte("cache-key"), UserKey: []byte("user-key")}

	svc := NewService(Deps{
		Config: cfg,
		Remote: &fakeRemote{},
		Credentials: &fakeCredentialStore{
			tokenBundle: session.TokenBundle{
				AccountID:    "acct-1",
				Email:        ref.Email,
				ServerURL:    ref.ServerURL,
				AccessToken:  []byte("at"),
				RefreshToken: []byte("rt"),
				TokenType:    "Bearer",
				ExpiresAt:    time.Now().Add(time.Hour),
			},
			envelope: envelope,
		},
		BootID: &fakeBootID{id: bootID},
		PINEnvelope: &fakePINEnvelope{
			result:       envelope.Clone(),
			openMaterial: material,
			openUpdated:  envelope.Clone(),
		},
	})

	require.NoError(t, svc.UnlockWithPIN(context.Background(), email, pin))

	svc.mu.Lock()
	require.Equal(t, backgroundSyncDisabled, svc.backgroundSyncMode)
	require.Nil(t, svc.cancelWorkers)
	svc.mu.Unlock()
}
