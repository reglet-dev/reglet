package hostfuncs

import (
	"context"
	"testing"

	"github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet-hostlib"
	"github.com/stretchr/testify/assert"
)

// TestIsAlwaysBlockedEnv verifies always-blocked environment variable detection
func TestIsAlwaysBlockedEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		// Linux linker injection
		{"LD_PRELOAD", "LD_PRELOAD", true},
		{"LD_LIBRARY_PATH", "LD_LIBRARY_PATH", true},
		{"LD_AUDIT", "LD_AUDIT", true},
		{"LD_DEBUG", "LD_DEBUG", true},
		{"LD_BIND_NOW", "LD_BIND_NOW", true},

		// macOS linker injection
		{"DYLD_INSERT_LIBRARIES", "DYLD_INSERT_LIBRARIES", true},
		{"DYLD_LIBRARY_PATH", "DYLD_LIBRARY_PATH", true},
		{"DYLD_FRAMEWORK_PATH", "DYLD_FRAMEWORK_PATH", true},

		// Shell behavior modifiers
		{"IFS", "IFS", true},
		{"BASH_ENV", "BASH_ENV", true},
		{"ENV", "ENV", true},
		{"LOCPATH", "LOCPATH", true},

		// Safe variables (not blocked)
		{"HOME", "HOME", false},
		{"PATH", "PATH", false}, // This is capability-gated, not blocked
		{"USER", "USER", false},
		{"LANG", "LANG", false},
		{"TERM", "TERM", false},
		{"MY_CUSTOM_VAR", "MY_CUSTOM_VAR", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostlib.IsAlwaysBlockedEnv(tt.key)
			assert.Equal(t, tt.expected, result, "key: %s", tt.key)
		})
	}
}

