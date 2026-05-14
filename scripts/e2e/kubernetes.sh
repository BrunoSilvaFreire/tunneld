#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

CLUSTER_NAME="${CLUSTER_NAME:-tunneld-e2e}"
E2E_NAMESPACE="${E2E_NAMESPACE:-tunneld-e2e}"
OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-tunneld-system}"
IMAGE_TAG="${IMAGE_TAG:-$(git rev-parse --short=12 HEAD)}"
TUNNELD_IMAGE="${TUNNELD_IMAGE:-ghcr.io/brunosilvafreire/tunneld-ci}"
CONTROLLER_IMAGE="${CONTROLLER_IMAGE:-ghcr.io/brunosilvafreire/tunneld-ci-controller}"
AGENT_IMAGE="${AGENT_IMAGE:-ghcr.io/brunosilvafreire/tunneld-ci-agent}"
KEEP_CLUSTER="${KEEP_CLUSTER:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"

require() {
	local cmd="$1"
	if ! command -v "${cmd}" >/dev/null 2>&1; then
		echo "required command not found: ${cmd}" >&2
		exit 127
	fi
}

dump_diagnostics() {
	local out="${ROOT_DIR}/e2e-artifacts"
	rm -rf "${out}"
	mkdir -p "${out}"

	kubectl get nodes -o wide >"${out}/nodes.txt" 2>&1 || true
	kubectl get all -A -o wide >"${out}/all.txt" 2>&1 || true
	kubectl get events -A --sort-by=.lastTimestamp >"${out}/events.txt" 2>&1 || true
	kubectl get crd -o yaml >"${out}/crds.yaml" 2>&1 || true
	kubectl get tunnels,tunnelgroups,tunnelkeys -A -o yaml >"${out}/tunneld-crs.yaml" 2>&1 || true
	kubectl -n "${OPERATOR_NAMESPACE}" logs deploy/tunneld-controller --all-containers=true >"${out}/controller.log" 2>&1 || true
	kubectl -n "${OPERATOR_NAMESPACE}" logs ds/tunneld-agent --all-containers=true >"${out}/agent.log" 2>&1 || true
	kubectl -n "${OPERATOR_NAMESPACE}" describe pods >"${out}/operator-pods.txt" 2>&1 || true
	kubectl -n "${E2E_NAMESPACE}" describe all >"${out}/e2e-namespace.txt" 2>&1 || true
	echo "diagnostics written to ${out}" >&2
}

cleanup() {
	local code=$?
	if [[ ${code} -ne 0 ]]; then
		dump_diagnostics
	fi
	if [[ "${KEEP_CLUSTER}" != "true" ]]; then
		kind delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
	fi
	exit "${code}"
}
trap cleanup EXIT

for cmd in docker kind kubectl helm; do
	require "${cmd}"
done

if ! kind get clusters | grep -qx "${CLUSTER_NAME}"; then
	kind create cluster --name "${CLUSTER_NAME}" --wait 120s
fi
kubectl cluster-info --context "kind-${CLUSTER_NAME}"

if [[ "${SKIP_BUILD}" != "true" ]]; then
	docker build -f containers/Containerfile.tunneld -t "${TUNNELD_IMAGE}:${IMAGE_TAG}" .
	docker build -f k8s/Dockerfile.controller -t "${CONTROLLER_IMAGE}:${IMAGE_TAG}" .
	docker build -f k8s/Dockerfile.agent -t "${AGENT_IMAGE}:${IMAGE_TAG}" .
fi

kind load docker-image --name "${CLUSTER_NAME}" "${TUNNELD_IMAGE}:${IMAGE_TAG}"
kind load docker-image --name "${CLUSTER_NAME}" "${CONTROLLER_IMAGE}:${IMAGE_TAG}"
kind load docker-image --name "${CLUSTER_NAME}" "${AGENT_IMAGE}:${IMAGE_TAG}"

helm upgrade --install tunneld ./k8s/charts/tunneld-operator \
	--namespace "${OPERATOR_NAMESPACE}" \
	--create-namespace \
	--set mode=daemonset \
	--set controller.image.repository="${CONTROLLER_IMAGE}" \
	--set controller.image.tag="${IMAGE_TAG}" \
	--set controller.image.pullPolicy=IfNotPresent \
	--set agent.image.repository="${AGENT_IMAGE}" \
	--set agent.image.tag="${IMAGE_TAG}" \
	--set agent.image.pullPolicy=IfNotPresent \
	--set tunneld.image.repository="${TUNNELD_IMAGE}" \
	--set tunneld.image.tag="${IMAGE_TAG}" \
	--set tunneld.image.pullPolicy=IfNotPresent \
	--wait \
	--timeout 5m

