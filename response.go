package synthient

import (
	"context"
	"net/http"
)

// ResponseMetadata describes one HTTP response from the Synthient API.
type ResponseMetadata struct {
	StatusCode int
	RequestID  string
	// Header is an independent copy of the response headers, including
	// RateLimit-Limit, RateLimit-Remaining, and RateLimit-Reset when present.
	// Headers can contain sensitive data, such as presigned download URLs.
	Header http.Header
}

// WithResponseHook calls hook synchronously after each API response's headers
// arrive, including retries, pages, stream handshakes, and decoding failures.
// Storage download responses are excluded. The context belongs to the caller's
// operation. The hook must be safe for concurrent use; it may retain or modify
// the metadata without affecting the client. A nil hook disables the callback.
func WithResponseHook(hook func(context.Context, ResponseMetadata)) Option {
	return func(c *config) {
		c.onResponse = hook
	}
}
