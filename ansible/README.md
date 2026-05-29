# tunneld Ansible Collection

This collection provides custom Ansible modules for managing `tunneld` tunnels via the `tunnelctl` CLI.

## Modules

- `tunneld_tunnel` — Manage the lifecycle of a `tunneld` tunnel (`create`, `load`, `start`, `stop`, `delete`).
- `tunneld_info` — Gather facts about tunnels managed by `tunneld`.

## Prerequisites

- Python 3.x
- Ansible
- `tunneld` and `tunnelctl` installed and configured on the target system (or locally if running against `localhost`).

## Installation

### Via Ansible Galaxy (Recommended)

Install the collection directly from GitHub:

```bash
ansible-galaxy collection install git+https://github.com/BrunoSilvaFreire/tunneld.git#/ansible
```

When using this method, reference modules by their **fully qualified collection name (FQCN)**:

```yaml
- name: Ensure an SSH tunnel is running
  tunneld.tunneld.tunneld_tunnel:
    name: my-ssh-tunnel
    state: started
    persistent: true
    definition:
      enabled: true
      type: ssh
      ssh:
        user: admin
        host: remote.example.com
        port: 22
        local_forwards:
          - listen_port: 8080
            target_host: localhost
            target_port: 80
      health:
        type: tcp
        address: localhost:8080
        interval: 5s

- name: Gather facts about all tunnels
  tunneld.tunneld.tunneld_info:
  register: all_tunnels
```

### Manual Installation

If you prefer not to install via Galaxy, add the `plugins/modules` directory to your `ANSIBLE_LIBRARY` environment variable, or place it in your playbook's `library/` directory (or configure the `library` path in `ansible.cfg`).

Example using the environment variable:

```bash
export ANSIBLE_LIBRARY=/path/to/tunneld/ansible/plugins/modules
ansible-playbook playbook.yaml
```

When using manual installation, you can use the short module names:

```yaml
- name: Ensure an SSH tunnel is running
  tunneld_tunnel:
    name: my-ssh-tunnel
    state: started
    persistent: true
    definition:
      ...
```

## Usage

### Managing Tunnels (`tunneld_tunnel`)

The `tunneld_tunnel` module allows declarative management of tunnels.
Set `persistent: true` when newly loaded tunnels should be saved by `tunneld`
and survive daemon restarts.

```yaml
- name: Ensure an SSH tunnel is running
  tunneld.tunneld.tunneld_tunnel:
    name: my-ssh-tunnel
    state: started
    persistent: true
    definition:
      enabled: true
      type: ssh
      ssh:
        user: admin
        host: remote.example.com
        port: 22
        local_forwards:
          - listen_port: 8080
            target_host: localhost
            target_port: 80
      health:
        type: tcp
        address: localhost:8080
        interval: 5s
```

**States:**

- `started` — Ensures the tunnel exists and is running.
- `stopped` — Ensures the tunnel exists but is stopped.
- `present` — Ensures the tunnel exists (loaded into `tunneld`).
- `absent` — Ensures the tunnel is deleted.

### Gathering Facts (`tunneld_info`)

```yaml
- name: Gather facts about all tunnels
  tunneld.tunneld.tunneld_info:
  register: all_tunnels

- debug:
    var: all_tunnels.tunnels
```

## Testing

A `test_playbook.yaml` is provided to demonstrate functionality locally. You can execute it if you have `tunneld` running and `ansible` installed.

```bash
# Terminal 1: Start tunneld
./tunneld --config tunnels.yaml.sample

# Terminal 2: Run playbook
export ANSIBLE_LIBRARY=./ansible/plugins/modules
ansible-playbook ansible/test_playbook.yaml
```
