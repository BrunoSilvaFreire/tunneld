#!/bin/bash
set -e

# Resolve the version from the latest git tag.
# Strips the 'v' prefix (e.g., v0.2.0 becomes 0.2.0).
# Falls back to 0.0.0-dev if no tags exist.
git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0-dev"
