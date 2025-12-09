package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
)

// GenerateSchema creates a JSON schema from a Go struct.
// It uses the `invopop/jsonschema` library to reflect on the struct
// and generate a standard JSON Schema (Draft 2020-12).
func GenerateSchema(v interface{}) ([]byte, error) {
	reflector := jsonschema.Reflector{
		ExpandedStruct: true, // Expand struct definitions inline
	}
	schema := reflector.Reflect(v)

	jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	return jsonBytes, nil
}
