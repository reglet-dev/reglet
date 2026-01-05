package output

import (
	"bytes"
	"testing"
	"time"

	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
)

// FuzzSARIFGeneration fuzzes SARIF output generation
func FuzzSARIFGeneration(f *testing.F) {
	seeds := []string{
		"test output",
		"",
		"Control ID: 123",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, message string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input %q: %v", message, r)
			}
		}()

		// Construct a minimal ExecutionResult
		res := execution.NewExecutionResult("test-profile", "1.0.0")

		ctrl := execution.ControlResult{
			ID:       "ctrl-1",
			Status:   values.StatusFail,
			Message:  message,
			Duration: 100 * time.Millisecond,
		}
		res.AddControlResult(ctrl)
		res.Finalize()

		// Format as SARIF
		buf := &bytes.Buffer{}
		formatter := NewSARIFFormatter(buf, "profile.yaml")
		_ = formatter.Format(res)
	})
}
