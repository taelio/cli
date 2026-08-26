#!/usr/bin/env bash
#
# Tael CLI installer.
#
#   curl -fsSL https://raw.githubusercontent.com/taelio/cli/main/install.sh | bash
#
# Installs the latest release by default. Pin a version with TAEL_VERSION,
# change the destination with INSTALL_DIR:
#
#   TAEL_VERSION=v0.2.0 INSTALL_DIR="$HOME/.local/bin" bash install.sh
#
set -euo pipefail

readonly REPOSITORY="taelio/cli"
readonly BINARY_NAME="tael"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

step()    { printf "${GREEN}==>${NC} %s\n" "$1"; }
warn()    { printf "${YELLOW}warning:${NC} %s\n" "$1" >&2; }
failure() { printf "${RED}error:${NC} %s\n" "$1" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || failure "curl is required"

operating_system=$(uname -s | tr '[:upper:]' '[:lower:]')
architecture=$(uname -m)
case "$architecture" in
    x86_64|amd64) architecture="amd64" ;;
    arm64|aarch64) architecture="arm64" ;;
    *) failure "unsupported architecture: ${architecture}" ;;
esac
case "$operating_system" in
    linux|darwin) ;;
    *) failure "unsupported operating system: ${operating_system} (Windows users: download the .exe from the releases page)" ;;
esac

version="${TAEL_VERSION:-}"
if [[ -z "$version" ]]; then
    step "Resolving the latest release..."
    version=$(curl -fsSL "https://api.github.com/repos/${REPOSITORY}/releases/latest" \
        | grep '"tag_name"' | head -1 | cut -d '"' -f 4 || true)
    [[ -n "$version" ]] || failure "could not resolve the latest release; set TAEL_VERSION to install a specific tag"
fi

asset="tael-${operating_system}-${architecture}"
download_url="https://github.com/${REPOSITORY}/releases/download/${version}/${asset}"
checksum_url="${download_url}.sha256"

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT

step "Downloading ${BINARY_NAME} ${version} (${operating_system}/${architecture})..."
curl -fsSL -o "${workdir}/${BINARY_NAME}" "$download_url" \
    || failure "download failed: ${download_url}"

# Verify the checksum when the release publishes one. A present-but-wrong
# checksum is fatal; a missing one is only a warning, so older releases
# still install.
if curl -fsSL -o "${workdir}/${BINARY_NAME}.sha256" "$checksum_url" 2>/dev/null; then
    expected=$(awk '{print $1}' "${workdir}/${BINARY_NAME}.sha256")
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "${workdir}/${BINARY_NAME}" | awk '{print $1}')
    else
        actual=$(shasum -a 256 "${workdir}/${BINARY_NAME}" | awk '{print $1}')
    fi
    [[ "$expected" == "$actual" ]] || failure "checksum mismatch (expected ${expected}, got ${actual})"
    step "Checksum verified."
else
    warn "no published checksum for ${version}; skipping verification"
fi

chmod +x "${workdir}/${BINARY_NAME}"
if [[ "$operating_system" == "darwin" ]]; then
    xattr -d com.apple.quarantine "${workdir}/${BINARY_NAME}" 2>/dev/null || true
fi

step "Installing to ${INSTALL_DIR}..."
if [[ -w "$INSTALL_DIR" ]]; then
    install -m 0755 "${workdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
    warn "${INSTALL_DIR} is not writable; using sudo"
    sudo install -m 0755 "${workdir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

if command -v "$BINARY_NAME" >/dev/null 2>&1; then
    step "$("$BINARY_NAME" version 2>/dev/null || echo installed)"
    printf "\nRun ${GREEN}tael login${NC} to get started.\n"
else
    warn "installed, but ${INSTALL_DIR} is not on your PATH"
fi
