# Configuration Reference

`tunneld` uses a YAML configuration file to define tunnels and their properties.

## Root Object

| Field | Type | Description |
|-------|------|-------------|
| `tunnels` | `map[string]TunnelConfig` | A map where keys are tunnel names and values are their configurations. |

## TunnelConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | `bool` | `true` | Whether the tunnel should be managed by the supervisor. |
| `type` | `string` | | One of `ssh` or `kubectl`. |
| `depends_on` | `[]string` | `[]` | List of tunnel names that must be healthy before this tunnel starts. |
| `ssh` | `SSHConfig` | | Required if `type` is `ssh`. |
| `kubectl` | `KubectlConfig` | | Required if `type` is `kubectl`. |
| `health` | `HealthCheckConfig` | | Health check parameters. |
| `restart` | `RestartPolicyConfig` | | Restart policy parameters. |
| `startup_timeout` | `duration` | `30s` | How long to wait for the tunnel to start. |
| `shutdown_timeout` | `duration` | `5s` | How long to wait for a clean shutdown. |

## SSHConfig

| Field | Type | Description |
|-------|------|-------------|
| `user` | `string` | SSH username. |
| `host` | `string` | SSH host. |
| `port` | `int` | SSH port (default 22). |
| `identity_key_file` | `string` | Absolute path to a local SSH private key. |
| `identity_key_ref` | `string` | Name of a key managed by the `tunneld` key store. |
| `local_forwards` | `[]SSHForward` | List of local port forwards. |
| `options` | `map[string]string` | Additional SSH options (`-o`). |

## KubectlConfig

| Field | Type | Description |
|-------|------|-------------|
| `kubeconfig_file` | `string` | Absolute path to a local kubeconfig file. |
| `kubeconfig_ref` | `string` | Name of a kubeconfig managed by the `tunneld` key store. |
| `context` | `string` | Kubernetes context to use. |
| `namespace` | `string` | Target namespace. |
| `resource` | `string` | Target resource (e.g., `svc/my-service`, `pod/my-pod`). |
| `api_server` | `string` | Override the API server address. |
| `insecure_skip_tls_verify` | `bool` | Skip TLS verification. |
| `forwards` | `[]KubectlForward` | List of port forwards. |

## RestartPolicyConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `policy` | `string` | `always` | `always`, `on-failure`, or `never`. |
| `delay` | `duration` | `5s` | Delay between restart attempts. |
| `max_attempts` | `int` | `0` (unlimited) | Maximum number of restart attempts. |
| `backoff` | `BackoffConfig` | | Optional exponential backoff. |

## BackoffConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `multiplier` | `float` | `2.0` | Multiplier for the delay after each failure. |
| `max_delay` | `duration` | | Cap for the maximum delay. |
