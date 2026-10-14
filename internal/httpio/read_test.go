package httpio

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReadResponseBoundsKnownAndStreamingBodies(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		length     int64
		tooLarge   bool
	}{
		{"below limit", "abc", 3, false},
		{"exact limit", "abcd", 4, false},
		{"known oversized", "", 5, true},
		{"chunked oversized", "abcde", -1, true},
		{"dishonest header", "abcde", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{ContentLength: tc.length, Body: io.NopCloser(strings.NewReader(tc.body))}
			data, err := ReadResponse(resp, 4)
			if tc.tooLarge {
				if !errors.Is(err, ErrResponseTooLarge) || data != nil {
					t.Fatalf("oversized body accepted: %q, %v", data, err)
				}
			} else if err != nil || string(data) != tc.body {
				t.Fatalf("valid body: %q, %v", data, err)
			}
		})
	}
}
