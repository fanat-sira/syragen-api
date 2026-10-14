package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenRouterClientImageGeneration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/images" && r.Method == http.MethodPost {
			var req ImageGenerationRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			if req.Prompt == "forbidden content" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]interface{}{
						"code":    "moderation_violation",
						"message": "Prompt rejected by content moderation safety filters",
					},
				})
				return
			}

			w.WriteHeader(http.StatusOK)
			cost := 0.04
			_ = json.NewEncoder(w).Encode(ImageGenerationResponse{
				Data: []ImageData{
					{
						B64JSON:   "dGVzdF9pbWFnZV9ieXRlcw==",
						MediaType: "image/png",
					},
				},
				Usage: &ImageUsage{
					Cost: &cost,
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	ctx := context.Background()

	// 1. Success case
	resp, err := client.GenerateImage(ctx, "test_api_key", ImageGenerationRequest{
		Model:  "x-ai/grok-imagine-image-2.0",
		Prompt: "a peaceful mountain landscape",
	})
	if err != nil {
		t.Fatalf("failed to generate image: %v", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].B64JSON != "dGVzdF9pbWFnZV9ieXRlcw==" {
		t.Errorf("unexpected image data in response: %+v", resp.Data)
	}

	// 2. Moderation rejection error detection
	_, err = client.GenerateImage(ctx, "test_api_key", ImageGenerationRequest{
		Model:  "x-ai/grok-imagine-image-2.0",
		Prompt: "forbidden content",
	})
	if err == nil {
		t.Fatal("expected error on moderation rejection, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok || !apiErr.IsModerationRejection() {
		t.Errorf("expected moderation rejection detection, got %v", err)
	}
}

func TestOpenRouterClientVideoGenerationAndPolling(t *testing.T) {
	var pollCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/videos" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(VideoGenerationResponse{
				ID:         "job_abc123",
				PollingURL: "/videos/job_abc123",
				Status:     "pending",
			})

		case r.URL.Path == "/videos/job_abc123" && r.Method == http.MethodGet:
			count := atomic.AddInt32(&pollCount, 1)
			w.WriteHeader(http.StatusOK)
			if count == 1 {
				_ = json.NewEncoder(w).Encode(VideoPollResponse{
					ID:     "job_abc123",
					Status: "in_progress",
				})
			} else {
				cost := 0.50
				_ = json.NewEncoder(w).Encode(VideoPollResponse{
					ID:           "job_abc123",
					Status:       "completed",
					UnsignedURLs: []string{"http://" + r.Host + "/videos/job_abc123/content"},
					Usage: &VideoPollUsage{
						Cost: &cost,
					},
				})
			}

		case r.URL.Path == "/videos/job_abc123/content" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fake video content stream bytes"))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pollResp, videoBytes, err := client.GenerateVideoAndWait(ctx, "test_api_key", VideoGenerationRequest{
		Model:  "x-ai/grok-imagine-video-1.5",
		Prompt: "a camera pan over a neon city",
	}, 10*time.Millisecond)

	if err != nil {
		t.Fatalf("failed to generate and poll video: %v", err)
	}
	if pollResp.Status != "completed" {
		t.Errorf("expected completed status, got %s", pollResp.Status)
	}
	if string(videoBytes) != "fake video content stream bytes" {
		t.Errorf("unexpected video bytes: %s", string(videoBytes))
	}
}

func TestOpenRouterClientRetries(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		att := atomic.AddInt32(&attempts, 1)
		if att < 2 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 retryable
			_, _ = w.Write([]byte(`{"error":{"message":"temporary overload"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ImageGenerationResponse{
			Data: []ImageData{{B64JSON: "success_after_retry"}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client())
	ctx := context.Background()

	var resp ImageGenerationResponse
	err := client.doRequest(ctx, http.MethodGet, "/status", "test_key", nil, &resp)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if resp.Data[0].B64JSON != "success_after_retry" {
		t.Errorf("expected success after retry")
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}
