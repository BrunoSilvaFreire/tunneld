# Running tunneld with systemd

For a permanent installation, it is recommended to run `tunneld` as a systemd service.

## 1. Installation

If you installed the `.deb` package, the systemd unit is already created for you. You just need to enable and start it:

```bash
sudo systemctl enable --now tunneld
```

## 2. Unit File Details

The default unit file (usually at `/lib/systemd/system/tunneld.service`) looks like this:

```ini
[Unit]
Description=Tunnel supervisor
After=network-online.target
Wants=network-online.target

[Service]
User=tunneld
Group=tunneld
ExecStart=/usr/local/bin/tunneld --config /etc/tunneld/tunnels.yaml --socket /run/tunneld/tunneld.sock run
Restart=always
RestartSec=3
KillSignal=SIGTERM
TimeoutStopSec=30
RuntimeDirectory=tunneld
RuntimeDirectoryMode=0750

[Install]
WantedBy=multi-user.target
```

## 3. Permissions

The `tunneld` user needs access to your keys and configuration.

- **Keys:** Upload keys using `tunnelctl key add`, which stores them in `/var/lib/tunneld/keys` with the correct ownership.
- **Config:** Store your global config in `/etc/tunneld/tunnels.yaml`.
- **Persistent Tunnels:** When using `tunnelctl create --persistent`, the daemon saves files to `/etc/tunneld/tunnels.d/`. Ensure the `tunneld` user has write access to this directory.

## 4. Socket Access

By default, the gRPC socket is created at `/run/tunneld/tunneld.sock`. To allow your local user to use `tunnelctl` without `sudo`, add your user to the `tunneld` group:

```bash
sudo usermod -aG tunneld $USER
# Log out and log back in for changes to take effect
```
