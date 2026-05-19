#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

if ! command -v go >/dev/null 2>&1 && [[ -x /usr/local/go/bin/go ]]; then
	export PATH="/usr/local/go/bin:${PATH}"
fi

export TUNNELD_IT=1
export KUBECONFIG="${KUBECONFIG:-${ROOT_DIR}/test/.runtime/kubeconfig}"
export GOCACHE="${GOCACHE:-${ROOT_DIR}/test/.runtime/go-build}"
mkdir -p "${GOCACHE}"

go test ./test/bdd -v

echo "Generating integration test coverage report..."
go tool covdata textfmt -i="${ROOT_DIR}/artifacts/coverage" -o="${ROOT_DIR}/artifacts/integration-coverage.out"
go tool covdata percent -i="${ROOT_DIR}/artifacts/coverage"
