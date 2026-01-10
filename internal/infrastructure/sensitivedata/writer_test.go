package sensitivedata_test

import (
	"bytes"
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_WithRedactor(t *testing.T) {
	// Setup
	var buf bytes.Buffer
	redactor, err := sensitivedata.New(sensitivedata.Config{
		Patterns: []string{"secret"},
	})
	require.NoError(t, err)

	writer := sensitivedata.NewWriter(&buf, redactor)

	n, err := writer.Write([]byte("This is a secret."))
	require.NoError(t, err)

	assert.Equal(t, len("This is a secret."), n)

	assert.Equal(t, "This is a [REDACTED].", buf.String())
}

func TestWriter_WithoutRedactor(t *testing.T) {
	var buf bytes.Buffer
	writer := sensitivedata.NewWriter(&buf, nil)

	input := "This is a secret."
	n, err := writer.Write([]byte(input))
	require.NoError(t, err)

	assert.Equal(t, len(input), n)
	assert.Equal(t, input, buf.String())
}

func TestWriter_MultipleWrites(t *testing.T) {
	// Setup
	var buf bytes.Buffer
	redactor, err := sensitivedata.New(sensitivedata.Config{
		Patterns: []string{"secret"},
	})
	require.NoError(t, err)

	writer := sensitivedata.NewWriter(&buf, redactor)

	texts := []string{
		"Part 1 with secret.\n",
		"Part 2 is safe.\n",
		"Part 3 has another secret.\n",
	}

	for _, text := range texts {
		n, err := writer.Write([]byte(text))
		require.NoError(t, err)
		assert.Equal(t, len(text), n)
	}

	expected := "Part 1 with [REDACTED].\nPart 2 is safe.\nPart 3 has another [REDACTED].\n"
	assert.Equal(t, expected, buf.String())
}
