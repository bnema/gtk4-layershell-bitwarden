package app

import (
	"context"
	"sync"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/auth"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/config"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/vault"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/ports/out"
)

// Deps holds the external dependencies the service needs.
type Deps struct {
	Remote    out.RemoteVault
	Cache     out.CacheStore
	SecretBox out.SecretBox
	Config    *config.Config
}

// Service implements the application's core business logic.
type Service struct {
	mu            sync.Mutex
	cfg           *config.Config
	state         auth.LockState
	items         []vault.Item
	folders       []vault.Folder
	index         *vault.SearchIndex
	events        chan Event
	cancelWorkers context.CancelFunc
	deps          Deps
}
