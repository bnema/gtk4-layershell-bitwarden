package bitwarden

import (
	"context"
	"errors"
	"io"

	sdk "github.com/bnema/bitwarden-go-sdk/bitwarden"
	coreconfig "github.com/bnema/gtk4-layershell-bitwarden/internal/core/config"
	corevault "github.com/bnema/gtk4-layershell-bitwarden/internal/core/vault"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/ports/out"
)

// Compile-time check that Client satisfies out.RemoteVault.
var _ out.RemoteVault = (*Client)(nil)

// Client wraps the Bitwarden Go SDK to implement the out.RemoteVault port.
type Client struct {
	sdk *sdk.Client
}

// NewClient creates a new adapter Client wrapping the SDK, configured from a
// core config. Additional SDK options may be appended (e.g. for testing).
func NewClient(cfg *coreconfig.Config, opts ...sdk.Option) (*Client, error) {
	var sdkOpts []sdk.Option

	switch cfg.Bitwarden.Region {
	case coreconfig.RegionSelfHosted:
		if cfg.Bitwarden.ServerURL != "" {
			sdkOpts = append(sdkOpts, sdk.WithServerURL(cfg.Bitwarden.ServerURL))
		}
	default:
		sdkOpts = append(sdkOpts, sdk.WithRegion(toSDKRegion(cfg.Bitwarden.Region)))
	}

	sdkOpts = append(sdkOpts, opts...)

	sdkClient, err := sdk.NewClient(sdkOpts...)
	if err != nil {
		return nil, err
	}

	return &Client{sdk: sdkClient}, nil
}

// NewFromSDK wraps an existing SDK client. Useful for tests and future wiring.
func NewFromSDK(client *sdk.Client) *Client {
	return &Client{sdk: client}
}

// Login authenticates with master password.
func (c *Client) Login(ctx context.Context, email, password string) error {
	return c.sdk.Login(ctx, sdk.LoginOptions{Email: email, Password: password})
}

// CompleteTwoFactor returns an unsupported error because the current
// RemoteVault port does not expose a two-factor challenge handle.
func (c *Client) CompleteTwoFactor(_ context.Context, _, _ string, _ bool) error {
	return errors.New("unsupported: two-factor challenge not exposed by port, use BeginLogin/CompleteLogin directly")
}

// Lock locks the vault client, clearing in-memory key material.
func (c *Client) Lock(_ context.Context) error {
	c.sdk.Lock()
	return nil
}

// Revision returns an opaque revision string. SDK v0.1.0 has no public
// revision-date endpoint, so the adapter returns "unknown" to force a sync.
func (c *Client) Revision(_ context.Context) (string, error) {
	return "unknown", nil
}

// Sync refreshes vault state from the server and returns all items, folders,
// and an opaque revision string.
func (c *Client) Sync(ctx context.Context) ([]corevault.Item, []corevault.Folder, string, error) {
	if err := c.sdk.Sync(ctx); err != nil {
		return nil, nil, "", err
	}

	sdkItems, err := c.sdk.Vault().List(ctx)
	if err != nil {
		return nil, nil, "", err
	}

	sdkFolders, err := c.sdk.Folders().List(ctx)
	if err != nil {
		return nil, nil, "", err
	}

	items := make([]corevault.Item, 0, len(sdkItems))
	for _, si := range sdkItems {
		ci, err := toCoreItem(si)
		if err != nil {
			return nil, nil, "", err
		}
		items = append(items, ci)
	}

	folders := make([]corevault.Folder, len(sdkFolders))
	for i, sf := range sdkFolders {
		folders[i] = toCoreFolder(sf)
	}

	return items, folders, "unknown", nil
}

// Create creates a new vault item.
func (c *Client) Create(ctx context.Context, item corevault.Item) (corevault.Item, error) {
	created, err := c.sdk.Vault().Create(ctx, toSDKItem(item))
	if err != nil {
		return corevault.Item{}, err
	}
	return toCoreItem(created)
}

// Update updates an existing vault item by ID.
func (c *Client) Update(ctx context.Context, id string, item corevault.Item) (corevault.Item, error) {
	updated, err := c.sdk.Vault().Update(ctx, sdk.ItemID(id), toSDKItem(item))
	if err != nil {
		return corevault.Item{}, err
	}
	return toCoreItem(updated)
}

// Trash soft-deletes (trashes) a vault item.
func (c *Client) Trash(ctx context.Context, id string) error {
	return c.sdk.Vault().Trash(ctx, sdk.ItemID(id))
}

// Restore restores a trashed vault item.
func (c *Client) Restore(ctx context.Context, id string) (corevault.Item, error) {
	restored, err := c.sdk.Vault().Restore(ctx, sdk.ItemID(id))
	if err != nil {
		return corevault.Item{}, err
	}
	return toCoreItem(restored)
}

// Delete permanently deletes a vault item.
func (c *Client) Delete(ctx context.Context, id string) error {
	return c.sdk.Vault().Delete(ctx, sdk.ItemID(id))
}

// ListAttachments returns attachments for a vault item. The public SDK Item
// type does not expose an Attachments field, so this returns an empty slice.
func (c *Client) ListAttachments(_ context.Context, _ string) ([]corevault.Attachment, error) {
	return nil, nil
}

// DownloadAttachment downloads and decrypts an attachment to dst.
func (c *Client) DownloadAttachment(ctx context.Context, itemID, attachmentID string, dst io.Writer) error {
	return c.sdk.Attachments().Download(ctx, itemID, attachmentID, dst)
}

// UploadAttachment encrypts and uploads an attachment from src.
func (c *Client) UploadAttachment(ctx context.Context, itemID, fileName string, size int64, src io.Reader) (corevault.Attachment, error) {
	opts := sdk.AttachmentUploadOptions{
		ItemID:   itemID,
		FileName: fileName,
		Size:     size,
		Reader:   src,
	}
	att, err := c.sdk.Attachments().Upload(ctx, opts)
	if err != nil {
		return corevault.Attachment{}, err
	}
	return toCoreAttachment(att), nil
}

// DeleteAttachment deletes an attachment.
func (c *Client) DeleteAttachment(ctx context.Context, itemID, attachmentID string) error {
	return c.sdk.Attachments().Delete(ctx, itemID, attachmentID)
}
