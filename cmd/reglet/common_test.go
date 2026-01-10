package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonOptions_ApplyToContext(t *testing.T) {
	t.Parallel()

	t.Run("with timeout", func(t *testing.T) {
		t.Parallel()
		opts := CommonOptions{Timeout: 100 * time.Millisecond}
		ctx, cancel := opts.ApplyToContext(context.Background())
		defer cancel()

		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(100*time.Millisecond), deadline, 10*time.Millisecond)
	})

	t.Run("no timeout", func(t *testing.T) {
		t.Parallel()
		opts := CommonOptions{Timeout: 0}
		ctx, cancel := opts.ApplyToContext(context.Background())
		defer cancel()

		_, ok := ctx.Deadline()
		assert.False(t, ok)
	})
}

func TestCommonOptions_ValidateFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    CommonOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options",
			opts: CommonOptions{
				Format:  "table",
				Verbose: false,
				Quiet:   false,
			},
			wantErr: false,
		},
		{
			name: "verbose and quiet",
			opts: CommonOptions{
				Format:  "table",
				Verbose: true,
				Quiet:   true,
			},
			wantErr: true,
			errMsg:  "mutually exclusive",
		},
		{
			name: "invalid format",
			opts: CommonOptions{
				Format: "xml",
			},
			wantErr: true,
			errMsg:  "invalid format",
		},
		{
			name: "valid format json",
			opts: CommonOptions{
				Format: "json",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.opts.ValidateFlags()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultCommonOptions(t *testing.T) {
	t.Parallel()
	opts := DefaultCommonOptions()
	assert.Equal(t, 2*time.Minute, opts.Timeout)
	assert.Equal(t, "table", opts.Format)
	assert.True(t, opts.Parallel)
}
