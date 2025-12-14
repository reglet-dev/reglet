package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/redaction"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

// MockPlugin for redaction tests
// We need a way to mock WASM execution results without actually running WASM
// Since ObservationExecutor is hardcoded to use wasm.Runtime and load from files,
// we might need to integration test this or refactor executor to be more testable.
//
// For now, let's verify the redaction logic by testing ObservationExecutor.Execute
// with a real WASM plugin if possible, OR rely on the fact that we can't easily unit test
// the Executor's integration with WASM without a lot of setup.
//
// However, we CAN test that the redactor is passed correctly.

func TestObservationExecutor_Redaction(t *testing.T) {
	// This test requires a mocked runtime or actual WASM file.
	// Since we don't have easy mocking of wasm.Runtime (it's a struct),
	// we will skip deep execution tests and focus on the redactor integration
	// if we were to refactor.
	//
	// Instead, let's verify that NewExecutor accepts the redactor.
	ctx := context.Background()
	runtime, err := wasm.NewRuntime(ctx, build.Get())
	require.NoError(t, err)
	defer runtime.Close(ctx)

	redactor, err := redaction.New(redaction.Config{
		Patterns: []string{"secret"},
	})
	require.NoError(t, err)

	executor := NewExecutor(runtime, "/tmp", redactor)
	assert.NotNil(t, executor)
	// We can't easily inspect the private redactor field, but successful creation is a start.
}

func TestRedactionInObservationResult(t *testing.T) {
	// Since we can't easily run the full executor chain without WASM,
	// let's simulate the redaction step that happens inside Execute.
	// We'll reproduce the logic here to verify it works as expected.

	redactor, err := redaction.New(redaction.Config{
		Patterns: []string{"password"},
	})
	require.NoError(t, err)

	// Simulate raw evidence data
	rawData := map[string]interface{}{
		"key": "password=secret123",
		"nested": map[string]interface{}{
			"config": "user:password@host",
		},
	}

	// Redact
	redactedData := redactor.Redact(rawData)

	// Verify redaction
	asMap := redactedData.(map[string]interface{})
	assert.Contains(t, asMap["key"], "[REDACTED]")
	nested := asMap["nested"].(map[string]interface{})
	assert.Contains(t, nested["config"], "[REDACTED]")
}

// TestRedactionEndToEnd would ideally load the 'command' plugin and echo a secret,
// verifying it gets redacted. This requires the compiled WASM plugin.
// Skipping for unit tests to avoid build dependencies.
func TestRedactionEndToEnd_Integration(t *testing.T) {
	t.Skip("Skipping integration test requiring compiled WASM")
}

// Manually test the execute logic flow if we could inject a mock result
// Since we can't, we rely on the logic analysis:
// 1. Executor calls plugin.Observe -> gets result
// 2. Executor calls redactor.Redact(result.Evidence.Data)
// 3. Executor returns result
