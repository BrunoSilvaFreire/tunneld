#!/bin/bash

# Improved force_reinstall.sh with better resiliency and logging
set -euo pipefail

# Colors for logging
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; exit "${2:-1}"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }

# Ensure we are in the project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Pre-checks: Required tools
for tool in go make sudo apt systemctl dpkg-deb; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        error "Required tool '$tool' is not installed."
    fi
done

# Environment variables
ARCH=$(go env GOARCH)
VERSION=$(scripts/get_version.sh)
PACKAGE="tunneld_${VERSION}_${ARCH}.deb"

log "Building package: ${PACKAGE}..."
make package-deb GOARCH="$ARCH" VERSION="$VERSION" || error "Failed to build package."

if [[ ! -f "./${PACKAGE}" ]]; then
    error "Package file ./${PACKAGE} not found after build."
fi

log "Purging existing installation (if any)..."
if dpkg -l tunneld >/dev/null 2>&1; then
    sudo apt purge tunneld -y || error "Failed to purge existing installation."
else
    log "tunneld is not currently installed. Skipping purge."
fi

log "Installing package ./${PACKAGE}..."
sudo apt install "./${PACKAGE}" -y || error "Failed to install package."

log "Reloading systemd and restarting service..."
sudo systemctl daemon-reload || error "Failed to reload systemd."
sudo systemctl enable tunneld.service || error "Failed to enable service."
sudo systemctl restart tunneld.service || error "Failed to restart service."

# Verification
log "Verifying service status..."
if systemctl is-active --quiet tunneld.service; then
    success "tunneld version ${VERSION} (${ARCH}) installed and running successfully."
else
    error "Service tunneld is not active after restart."
fi

# Cleanup (optional, but keep by default unless requested)
# rm "./${PACKAGE}"
