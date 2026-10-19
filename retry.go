package synthient

import (
	"context"
	"crypto/tls"
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxBackoff    = 30 * time.Second
	maxRetryAfter = time.Minute
)

type retryPolicy int

const (
	// retryTransient is for operations that cost no credits.
	retryTransient retryPolicy = iota
	// retryRateLimit retries only rate limits. It is for billed requests,
	// where a retried server error could be charged twice.
	retryRateLimit
)

func (p retryPolicy) retries(resp *http.Response) bool {
	if resp.StatusCode == http.StatusTooManyRequests {
		// Only the rate limiter sets Retry-After. Other 429s are not transient.
		return resp.Header.Get("Retry-After") != ""
	}
	if p != retryTransient {
		return false
	}
	switch resp.StatusCode {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (c *Client) send(ctx context.Context, r request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		canRetry := attempt < c.maxRetries

		resp, err := c.attempt(ctx, r)
		if err != nil {
			var certErr *tls.CertificateVerificationError
			permanent := errors.As(err, &certErr) || errors.Is(err, http.ErrSchemeMismatch)
			if !canRetry || r.policy != retryTransient || permanent || ctx.Err() != nil {
				return nil, err
			}
			err = c.wait(ctx, backoff(attempt))
			if err != nil {
				return nil, err
			}
			continue
		}

		if !canRetry || !r.policy.retries(resp) {
			return resp, nil
		}
		retryAfter := parseRetryAfter(resp.Header)
		delay := max(backoff(attempt), retryAfter)
		deadline, ok := ctx.Deadline()
		pastDeadline := ok && time.Until(deadline) <= delay
		if retryAfter > maxRetryAfter || pastDeadline {
			return resp, nil
		}
		closeBody(resp)
		err = c.wait(ctx, delay)
		if err != nil {
			return nil, err
		}
	}
}

func backoff(attempt int) time.Duration {
	d := min(time.Second<<min(attempt, 5), maxBackoff)
	jitter := 0.75 + rand.Float64()/2 //nolint:gosec // retry jitter does not need a secure random source
	return time.Duration(float64(d) * jitter)
}

func parseRetryAfter(h http.Header) time.Duration {
	value := strings.TrimSpace(h.Get("Retry-After"))
	if value == "" {
		return 0
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return time.Duration(min(max(seconds, 0), 1<<31)) * time.Second
	}
	at, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	return max(time.Until(at), 0)
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
