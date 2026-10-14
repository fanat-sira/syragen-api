// Package httpio bounds response bodies before buffering them in memory.
package httpio

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

const (
	MaxImageBytes int64 = 20 << 20
	MaxVideoBytes int64 = 50 << 20
	MaxJSONBytes  int64 = 64 << 20
	MaxErrorBytes int64 = 1 << 20
)

var ErrResponseTooLarge = errors.New("response body exceeds size limit")

func ReadResponse(resp *http.Response, limit int64) ([]byte, error) {
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, limit)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, limit)
	}
	return data, nil
}
