package entities

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_CheckForControlDependencyCycles_DeepChain verifies that the cycle detection
// can handle deep dependency chains without causing a stack overflow.
// This is a regression test for the move from recursive to iterative DFS.
func Test_CheckForControlDependencyCycles_DeepChain(t *testing.T) {
	// 50,000 is usually enough to blow the default 1GB stack limit if using inefficient recursion,
	// but default goroutine stack is small (2KB), growing up to 1GB (64-bit).
	// deeply nested recursion of 50k frames might exceed limits or just be very slow/memory hungry.
	// On many systems, default stack max is 8MB or so (ulimit -s). Go handling is dynamic.
	// 20,000 is often enough to trigger stack issues in some environments or verify robustness.
	const chainLength = 100000

	config := Profile{
		Metadata: ProfileMetadata{
			Name:    "Deep Chain",
			Version: "1.0",
		},
		Controls: ControlsSection{
			Items: make([]Control, chainLength),
		},
	}

	for i := 0; i < chainLength; i++ {
		id := fmt.Sprintf("c-%d", i)
		ctrl := Control{
			ID:   id,
			Name: fmt.Sprintf("Control %d", i),
			ObservationDefinitions: []ObservationDefinition{
				{Plugin: "dummy"},
			},
		}

		if i > 0 {
			// Depends on previous one: c-1 <- c-2 <- ...
			prevID := fmt.Sprintf("c-%d", i-1)
			ctrl.DependsOn = []string{prevID}
		}
		config.Controls.Items[i] = ctrl
	}

	// Validate invokes CheckForControlDependencyCycles
	err := config.Validate()
	require.NoError(t, err)

	// Also verify a cycle at the end of a deep chain is caught
	// c-0 <- ... <- c-19999 <- c-0
	config.Controls.Items[0].DependsOn = []string{fmt.Sprintf("c-%d", chainLength-1)}
	err = config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}
