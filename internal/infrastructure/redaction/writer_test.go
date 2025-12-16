package redaction

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_WithRedactor(t *testing.T) {
	// Create a redactor that scrubs "secret" and "password"
	redactor, err := New(Config{
		Patterns: []string{`secret`, `password`},
	})
	require.NoError(t, err)

	// Create buffer to capture output
	buf := &bytes.Buffer{}

	// Create redacting writer
	writer := NewWriter(buf, redactor)

	// Write data containing secrets
	testData := []byte("Connecting with secret credentials and password=12345")
	n, err := writer.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n, "should return original length")

	// Verify output was redacted
	output := buf.String()
	assert.Contains(t, output, "[REDACTED]")
	assert.NotContains(t, output, "secret")
	assert.NotContains(t, output, "password")
}

func TestWriter_WithoutRedactor(t *testing.T) {
	// Create buffer to capture output
	buf := &bytes.Buffer{}

	// Create writer with nil redactor (pass-through mode)
	writer := NewWriter(buf, nil)

	// Write data
	testData := []byte("This contains secret data")
	n, err := writer.Write(testData)
	require.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// Verify output was NOT redacted (pass-through)
	output := buf.String()
	assert.Equal(t, string(testData), output)
}

func TestWriter_MultipleWrites(t *testing.T) {
	// Create redactor
	redactor, err := New(Config{
		Patterns: []string{`API_KEY=[A-Za-z0-9]+`},
	})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	writer := NewWriter(buf, redactor)

	// Write multiple times
	writes := []string{
		"First line with API_KEY=abc123\n",
		"Second line is clean\n",
		"Third line with API_KEY=xyz789\n",
	}

	for _, data := range writes {
		n, err := writer.Write([]byte(data))
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
	}

	// Verify all API keys were redacted
	output := buf.String()
	assert.NotContains(t, output, "abc123")
	assert.NotContains(t, output, "xyz789")
	assert.Contains(t, output, "[REDACTED]")
	assert.Contains(t, output, "Second line is clean")
}

func TestWriter_ThreadSafety(t *testing.T) {
	// Create redactor
	redactor, err := New(Config{
		Patterns: []string{`secret`},
	})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	writer := NewWriter(buf, redactor)

	// Write from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				_, _ = writer.Write([]byte("secret data\n"))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no secrets leaked
	output := buf.String()
	assert.NotContains(t, output, "secret data")
	assert.Contains(t, output, "[REDACTED]")
}

func TestWriter_EmptyWrite(t *testing.T) {
	redactor, err := New(Config{
		Patterns: []string{`secret`},
	})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	writer := NewWriter(buf, redactor)

	// Write empty data
	n, err := writer.Write([]byte(""))
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, "", buf.String())
}
