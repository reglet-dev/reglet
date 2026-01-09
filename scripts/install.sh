#!/bin/bash
# Install Reglet - Infrastructure compliance validation
# Usage: curl -sSL https://raw.githubusercontent.com/reglet-dev/reglet/main/scripts/install.sh | sh
set -euo pipefail

REPO="reglet-dev/reglet"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
USE_SUDO="${USE_SUDO:-true}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo ""
echo "╭─────────────────────────────────────╮"
echo "│   Reglet Installer                  │"
echo "│   Infrastructure compliance with    │"
echo "│   WASM plugins                      │"
echo "╰─────────────────────────────────────╯"
echo ""

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo -e "${RED}✗ Unsupported architecture: $ARCH${NC}"
        echo "  Supported: x86_64, aarch64, arm64"
        exit 1
        ;;
esac

case $OS in
    linux|darwin) ;;
    *)
        echo -e "${RED}✗ Unsupported OS: $OS${NC}"
        echo "  Supported: Linux, macOS (Darwin)"
        echo ""
        echo "Alternative installation methods:"
        echo "  • Docker (all platforms): docker pull ghcr.io/reglet-dev/reglet:latest"
        echo "  • Manual download: https://github.com/${REPO}/releases"
        exit 1
        ;;
esac

echo "→ Detected: ${OS}/${ARCH}"

# Get latest release version
echo "→ Fetching latest release..."
VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$VERSION" ]; then
    echo -e "${RED}✗ Failed to get latest version${NC}"
    echo "  Check https://github.com/${REPO}/releases"
    exit 1
fi

echo "→ Latest version: ${VERSION}"

# Download URL
URL="https://github.com/${REPO}/releases/download/${VERSION}/reglet-${OS}-${ARCH}.tar.gz"
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

echo "→ Downloading from ${URL}..."
if ! curl -fsSL "$URL" | tar -xz -C "$TMP_DIR"; then
    echo -e "${RED}✗ Download failed${NC}"
    echo "  URL: $URL"
    echo ""
    echo "Alternative installation methods:"
    if [ "$OS" = "darwin" ]; then
        echo "  • Homebrew: brew install reglet-dev/tap/reglet"
    fi
    echo "  • Docker: docker pull ghcr.io/reglet-dev/reglet:latest"
    echo "  • Manual: https://github.com/${REPO}/releases"
    exit 1
fi

# Install
BINARY="${TMP_DIR}/reglet"

if [ ! -f "$BINARY" ]; then
    echo -e "${RED}✗ Binary not found in tarball${NC}"
    echo "  Expected: $BINARY"
    echo ""
    echo "Available files:"
    ls -la "$TMP_DIR"
    echo ""
    echo "Alternative installation methods:"
    if [ "$OS" = "darwin" ]; then
        echo "  • Homebrew: brew install reglet-dev/tap/reglet"
    fi
    echo "  • Docker: docker pull ghcr.io/reglet-dev/reglet:latest"
    echo "  • Manual: https://github.com/${REPO}/releases"
    exit 1
fi

echo "→ Installing to ${INSTALL_DIR}/reglet..."

if [ "$USE_SUDO" = "true" ]; then
    sudo mv "$BINARY" "${INSTALL_DIR}/reglet"
    sudo chmod +x "${INSTALL_DIR}/reglet"
else
    mv "$BINARY" "${INSTALL_DIR}/reglet"
    chmod +x "${INSTALL_DIR}/reglet"
fi

# Verify installation
if command -v reglet >/dev/null 2>&1; then
    echo ""
    echo -e "${GREEN}✓ Reglet installed successfully!${NC}"
    echo ""
    reglet version
    echo ""
    echo "Next steps:"
    echo "  1. Try an example:"
    echo "     curl -fsSL https://raw.githubusercontent.com/reglet-dev/reglet/main/docs/examples/01-quickstart.yaml > quickstart.yaml"
    echo "     reglet check quickstart.yaml"
    echo ""
    echo "  2. Read the documentation:"
    echo "     https://github.com/reglet-dev/reglet#readme"
    echo ""
else
    echo ""
    echo -e "${YELLOW}⚠ Installation complete, but 'reglet' not in PATH${NC}"
    echo ""
    echo "Add ${INSTALL_DIR} to your PATH:"
    echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
    echo ""
fi
