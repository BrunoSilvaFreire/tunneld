# Kubernetes support

`tunneld` ships with three CRDs in API group `tunneld.io/v1alpha1`:

- `Tunnel` — a single SSH or kubectl port-forward.
- `TunnelGroup` — a co-scheduled DAG of related tunnels.
- `TunnelKey` — authentication material (SSH private key or kubeconfig) backed by a Secret.

CRDs are reconciled by **two cooperating components**:

| Component | Workload | Purpose |
|---|---|---|
| `tunneld-controller` | `Deployment`, leader-elected | Validates CRs, schedules tunnels onto nodes, manages `Service`/`Endpoints`. |
| `tunneld-agent` | `DaemonSet`, one pod per node | Watches CRs bound to its node and applies them to the local `tunneld` daemon over its Unix socket. |

## Deployment modes

**Mode A — Host-installed tunneld.** The daemon runs on the host (systemd unit shipped with the `.deb`). The agent pod mounts `/run/tunneld` and `/var/lib/tunneld/keys` from the host. Tunnels bind on the host network directly.

**Mode B — DaemonSet-bundled tunneld.** A two-container pod (`tunneld` sidecar + `tunneld-agent`) per node. The pod runs with `hostNetwork: true` so tunnel ports bind on the node IP. They share an `emptyDir` for the socket and key store.

In both modes the controller publishes each `Tunnel` (with `spec.expose.service: true`) as a `Service` whose `Endpoints` point at the chosen node's internal IP and the dynamically-resolved port. Pods in the cluster reach the tunnel via `<tunnel>.<ns>.svc.cluster.local`.

## Quickstart

```bash
# With Helm:
helm install tunneld k8s/charts/tunneld-operator

# Or with kustomize (CRDs + RBAC + controller + Mode A agent):
kubectl apply -k k8s/config/default

# Mode A requires you to label opt-in nodes so the DaemonSet schedules:
kubectl label nodes <node> tunneld.io/installed=true

# To use Mode B instead (bundled tunneld sidecar, no host install needed):
kubectl apply -f k8s/config/agent/daemonset-mode-b.yaml
# (and remove the Mode A DaemonSet if you applied the default bundle)

# Try a sample tunnel:
kubectl apply -f k8s/config/samples/
```

## Networking notes

- Tunnels must bind on `0.0.0.0` (or the node's pod-routable IP) for pods to reach them through the cluster Service. Sample manifests use `listenAddress: 0.0.0.0`.
- If the cluster has default-deny ingress NetworkPolicy on nodes, allow pod CIDR → node IP on the tunnel ports.
- Use `listenPort: 0` (dynamic) when possible — the controller will schedule freely without worrying about port collisions across tunnels on the same node.

## Status

- v1alpha1: CRD types and manifests are in tree. Controller/agent implementations are being added in phases — see `/home/brunorbsf/.claude/plans/make-a-plan-for-prancy-fountain.md` for the roadmap.
