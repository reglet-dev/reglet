package netutil

import (
	"net/http"
	"strconv"
	"time"
)

// RetryTransport wraps an http.RoundTripper with retry logic.
// It implements exponential backoff and respects Retry-After headers.
type RetryTransport struct {
	// Base is the underlying transport.
	// Default: http.DefaultTransport if nil.
	Base http.RoundTripper

	// OnRetry is called before each retry attempt.
	// The callback receives the attempt number (1-based) and wait duration.
	OnRetry func(attempt int, waitDuration time.Duration, statusCode int)

	// MaxRetries is the maximum number of retry attempts.
	// Default: 3 if zero.
	MaxRetries int

	// InitialBackoff is the initial backoff duration.
	// Default: 1s if zero.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration.
	// Default: 30s if zero.
	MaxBackoff time.Duration
}

// RoundTrip implements http.RoundTripper with retry logic.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	maxRetries := t.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	initialBackoff := t.InitialBackoff
	if initialBackoff == 0 {
		initialBackoff = time.Second
	}

	maxBackoff := t.MaxBackoff
	if maxBackoff == 0 {
		maxBackoff = 30 * time.Second
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Clone the request for retry (body must be re-readable)
		reqClone := req.Clone(req.Context())
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			reqClone.Body = body
		}

		resp, err := base.RoundTrip(reqClone)
		if err != nil {
			lastErr = err
			// Network errors are retryable
			if attempt < maxRetries {
				waitDuration := t.calculateBackoff(attempt, initialBackoff, maxBackoff, nil)
				if t.OnRetry != nil {
					t.OnRetry(attempt+1, waitDuration, 0)
				}
				time.Sleep(waitDuration)
				continue
			}
			return nil, lastErr
		}

		// Check if we should retry based on status code
		if !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// Don't retry 4xx errors (except 429)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		lastResp = resp
		lastErr = nil

		if attempt < maxRetries {
			waitDuration := t.calculateBackoff(attempt, initialBackoff, maxBackoff, resp)
			if t.OnRetry != nil {
				t.OnRetry(attempt+1, waitDuration, resp.StatusCode)
			}
			// Close the response body before retry
			_ = resp.Body.Close()
			time.Sleep(waitDuration)
			continue
		}
	}

	if lastResp != nil {
		return lastResp, nil
	}
	return nil, lastErr
}

// calculateBackoff determines the wait duration for the given attempt.
// It respects Retry-After headers when present.
func (t *RetryTransport) calculateBackoff(attempt int, initial, maxDuration time.Duration, resp *http.Response) time.Duration {
	// Check for Retry-After header
	if resp != nil {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			// Try parsing as seconds
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				duration := time.Duration(seconds) * time.Second
				if duration > maxDuration {
					return maxDuration
				}
				return duration
			}
			// Try parsing as HTTP date (RFC 1123)
			if tParser, err := http.ParseTime(retryAfter); err == nil {
				duration := time.Until(tParser)
				if duration < 0 {
					return initial
				}
				if duration > maxDuration {
					return maxDuration
				}
				return duration
			}
		}
	}

	// Exponential backoff: initial * 2^attempt
	backoff := initial * (1 << attempt)
	if backoff > maxDuration {
		return maxDuration
	}
	return backoff
}

// isRetryableStatus returns true if the status code indicates a transient error.
func isRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests, // 429
		http.StatusBadGateway,         // 502
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout:     // 504
		return true
	default:
		return false
	}
}

// IsRetryableStatus is exported for testing and external use.
func IsRetryableStatus(statusCode int) bool {
	return isRetryableStatus(statusCode)
}
