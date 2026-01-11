package embedded

import (
	"testing"
)

func TestEmbeddedSource(t *testing.T) {
	// Since we rely on global embed.FS and there might be no plugins in the test environment,
	// this test is basic sanity check.
	// We assume there are NO real plugins committed in the repo for now, or at least we don't depend on specific names.

	source := NewEmbeddedSource()

	t.Run("Get_NonExistent", func(t *testing.T) {
		got := source.Get("non-existent-plugin")
		if got != "" {
			t.Errorf("Get should return empty string for missing plugin, got %s", got)
		}
	})

	t.Run("List_Safe", func(t *testing.T) {
		list := source.List()
		// Just ensure it doesn't panic and returns a slice (possibly empty)
		if list == nil {
			t.Error("List should return non-nil slice")
		}
	})
}
