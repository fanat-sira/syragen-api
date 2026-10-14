package openrouter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SubmitVideo submits a video generation job to OpenRouter.
func (c *Client) SubmitVideo(ctx context.Context, apiKey string, req VideoGenerationRequest) (*VideoGenerationResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required for video generation")
	}
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required for video generation")
	}

	var resp VideoGenerationResponse
	err := c.doRequest(ctx, http.MethodPost, "/videos", apiKey, req, &resp)
	if err != nil {
		return nil, err
	}

	if resp.ID == "" {
		return nil, fmt.Errorf("no job ID returned from video generation request")
	}

	return &resp, nil
}

// PollVideo checks the status of an ongoing video generation job.
func (c *Client) PollVideo(ctx context.Context, apiKey, jobID string) (*VideoPollResponse, error) {
	var resp VideoPollResponse
	err := c.doRequest(ctx, http.MethodGet, "/videos/"+jobID, apiKey, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DownloadVideoContent downloads the generated video binary data from the specified URL or endpoint.
func (c *Client) DownloadVideoContent(ctx context.Context, apiKey, urlStr string) ([]byte, error) {
	if strings.HasPrefix(urlStr, "/") {
		urlStr = c.BaseURL + urlStr
	}

	// Send credentials only to the configured API origin, never to media hosts.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	apiURL, parseErr := url.Parse(c.BaseURL)
	if parseErr == nil && req.URL.Scheme == apiURL.Scheme && strings.EqualFold(req.URL.Host, apiURL.Host) && req.URL.User == nil {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// net/http can forward Authorization to subdomains on redirects. Strip it
	// on every redirect outside the API origin, including CDN redirects.
	downloadClient := *c.HTTPClient
	previousRedirectCheck := downloadClient.CheckRedirect
	downloadClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if parseErr != nil || next.URL.Scheme != apiURL.Scheme || !strings.EqualFold(next.URL.Host, apiURL.Host) || next.URL.User != nil {
			next.Header.Del("Authorization")
		}
		if previousRedirectCheck != nil {
			return previousRedirectCheck(next, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download video: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, parseAPIError(resp.StatusCode, body)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read video content: %w", err)
	}

	return data, nil
}

// GenerateVideoAndWait submits a video generation job, polls until completion, and downloads the video file.
func (c *Client) GenerateVideoAndWait(ctx context.Context, apiKey string, req VideoGenerationRequest, pollInterval time.Duration) (*VideoPollResponse, []byte, error) {
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}

	submitResp, err := c.SubmitVideo(ctx, apiKey, req)
	if err != nil {
		return nil, nil, err
	}

	jobID := submitResp.ID

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("video generation timed out or was cancelled: %w", ctx.Err())
		case <-ticker.C:
			pollResp, err := c.PollVideo(ctx, apiKey, jobID)
			if err != nil {
				// If poll fails transiently, continue until context deadline
				if apiErr, ok := err.(*APIError); ok && !apiErr.IsRetryable() {
					return nil, nil, err
				}
				continue
			}

			switch pollResp.Status {
			case "completed":
				downloadURL := ""
				if len(pollResp.UnsignedURLs) > 0 && pollResp.UnsignedURLs[0] != "" {
					downloadURL = pollResp.UnsignedURLs[0]
				} else {
					downloadURL = c.BaseURL + "/videos/" + jobID + "/content?index=0"
				}

				videoBytes, err := c.DownloadVideoContent(ctx, apiKey, downloadURL)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to download completed video: %w", err)
				}
				return pollResp, videoBytes, nil

			case "failed":
				errMsg := "video generation failed"
				if pollResp.Error != nil {
					errMsg = fmt.Sprintf("video generation failed: %v", pollResp.Error)
				}
				return nil, nil, &APIError{
					StatusCode: http.StatusBadRequest,
					Message:    errMsg,
				}

			case "pending", "in_progress":
				// Continue polling
			default:
				// Unknown status, continue
			}
		}
	}
}
