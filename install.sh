#!/usr/bin/env sh
# install.sh — install the studio binary
#
# Usage (one-liner):
#   curl -fsSL https://raw.githubusercontent.com/maximilianbuff/agent-studio/main/install.sh | sh
#
# Options (environment variables):
#   STUDIO_BIN_DIR   install location  (default: ~/.local/bin)
#   STUDIO_VERSION   release tag       (default: latest)

set -e

REPO="maximilianbuff/agent-studio"
BIN_DIR="${STUDIO_BIN_DIR:-$HOME/.local/bin}"
VERSION="${STUDIO_VERSION:-latest}"
TMP_DIR="$(mktemp -d)"

trap 'rm -rf "$TMP_DIR"' EXIT

echo "Installing studio..."
echo ""

# ---- helpers ---------------------------------------------------------

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: $1 is required but not found."
    echo "       $2"
    exit 1
  fi
}

download() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$2" "$1"
  else
    echo "Error: curl or wget is required."
    exit 1
  fi
}

# ---- detect platform -------------------------------------------------

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    ARCH=""
    ;;
esac

# ---- try pre-built binary --------------------------------------------

INSTALLED=0

if [ -n "$ARCH" ]; then
  ASSET="studio_${OS}_${ARCH}.tar.gz"

  if [ "$VERSION" = "latest" ]; then
    URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/latest/download/checksums.txt"
  else
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
  fi

  echo "Downloading $ASSET..."

  if download "$URL" "$TMP_DIR/$ASSET" 2>/dev/null; then
    # Verify checksum if sha256sum is available.
    if command -v sha256sum >/dev/null 2>&1; then
      if download "$CHECKSUM_URL" "$TMP_DIR/checksums.txt" 2>/dev/null; then
        cd "$TMP_DIR"
        grep "$ASSET" checksums.txt | sha256sum -c --status \
          && echo "Checksum verified." \
          || { echo "Error: checksum mismatch — aborting."; exit 1; }
        cd - >/dev/null
      fi
    fi

    tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
    mkdir -p "$BIN_DIR"
    install -m 0755 "$TMP_DIR/studio_${OS}_${ARCH}" "$BIN_DIR/studio"
    INSTALLED=1
  else
    echo "Pre-built binary not available for ${OS}/${ARCH} — building from source..."
    echo ""
  fi
fi

# ---- fallback: build from source ------------------------------------

if [ "$INSTALLED" -eq 0 ]; then
  need git "https://git-scm.com/"
  need go  "https://go.dev/dl/"

  if [ "$VERSION" = "latest" ]; then
    CLONE_REF="main"
  else
    CLONE_REF="$VERSION"
  fi

  echo "Cloning agent-studio ($CLONE_REF)..."
  git clone --depth=1 --branch "$CLONE_REF" \
    "https://github.com/${REPO}.git" "$TMP_DIR/repo" 2>/dev/null \
    || git clone --depth=1 "https://github.com/${REPO}.git" "$TMP_DIR/repo"

  BUILD_VERSION="$(git -C "$TMP_DIR/repo" describe --tags --always 2>/dev/null || echo "$CLONE_REF")"
  echo "Building studio@$BUILD_VERSION..."

  mkdir -p "$BIN_DIR"
  cd "$TMP_DIR/repo"
  go build \
    -ldflags "-X main.version=$BUILD_VERSION -s -w" \
    -o "$BIN_DIR/studio" \
    ./cli
  cd - >/dev/null
fi

# ---- done ------------------------------------------------------------

echo ""
echo "Installed: $BIN_DIR/studio  ($(studio --version 2>/dev/null || echo 'version unknown'))"
echo ""

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo "NOTE: $BIN_DIR is not on your PATH."
    echo "      Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo "        export PATH=\"$BIN_DIR:\$PATH\""
    echo ""
    ;;
esac

echo "Next step:"
echo ""
echo "  studio install"
echo ""
