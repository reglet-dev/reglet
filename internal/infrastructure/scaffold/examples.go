package scaffold

// pluginExamples contains example controls for each supported plugin.
// These are embedded as Go code rather than external files for simplicity.
var pluginExamples = map[string]PluginExample{
	"file": {
		PluginName:  "file",
		ControlID:   "file-exists-check",
		ControlName: "File existence check",
		Description: "Checks that the specified file exists and is readable",
		ConfigYAML:  `path: /etc/hostname  # Change to your target file`,
		ExpectExpressions: []string{
			"data.exists",
		},
		Capabilities: []CapabilityGrant{
			{Kind: "fs", Pattern: "read:/etc/hostname"},
		},
	},
	"http": {
		PluginName:  "http",
		ControlID:   "http-health-check",
		ControlName: "HTTP health check",
		Description: "Checks that an HTTP endpoint is accessible and returns 200 OK",
		ConfigYAML: `url: "https://example.com/health"  # Change to your endpoint
timeout: "5s"`,
		ExpectExpressions: []string{
			"data.status_code == 200",
		},
		Capabilities: []CapabilityGrant{
			{Kind: "network", Pattern: "outbound:80,443"},
		},
	},
	"dns": {
		PluginName:  "dns",
		ControlID:   "dns-resolution-check",
		ControlName: "DNS resolution check",
		Description: "Verifies that a domain resolves to expected records",
		ConfigYAML: `domain: "example.com"  # Change to your domain
record_type: "A"`,
		ExpectExpressions: []string{
			"len(data.records) > 0",
		},
		Capabilities: []CapabilityGrant{
			{Kind: "network", Pattern: "outbound:53"},
		},
	},
	"tcp": {
		PluginName:  "tcp",
		ControlID:   "tcp-port-check",
		ControlName: "TCP port connectivity check",
		Description: "Tests that a TCP port is open and accepting connections",
		ConfigYAML: `host: "localhost"  # Change to your target host
port: 22
timeout: "5s"`,
		ExpectExpressions: []string{
			"data.connected",
		},
		Capabilities: []CapabilityGrant{
			{Kind: "network", Pattern: "outbound:22"},
		},
	},
	"command": {
		PluginName:  "command",
		ControlID:   "command-output-check",
		ControlName: "Command output check",
		Description: "Executes a shell command and validates its output",
		ConfigYAML: `command: "uname -s"  # Change to your command
timeout: "10s"`,
		ExpectExpressions: []string{
			"data.exit_code == 0",
			`data.stdout contains "Linux" || data.stdout contains "Darwin"`,
		},
		Capabilities: []CapabilityGrant{
			{Kind: "exec", Pattern: "/bin/sh"},
		},
	},
	"smtp": {
		PluginName:  "smtp",
		ControlID:   "smtp-connection-check",
		ControlName: "SMTP server check",
		Description: "Validates that an SMTP server is accessible and responding",
		ConfigYAML: `host: "smtp.example.com"  # Change to your SMTP server
port: 587
timeout: "10s"`,
		ExpectExpressions: []string{
			"data.connected",
			"data.banner != \"\"",
		},
		Capabilities: []CapabilityGrant{
			{Kind: "network", Pattern: "outbound:25,587"},
		},
	},
}

// GetPluginExample returns the example configuration for a specific plugin.
// Returns nil if the plugin is not found.
func GetPluginExample(pluginName string) *PluginExample {
	if example, ok := pluginExamples[pluginName]; ok {
		return &example
	}
	return nil
}

// GetPluginExamples returns examples for the specified plugins.
// Unknown plugins are silently skipped.
func GetPluginExamples(plugins []string) []PluginExample {
	examples := make([]PluginExample, 0, len(plugins))
	for _, name := range plugins {
		if example := GetPluginExample(name); example != nil {
			examples = append(examples, *example)
		}
	}
	return examples
}
