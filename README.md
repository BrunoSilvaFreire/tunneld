# tunneld

Keep chained SSH and kubectl port-forward tunnels alive.

`tunneld` is a dependency-aware tunnel supervisor. It keeps chains of local tunnels alive, resolving the "brittle tunnel" problem where network interruptions, VPN reconnects, or host sleep cycles break your development infrastructure.

Use it instead of fragile shell scripts, tmux panes, or `autossh` glue when:
- **You access private Kubernetes clusters through a bastion host.**
- **Your kubectl port-forward depends on an SSH tunnel.**
- **You want a robust daemon + CLI** instead of raw shell process management.
- **You need automatic health gates** and cascading failure recovery.

[![Build](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/build.yml/badge.svg?branch=master)](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/build.yml)
[![Release](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/release.yml/badge.svg)](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/BrunoSilvaFreire/tunneld)](https://github.com/BrunoSilvaFreire/tunneld/blob/master/go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## The Killer Difference: Why not `autossh`?

`autossh` keeps one SSH process alive. **`tunneld` supervises a graph of tunnels.**

| Feature | `autossh` | `tunneld` |
|---------|-----------|-----------|
| **Persistent Process** | ✅ | ✅ |
| **Dependency-Ordered Startup** | ❌ | ✅ |
| **TCP Health Gates** | ❌ | ✅ |
| **Cascading Downstream Stops** | ❌ | ✅ |
| **Kubectl Port-Forward Native** | ❌ | ✅ |
| **Programmatic gRPC/CLI Control** | ❌ | ✅ |

`tunneld` ensures that if your bastion SSH tunnel goes down, your dependent `kubectl port-forward` is stopped immediately and only restarted once the bastion is healthy again.

---

## Features

- **Persistent SSH & Kubectl** — Keep local forwards and port-forwards alive across disconnects.
- **Smart Dependencies** — Define your infrastructure as a graph; `tunneld` handles the ordering.
- **Health Checks** — Built-in TCP probes ensure a tunnel is truly ready before starting dependents.
- **Exponential Backoff** — Smart retry logic with configurable multipliers and maximum delays.
- **gRPC API** — Programmatic control for automation, Pulumi, or Argo Workflows.
- **CLI Client (`tunnelctl`)** — Powerful control with `status`, `logs`, `start/stop`, and `load`.
- **Ansible Collection** — Declarative tunnel management from your playbooks.

## Demo

![tunneld demo](docs/demo.gif)
*(Asciinema/GIF showing `tunnelctl status` and automatic recovery when an upstream tunnel is killed)*

## Quick Start

```bash
# Pull the images
docker pull ghcr.io/brunosilvafreire/tunneld:latest
docker pull ghcr.io/brunosilvafreire/tunnelctl:latest

# Start the daemon
docker run -d \
  --name tunneld \
  -v $(pwd)/tunnels.yaml:/etc/tunneld/tunnels.yaml:ro \
  -v /run/tunneld/tunneld.sock:/run/tunneld/tunneld.sock \
  ghcr.io/brunosilvafreire/tunneld:latest

# Check status
tunnelctl status
```

## Installation

### Binary Installation (Linux)

```bash
# Download and install the latest .deb
curl -fsSL https://github.com/BrunoSilvaFreire/tunneld/releases/latest/download/tunneld_amd64.deb -o tunneld.deb
sudo dpkg -i tunneld.deb
sudo systemctl enable --now tunneld
```

### Docker

```bash
docker run --rm -v $(pwd)/tunnels.yaml:/etc/tunneld/tunnels.yaml \
  ghcr.io/brunosilvafreire/tunneld:latest
```

### Kubernetes

A controller + per-node agent expose tunnels as Kubernetes `Service`s via three
CRDs (`Tunnel`, `TunnelGroup`, `TunnelKey`). One-shot install:

```bash
kubectl apply -k k8s/config/default
```

See [docs/kubernetes.md](docs/kubernetes.md) for deployment modes (host-installed
vs DaemonSet-bundled tunneld), networking notes, and example manifests.

## Configuration

Tunnels are defined in YAML. `tunneld` distinguishes between absolute file paths (`*_file`) and daemon-managed keys (`*_ref`).

### Example: Bastion → Kubernetes Dashboard

```yaml
tunnels:
  bastion:
    enabled: true
    type: ssh
    ssh:
      user: user
      host: bastion.example.com
      port: 22
      identity_key_file: /home/user/.ssh/id_ed25519
      local_forwards:
        - listen_address: 127.0.0.1
          listen_port: 16443
          target_host: kubeapi.internal
          target_port: 6443
    health:
      type: tcp
      address: 127.0.0.1:16443
      interval: 5s
      timeout: 2s
      startup_timeout: 30s
    restart:
      policy: always
      delay: 5s
      backoff:
        multiplier: 2
        max_delay: 1m

  dashboard:
    enabled: true
    type: kubectl
    depends_on:
      - bastion
    kubectl:
      kubeconfig_ref: prod-kubeconfig # Managed by tunneld
      context: production
      namespace: kubernetes-dashboard
      resource: svc/kubernetes-dashboard
      api_server: https://127.0.0.1:16443
      insecure_skip_tls_verify: true
      forwards:
        - local_address: 127.0.0.1
          local_port: 8443
          remote_port: 443
    health:
      type: tcp
      address: 127.0.0.1:8443
    restart:
      policy: always
      delay: 2s
```

## CLI Cheat Sheet

```bash
tunnelctl status                           # List all tunnels
tunnelctl logs dashboard --follow          # Stream logs
tunnelctl wait dashboard                   # Wait until healthy
tunnelctl create my-tunnel --config spec.yaml --persistent
tunnelctl key add my-key --file ~/.ssh/id_rsa
```

## Security Model

`tunneld` is designed to run as a system daemon (user `tunneld`).
- **Socket Permissions:** The gRPC socket at `/run/tunneld/tunneld.sock` should be restricted to the `tunneld` group.
- **Key Storage:** Keys uploaded via `tunnelctl key add` are stored in `/var/lib/tunneld/keys` with `0600` permissions, owned by the `tunneld` user.
- **Process Isolation:** Tunnels run as sub-processes of the daemon.

## Project Status

`tunneld` is early but usable.

**Current supported tunnel types:**
- SSH local forwards (`-L`)
- `kubectl port-forward`

**Known limitations:**
- No reverse SSH forwarding (`-R`) yet.
- No SOCKS proxy (`-D`) yet.
- No Windows service support yet.

## License

[MIT](LICENSE)