kubectl -n "${OPERATOR_NAMESPACE}" rollout status deploy/tunneld-controller --timeout=180s
kubectl -n "${OPERATOR_NAMESPACE}" rollout status ds/tunneld-agent --timeout=180s
kubectl wait --for=condition=Established crd/tunnels.tunneld.io crd/tunnelgroups.tunneld.io crd/tunnelkeys.tunneld.io --timeout=120s

kubectl create namespace "${E2E_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${E2E_NAMESPACE}" create deployment e2e-echo --image=nginx:1.27-alpine --port=80 --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${E2E_NAMESPACE}" expose deployment e2e-echo --port=80 --target-port=80 --dry-run=client -o yaml | kubectl apply -f -
kubectl -n "${E2E_NAMESPACE}" rollout status deploy/e2e-echo --timeout=180s

cat <<EOF | kubectl apply -f -
apiVersion: tunneld.io/v1alpha1
kind: Tunnel
metadata:
  name: e2e-kubectl
  namespace: ${E2E_NAMESPACE}
spec:
  kubectl:
    namespace: ${E2E_NAMESPACE}
    resource: svc/e2e-echo
    forwards:
      - localAddress: 0.0.0.0
        localPort: 0
        remotePort: 80
  health:
    type: tcp
    address: 0.0.0.0:0
    interval: 1s
    timeout: 1s
    startupTimeout: 60s
  restart:
    policy: always
    delay: 1s
  expose:
    service: true
EOF

kubectl -n "${E2E_NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Healthy tunnel/e2e-kubectl --timeout=180s

for _ in $(seq 1 60); do
	actual_port="$(kubectl -n "${E2E_NAMESPACE}" get tunnel e2e-kubectl -o jsonpath='{.status.resolvedForwards[0].actualPort}' 2>/dev/null || true)"
	endpoints="$(kubectl -n "${E2E_NAMESPACE}" get endpoints e2e-kubectl -o jsonpath='{.subsets[0].ports[0].port}' 2>/dev/null || true)"
	if [[ -n "${actual_port}" && "${actual_port}" != "0" && "${endpoints}" == "${actual_port}" ]]; then
		break
	fi
	sleep 2
done

if [[ -z "${actual_port:-}" || "${actual_port}" == "0" || "${endpoints:-}" != "${actual_port}" ]]; then
	echo "tunnel service endpoints did not converge" >&2
	exit 1
fi

kubectl -n "${E2E_NAMESPACE}" run curl --rm -i --restart=Never --image=curlimages/curl:8.11.1 -- \
	--fail --silent --show-error --max-time 20 http://e2e-kubectl.${E2E_NAMESPACE}.svc.cluster.local:80/ >/tmp/tunneld-e2e-curl.out

cat <<EOF | kubectl apply -f -
apiVersion: tunneld.io/v1alpha1
kind: Tunnel
metadata:
  name: e2e-kubectl
  namespace: ${E2E_NAMESPACE}
spec:
  kubectl:
    namespace: ${E2E_NAMESPACE}
    resource: svc/e2e-echo
    forwards:
      - localAddress: 0.0.0.0
        localPort: 0
        remotePort: 80
  health:
    type: tcp
    address: 0.0.0.0:0
    interval: 1s
    timeout: 1s
    startupTimeout: 60s
  restart:
    policy: always
    delay: 1s
  expose:
    service: true
    serviceName: e2e-kubectl-renamed
EOF

kubectl -n "${E2E_NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Healthy tunnel/e2e-kubectl --timeout=180s
kubectl -n "${E2E_NAMESPACE}" get service e2e-kubectl-renamed >/dev/null

kubectl -n "${E2E_NAMESPACE}" delete tunnel e2e-kubectl --wait=true
kubectl -n "${E2E_NAMESPACE}" wait --for=delete service/e2e-kubectl-renamed --timeout=120s

echo "kubernetes e2e suite passed"
