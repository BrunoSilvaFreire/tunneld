# Getting Started with tunneld

This guide will help you get `tunneld` up and running in minutes.

## 1. Installation

### Debian/Ubuntu

Download the latest `.deb` from the [Releases](https://github.com/BrunoSilvaFreire/tunneld/releases) page.

```bash
curl -fsSL https://github.com/BrunoSilvaFreire/tunneld/releases/latest/download/tunneld_amd64.deb -o tunneld.deb
sudo dpkg -i tunneld.deb
sudo systemctl enable --now tunneld
```

### Binary (other Linux)

Download the `.zip` archive, extract the binaries, and move them to your path.

## 2. Your First Tunnel

Create a file named `my-tunnels.yaml`:

```yaml
tunnels:
  google:
    enabled: true
    type: ssh
    ssh:
      user: your-user
      host: your-ssh-host.com
      port: 22
      identity_key_file: ~/.ssh/id_rsa
      local_forwards:
        - listen_port: 8080
          target_host: www.google.com
          target_port: 80
    health:
      type: tcp
      address: 127.0.0.1:8080
```

## 3. Load and Start

Use `tunnelctl` to load your configuration:

```bash
tunnelctl load my-tunnels.yaml --persistent
```

Check the status:

```bash
tunnelctl status
```

You should see the `google` tunnel starting and then becoming healthy. You can now access Google at `http://localhost:8080`.

## 4. Next Steps

- Learn about [chained dependencies](examples/bastion-kubernetes.md).
- Explore [configuration options](configuration.md).
- Set up [systemd](systemd.md) for a production-grade deployment.
