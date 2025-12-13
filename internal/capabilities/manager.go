// Package capabilities provides capability-based security management for WASM plugins.
package capabilities

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
	"golang.org/x/sync/errgroup"
)

// Manager handles capability grants and user prompts
type Manager struct {
	configPath  string
	trustAll    bool
	interactive bool
	grants      capabilities.Grant
}

// NewManager creates a capability manager
func NewManager(trustAll bool) *Manager {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".reglet", "config.yaml")

	return &Manager{
		configPath:  configPath,
		trustAll:    trustAll,
		interactive: isInteractive(),
		grants:      capabilities.NewGrant(),
	}
}

// isInteractive checks if we're running in an interactive terminal
func isInteractive() bool {
	// Check if stdin is a terminal (that's what we're reading from)
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	// Check if it's a character device (terminal) and not a pipe/file
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// CollectRequiredCapabilities loads all plugins in parallel and collects their required capabilities
func (m *Manager) CollectRequiredCapabilities(ctx context.Context, profile *entities.Profile, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error) {
	// Get unique plugin names from profile
	pluginNames := make(map[string]bool)
	for _, ctrl := range profile.Controls.Items {
		for _, obs := range ctrl.Observations {
			pluginNames[obs.Plugin] = true
		}
	}

	// Convert to slice for parallel iteration
	names := make([]string, 0, len(pluginNames))
	for name := range pluginNames {
		names = append(names, name)
	}

	// Thread-safe map for collecting capabilities
	var mu sync.Mutex
	required := make(map[string][]capabilities.Capability)

	// Load plugins in parallel
	g, ctx := errgroup.WithContext(ctx)
	for _, name := range names {
		name := name // Capture loop variable
		g.Go(func() error {
			// Plugin name is validated in config.validatePluginName() to prevent path traversal
			pluginPath := filepath.Join(pluginDir, name, name+".wasm")
			wasmBytes, err := os.ReadFile(pluginPath)
			if err != nil {
				return fmt.Errorf("failed to read plugin %s: %w", name, err)
			}

			// Load plugin (we need a temporary runtime with no capabilities for this)
			plugin, err := runtime.LoadPlugin(ctx, name, wasmBytes)
			if err != nil {
				return fmt.Errorf("failed to load plugin %s: %w", name, err)
			}

			// Get plugin metadata
			info, err := plugin.Describe(ctx)
			if err != nil {
				return fmt.Errorf("failed to get capabilities from plugin %s: %w", name, err)
			}

			// Collect capabilities (thread-safe)
			mu.Lock()
			var caps []capabilities.Capability
			for _, cap := range info.Capabilities {
				caps = append(caps, capabilities.Capability{
					Kind:    cap.Kind,
					Pattern: cap.Pattern,
				})
			}
			required[name] = caps
			mu.Unlock()

			return nil
		})
	}

	// Wait for all plugins to load
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return required, nil
}

// GrantCapabilities determines which capabilities to grant based on user input
func (m *Manager) GrantCapabilities(required map[string][]capabilities.Capability) (map[string][]capabilities.Capability, error) {
	// Flatten all required capabilities to a unique set for user prompting
	flatRequired := capabilities.NewGrant()
	for _, caps := range required {
		for _, cap := range caps {
			flatRequired.Add(cap)
		}
	}

	// If --trust-plugins flag is set, grant everything
	if m.trustAll {
		slog.Warn("Auto-granting all requested capabilities (--trust-plugins enabled)")
		m.grants = flatRequired
		return required, nil
	}

	// Load existing grants from config file
	existingGrants, err := m.loadConfig()
	if err != nil {
		// Config file doesn't exist yet - that's okay
		existingGrants = capabilities.NewGrant()
	}

	// Determine which capabilities are not already granted
	missing := m.findMissingCapabilities(flatRequired, existingGrants)

	var grantedGlobal capabilities.Grant
	if len(missing) == 0 {
		// All capabilities already granted
		grantedGlobal = existingGrants
	} else {
		// Prompt for missing capabilities
		if !m.interactive {
			// Non-interactive mode - fail with clear instructions
			return nil, m.formatNonInteractiveError(missing)
		}

		// Interactive prompts
		newGrants := existingGrants
		shouldSave := false

		for _, cap := range missing {
			granted, always, err := m.promptForCapability(cap)
			if err != nil {
				return nil, err
			}

			if granted {
				newGrants.Add(cap)
				if always {
					shouldSave = true
				}
			} else {
				return nil, fmt.Errorf("capability denied by user: %s:%s", cap.Kind, cap.Pattern)
			}
		}

		// Save to config if user chose "always" for any capability
		if shouldSave {
			if err := m.saveConfig(newGrants); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to save config: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "✓ Permissions saved to %s\n", m.configPath)
			}
		}
		grantedGlobal = newGrants
	}

	m.grants = grantedGlobal

	// Filter the requested capabilities against the globally granted ones
	// ensuring each plugin only gets what it requested AND what was granted
	grantedPerPlugin := make(map[string][]capabilities.Capability)
	for name, caps := range required {
		var allowed capabilities.Grant
		for _, cap := range caps {
			if grantedGlobal.Contains(cap) {
				allowed.Add(cap)
			}
		}
		if len(allowed) > 0 {
			grantedPerPlugin[name] = allowed
		}
	}

	return grantedPerPlugin, nil
}

