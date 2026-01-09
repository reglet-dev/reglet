package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/reglet-dev/reglet/internal/templates"
	"github.com/spf13/cobra"
)

// CreatePluginOptions holds options for the create plugin command.
type CreatePluginOptions struct {
	name         string
	lang         string
	output       string
	sdkVersion   string
	modulePath   string
	capabilities []string
	force        bool
}

func newCreatePluginCmd() *cobra.Command {
	opts := &CreatePluginOptions{}

	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Create a new plugin scaffold",
		Long: `Generate a new plugin project with boilerplate code, tests, and build configuration.

Examples:
  # Create a basic plugin
  reglet create plugin --name my-check --lang go

  # Create with specific capabilities
  reglet create plugin --name dns-resolver --lang go --capabilities "network:dns,fs:read"

  # Create in a specific directory
  reglet create plugin --name file-validator --lang go --output ./plugins/file-validator

  # Create with custom module path
  reglet create plugin --name my-check --lang go --module github.com/myorg/my-check`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCreatePlugin(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.name, "name", "n", "", "Plugin name (required, e.g., 'my-check')")
	cmd.Flags().StringVarP(&opts.lang, "lang", "l", "go", "Language: go (future: rust)")
	cmd.Flags().StringSliceVarP(&opts.capabilities, "capabilities", "c", nil, "Comma-separated capabilities (e.g., 'network:dns,fs:read')")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output directory (default: ./<name>)")
	cmd.Flags().StringVar(&opts.sdkVersion, "sdk-version", "latest", "SDK version to use")
	cmd.Flags().StringVar(&opts.modulePath, "module", "", "Go module path (default: github.com/<name>, set for real projects)")
	cmd.Flags().BoolVarP(&opts.force, "force", "f", false, "Overwrite existing files")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runCreatePlugin(opts *CreatePluginOptions) error {
	// Validate plugin name
	if err := validatePluginName(opts.name); err != nil {
		return err
	}

	// Validate language
	if opts.lang != "go" {
		return fmt.Errorf("unsupported language: %s (supported: go)", opts.lang)
	}

	// Set defaults
	if opts.output == "" {
		opts.output = "./" + opts.name
	}
	if opts.modulePath == "" {
		opts.modulePath = "github.com/" + opts.name
	}

	// Parse capabilities
	caps, err := parseCapabilities(opts.capabilities)
	if err != nil {
		return fmt.Errorf("invalid capabilities: %w", err)
	}

	// Build template data
	data := templates.PluginData{
		PluginName:       opts.name,
		PluginTitle:      toTitleCase(opts.name),
		PluginStructName: toPluginStructName(opts.name),
		ModulePath:       opts.modulePath,
		SDKVersion:       opts.sdkVersion,
		Capabilities:     caps,
	}

	// Create output directory
	outputDir, err := filepath.Abs(opts.output)
	if err != nil {
		return fmt.Errorf("resolving output path: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Load templates
	tmpl, err := templates.GoTemplates()
	if err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	// Get list of files to generate
	files, err := templates.TemplateFiles(opts.lang)
	if err != nil {
		return err
	}

	// Generate each file
	for _, file := range files {
		outputPath := filepath.Join(outputDir, file)

		// Check if file exists
		if !opts.force {
			if _, err := os.Stat(outputPath); err == nil {
				return fmt.Errorf("file already exists: %s (use --force to overwrite)", outputPath)
			}
		}

		// Render template
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, file, data); err != nil {
			return fmt.Errorf("rendering %s: %w", file, err)
		}

		// Write file
		//nolint:gosec // G306: User-created plugin files need reasonable permissions
		if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", outputPath, err)
		}

		slog.Debug("created file", "path", outputPath)
	}

	// Print success message
	fmt.Printf("✓ Created plugin '%s' in %s\n\n", opts.name, outputDir)
	fmt.Println("Next steps:")
	fmt.Printf("  1. cd %s\n", opts.output)
	fmt.Println("  2. Implement your check logic in plugin.go")
	fmt.Println("  3. Define configuration fields in the Config struct")
	fmt.Println("  4. Run 'make build' to compile to WASM")
	fmt.Println("  5. Run 'make test' to run tests")

	return nil
}

// validatePluginName checks that the plugin name is valid.
func validatePluginName(name string) error {
	if name == "" {
		return fmt.Errorf("plugin name is required")
	}

	// Must be lowercase, alphanumeric, hyphens allowed
	validName := regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid plugin name '%s': must be lowercase alphanumeric with hyphens, starting with a letter", name)
	}

	// No consecutive hyphens
	if strings.Contains(name, "--") {
		return fmt.Errorf("invalid plugin name '%s': consecutive hyphens not allowed", name)
	}

	return nil
}

// parseCapabilities converts capability strings to Capability structs.
// Format: "kind:pattern" or "kind" (pattern defaults to "*")
func parseCapabilities(caps []string) ([]templates.Capability, error) {
	result := make([]templates.Capability, 0, len(caps))

	for _, cap := range caps {
		parts := strings.SplitN(cap, ":", 2)
		kind := parts[0]
		pattern := "*"
		if len(parts) == 2 {
			pattern = parts[1]
		}

		if kind == "" {
			return nil, fmt.Errorf("empty capability kind")
		}

		result = append(result, templates.Capability{
			Kind:    kind,
			Pattern: pattern,
		})
	}

	return result, nil
}

// toTitleCase converts "my-plugin" to "My Plugin".
func toTitleCase(s string) string {
	words := strings.Split(s, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// toPluginStructName converts "my-plugin" to "myPluginPlugin".
func toPluginStructName(s string) string {
	words := strings.Split(s, "-")
	var result strings.Builder

	for i, word := range words {
		if len(word) == 0 {
			continue
		}

		runes := []rune(word)
		if i == 0 {
			// First word: lowercase first letter
			runes[0] = unicode.ToLower(runes[0])
		} else {
			// Subsequent words: uppercase first letter
			runes[0] = unicode.ToUpper(runes[0])
		}
		result.WriteString(string(runes))
	}

	result.WriteString("Plugin")
	return result.String()
}
