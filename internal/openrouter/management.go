package openrouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// CreateKey creates a new ephemeral API key using the management key.
func (c *Client) CreateKey(ctx context.Context, managementKey, name string, limit float64, expiresAt time.Time) (*CreateKeyResponse, error) {
	expStr := expiresAt.UTC().Format(time.RFC3339)
	reqBody := CreateKeyRequest{
		Name:      name,
		Limit:     &limit,
		ExpiresAt: &expStr,
	}

	var resp CreateKeyResponse
	err := c.doRequest(ctx, http.MethodPost, "/keys", managementKey, reqBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to create API key via management API: %w", err)
	}

	if resp.Key == "" || resp.Data.Hash == "" {
		return nil, fmt.Errorf("invalid response from create API key: key or hash is empty")
	}

	return &resp, nil
}

// GetKey retrieves metadata and spend usage for an API key by hash using the management key.
func (c *Client) GetKey(ctx context.Context, managementKey, hash string) (*GetKeyResponse, error) {
	var resp GetKeyResponse
	err := c.doRequest(ctx, http.MethodGet, "/keys/"+hash, managementKey, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key %s: %w", hash, err)
	}
	return &resp, nil
}

// DeleteKey revokes/deletes an API key by hash using the management key.
func (c *Client) DeleteKey(ctx context.Context, managementKey, hash string) error {
	err := c.doRequest(ctx, http.MethodDelete, "/keys/"+hash, managementKey, nil, nil)
	if err != nil {
		// If the key was already deleted (404), treat as success for idempotency
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete API key %s: %w", hash, err)
	}
	return nil
}
