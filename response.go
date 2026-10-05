package synthient

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

// ResponseMetadata describes one HTTP response from the Synthient API.
type ResponseMetadata struct {
	StatusCode int
	RequestID  string
	// RateLimit is the rate limit that applies to the operation, or nil if the
	// response does not report one, as on Health.
	RateLimit *RateLimit
	// Header is an independent copy of the response headers. Headers can
	// contain sensitive data, such as presigned download URLs.
	Header http.Header
}

// RateLimit is the state of an operation's rate limit, from the
// RateLimit-Limit, RateLimit-Remaining, and RateLimit-Reset headers. Limits
// are shared by every key of a team.
type RateLimit struct {
	// Limit is the bucket size, which is the burst the operation allows.
	Limit int
	// Remaining is the number of requests left in the bucket.
	Remaining int
	// Reset is how long until the bucket is full again.
	Reset time.Duration
}

func parseRateLimit(h http.Header) *RateLimit {
	limit, err := strconv.Atoi(h.Get("Ratelimit-Limit"))
	if err != nil {
		return nil
	}
	remaining, err := strconv.Atoi(h.Get("Ratelimit-Remaining"))
	if err != nil {
		return nil
	}
	reset, err := strconv.ParseInt(h.Get("Ratelimit-Reset"), 10, 64)
	if err != nil {
		return nil
	}
	return &RateLimit{
		Limit:     limit,
		Remaining: remaining,
		Reset:     time.Duration(min(max(reset, 0), 1<<31)) * time.Second,
	}
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
