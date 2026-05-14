# Example: Bastion to Kubernetes

This is the classic use case for `tunneld`: accessing a private Kubernetes cluster through an SSH bastion.

## The Scenario

- **Bastion:** `bastion.corp.com`, reachable via SSH.
- **Kubernetes API:** `kube-internal.cluster.local:6443`, only reachable from the Bastion.
- **Goal:** Run `kubectl` locally and access a service in the cluster.

## The Configuration

```yaml
tunnels:
  # 1. The Gateway
  bastion:
    type: ssh
    ssh:
      user: engineer
      host: bastion.corp.com
      identity_key_file: ~/.ssh/id_rsa
      local_forwards:
        - listen_port: 16443
          target_host: kube-internal.cluster.local
          target_port: 6443
    health:
      type: tcp
      address: 127.0.0.1:16443

  # 2. The Service Tunnel
  api-server:
    type: kubectl
    depends_on:
      - bastion
    kubectl:
      kubeconfig_file: ~/.kube/config
      context: private-cluster
      resource: svc/my-service
      namespace: default
      api_server: https://127.0.0.1:16443 # Point to the bastion forward
      insecure_skip_tls_verify: true
      forwards:
        - local_port: 8080
          remote_port: 80
```

## How it works

1. `tunneld` starts the `bastion` tunnel.
2. It waits for `127.0.0.1:16443` to become responsive.
3. Once the bastion tunnel is healthy, it starts the `api-server` tunnel.
4. If the SSH connection drops, `tunneld` detects the failure, stops the `api-server` tunnel (as it's now broken), and begins trying to restart the `bastion` with exponential backoff.
5. When the `bastion` is back, `api-server` is automatically restarted.
