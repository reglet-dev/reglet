package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
)

// ValidateConfig validates a Config map against a struct with validation tags.
// It first marshals the map to JSON, then unmarshals it into the target struct,
// and finally runs the validator on the struct.
func ValidateConfig(config Config, targetStruct interface{}) error {
	// 1. Convert map[string]interface{} to JSON bytes
	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config map: %w", err)
	}

	// 2. Unmarshal JSON bytes into the target struct
	if err := json.Unmarshal(jsonBytes, targetStruct); err != nil {
		return fmt.Errorf("failed to unmarshal config into struct: %w", err)
	}

	// 3. Validate the struct using go-playground/validator
	validate := validator.New()
	if err := validate.Struct(targetStruct); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}
