package hostfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_BoundedBuffer_Truncation_ExceedsLimit verifies that the buffer is marked truncated
// when the output exceeds the limit.
func Test_BoundedBuffer_Truncation_ExceedsLimit(t *testing.T) {
	limit := 10 // small limit for testing
	buffer := NewBoundedBuffer(limit)

	input := []byte("123456789012345") // 15 bytes, exceeds 10 byte limit
	n, err := buffer.Write(input)

	require.NoError(t, err)
	assert.Equal(t, 15, n, "Should return length of input even if truncated")
	assert.True(t, buffer.Truncated, "Should be marked truncated")
	assert.Equal(t, "1234567890", buffer.String(), "Should only contain limited output")
}

// Test_BoundedBuffer_Truncation_WithinLimit verifies that the buffer is NOT marked truncated
// when the output is within the limit.
func Test_BoundedBuffer_Truncation_WithinLimit(t *testing.T) {
	limit := 20
	buffer := NewBoundedBuffer(limit)

	input := []byte("1234567890") // 10 bytes
	n, err := buffer.Write(input)

	require.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.False(t, buffer.Truncated, "Should not be truncated")
	assert.Equal(t, "1234567890", buffer.String())
}

// Test_BoundedBuffer_Truncation_ExactlyAtLimit verifies that the buffer is NOT marked truncated
// when the output is exactly at the limit.
func Test_BoundedBuffer_Truncation_ExactlyAtLimit(t *testing.T) {
	limit := 10
	buffer := NewBoundedBuffer(limit)

	input := []byte("1234567890") // 10 bytes
	n, err := buffer.Write(input)

	require.NoError(t, err)
	assert.Equal(t, 10, n)
	assert.False(t, buffer.Truncated, "Should not be truncated")
	assert.Equal(t, "1234567890", buffer.String())

	// Write one more byte
	n, err = buffer.Write([]byte("1"))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.True(t, buffer.Truncated, "Should be truncated after writing more")
	assert.Equal(t, "1234567890", buffer.String())
}
