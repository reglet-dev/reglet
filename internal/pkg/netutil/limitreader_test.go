package netutil_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/internal/pkg/netutil"
)

func Test_LimitedReader_EnforcesLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		limit     int64
		wantError bool
		wantBytes int64
	}{
		{
			name:      "content under limit",
			content:   "hello",
			limit:     10,
			wantError: false,
			wantBytes: 5,
		},
		{
			name:      "content at limit - errors because overflow check reads +1",
			content:   "hello",
			limit:     5,
			wantError: true, // LimitedReader reads +1 byte to detect overflow
			wantBytes: 5,
		},
		{
			name:      "content over limit",
			content:   "hello world",
			limit:     5,
			wantError: true,
			wantBytes: 6, // reads up to limit + 1 to detect overflow
		},
		{
			name:      "empty content",
			content:   "",
			limit:     10,
			wantError: false,
			wantBytes: 0,
		},
		{
			name:      "zero limit blocks all",
			content:   "hello",
			limit:     0,
			wantError: true,
			wantBytes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := netutil.NewLimitedReader(strings.NewReader(tt.content), tt.limit)
			_, err := io.ReadAll(reader)

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if !netutil.IsSizeLimitExceededError(err) {
					t.Errorf("expected SizeLimitExceededError, got %T: %v", err, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func Test_LimitedReader_StreamingEnforcement(t *testing.T) {
	t.Parallel()

	// Create a reader larger than limit
	content := bytes.Repeat([]byte("x"), 1000)
	limit := int64(100)

	reader := netutil.NewLimitedReader(bytes.NewReader(content), limit)

	// Read in small chunks
	buf := make([]byte, 10)
	var totalRead int64
	var hitError bool

	for {
		n, err := reader.Read(buf)
		totalRead += int64(n)
		if err != nil {
			if netutil.IsSizeLimitExceededError(err) {
				hitError = true
			}
			break
		}
	}

	if !hitError {
		t.Error("expected SizeLimitExceededError during streaming read")
	}

	if totalRead > limit+1 {
		t.Errorf("read too many bytes: got %d, limit was %d", totalRead, limit)
	}
}

func Test_SizeLimitExceededError_Message(t *testing.T) {
	t.Parallel()

	err := &netutil.SizeLimitExceededError{Limit: 1024, Read: 2048}
	msg := err.Error()

	if !strings.Contains(msg, "1024") || !strings.Contains(msg, "2048") {
		t.Errorf("error message missing limit/read values: %s", msg)
	}
}

func Test_FormatSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 bytes"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := netutil.FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
