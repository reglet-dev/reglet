package sensitivedata_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactor_AWSKeyDetection(t *testing.T) {
	// Setup redactor with defaults
	redactor, err := sensitivedata.NewRedactor(
		sensitivedata.WithGitleaksDisabled(true), // Use internal regex fallback
	)
	require.NoError(t, err)

	// Test case: AWS Access Key
	input := "My AWS key is AKIAIOSFODNN7EXAMPLE."
	expected := "My AWS key is [REDACTED]."
	got := redactor.ScrubString(input)

	assert.Equal(t, expected, got)
}

func TestRedactor_GitHubTokenDetection(t *testing.T) {
	// Setup redactor with defaults
	redactor, err := sensitivedata.NewRedactor(
		sensitivedata.WithGitleaksDisabled(true), // Use internal regex fallback
	)
	require.NoError(t, err)

	// Test case: GitHub Token
	// Note: ghp_ pattern needs 36 chars
	token := "ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	input := "My token is " + token
	expected := "My token is [REDACTED]"
	got := redactor.ScrubString(input)

	assert.Equal(t, expected, got)
}

func TestRedactor_HashMode(t *testing.T) {
	// Setup redactor with hash mode
	redactor, err := sensitivedata.NewRedactor(
		sensitivedata.WithGitleaksDisabled(true),
		sensitivedata.WithHashMode(true),
		sensitivedata.WithSalt("test-salt-123"),
		sensitivedata.WithPatterns([]string{"secret"}),
	)
	require.NoError(t, err)

	// Test case: Hashing
	input := "This is a secret message."
	got := redactor.ScrubString(input)

	assert.NotContains(t, got, "secret")
	assert.Contains(t, got, "[hmac:")
	// Ensure format [hmac:...]
	assert.True(t, strings.HasPrefix(strings.Split(got, " ")[3], "[hmac:"))
}

func TestRedactor_RaceOnSliceMutation(t *testing.T) {
	// 1. Setup a shared data structure containing a slice
	// The slice "sharedSlice" will be accessed by multiple goroutines.
	sharedSlice := []interface{}{
		"sensitive-data-1",
		"safe-data",
		map[string]interface{}{"key": "sensitive-value"},
	}

	// We wrap it in a map to simulate a typical config structure
	data := map[string]interface{}{
		"list": sharedSlice,
	}

	redactor, err := sensitivedata.NewRedactor(
		sensitivedata.WithPatterns([]string{"sensitive"}),
	)
	require.NoError(t, err)

	// 2. Run concurrent redactions
	var wg sync.WaitGroup
	workers := 10
	wg.Add(workers)

	// Channel to capture panics or errors effectively
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					// In case of panic due to race (e.g. concurrent map write, though here we test slice)
					// Slice concurrent read/write usually doesn't panic unless append happens,
					// but race detector will catch it.
					t.Logf("Recovered panic in worker: %v", r)
				}
			}()

			// Artificial delay to increase overlap chance
			time.Sleep(time.Millisecond)

			// Perform redaction
			// If redactor modifies sharedSlice in place, this constitutes a data race
			// because multiple goroutines are writing to the same indices of the same slice.
			_ = redactor.Redact(data)
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("Worker error: %v", err)
	}
}
