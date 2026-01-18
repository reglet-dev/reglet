package netutil

import (
	"fmt"
	"io"
)

// LimitedReader wraps an io.Reader with a maximum size limit.
// It returns an error when the limit is exceeded.
type LimitedReader struct {
	R     io.Reader // underlying reader
	N     int64     // max bytes remaining
	Limit int64     // original limit (for error messages)
	read  int64     // bytes read so far
}

// NewLimitedReader creates a new LimitedReader that will read at most limit bytes.
func NewLimitedReader(r io.Reader, limit int64) *LimitedReader {
	return &LimitedReader{
		R:     r,
		N:     limit,
		Limit: limit,
	}
}

// Read implements io.Reader with size limit enforcement.
func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, &SizeLimitExceededError{Limit: l.Limit, Read: l.read}
	}

	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}

	n, err = l.R.Read(p)
	l.N -= int64(n)
	l.read += int64(n)

	// Check if we hit the limit exactly
	if l.N == 0 && err == nil {
		// Try to read one more byte to check if there's more data
		var buf [1]byte
		extra, extraErr := l.R.Read(buf[:])
		if extra > 0 {
			return n, &SizeLimitExceededError{Limit: l.Limit, Read: l.read + 1}
		}
		if extraErr != nil && extraErr != io.EOF {
			return n, extraErr
		}
	}

	return n, err
}

// BytesRead returns the number of bytes read so far.
func (l *LimitedReader) BytesRead() int64 {
	return l.read
}

// SizeLimitExceededError is returned when the size limit is exceeded.
type SizeLimitExceededError struct {
	Limit int64
	Read  int64
}

func (e *SizeLimitExceededError) Error() string {
	return fmt.Sprintf("size limit exceeded: read %d bytes, limit is %d bytes", e.Read, e.Limit)
}

// IsSizeLimitExceededError returns true if the error is a SizeLimitExceededError.
func IsSizeLimitExceededError(err error) bool {
	_, ok := err.(*SizeLimitExceededError)
	return ok
}

// FormatSize returns a human-readable size string.
func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
