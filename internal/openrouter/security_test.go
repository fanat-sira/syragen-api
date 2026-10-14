package openrouter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type downloadTransport func(*http.Request) (*http.Response, error)

func (f downloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadVideoKeepsCredentialsOnAPIOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, target, redirect string
		wantAuth               bool
	}{
		{name: "relative API path", target: "/videos/job/content", wantAuth: true},
		{name: "API URL", target: "https://openrouter.ai/api/v1/videos/job/content", wantAuth: true},
		{name: "external media", target: "https://media.example/video.mp4"},
		{name: "lookalike hostname", target: "https://openrouter.ai.example/video.mp4"},
		{name: "API name in query", target: "https://media.example/video?source=openrouter.ai"},
		{name: "HTTP downgrade", target: "http://openrouter.ai/video.mp4"},
		{name: "different port", target: "https://openrouter.ai:8443/video.mp4"},
		{name: "external redirect", target: "/videos/job/content", redirect: "https://media.example/video.mp4"},
		{name: "subdomain redirect", target: "/videos/job/content", redirect: "https://media.openrouter.ai/video.mp4"},
		{name: "same origin redirect", target: "/videos/job/content", redirect: "https://openrouter.ai/video.mp4", wantAuth: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient("", &http.Client{Transport: downloadTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if tc.redirect != "" && calls == 1 {
					if r.Header.Get("Authorization") != "Bearer test-download-key" {
						t.Error("missing API authorization")
					}
					return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{tc.redirect}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				}
				expected := ""
				if tc.wantAuth {
					expected = "Bearer test-download-key"
				}
				if got := r.Header.Get("Authorization"); got != expected {
					t.Errorf("authorization at download destination: got %q, want %q", got, expected)
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("video")), Request: r}, nil
			})})
			data, err := client.DownloadVideoContent(context.Background(), "test-download-key", tc.target)
			if err != nil || string(data) != "video" {
				t.Fatalf("download: %q, %v", data, err)
			}
		})
	}
}

func TestMalformedKeyResponseDoesNotExposeCredentials(t *testing.T) {
	const secret = "synthetic-sensitive-response-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"key":"`+secret+`","data":{"hash":123}}`)
	}))
	defer server.Close()
	client := NewClient(server.URL, server.Client())
	var response CreateKeyResponse
	err := client.doRequest(context.Background(), http.MethodPost, "/keys", "test-management-key", nil, &response)
	if err == nil {
		t.Fatal("expected invalid key response to fail")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), `"key"`) {
		t.Fatalf("response body leaked in error: %v", err)
	}
}

// POST ambiguity must never cause a second paid generation or key creation.
func TestPostRequestsAreNeverReplayed(t *testing.T) {
	for _, path := range []string{"/images", "/videos", "/keys"} {
		for _, failure := range []string{"429", "503", "transport", "body", "307", "308"} {
			t.Run(path+"/"+failure, func(t *testing.T) {
				calls := 0
				client := NewClient("https://api.example", &http.Client{Transport: downloadTransport(func(r *http.Request) (*http.Response, error) {
					calls++
					if failure == "transport" {
						return nil, io.ErrUnexpectedEOF
					}
					response := &http.Response{StatusCode: 503, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"temporary"}}`)), Request: r}
					switch failure {
					case "429":
						response.StatusCode = 429
					case "body":
						response.StatusCode = 200
						response.Body = failingResponseBody{}
					case "307":
						response.StatusCode = 307
						response.Header.Set("Location", "https://api.example/replay")
					case "308":
						response.StatusCode = 308
						response.Header.Set("Location", "https://api.example/replay")
					}
					return response, nil
				})})
				err := client.doRequest(context.Background(), http.MethodPost, path, "test-key", map[string]string{"prompt": "test"}, nil)
				if err == nil || calls != 1 {
					t.Fatalf("POST calls=%d, error=%v", calls, err)
				}
			})
		}
	}
}

type failingResponseBody struct{}

func (failingResponseBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (failingResponseBody) Close() error             { return nil }

func TestOversizedVideoIsRejected(t *testing.T) {
	client := NewClient("https://api.example", &http.Client{Transport: downloadTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: (50 << 20) + 1, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})})
	if data, err := client.DownloadVideoContent(context.Background(), "test-key", "/video"); err == nil || data != nil {
		t.Fatalf("oversized video accepted: %d bytes, %v", len(data), err)
	}
}

func TestDeleteKeyDoesNotMistakeMessageForNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"request reference 404 failed"}}`)
	}))
	defer server.Close()
	if err := NewClient(server.URL, server.Client()).DeleteKey(context.Background(), "test-key", "test-hash"); err == nil {
		t.Fatal("non-404 error was incorrectly treated as successful revocation")
	}
}
