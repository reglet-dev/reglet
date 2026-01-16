package loopexpander

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteLoopInString_Simple(t *testing.T) {
	loopCtx := &Context{
		Item:   "/etc/passwd",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
	}

	result := SubstituteLoopInString("{{ .loop.item }}", loopCtx, "")
	assert.Equal(t, "/etc/passwd", result)
}

func TestSubstituteLoopInString_MapItem(t *testing.T) {
	loopCtx := &Context{
		Item:   map[string]interface{}{"path": "/etc/ssh/sshd_config", "name": "ssh"},
		Index:  0,
		First:  true,
		Last:   false,
		Length: 2,
	}

	// Custom name should work
	result := SubstituteLoopInString("{{ .svc.path }}", loopCtx, "svc")
	assert.Equal(t, "/etc/ssh/sshd_config", result)
}

func TestSubstituteLoopInString_Index(t *testing.T) {
	loopCtx := &Context{
		Item:   "test",
		Index:  5,
		First:  false,
		Last:   false,
		Length: 10,
	}

	result := SubstituteLoopInString("item-{{ .loop.index }}", loopCtx, "")
	assert.Equal(t, "item-5", result)
}

func TestSubstituteLoopInString_FirstLast(t *testing.T) {
	loopCtx := &Context{
		Item:   "test",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
	}

	result := SubstituteLoopInString("first={{ .loop.first }} last={{ .loop.last }}", loopCtx, "")
	assert.Equal(t, "first=true last=false", result)
}

func TestSubstituteLoopInMap(t *testing.T) {
	loopCtx := &Context{
		Item:   "/etc/passwd",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
	}

	config := map[string]interface{}{
		"path": "{{ .loop.item }}",
	}

	result := SubstituteLoopInMap(config, loopCtx, "")
	assert.Equal(t, "/etc/passwd", result["path"])
}

func TestSubstituteLoopInMap_Nested(t *testing.T) {
	loopCtx := &Context{
		Item:   "value",
		Index:  2,
		First:  false,
		Last:   true,
		Length: 3,
	}

	config := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "{{ .loop.item }}",
			"index": "{{ .loop.index }}",
		},
		"array": []interface{}{
			"{{ .loop.item }}",
			"static",
		},
	}

	result := SubstituteLoopInMap(config, loopCtx, "")

	outer := result["outer"].(map[string]interface{})
	assert.Equal(t, "value", outer["inner"])
	assert.Equal(t, "2", outer["index"])

	array, ok := result["array"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, "value", array[0])
	assert.Equal(t, "static", array[1])
}

func TestResolveLoopItems_SimpleList(t *testing.T) {
	vars := map[string]interface{}{
		"config_files": []interface{}{"/etc/passwd", "/etc/group", "/etc/hosts"},
	}

	items, err := ResolveLoopItems("{{ .vars.config_files }}", vars)
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "/etc/passwd", items[0])
	assert.Equal(t, "/etc/group", items[1])
	assert.Equal(t, "/etc/hosts", items[2])
}

func TestResolveLoopItems_StringSlice(t *testing.T) {
	vars := map[string]interface{}{
		"paths": []string{"/tmp", "/var", "/opt"},
	}

	items, err := ResolveLoopItems("{{ .vars.paths }}", vars)
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "/tmp", items[0])
}

func TestResolveLoopItems_ListOfMaps(t *testing.T) {
	vars := map[string]interface{}{
		"services": []interface{}{
			map[string]interface{}{"name": "ssh", "path": "/etc/ssh/sshd_config"},
			map[string]interface{}{"name": "resolv", "path": "/etc/resolv.conf"},
		},
	}

	items, err := ResolveLoopItems("{{ .vars.services }}", vars)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	first, ok := items[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/etc/ssh/sshd_config", first["path"])
}

func TestResolveLoopItems_NestedVar(t *testing.T) {
	vars := map[string]interface{}{
		"config": map[string]interface{}{
			"files": []interface{}{"a", "b", "c"},
		},
	}

	items, err := ResolveLoopItems("{{ .vars.config.files }}", vars)
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "a", items[0])
}

func TestResolveLoopItems_InvalidExpression(t *testing.T) {
	vars := map[string]interface{}{}

	_, err := ResolveLoopItems("invalid", vars)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid loop items expression")
}

func TestResolveLoopItems_NotFound(t *testing.T) {
	vars := map[string]interface{}{}

	_, err := ResolveLoopItems("{{ .vars.missing }}", vars)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "variable not found")
}

func TestResolveLoopItems_NotAList(t *testing.T) {
	vars := map[string]interface{}{
		"value": "string",
	}

	_, err := ResolveLoopItems("{{ .vars.value }}", vars)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a list")
}

func TestLookupNestedVar_Simple(t *testing.T) {
	vars := map[string]interface{}{
		"key": "value",
	}

	result, err := LookupNestedVar(vars, "key")
	require.NoError(t, err)
	assert.Equal(t, "value", result)
}

func TestLookupNestedVar_Nested(t *testing.T) {
	vars := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"key": "deep-value",
			},
		},
	}

	result, err := LookupNestedVar(vars, "level1.level2.key")
	require.NoError(t, err)
	assert.Equal(t, "deep-value", result)
}

func TestLookupNestedVar_NotFound(t *testing.T) {
	vars := map[string]interface{}{
		"key": "value",
	}

	_, err := LookupNestedVar(vars, "missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "variable not found")
}

func TestLookupNestedVar_NotAMap(t *testing.T) {
	vars := map[string]interface{}{
		"key": "value",
	}

	_, err := LookupNestedVar(vars, "key.nested")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a map")
}