// findMissingCapabilities returns capabilities in required that are not in granted
func (m *Manager) findMissingCapabilities(required, granted capabilities.Grant) capabilities.Grant {
	missing := capabilities.NewGrant()
	for _, cap := range required {
		if !granted.Contains(cap) {
			missing.Add(cap)
		}
	}
	return missing
}

// promptForCapability asks the user whether to grant a capability
func (m *Manager) promptForCapability(cap capabilities.Capability) (granted bool, always bool, err error) {
	fmt.Fprintf(os.Stderr, "\nPlugin requires permission:\n")
	fmt.Fprintf(os.Stderr, "  ✓ %s\n", m.describeCapability(cap))
	fmt.Fprintf(os.Stderr, "\nAllow this permission? [y/N/always]: ")

	// Create a new buffered reader from stdin
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		// On error (EOF, etc), treat as "no"
		return false, false, nil
	}

	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		return true, false, nil
	case "a", "always":
		return true, true, nil
	case "n", "no", "":
		// Empty response (just Enter) counts as "no"
		return false, false, nil
	default:
		// Unknown response - default to deny
		return false, false, nil
	}
}

// describeCapability returns a human-readable description of a capability
func (m *Manager) describeCapability(cap capabilities.Capability) string {
	switch cap.Kind {
	case "network":
		if cap.Pattern == "outbound:*" {
			return "Network access to any port"
		}
		if cap.Pattern == "outbound:private" {
			return "Network access to private/reserved IPs (localhost, 192.168.x.x, 10.x.x.x, 169.254.169.254, etc.)"
		}
		if strings.HasPrefix(cap.Pattern, "outbound:") {
			ports := strings.TrimPrefix(cap.Pattern, "outbound:")
			return fmt.Sprintf("Network access to port %s", ports)
		}
		return fmt.Sprintf("Network: %s", cap.Pattern)
	case "fs":
		if strings.HasPrefix(cap.Pattern, "read:") {
			path := strings.TrimPrefix(cap.Pattern, "read:")
			return fmt.Sprintf("Read files: %s", path)
		}
		if strings.HasPrefix(cap.Pattern, "write:") {
			path := strings.TrimPrefix(cap.Pattern, "write:")
			return fmt.Sprintf("Write files: %s", path)
		}
		return fmt.Sprintf("Filesystem: %s", cap.Pattern)
	case "exec":
		if cap.Pattern == "/bin/sh" {
			return "Shell execution (executes shell commands)"
		}
		return fmt.Sprintf("Execute commands: %s", cap.Pattern)
	case "env":
		return fmt.Sprintf("Read environment variables: %s", cap.Pattern)
	default:
		return fmt.Sprintf("%s: %s", cap.Kind, cap.Pattern)
	}
}

// formatNonInteractiveError creates a helpful error message for non-interactive mode
func (m *Manager) formatNonInteractiveError(missing capabilities.Grant) error {
	var msg strings.Builder
	msg.WriteString("Plugins require additional permissions (running in non-interactive mode)\n\n")
	msg.WriteString("Required permissions:\n")

	for _, cap := range missing {
		msg.WriteString(fmt.Sprintf("  - %s\n", m.describeCapability(cap)))
	}

	msg.WriteString("\nTo grant these permissions:\n")
	msg.WriteString("  1. Run interactively and approve when prompted\n")
	msg.WriteString("  2. Use --trust-plugins flag (grants all permissions)\n")
	msg.WriteString(fmt.Sprintf("  3. Manually edit: %s\n", m.configPath))

	return fmt.Errorf("%s", msg.String())
}

// configFile represents the YAML structure of ~/.reglet/config.yaml
type configFile struct {
	Capabilities []struct {
		Kind    string `yaml:"kind"`
		Pattern string `yaml:"pattern"`
	} `yaml:"capabilities"`
}

// loadConfig loads capability grants from ~/.reglet/config.yaml
func (m *Manager) loadConfig() (capabilities.Grant, error) {
	// Check if config file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return capabilities.NewGrant(), nil
	}

	// Read config file
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Convert to capability slice
	caps := capabilities.NewGrant()
	for _, c := range cfg.Capabilities {
		caps.Add(capabilities.Capability{
			Kind:    c.Kind,
			Pattern: c.Pattern,
		})
	}

	return caps, nil
}

// saveConfig saves capability grants to ~/.reglet/config.yaml
func (m *Manager) saveConfig(grants capabilities.Grant) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// TODO: Implement proper YAML serialization
	// For now, create a simple config file
	var content strings.Builder
	content.WriteString("# Reglet capability grants\n")
	content.WriteString("# Generated automatically - edit with caution\n\n")
	content.WriteString("capabilities:\n")

	for _, cap := range grants {
		content.WriteString(fmt.Sprintf("  - kind: %s\n", cap.Kind))
		content.WriteString(fmt.Sprintf("    pattern: %s\n", cap.Pattern))
	}

	return os.WriteFile(m.configPath, []byte(content.String()), 0600)
}