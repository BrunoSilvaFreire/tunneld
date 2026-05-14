# Example: Postgres Through Bastion

Sometimes you just need to access a database sitting behind a firewall.

## The Configuration

```yaml
tunnels:
  bastion:
    type: ssh
    ssh:
      user: db-admin
      host: bastion.db-zone.com
      identity_key_file: ~/.ssh/id_rsa
      local_forwards:
        - listen_port: 5432
          target_host: pg-prod.internal
          target_port: 5432
    health:
      type: tcp
      address: 127.0.0.1:5432
    restart:
      policy: always
      delay: 5s
      backoff:
        multiplier: 1.5
        max_delay: 30s
```

## Why use tunneld for this?

While a simple `ssh -L` command works, `tunneld` provides:
1. **Visibility:** `tunnelctl status` shows you exactly if the DB connection is healthy.
2. **Reliability:** If your laptop goes to sleep or you switch Wi-Fi, `tunneld` brings the connection back up automatically.
3. **Logging:** `tunnelctl logs bastion` gives you visibility into the SSH process output, helping you debug auth or networking issues.
4. **Ansible Integration:** You can ensure this tunnel exists on your team's development machines using the included Ansible collection.
