package execution

import (
	"encoding/json"
	"fmt"
)

// TruncationStrategy defines how evidence should be truncated when it exceeds limits.
type TruncationStrategy interface {
	Truncate(data map[string]interface{}, limit int) (map[string]interface{}, *EvidenceMeta, error)
}

// GreedyTruncator implements a simple greedy truncation strategy.
// It truncates large string fields or replaces large complex objects until the size is reduced.
type GreedyTruncator struct{}

// Truncate returns a truncated copy of the evidence if it exceeds the limit.
func (t *GreedyTruncator) Truncate(data map[string]interface{}, limit int) (map[string]interface{}, *EvidenceMeta, error) {
	if limit <= 0 {
		return data, nil, nil // No limit
	}

	// 1. Serialize to measure size
	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to measure evidence size: %w", err)
	}

	originalSize := len(serialized)
	if originalSize <= limit {
		return data, nil, nil // Within limit
	}

	// 2. Deep copy via round-trip to avoid mutating original
	var truncated map[string]interface{}
	if err := json.Unmarshal(serialized, &truncated); err != nil {
		return nil, nil, fmt.Errorf("failed to deep copy evidence: %w", err)
	}

	// 3. Truncate fields
	threshold := limit / 2 // simple heuristic

	for key, value := range truncated {
		switch v := value.(type) {
		case string:
			if len(v) > threshold {
				truncated[key] = v[:threshold] + "\n... [TRUNCATED] ..."
			}
		default:
			// For complex objects, re-serialize to check size
			vBytes, _ := json.Marshal(v)
			if len(vBytes) > threshold {
				truncated[key] = map[string]string{
					"_truncated": "value exceeded size limit",
					"_type":      fmt.Sprintf("%T", v),
				}
			}
		}
	}

	return truncated, &EvidenceMeta{
		Truncated:    true,
		OriginalSize: originalSize,
		TruncatedAt:  limit,
		Reason:       fmt.Sprintf("evidence exceeded %d bytes limit (greedy strategy)", limit),
	}, nil
}
