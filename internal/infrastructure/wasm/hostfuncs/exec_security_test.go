package hostfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetBasename verifies basename extraction from paths
func TestGetBasename(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{"simple binary", "python", "python"},
		{"absolute path", "/usr/bin/python", "python"},
		{"relative path", "./scripts/python", "python"},
		{"versioned", "/usr/bin/python3.11", "python3.11"},
		{"nested path", "/usr/local/bin/custom/ruby", "ruby"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBasename(tt.command)
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
			result := isKnownInterpreter(tt.command)
			assert.Equal(t, tt.expected, result, "command: %s", tt.command)
		})
	}
}

// TestHasCodeExecutionFlags verifies detection of dangerous interpreter flags
func TestHasCodeExecutionFlags(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		expected bool
	}{
		// Python dangerous
		{"python -c", "python", []string{"-c", "print('test')"}, true},
		{"python --command", "python", []string{"--command", "import os"}, true},
		{"python3 -c", "python3", []string{"-c", "malicious"}, true},
		{"python path -c", "/usr/bin/python", []string{"-c", "code"}, true},

		// Python safe
		{"python script", "python", []string{"/path/to/script.py"}, false},
		{"python module", "python", []string{"-m", "pytest"}, false},
		{"python flags", "python", []string{"-u", "-W", "ignore"}, false},

		// Perl dangerous
		{"perl -e", "perl", []string{"-e", "print 'test'"}, true},
		{"perl -E", "perl", []string{"-E", "say 'test'"}, true},

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
		{"node flags", "node", []string{"--inspect", "app.js"}, false},

		// PHP dangerous
		{"php -r", "php", []string{"-r", "echo 'test';"}, true},

		// PHP safe
		{"php script", "php", []string{"script.php"}, false},

		// Lua dangerous
		{"lua -e", "lua", []string{"-e", "print('test')"}, true},

		// Lua safe
		{"lua script", "lua", []string{"script.lua"}, false},

		// AWK dangerous (BEGIN/END blocks)
		{"awk BEGIN", "awk", []string{"BEGIN{system(\"ls\")}"}, true},
		{"awk BEGIN space", "awk", []string{"BEGIN {print 1}"}, true},
		{"awk END", "awk", []string{"END{print NR}"}, true},

		// AWK safe
		{"awk pattern", "awk", []string{"-F", ",", "{print $1}"}, false},
		{"awk script", "awk", []string{"-f", "script.awk"}, false},

		// Unknown interpreter (not in our list)
		{"unknown", "obscure-lang", []string{"-c", "code"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasCodeExecutionFlags(tt.command, tt.args)
			assert.Equal(t, tt.expected, result,
				"command: %s, args: %v", tt.command, tt.args)
		})
	}
}

// TestHasSuspiciousFlags verifies heuristic detection
func TestHasSuspiciousFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		// Suspicious
		{"-c flag", []string{"-c", "code"}, true},
		{"-e flag", []string{"-e", "code"}, true},
		{"-E flag", []string{"-E", "code"}, true},
		{"-r flag", []string{"-r", "code"}, true},
		{"--eval flag", []string{"--eval", "code"}, true},
		{"--command flag", []string{"--command", "code"}, true},

		// Safe
		{"normal flags", []string{"-v", "--version"}, false},
		{"file args", []string{"script.sh"}, false},
		{"multiple safe", []string{"-u", "-W", "ignore"}, false},
		{"no args", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSuspiciousFlags(tt.args)
			assert.Equal(t, tt.expected, result, "args: %v", tt.args)
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
			isShell := isShellExecution(tt.command) && len(tt.args) > 0
			isInterpreterCode := hasCodeExecutionFlags(tt.command, tt.args)
			isSuspicious := hasSuspiciousFlags(tt.args)
			isDangerous := isShell || isInterpreterCode || isSuspicious

			assert.Equal(t, tt.shouldBlock, isDangerous,
				"%s - command: %s, args: %v", tt.reason, tt.command, tt.args)
		})
	}
}
