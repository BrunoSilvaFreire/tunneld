#!/bin/bash
set -euo pipefail

# Resolve the version from the latest git tag.
# Strips the 'v' prefix (e.g., v0.2.0 becomes 0.2.0).
# Falls back to 0.0.0-dev if no tags exist.
tag="$(git describe --tags --abbrev=0 2>/dev/null || true)"
if [ -z "$tag" ]; then
	echo "0.0.0-dev"
	exit 0
fi

echo "$tag" | sed 's/^v//'
