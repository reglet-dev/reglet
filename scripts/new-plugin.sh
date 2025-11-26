#!/bin/bash
#
# new-plugin.sh - Generate a new WASM plugin from template
#
# Usage: ./scripts/new-plugin.sh <plugin-name> [description] [capability-kind] [capability-pattern]
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print usage
usage() {
    cat <<EOF
Usage: $0 <plugin-name> [options]

Creates a new WASM plugin from template in plugins/<plugin-name>/

Arguments:
  plugin-name           Name of the plugin (e.g., http, dns, tcp)

Options:
  --description DESC    Plugin description (default: "Plugin description")
  --capability-kind K   Capability kind: fs, network, exec, env (default: "network")
  --capability-pattern P Capability pattern (default: "outbound:*")

Examples:
  $0 http --description "HTTP/HTTPS request checking"
  $0 dns --capability-kind network --capability-pattern "outbound:53"
  $0 file --capability-kind fs --capability-pattern "read:**"

EOF
    exit 1
}

# Check arguments
if [ $# -lt 1 ]; then
    usage
fi

PLUGIN_NAME="$1"
shift

# Defaults
DESCRIPTION="$PLUGIN_NAME plugin description"
CAPABILITY_KIND="network"
CAPABILITY_PATTERN="outbound:*"

# Parse options
while [ $# -gt 0 ]; do
    case "$1" in
        --description)
            DESCRIPTION="$2"
            shift 2
            ;;
        --capability-kind)
            CAPABILITY_KIND="$2"
            shift 2
            ;;
        --capability-pattern)
            CAPABILITY_PATTERN="$2"
            shift 2
            ;;
        *)
            echo -e "${RED}Error: Unknown option: $1${NC}"
            usage
            ;;
    esac
done

# Validate plugin name
if [[ ! "$PLUGIN_NAME" =~ ^[a-z][a-z0-9-]*$ ]]; then
    echo -e "${RED}Error: Plugin name must be lowercase alphanumeric (with hyphens) starting with a letter${NC}"
    exit 1
fi

# Check if plugin already exists
PLUGIN_DIR="plugins/$PLUGIN_NAME"
if [ -d "$PLUGIN_DIR" ]; then
    echo -e "${RED}Error: Plugin directory already exists: $PLUGIN_DIR${NC}"
    exit 1
fi

# Create plugin directory
echo -e "${GREEN}Creating plugin: $PLUGIN_NAME${NC}"
mkdir -p "$PLUGIN_DIR"

# Function to replace template variables
render_template() {
    local template_file="$1"
    local output_file="$2"

    sed -e "s/{{\.Name}}/$PLUGIN_NAME/g" \
        -e "s/{{\.Description}}/$DESCRIPTION/g" \
        -e "s/{{\.CapabilityKind}}/$CAPABILITY_KIND/g" \
        -e "s/{{\.CapabilityPattern}}/$CAPABILITY_PATTERN/g" \
        "$template_file" > "$output_file"
}

# Generate files from templates
echo "  Generating main.go..."
render_template "scripts/templates/plugin/main.go.tmpl" "$PLUGIN_DIR/main.go"

echo "  Generating Makefile..."
render_template "scripts/templates/plugin/Makefile.tmpl" "$PLUGIN_DIR/Makefile"

echo "  Generating README.md..."
render_template "scripts/templates/plugin/README.md.tmpl" "$PLUGIN_DIR/README.md"

# Success message
cat <<EOF

${GREEN}✓ Plugin scaffolding created successfully!${NC}

Location: $PLUGIN_DIR

Next steps:
  1. cd $PLUGIN_DIR
  2. Edit main.go and implement:
     - schema() - Define your configuration schema
     - observe() - Implement your observation logic
  3. Update README.md with documentation
  4. Build: make build
  5. Test: Add integration tests in internal/wasm/plugin_integration_test.go

Files created:
  - main.go    Plugin implementation with memory management boilerplate
  - Makefile   Build configuration
  - README.md  Documentation template

Happy coding! 🚀
EOF
