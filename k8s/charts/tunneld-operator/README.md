# tunneld-operator

Helm chart for the tunneld Kubernetes operator: a central controller plus a
per-node DaemonSet agent that reconcile `Tunnel`, `TunnelGroup`, and `TunnelKey`
CRDs onto a running tunneld daemon.

## Install

```bash
helm install tunneld ./k8s/charts/tunneld-operator
```

The chart installs CRDs from `crds/` on first install. For upgrades, re-apply
them manually:

```bash
kubectl apply -f k8s/charts/tunneld-operator/crds/
```

## Deployment modes

| Value | Behavior |
|---|---|
| `mode: host-installed` (default) | tunneld runs on the host (e.g. `.deb` install). Agent pod hostPath-mounts the daemon's socket and key dir. Schedules only on nodes labeled `tunneld.io/installed=true`. |
| `mode: daemonset` | Bundled tunneld sidecar in the agent pod with `hostNetwork: true`. Tunnel ports bind on the node IP. No node labels required. |

Switch modes with `--set mode=daemonset`.

## Common values

| Key | Default | Description |
|---|---|---|
| `controller.image.repository` | `ghcr.io/brunosilvafreire/tunneld-controller` | Controller image |
| `controller.replicas` | `1` | Always 1 — leader election guards against split brain |
| `agent.image.repository` | `ghcr.io/brunosilvafreire/tunneld-agent` | Agent image |
| `tunneld.image.repository` | `ghcr.io/brunosilvafreire/tunneld` | Mode B sidecar image |
| `agent.hostSocketPath` | `/run/tunneld` | Mode A only |
| `agent.hostKeyDir` | `/var/lib/tunneld/keys` | Mode A only |
| `rbac.create` | `true` | Set false if cluster operators manage RBAC out-of-band |

See `values.yaml` for the full list.
