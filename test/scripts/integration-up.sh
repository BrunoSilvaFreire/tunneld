#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

if ! command -v go >/dev/null 2>&1 && [[ -x /usr/local/go/bin/go ]]; then
	export PATH="/usr/local/go/bin:${PATH}"
fi

NETWORK_NAME="${NETWORK_NAME:-tunneld-it}"
CLUSTER_NAME="${CLUSTER_NAME:-tunneld-it}"
K3D_API_PORT="${K3D_API_PORT:-127.0.0.1:6445}"
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.32.1}"

require() {
	local cmd="$1"
	if ! command -v "${cmd}" >/dev/null 2>&1; then
		echo "required command not found: ${cmd}" >&2
		exit 127
	fi
}

for cmd in go docker k3d kubectl ssh-keygen; do
	require "${cmd}"
done

mkdir -p test/.runtime/socket test/fixtures/ssh artifacts/coverage
rm -rf artifacts/coverage/*
export GOCACHE="${GOCACHE:-${ROOT_DIR}/test/.runtime/go-build}"
mkdir -p "${GOCACHE}"

if [[ ! -f test/fixtures/ssh/id_ed25519 ]]; then
	ssh-keygen -t ed25519 -N "" -f test/fixtures/ssh/id_ed25519 -C "tunneld-integration-test" >/dev/null
fi
chmod 600 test/fixtures/ssh/id_ed25519
chmod 644 test/fixtures/ssh/id_ed25519.pub

go build -cover -o tunneld ./cmd/tunneld/main.go
go build -cover -o tunnelctl ./cmd/tunnelctl/main.go

if ! docker network inspect "${NETWORK_NAME}" >/dev/null 2>&1; then
	docker network create "${NETWORK_NAME}" >/dev/null
fi


if ! k3d cluster list | awk 'NR > 1 {print $1}' | grep -qx "${CLUSTER_NAME}"; then
	k3d cluster create "${CLUSTER_NAME}" \
		--servers 1 \
		--agents 2 \
		--network "${NETWORK_NAME}" \
		--api-port "${K3D_API_PORT}" \
		--wait
fi

k3d kubeconfig get "${CLUSTER_NAME}" > test/.runtime/kubeconfig
chmod 600 test/.runtime/kubeconfig
export KUBECONFIG="${ROOT_DIR}/test/.runtime/kubeconfig"

kubectl apply -f test/k8s/echo.yaml
kubectl rollout status deploy/echo --timeout=180s


echo "integration environment is ready"
