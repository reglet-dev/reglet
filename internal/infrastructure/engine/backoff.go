package engine

import (
	"context"
	"errors"
	"net"
	"syscall"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// CalculateBackoff computes the delay for the next retry attempt.
func CalculateBackoff(
	strategy entities.BackoffType,
	attempt int,
	initialDelay time.Duration,
	maxDelay time.Duration,
) time.Duration {
	switch strategy {
	case entities.BackoffNone:
		return initialDelay
	case entities.BackoffLinear:
		// Linear: attempt * initialDelay
		// e.g., 1s, 2s, 3s...
		delay := time.Duration(attempt) * initialDelay
		if maxDelay > 0 && delay > maxDelay {
			return maxDelay
		}
		return delay
	case entities.BackoffExponential:
		// Exponential: (2^attempt) * initialDelay
		// e.g., 2s, 4s, 8s...
		// Shift left by attempt is 2^attempt
		// We use 1<<attempt, but prevent overflow
		if attempt > 62 { // clear bound to avoid overflow
			return maxDelay
		}
		factor := time.Duration(1 << attempt)
		delay := factor * initialDelay
		if maxDelay > 0 && delay > maxDelay {
			return maxDelay
		}
		return delay
	default:
		// Default to exponential if unknown, or fallback to initialDelay?
		// Plan implies default case returns initialDelay
		return initialDelay
	}
}

// isTransientError checks if an error is likely to be temporary.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are never transient in the sense that we should retry blindly
	// If context is canceled/deadline exceeded, we should stop.
	// Wait, the plan says:
	// "Context errors are never transient... stop retrying"
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		return false
	}

	// Network timeout (this still works)
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Specific transient syscall errors
	if errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ECONNABORTED) {
		return true
	}

	// DNS temporary errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsTemporary {
		return true
	}

	// Note: HTTP status codes would be checked here if we had a specific error type
	// wrapping them. Since hostfuncs/http.go doesn't currently export a
	// HTTPStatusError type, we primarily rely on network/syscall errors.

	return false
}
