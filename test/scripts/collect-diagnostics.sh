#!/usr/bin/env bash
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

OUT="${ROOT_DIR}/artifacts"
mkdir -p "${OUT}"

docker ps -a > "${OUT}/docker-ps.txt" 2>&1 || true
docker network inspect tunneld-it > "${OUT}/docker-network-tunneld-it.json" 2>&1 || true
	docker logs node-client > "${OUT}/docker-node-client.log" 2>&1 || true
	docker logs node-bastion > "${OUT}/docker-node-bastion.log" 2>&1 || true
	docker logs node-target > "${OUT}/docker-node-target.log" 2>&1 || true
docker exec node-client sh -lc 'ls -la /tmp/tunneld-it && cat /tmp/tunneld-it/tunneld.log' > "${OUT}/node-client-tunneld.log" 2>&1 || true
docker exec node-client tunnelctl --socket /run/tunneld-it/tunneld.sock status > "${OUT}/tunnelctl-status.txt" 2>&1 || true
k3d cluster list > "${OUT}/k3d-clusters.txt" 2>&1 || true

if [[ -f "${ROOT_DIR}/test/.runtime/kubeconfig" ]]; then
	export KUBECONFIG="${ROOT_DIR}/test/.runtime/kubeconfig"
fi

kubectl get nodes -o wide > "${OUT}/kubectl-nodes.txt" 2>&1 || true
kubectl get pods -A -o wide > "${OUT}/kubectl-pods.txt" 2>&1 || true
kubectl describe pods -A > "${OUT}/kubectl-describe-pods.txt" 2>&1 || true
kubectl get events -A --sort-by=.lastTimestamp > "${OUT}/kubectl-events.txt" 2>&1 || true

echo "diagnostics written to ${OUT}"
