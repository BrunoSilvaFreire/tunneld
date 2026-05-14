# Tunneld Agent Knowledge Base

This document provides a technical overview and architectural insights for agents or developers working on the `tunneld` project.

## 1. Project Essence
`tunneld` is a persistent local tunnel supervisor daemon. Its primary role is to manage complex chains of local network tunnels (SSH, Kubectl) where healthy connectivity in one tunnel is a prerequisite for subsequent tunnels in a dependency graph.

## 2. Core Architecture

### Dependency Management (`internal/dependency`)
- **Library:** Uses `github.com/BrunoSilvaFreire/gographs`.
- **Logic:** Tunnels are nodes in a Directed Acyclic Graph (DAG).
- **Planner:** The `Planner` uses Kahn's algorithm for topological sorting to determine startup order.
- **Failure Cascading:** Uses reverse-lookup (BFS/Flood) to identify and stop all dependent tunnels if an upstream dependency fails.

### Supervision Loop (`internal/daemon`)
- **Process:** Each tunnel runs as an `os/exec.Cmd` instance. Logs are streamed to stdout/stderr with a tunnel-specific prefix.
- **Supervisor:** Orchestrates the lifecycle. It ensures that a tunnel is **Healthy** (verified by `internal/health`) before starting its dependents.
- **Reactivity:** Listens for process exits or health check failures. Performs automated restarts based on `RestartPolicySpec`.

### Communication Layer (`internal/api`)
- **gRPC:** The primary control mechanism. Defined in `api/v1/tunnel.proto` and `api/v1/spec.proto`.
- **Socket:** Listens on a Unix Domain Socket (default: `/run/tunneld/tunneld.sock`).
- **Dynamic Control:** Supports runtime creation and deletion of tunnels without restarting the daemon.

## 3. Protocol & Data Structures
The system is Protobuf-first. All configurations (`TunnelSpec`) and statuses are defined in `.proto` files to ensure type fidelity between the daemon (`tunneld`), the client (`tunnelctl`), and external automation pipelines.

### Supported Tunnel Types
1. **SSH:** Local port forwarding (`-L`).
2. **Kubectl:** Kubernetes port-forwarding for services or resources.

## 4. Operational Workflows

### Development
- **Local Linking:** Uses `go.work` to link to the sibling `gographs` library. **Do not commit `go.work`**.
- **Protobuf Generation:** Use `make proto`. Requires `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc`.

### CI/CD
- **Workflows:** Located in `.github/workflows/`.
- **Packaging:** Produces `.deb` (Debian/Ubuntu) and `.zip` artifacts.
- **Release:** Triggered by `v*` tags or manual `workflow_dispatch`. Requires a `tag_name` for manual runs.
- **Checksums:** Uses `GONOSUMDB=github.com/BrunoSilvaFreire` to handle newly created, non-indexed modules.

## 5. Integration Patterns

### Manual Control
Use `tunnelctl`:
```bash
tunnelctl status
tunnelctl create my-tunnel --config spec.yaml
tunnelctl wait my-tunnel --timeout 60
```

### Automation (e.g., Pulumi/Argo)
Pipelines should interact with `tunneld` via the gRPC socket to establish necessary infrastructure tunnels (bastions, S3 forwards) dynamically before performing data-heavy operations (like tile rendering/publishing).

## 6. Engineering Standards
- **Surgical Updates:** Prefer targeted `replace` or `write_file` calls over bulk rewrites.
- **Context Efficiency:** Keep the `go.sum` and `go.mod` synchronized without hardcoded local paths in committed code.
- **Logging:** All lifecycle events (Start, Stop, Fail, Restart) must be logged with the tunnel name as a prefix.
