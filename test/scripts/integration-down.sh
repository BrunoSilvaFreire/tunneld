#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

NETWORK_NAME="${NETWORK_NAME:-tunneld-it}"
CLUSTER_NAME="${CLUSTER_NAME:-tunneld-it}"

k3d cluster delete "${CLUSTER_NAME}" >/dev/null 2>&1 || true
docker rm -f node-client node-bastion node-target >/dev/null 2>&1 || true
if docker network inspect "${NETWORK_NAME}" >/dev/null 2>&1; then
	if [[ "$(docker network inspect -f '{{len .Containers}}' "${NETWORK_NAME}" 2>/dev/null || echo 1)" == "0" ]]; then
		docker network rm "${NETWORK_NAME}" >/dev/null 2>&1 || true
	fi
fi

rm -rf test/.runtime/socket
echo "integration environment removed"
