# Security Model

`tunneld` manages sensitive infrastructure access. Understanding its security model is crucial for a safe deployment.

## 1. Least Privilege

The `tunneld` daemon is designed to run as a non-root system user (typically `tunneld`). It only requires:
- Network access to reach SSH bastions and Kubernetes APIs.
- Write access to its own state directories (`/var/lib/tunneld`, `/run/tunneld`).
- Read access to any local `identity_key_file` or `kubeconfig_file` you specify in the configuration.

## 2. Key Management

- **Local Files:** When you use `identity_key_file`, the daemon reads the file directly. Ensure the `tunneld` user has read permissions.
- **Managed Keys:** When you use `tunnelctl key add`, the key content is sent over the gRPC socket and stored in `/var/lib/tunneld/keys`. These files are created with `0600` permissions and owned by `tunneld`.

## 3. gRPC Socket Security

The primary attack surface is the Unix Domain Socket.
- Default path: `/run/tunneld/tunneld.sock`.
- Recommended permissions: `0750` on the `RuntimeDirectory` (`/run/tunneld`), owned by `tunneld:tunneld`.
- Anyone in the `tunneld` group can control the daemon, create tunnels, and access logs.

## 4. Kubernetes Access

`tunneld` uses the standard `kubectl` binary for port-forwarding. It inherits the security properties of `kubectl`. Using `kubeconfig_ref` allows the daemon to manage the configuration, while `kubeconfig_file` allows you to point to existing files.

## 5. Best Practices

- **Avoid running as root.**
- **Use the `tunneld` group** to delegate control to specific users.
- **Use Managed Keys (`*_ref`)** for a cleaner separation of concerns, especially in multi-user environments.
- **Audit your `tunnels.yaml`** for unauthorized `local_forwards` that might expose internal services to the wrong interfaces.
