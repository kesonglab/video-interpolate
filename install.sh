#!/usr/bin/env bash
set -euo pipefail

# vif installer
# Usage: curl -fsSL https://raw.githubusercontent.com/kesonglab/video-interpolate/main/install.sh | bash
# Options:
#   VIF_INSTALL_DIR=/path    install to /path (default /usr/local/bin or ~/.local/bin)
#   VIF_VERSION=v0.1.0       install specific version (default: latest)

REPO="kesonglab/video-interpolate"
INSTALL_DIR="${VIF_INSTALL_DIR:-}"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# detect install dir: prefer ~/.local/bin if no sudo, else /usr/local/bin
if [[ -z "$INSTALL_DIR" ]]; then
    if [[ -w "/usr/local/bin" ]]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
fi

# detect OS/arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac

# version
if [[ -n "${VIF_VERSION:-}" ]]; then
    VERSION="$VIF_VERSION"
else
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
        | grep '"tag_name"' | head -1 | cut -d'"' -f4)
    if [[ -z "$VERSION" ]]; then
        echo "failed to detect latest version" >&2; exit 1
    fi
fi

VERSION="${VERSION#v}"
URL="https://github.com/$REPO/releases/download/v${VERSION}/vif_${VERSION}_${OS}_${ARCH}.tar.gz"

echo "→ installing vif $VERSION ($OS/$ARCH) to $INSTALL_DIR"
curl -fsSL -o "$TMP_DIR/vif.tar.gz" "$URL"
tar -xzf "$TMP_DIR/vif.tar.gz" -C "$TMP_DIR"

if [[ -w "$INSTALL_DIR" ]]; then
    mv "$TMP_DIR/vif" "$INSTALL_DIR/vif"
else
    sudo mv "$TMP_DIR/vif" "$INSTALL_DIR/vif"
fi
chmod +x "$INSTALL_DIR/vif"

echo "✓ installed: $($INSTALL_DIR/vif version)"
echo ""
echo "next steps:"
echo "  • run 'vif doctor' to check dependencies"
echo "  • if $INSTALL_DIR is not in PATH, add: export PATH=\"$INSTALL_DIR:\$PATH\""