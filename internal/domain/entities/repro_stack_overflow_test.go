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
