# Tunneld

Persistent local tunnel supervisor daemon with gRPC control.

## Features

- **Declarative Tunnels:** Manage SSH and Kubectl tunnels via YAML configuration.
- **Dependency Management:** Automatic startup order and failure cascading.
- **gRPC API:** Programmatic control for automation pipelines.
- **CLI Client:** `tunnelctl` for manual interaction.
- **Health Checks:** Built-in TCP connectivity verification.

## Installation

```bash
make build
sudo make package-deb
sudo dpkg -i tunneld_0.1.0_amd64.deb
```

## Usage

```bash
# Start the daemon
tunneld --config /etc/tunneld/tunnels.yaml

# Check status
tunnelctl status
```

## License

MIT
