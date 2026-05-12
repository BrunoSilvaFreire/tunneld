# tunneld Ansible Modules

This directory contains custom Ansible modules for managing `tunneld` tunnels via the `tunnelctl` CLI.

## Modules

*   `tunneld_tunnel`: Manage the lifecycle of a `tunneld` tunnel (create, load, start, stop, delete).
*   `tunneld_info`: Gather facts about tunnels managed by `tunneld`.

## Prerequisites

*   Python 3.x
*   Ansible
*   `tunneld` and `tunnelctl` installed and configured on the target system (or locally if running against `localhost`).

## Installation

To use these modules in your playbooks, you can add this directory to your `ANSIBLE_LIBRARY` environment variable, or place the `plugins/modules` directory in the root of your Ansible playbook repository alongside your playbooks, typically structured as `library/` or configure the `library` path in `ansible.cfg`.

Example using environment variable:
```bash
export ANSIBLE_LIBRARY=/path/to/tunneld/ansible/plugins/modules
ansible-playbook playbook.yaml
```

## Usage

### Managing Tunnels (`tunneld_tunnel`)

The `tunneld_tunnel` module allows declarative management of tunnels.

```yaml
- name: Ensure an SSH tunnel is running
  tunneld_tunnel:
    name: my-ssh-tunnel
    state: started
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
*   `started`: Ensures the tunnel exists and is running.
*   `stopped`: Ensures the tunnel exists but is stopped.
*   `present`: Ensures the tunnel exists (loaded into `tunneld`).
*   `absent`: Ensures the tunnel is deleted.

### Gathering Facts (`tunneld_info`)

```yaml
- name: Gather facts about all tunnels
  tunneld_info:
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