// TestSanitizeEnv_AlwaysBlocked tests that always-blocked vars are filtered
func TestSanitizeEnv_AlwaysBlocked(t *testing.T) {
	ctx := context.Background()
	// Empty capability map - no grants - capGetter always returns false
	capGetter := func(pluginName, capability string) bool { return false }

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			"LD_PRELOAD blocked",
			[]string{"SAFE=value", "LD_PRELOAD=/evil.so", "OTHER=ok"},
			[]string{"SAFE=value", "OTHER=ok"},
		},
		{
			"Multiple LD_ vars blocked",
			[]string{"LD_PRELOAD=/evil.so", "LD_LIBRARY_PATH=/evil", "SAFE=ok"},
			[]string{"SAFE=ok"},
		},
		{
			"DYLD vars blocked",
			[]string{"DYLD_INSERT_LIBRARIES=/evil.dylib", "KEEP=this"},
			[]string{"KEEP=this"},
		},
		{
			"IFS blocked",
			[]string{"IFS=;", "VALID=yes"},
			[]string{"VALID=yes"},
		},
		{
			"Case insensitive blocking",
			[]string{"ld_preload=/evil.so", "Ld_Library_Path=/bad"},
			[]string{},
		},
		{
			"All safe vars pass through",
			[]string{"FOO=bar", "BAZ=qux", "TERM=xterm"},
			[]string{"FOO=bar", "BAZ=qux", "TERM=xterm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostlib.SanitizeEnv(ctx, tt.input, "test-plugin", capGetter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSanitizeEnv_CapabilityGated tests capability-gated variables
func TestSanitizeEnv_CapabilityGated(t *testing.T) {
	ctx := context.Background()

	t.Run("PATH blocked without capability", func(t *testing.T) {
		// No grants - capGetter always returns false
		capGetter := func(pluginName, capability string) bool { return false }
		input := []string{"PATH=/usr/bin", "SAFE=value"}
		result := hostlib.SanitizeEnv(ctx, input, "test-plugin", capGetter)
		assert.Equal(t, []string{"SAFE=value"}, result)
	})

	t.Run("PATH allowed with exec:env:PATH capability", func(t *testing.T) {
		checker := NewCapabilityChecker(map[string]*entities.GrantSet{
			"test-plugin": {
				Exec: &entities.ExecCapability{
					Commands: []string{"env:PATH"},
				},
			},
		})
		capGetter := checker.ToCapabilityGetter("test-plugin")

		input := []string{"PATH=/usr/bin", "SAFE=value"}
		result := hostlib.SanitizeEnv(ctx, input, "test-plugin", capGetter)
		assert.Equal(t, []string{"PATH=/usr/bin", "SAFE=value"}, result)
	})

	t.Run("PYTHONPATH blocked without capability", func(t *testing.T) {
		capGetter := func(pluginName, capability string) bool { return false }
		input := []string{"PYTHONPATH=/evil", "OK=yes"}
		result := hostlib.SanitizeEnv(ctx, input, "test-plugin", capGetter)
		assert.Equal(t, []string{"OK=yes"}, result)
	})

	t.Run("PYTHONPATH allowed with capability", func(t *testing.T) {
		checker := NewCapabilityChecker(map[string]*entities.GrantSet{
			"test-plugin": {
				Exec: &entities.ExecCapability{
					Commands: []string{"env:PYTHONPATH"},
				},
			},
		})
		capGetter := checker.ToCapabilityGetter("test-plugin")

		input := []string{"PYTHONPATH=/custom/lib", "FOO=bar"}
		result := hostlib.SanitizeEnv(ctx, input, "test-plugin", capGetter)
		assert.Equal(t, []string{"PYTHONPATH=/custom/lib", "FOO=bar"}, result)
	})

	t.Run("Multiple gated vars with partial grants", func(t *testing.T) {
		// Only PATH granted
		checker := NewCapabilityChecker(map[string]*entities.GrantSet{
			"test-plugin": {
				Exec: &entities.ExecCapability{
					Commands: []string{"env:PATH"},
				},
			},
		})
		capGetter := checker.ToCapabilityGetter("test-plugin")

		input := []string{"PATH=/bin", "NODE_OPTIONS=--debug", "HOME=/root"}
		result := hostlib.SanitizeEnv(ctx, input, "test-plugin", capGetter)
		// PATH allowed (granted), NODE_OPTIONS blocked (not granted), HOME blocked (not granted)
		assert.Equal(t, []string{"PATH=/bin"}, result)
	})
}

// TestSanitizeEnv_MalformedVars tests handling of malformed/edge cases
func TestSanitizeEnv_MalformedVars(t *testing.T) {
	ctx := context.Background()
	capGetter := func(pluginName, capability string) bool { return false }

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			"Malformed var without equals",
			[]string{"NOEQUALSSIGN", "VALID=yes"},
			[]string{"VALID=yes"},
		},
		{
			"Empty value is valid",
			[]string{"EMPTY=", "ANOTHER=value"},
			[]string{"EMPTY=", "ANOTHER=value"},
		},
		{
			"Value with equals sign",
			[]string{"COMPLEX=foo=bar=baz"},
			[]string{"COMPLEX=foo=bar=baz"},
		},
		{
			"Empty input",
			[]string{},
			[]string{},
		},
		{
			"Nil input",
			nil,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostlib.SanitizeEnv(ctx, tt.input, "test-plugin", capGetter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsKnownInterpreter verifies interpreter detection
func TestIsKnownInterpreter(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected bool
	}{
		// Python family
		{"python", "python", true},
		{"python2", "python2", true},
		{"python3", "python3", true},
		{"python3.11", "python3.11", true},
		{"python path", "/usr/bin/python", true},

		// Perl
		{"perl", "perl", true},
		{"perl path", "/usr/bin/perl", true},

		// Ruby
		{"ruby", "ruby", true},
		{"irb", "irb", true},

		// Node
		{"node", "node", true},
		{"nodejs", "nodejs", true},

		// PHP
		{"php", "php", true},
		{"php8", "php8", true},

		// Lua
		{"lua", "lua", true},
		{"lua5.4", "lua5.4", true},

		// AWK family
		{"awk", "awk", true},
		{"gawk", "gawk", true},
		{"mawk", "mawk", true},

		// Other interpreters
		{"tclsh", "tclsh", true},
		{"expect", "expect", true},

		// Not interpreters
		{"ls", "ls", false},
		{"grep", "grep", false},
		{"systemctl", "systemctl", false},
		{"custom-binary", "custom-binary", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostlib.IsKnownInterpreter(tt.command)
			assert.Equal(t, tt.expected, result, "command: %s", tt.command)
		})
	}
}

// TestIsDangerousExecution verifies detection of dangerous execution patterns
func TestIsDangerousExecution(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		expected bool
	}{
		// Python dangerous
		{"python -c", "python", []string{"-c", "print('test')"}, true},
		{"python3 -c", "python3", []string{"-c", "malicious"}, true},
		{"python path -c", "/usr/bin/python", []string{"-c", "code"}, true},

		// Python safe
		{"python script", "python", []string{"/path/to/script.py"}, false},
		{"python module", "python", []string{"-m", "pytest"}, false},

		// Perl dangerous
		{"perl -e", "perl", []string{"-e", "print 'test'"}, true},

		// Perl safe
		{"perl script", "perl", []string{"script.pl"}, false},

		// Ruby dangerous
		{"ruby -e", "ruby", []string{"-e", "puts 'test'"}, true},

		// Ruby safe
		{"ruby script", "ruby", []string{"script.rb"}, false},

		// Node dangerous
		{"node -e", "node", []string{"-e", "console.log(1)"}, true},
		{"node --eval", "node", []string{"--eval", "process.exit()"}, true},

		// Node safe
		{"node script", "node", []string{"index.js"}, false},

		// Shell with args
		{"bash -c", "bash", []string{"-c", "echo test"}, true},
		{"sh -c", "sh", []string{"-c", "ls"}, true},

		// Shell without args (safe)
		{"bare bash", "bash", []string{}, false},

		// Safe commands
		{"ls", "ls", []string{"-la"}, false},
		{"grep", "grep", []string{"pattern", "file"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hostlib.IsDangerousExecution(tt.command, tt.args)
			assert.Equal(t, tt.expected, result,
				"command: %s, args: %v", tt.command, tt.args)
		})
	}
}

// TestInterpreterBypassAttempts verifies we detect bypass techniques
func TestInterpreterBypassAttempts(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		shouldBlock bool
		reason      string
	}{
		{
			"direct python -c",
			"python",
			[]string{"-c", "import os; os.system('rm -rf /')"},
			true,
			"Direct code execution",
		},
		{
			"python via full path",
			"/usr/bin/python3.11",
			[]string{"-c", "malicious"},
			true,
			"Full path doesn't bypass detection",
		},
		{
			"perl inline code",
			"perl",
			[]string{"-e", "system('malicious')"},
			true,
			"Perl code execution",
		},
		{
			"ruby eval",
			"ruby",
			[]string{"-e", "system 'malicious'"},
			true,
			"Ruby code execution",
		},
		{
			"node eval",
			"node",
			[]string{"-e", "require('child_process').exec('malicious')"},
			true,
			"Node code execution",
		},
		{
			"php run code",
			"php",
			[]string{"-r", "system('malicious');"},
			true,
			"PHP code execution",
		},
		{
			"awk system call",
			"awk",
			[]string{"BEGIN{system(\"malicious\")}"},
			true,
			"AWK code execution",
		},
		{
			"safe python script",
			"python",
			[]string{"/opt/app/check.py"},
			false,
			"Executing specific script is safe",
		},
		{
			"safe node script",
			"node",
			[]string{"index.js", "--port", "3000"},
			false,
			"Normal script execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isDangerous := hostlib.IsDangerousExecution(tt.command, tt.args)
			assert.Equal(t, tt.shouldBlock, isDangerous,
				"%s - command: %s, args: %v", tt.reason, tt.command, tt.args)
		})
	}
}
