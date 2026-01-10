package sensitivedata

import (
	"io"
	"sync"
)

// Writer wraps an io.Writer and redacts all data before writing.
// Thread-safe: can be used concurrently by multiple goroutines.
type Writer struct {
	underlying io.Writer
	redactor   *Redactor
	mu         sync.Mutex // Protects writes to underlying writer
}

// NewWriter creates a redacting writer that scrubs sensitive patterns.
func NewWriter(w io.Writer, r *Redactor) *Writer {
	return &Writer{
		underlying: w,
		redactor:   r,
	}
}

// Write implements io.Writer, redacting data before passing to underlying writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	// If no redactor configured, pass through unchanged
	if w.redactor == nil {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.underlying.Write(p)
	}

	// Convert to string, redact, convert back to bytes
	original := string(p)
	redacted := w.redactor.ScrubString(original)
	redactedBytes := []byte(redacted)

	// Write redacted content (thread-safe)
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err = w.underlying.Write(redactedBytes)

	// Return original length to caller (io.Writer contract expects len(p))
	// This prevents short write errors even if redacted length differs
	if err == nil {
		n = len(p)
	}

	return n, err
}
