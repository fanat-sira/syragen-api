package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	DefaultBaseURL = "https://openrouter.ai/api/v1"
	MaxRetries     = 2
)

// Client handles communication with the OpenRouter API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new OpenRouter client.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 4 * time.Minute,
		}
	}
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	}
}

// doRequest performs an HTTP request with automatic retries for transient errors.
func (c *Client) doRequest(ctx context.Context, method, path, apiKey string, reqBody interface{}, respObj interface{}) error {
	var bodyBytes []byte
	var err error

	if reqBody != nil {
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	url := c.BaseURL + path
	var lastErr error

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			log.Printf("[openrouter] retrying request (%d/%d) to %s", attempt, MaxRetries, path)
		}

		var reqBodyReader io.Reader
		if bodyBytes != nil {
			reqBodyReader = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, reqBodyReader)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Title", "app")

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			// Check if context was cancelled
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := parseAPIError(resp.StatusCode, respBody)
			if apiErr.IsRetryable() && attempt < MaxRetries {
				lastErr = apiErr
				continue
			}
			return apiErr
		}

		if respObj != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, respObj); err != nil {
				// Key creation responses contain credentials; never include raw bodies in errors.
				return fmt.Errorf("failed to unmarshal response: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("request failed after %d retries: %w", MaxRetries, lastErr)
}

func parseAPIError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{
		StatusCode: statusCode,
		RawBody:    string(body),
	}

	var errorEnvelope struct {
		Error struct {
			Code    interface{} `json:"code"`
			Message string      `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errorEnvelope); err == nil && (errorEnvelope.Error.Message != "" || errorEnvelope.Error.Code != nil) {
		apiErr.Message = errorEnvelope.Error.Message
		if codeStr, ok := errorEnvelope.Error.Code.(string); ok {
			apiErr.Code = codeStr
		} else if codeNum, ok := errorEnvelope.Error.Code.(float64); ok {
			apiErr.Code = fmt.Sprintf("%.0f", codeNum)
		}
	} else {
		// Fallback message
		apiErr.Message = string(body)
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(statusCode)
		}
	}

	return apiErr
}
