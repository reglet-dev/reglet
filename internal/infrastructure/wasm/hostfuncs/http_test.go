package hostfuncs

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_TruncationDetection_ExceedsLimit tests that we detect truncation when body > 10MB
func Test_TruncationDetection_ExceedsLimit(t *testing.T) {
	// Simulate reading a body larger than 10MB
	maxBodySize := int64(10 * 1024 * 1024)
	bodySize := int64(11 * 1024 * 1024) // 11MB

	// Create a reader with 11MB of data
	largeBody := bytes.Repeat([]byte("A"), int(bodySize))
	reader := bytes.NewReader(largeBody)

	// Read with limit of maxBodySize + 1
	limitedReader := io.LimitReader(reader, maxBodySize+1)
	readBytes, err := io.ReadAll(limitedReader)
	require.NoError(t, err)

	// Check if we read more than maxBodySize (which means truncation needed)
	bodyTruncated := len(readBytes) > int(maxBodySize)

	assert.True(t, bodyTruncated, "Expected truncation to be detected for 11MB body")
	assert.Equal(t, int(maxBodySize)+1, len(readBytes), "Should have read maxBodySize+1 bytes")
}

// Test_TruncationDetection_WithinLimit tests that we don't falsely detect truncation for small bodies
func Test_TruncationDetection_WithinLimit(t *testing.T) {
	maxBodySize := int64(10 * 1024 * 1024)
	smallBody := []byte("Hello, World!")

	reader := bytes.NewReader(smallBody)
	limitedReader := io.LimitReader(reader, maxBodySize+1)
	readBytes, err := io.ReadAll(limitedReader)
	require.NoError(t, err)

	bodyTruncated := len(readBytes) > int(maxBodySize)

	assert.False(t, bodyTruncated, "Expected NO truncation for small body")
	assert.Equal(t, len(smallBody), len(readBytes), "Should have read entire small body")
	assert.Equal(t, smallBody, readBytes, "Body content should match")
}

// Test_TruncationDetection_ExactlyAtLimit tests edge case where body is exactly 10MB
func Test_TruncationDetection_ExactlyAtLimit(t *testing.T) {
	maxBodySize := int64(10 * 1024 * 1024)
	exactBody := bytes.Repeat([]byte("B"), int(maxBodySize))

	reader := bytes.NewReader(exactBody)
	limitedReader := io.LimitReader(reader, maxBodySize+1)
	readBytes, err := io.ReadAll(limitedReader)
	require.NoError(t, err)

	bodyTruncated := len(readBytes) > int(maxBodySize)

	assert.False(t, bodyTruncated, "Expected NO truncation for body exactly at limit")
	assert.Equal(t, int(maxBodySize), len(readBytes), "Should have read exactly maxBodySize bytes")
}
