# Tunneld
*(Pronounced "tunneled")*

[![Build](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/build.yml/badge.svg)](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/build.yml)
[![Release](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/release.yml/badge.svg)](https://github.com/BrunoSilvaFreire/tunneld/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/BrunoSilvaFreire/tunneld)](https://github.com/BrunoSilvaFreire/tunneld/blob/main/go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Persistent tunnel supervisor for SSH and Kubernetes port-forwards. Manages chained tunnel dependencies, health checks,
and lifecycle automation via a gRPC API.

---

## Why Tunneld?

Manually maintaining a chain of SSH local forwards (`-L`) and `kubectl port-forward` sessions is brittle. Network
interruptions, host sleep cycles, or upstream restarts break the chain and require manual intervention to re-establish
connectivity in the correct order.

Tunneld replaces manual tunnel management with a declarative configuration model. Tunnels are defined as nodes in a
Directed Acyclic Graph (DAG). The supervisor resolves startup order topologically, gates dependent tunnels on upstream
health checks, and propagates failure downstream — automatically restarting or terminating tunnels according to their
configured policies.

## Use Case: Chained Port-Forwarding

Consider accessing a Kubernetes service through a bastion host:

1. An SSH tunnel exposes the Kubernetes API server on a local port.
2. A `kubectl port-forward` targets a service through that local API endpoint.

Without a supervisor, the second tunnel fails if the first is not yet ready, or if the SSH session drops and the local
port disappears. Tunneld models this as a dependency: the `kubectl` tunnel declares `depends_on: [bastion]`. The
supervisor ensures the SSH tunnel passes its TCP health probe before starting the `kubectl` tunnel, and stops the
latter if the former becomes unhealthy.

| Tool | Persistent | Dependencies | Health Checks | K8s Port-Forward | Programmatic API |
|------|-----------|-------------|---------------|------------------|------------------|
| `autossh` | ✅ | ❌ | ❌ | ❌ | ❌ |
| `sshuttle` | ❌ | ❌ | ❌ | ❌ | ❌ |
| `kubectl port-forward` (raw) | ❌ | ❌ | ❌ | ✅ | ❌ |
| **Tunneld** | ✅ | ✅ | ✅ | ✅ | ✅ |

## Features

- **Declarative Tunnels** — Manage SSH (`-L`) and Kubernetes port-forwards via YAML.
- **Smart Dependencies** — DAG-based startup order with automatic cascading stop on failure.
- **Health Checks** — Built-in TCP probes ensure a tunnel is truly ready before starting dependents.
- **Auto-Restart** — Configurable restart policies (`always`, `on-failure`, `never`) with backoff.
- **gRPC API** — Programmatic control for automation pipelines, custom scripts, or infrastructure-as-code.
- **CLI Client (`tunnelctl`)** — Intuitive day-to-day operations with built-in bash completion.
- **Ansible Collection** — Declarative tunnel management from your playbooks.
- **Docker Images** — Pre-built images published to GitHub Container Registry.

## Quick Start

```bash
# Pull the images
docker pull ghcr.io/brunosilvafreire/tunneld:latest
docker pull ghcr.io/brunosilvafreire/tunnelctl:latest

# Start the daemon (see Configuration below for tunnels.yaml)
docker run -d \
  --name tunneld \
  -v $(pwd)/tunnels.yaml:/etc/tunneld/tunnels.yaml:ro \
  -v /tmp/tunneld.sock:/tmp/tunneld.sock \
  ghcr.io/brunosilvafreire/tunneld:latest \
  --config /etc/tunneld/tunnels.yaml run

# Check status
docker run --rm \
  -v /tmp/tunneld.sock:/tmp/tunneld.sock \
  ghcr.io/brunosilvafreire/tunnelctl:latest status
```

## Installation

### Download a Release

The latest `.deb` (Debian/Ubuntu) and `.zip` assets are attached to each
[GitHub Release](https://github.com/BrunoSilvaFreire/tunneld/releases).

```bash
# Fetch the download URL for the latest amd64 .deb from the GitHub API
DEB_URL=$(curl -s https://api.github.com/repos/BrunoSilvaFreire/tunneld/releases/latest \
  | grep "browser_download_url.*_amd64.deb" \
  | cut -d '"' -f 4)

# Download and install
curl -sL "$DEB_URL" -o tunneld_latest.deb
sudo dpkg -i tunneld_latest.deb
```

### Docker / GitHub Container Registry

Multi-arch images are published to GHCR for every release:

```bash
# Run the daemon
docker run --rm -v $(pwd)/tunnels.yaml:/etc/tunneld/tunnels.yaml \
  ghcr.io/brunosilvafreire/tunneld:latest \
  --config /etc/tunneld/tunnels.yaml run

# Run the CLI
docker run --rm -v /tmp/tunneld.sock:/tmp/tunneld.sock \
  ghcr.io/brunosilvafreire/tunnelctl:latest status
```

### Build from Source

Requires Go 1.26+:

```bash
make build
sudo make package-deb
sudo dpkg -i tunneld_*_amd64.deb
```

## Configuration

Tunnels are defined in a YAML file. Each tunnel declares its type (`ssh` or `kubectl`), health check parameters, restart policy, and optionally a list of dependencies.

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
      identity_key: /var/lib/tunneld/keys/id_ed25519
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

  dashboard:
    enabled: true
    type: kubectl
    depends_on:
      - bastion
    kubectl:
      kubeconfig: /etc/tunneld/kubeconfig
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
      interval: 5s
      timeout: 2s
      startup_timeout: 30s
    restart:
      policy: always
      delay: 5s
```

### Docker Compose

For a persistent container deployment:

```yaml
services:
  tunneld:
    image: ghcr.io/brunosilvafreire/tunneld:latest
    command: ["--config", "/etc/tunneld/tunnels.yaml", "run"]
    volumes:
      - ./tunnels.yaml:/etc/tunneld/tunnels.yaml:ro
      - ~/.ssh/id_ed25519:/var/lib/tunneld/keys/id_ed25519:ro
      - ./kubeconfig:/etc/tunneld/kubeconfig:ro
      - tunneld-socket:/tmp

  tunnelctl:
    image: ghcr.io/brunosilvafreire/tunnelctl:latest
    profiles: ["cli"]
    volumes:
      - tunneld-socket:/tmp
    entrypoint: ["/usr/local/bin/tunnelctl"]
    command: ["status"]

volumes:
  tunneld-socket:
```

```bash
docker compose up -d tunneld
docker compose run --rm tunnelctl status
```

## CLI Cheat Sheet

```bash
# Daemon
tunneld --config tunnels.yaml run          # Start the supervisor

# Status & Control
tunnelctl status                           # List all tunnels
tunnelctl status dashboard                 # Show one tunnel
tunnelctl logs dashboard                   # Stream logs
tunnelctl logs dashboard --follow          # Follow logs in real time

# Lifecycle
tunnelctl start dashboard                  # Start a tunnel
tunnelctl stop dashboard                   # Stop a tunnel
tunnelctl wait dashboard --timeout 60      # Wait until healthy

# Management
tunnelctl create my-tunnel --config spec.yaml   # Create from YAML
tunnelctl delete my-tunnel                      # Remove a tunnel
tunnelctl enable my-tunnel                      # Mark as enabled
tunnelctl disable my-tunnel                     # Mark as disabled
tunnelctl load --config tunnels.yaml            # Bulk load definitions

# SSH Keys
tunnelctl key add my-key --file ~/.ssh/id_rsa   # Upload a key
tunnelctl key list                                # List uploaded keys
tunnelctl key delete my-key                       # Remove a key
```

## Automation & Integrations

### gRPC API

Tunneld exposes a gRPC API over a Unix domain socket (default `/tmp/tunneld.sock`). The protobuf definitions live in
`api/v1/`. This makes it easy to integrate with Pulumi, Argo Workflows, or any custom automation that needs to spin up
tunnels on demand.

### Ansible

An Ansible collection is included in the `ansible/` directory. See [ansible/README.md](ansible/README.md) for
installation and usage.

```bash
ansible-galaxy collection install git+https://github.com/BrunoSilvaFreire/tunneld.git#/ansible
```

## Architecture at a Glance

- **Dependency Graph** — Tunnels are nodes in a DAG. A topological planner determines startup order using Kahn's
  algorithm.
- **Health Gates** — Before a dependent tunnel starts, its upstream must pass a configurable TCP health check.
- **Cascading Restarts** — If a tunnel fails, a reverse BFS identifies every downstream dependent and stops them
  cleanly. When the parent recovers, the chain restarts automatically.
- **Protobuf-First** — All configuration and status messages are defined in `.proto` files, ensuring type fidelity
  across the daemon, CLI, and any external consumers.

## Development

```bash
make build        # Build tunneld and tunnelctl
make test         # Run the test suite
make proto        # Regenerate protobuf code (requires protoc, protoc-gen-go, protoc-gen-go-grpc)
make package-deb  # Build a .deb package
make package-zip  # Build a .zip archive
make clean        # Remove build artifacts
```

## License

[MIT](LICENSE)
