package capabilities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrant_NewGrant(t *testing.T) {
	g := NewGrant()
	assert.Empty(t, g)
}

func TestGrant_Add(t *testing.T) {
	g := NewGrant()
	cap1 := Capability{Kind: "fs", Pattern: "read:/etc/passwd"}
	cap2 := Capability{Kind: "network", Pattern: "outbound:80"}

	g.Add(cap1)
	require.Len(t, g, 1)
	assert.Equal(t, cap1, g[0])

	g.Add(cap2)
	require.Len(t, g, 2)
	assert.Equal(t, cap2, g[1])

	// Adding duplicate should not change length
	g.Add(cap1)
	require.Len(t, g, 2)
}

func TestGrant_Contains(t *testing.T) {
	cap1 := Capability{Kind: "fs", Pattern: "read:/etc/passwd"}
	cap2 := Capability{Kind: "network", Pattern: "outbound:80"}
	g := Grant{cap1, cap2}

	assert.True(t, g.Contains(cap1))
	assert.True(t, g.Contains(cap2))
	assert.False(t, g.Contains(Capability{Kind: "fs", Pattern: "read:/etc/hosts"}))
}

func TestGrant_ContainsAny(t *testing.T) {
	cap1 := Capability{Kind: "fs", Pattern: "read:/etc/passwd"}
	cap2 := Capability{Kind: "network", Pattern: "outbound:80"}
	cap3 := Capability{Kind: "exec", Pattern: "/bin/sh"}
	g := Grant{cap1, cap2}

	assert.True(t, g.ContainsAny([]Capability{cap1, cap3}))
	assert.True(t, g.ContainsAny([]Capability{cap2}))
	assert.False(t, g.ContainsAny([]Capability{cap3}))
	assert.False(t, g.ContainsAny([]Capability{}))
}

func TestGrant_Remove(t *testing.T) {
	cap1 := Capability{Kind: "fs", Pattern: "read:/etc/passwd"}
	cap2 := Capability{Kind: "network", Pattern: "outbound:80"}
	cap3 := Capability{Kind: "exec", Pattern: "/bin/sh"}
	g := Grant{cap1, cap2, cap3}

	g.Remove(cap2)
	require.Len(t, g, 2)
	assert.False(t, g.Contains(cap2))
	assert.True(t, g.Contains(cap1))
	assert.True(t, g.Contains(cap3))

	// Removing non-existent cap should not change length
	g.Remove(Capability{Kind: "fs", Pattern: "read:/etc/hosts"})
	require.Len(t, g, 2)
}
