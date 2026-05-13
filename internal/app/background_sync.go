package app

import (
	"context"
	"time"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/auth"
)

type backgroundSyncMode int

const (
	backgroundSyncDisabled backgroundSyncMode = iota
	backgroundSyncResident
	backgroundSyncCacheOnly
)

func (s *Service) SetBackgroundSyncSuspended(ctx context.Context, suspended bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != auth.LockStateUnlocked || s.backgroundSyncMode == backgroundSyncDisabled {
		return nil
	}

	s.backgroundSyncSuspended = suspended
	return nil
}

func (s *Service) backgroundSyncEnabledLocked() bool {
	return s.cfg != nil && s.cfg.Security.BackgroundSync.Enabled
}

func (s *Service) startBackgroundSyncWorker(ctx context.Context, mode backgroundSyncMode) {
	go func() {
		s.syncOnceByMode(ctx, mode)

		ticker := time.NewTicker(s.syncInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.syncOnceByMode(ctx, mode)
			}
		}
	}()
}

func (s *Service) syncOnceByMode(ctx context.Context, mode backgroundSyncMode) {
	s.mu.Lock()
	locked := s.state != auth.LockStateUnlocked
	suspended := s.backgroundSyncSuspended
	s.mu.Unlock()

	if locked || suspended {
		return
	}

	switch mode {
	case backgroundSyncResident:
		s.syncOnceResident(ctx)
	case backgroundSyncCacheOnly:
		return
	}
}

func (s *Service) syncOnceResident(ctx context.Context) {
	s.syncOnce(ctx)
}
