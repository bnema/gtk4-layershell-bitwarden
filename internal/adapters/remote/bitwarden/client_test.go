package bitwarden

import (
	"context"
	"testing"

	sdk "github.com/bnema/bitwarden-go-sdk/bitwarden"
	coreconfig "github.com/bnema/gtk4-layershell-bitwarden/internal/core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientRejectsInvalidSelfHostedURL(t *testing.T) {
	// A self-hosted config with a non-https URL should be rejected by the SDK.
	cfg := &coreconfig.Config{
		Bitwarden: coreconfig.Bitwarden{
			Email:     "test@example.com",
			Region:    coreconfig.RegionSelfHosted,
			ServerURL: "http://bad",
		},
	}
	_, err := NewClient(cfg)
	require.Error(t, err, "expected error for invalid self-hosted URL")
}

func TestNewClientDefaultUSNoNetwork(t *testing.T) {
	// NewClient with default US region should construct without network calls.
	cfg := &coreconfig.Config{
		Bitwarden: coreconfig.Bitwarden{
			Email:  "test@example.com",
			Region: coreconfig.RegionUS,
		},
	}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, client)
	// White-box: verify the underlying SDK client is non-nil and locked.
	// We access client.sdk directly because IsLocked has no public
	// equivalent on the adapter — the adapter's Lock() method delegates
	// to the same SDK method and returns nil, but we cannot assert on
	// error alone to prove the SDK state changed.
	assert.True(t, client.sdk.IsLocked())
}

func TestRevisionReturnsOpaqueUnknown(t *testing.T) {
	// Use NewFromSDK with a bare SDK client (no network required).
	sdkClient, err := sdk.NewClient()
	require.NoError(t, err)
	adapter := NewFromSDK(sdkClient)

	rev, err := adapter.Revision(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "unknown", rev)
}

func TestNewFromSDK(t *testing.T) {
	sdkClient, err := sdk.NewClient()
	require.NoError(t, err)
	adapter := NewFromSDK(sdkClient)
	require.NotNil(t, adapter)
	assert.Same(t, sdkClient, adapter.sdk)
}

func TestLockReturnsNil(t *testing.T) {
	sdkClient, err := sdk.NewClient()
	require.NoError(t, err)
	adapter := NewFromSDK(sdkClient)

	err = adapter.Lock(context.Background())
	require.NoError(t, err)
	assert.True(t, sdkClient.IsLocked())
}
