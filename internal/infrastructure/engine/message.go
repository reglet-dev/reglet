package engine

import (
	"fmt"

	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// generateControlMessage generates a human-readable message for the control result.
func generateControlMessage(status values.Status, observations []execution.ObservationResult) string {
	switch status {
	case values.StatusPass:
		if len(observations) == 1 {
			return "Check passed"
		}
		return fmt.Sprintf("All %d checks passed", len(observations))

	case values.StatusFail:
		failCount := 0
		for _, obs := range observations {
			if obs.Status == values.StatusFail {
				failCount++
			}
		}
		if failCount == 1 {
			return "1 check failed"
		}
		return fmt.Sprintf("%d checks failed", failCount)

	case values.StatusError:
		errorCount := 0
		for _, obs := range observations {
			if obs.Status == values.StatusError {
				errorCount++
			}
		}
		if errorCount == 1 {
			for _, obs := range observations {
				if obs.Status == values.StatusError && obs.Error != nil {
					return obs.Error.Message
				}
			}
			return "Check encountered an error"
		}
		return fmt.Sprintf("%d checks encountered errors", errorCount)

	case values.StatusSkipped:
		return "Skipped due to failed dependency"

	default:
		return "Unknown status"
	}
}
