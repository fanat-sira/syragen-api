package openrouter

import (
	"context"
	"fmt"
	"net/http"
)

// GenerateImage sends an image generation or edit request to OpenRouter's Image API.
func (c *Client) GenerateImage(ctx context.Context, apiKey string, req ImageGenerationRequest) (*ImageGenerationResponse, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("model is required for image generation")
	}
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required for image generation")
	}

	var resp ImageGenerationResponse
	err := c.doRequest(ctx, http.MethodPost, "/images", apiKey, req, &resp)
	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 || resp.Data[0].B64JSON == "" {
		return nil, fmt.Errorf("no image data returned in OpenRouter response")
	}

	return &resp, nil
}
