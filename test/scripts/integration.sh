#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

cleanup() {
	local code=$?
	if [[ ${code} -ne 0 ]]; then
		./test/scripts/collect-diagnostics.sh || true
	fi
	./test/scripts/integration-down.sh || true
	exit "${code}"
}
trap cleanup EXIT

./test/scripts/integration-up.sh
./test/scripts/integration-test.sh
