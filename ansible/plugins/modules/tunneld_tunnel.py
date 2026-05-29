#!/usr/bin/python
# -*- coding: utf-8 -*-

from __future__ import (absolute_import, division, print_function)
__metaclass__ = type

DOCUMENTATION = r'''
---
module: tunneld_tunnel
short_description: Manage tunneld tunnels
description:
  - Manage the lifecycle of a tunneld tunnel using the tunnelctl CLI.
  - Tunnels can be created, started, stopped, or deleted.
options:
  name:
    description: Name of the tunnel.
    required: true
    type: str
  state:
    description:
      - Desired state of the tunnel.
      - C(present) ensures the tunnel is loaded in tunneld.
      - C(absent) ensures the tunnel is deleted from tunneld.
      - C(started) ensures the tunnel is loaded and running.
      - C(stopped) ensures the tunnel is loaded and stopped.
    default: started
    choices: [present, absent, started, stopped]
    type: str
  definition:
    description:
      - Dictionary defining the tunnel configuration.
      - Required when I(state) is C(present), C(started), or C(stopped) and the tunnel does not exist.
      - The structure matches the C(tunnels.yaml) format for a single tunnel.
    type: dict
  bin_path:
    description: Explicit path to the C(tunnelctl) binary.
    type: str
  socket_path:
    description: Path to the tunneld Unix socket.
    type: str
  persistent:
    description: Whether newly loaded tunnels should be persisted to disk.
    type: bool
    default: no
  wait:
    description: Whether to wait for the tunnel to become healthy before returning.
    type: bool
    default: yes
  wait_timeout:
    description: Timeout in seconds to wait for health.
    type: int
    default: 30
author:
  - Gemini CLI
'''

EXAMPLES = r'''
- name: Ensure an SSH tunnel is running
  tunneld_tunnel:
    name: my-ssh-tunnel
    state: started
    persistent: true
    definition:
      enabled: true
      type: ssh
      ssh:
        user: bruno
        host: remote.example.com
        port: 22
        identity_key_ref: ssh-identity-key
        local_forwards:
          - listen_port: 8080
            target_host: localhost
            target_port: 80
      health:
        type: tcp
        address: localhost:8080
        interval: 5s

- name: Stop a tunnel
  tunneld_tunnel:
    name: my-ssh-tunnel
    state: stopped

- name: Delete a tunnel
  tunneld_tunnel:
    name: my-ssh-tunnel
    state: absent
'''

import os
import yaml
import tempfile
from ansible.module_utils.basic import AnsibleModule

class TunneldManager(object):
    def __init__(self, module):
        self.module = module
        self.bin_path = module.params.get('bin_path') or module.get_bin_path('tunnelctl', required=True)
        self.socket_path = module.params.get('socket_path')

    def _run_command_raw(self, args):
        cmd = [self.bin_path]
        if self.socket_path:
            cmd.extend(['--socket', self.socket_path])
        cmd.extend(args)
        return self.module.run_command(cmd)

    def run_command(self, args):
        rc, stdout, stderr = self._run_command_raw(args)
        if rc != 0:
            self.module.fail_json(msg="Command %s failed (rc=%d): %s %s" % (" ".join(args), rc, stdout, stderr))
        return rc, stdout, stderr

    def get_status(self, name):
        rc, stdout, stderr = self._run_command_raw(['status', name])
        if rc != 0:
            if "not found" in stderr.lower() or "not found" in stdout.lower():
                return None
            self.module.fail_json(msg="Failed to get tunnel status: %s %s" % (stdout, stderr))

        # tunnelctl status <name> prints a detailed format:
        # Tunnel: my-ssh-tunnel
        # Status: running
        detailed = {}
        for line in stdout.strip().splitlines():
            if ':' not in line:
                continue
            key, value = line.split(':', 1)
            detailed[key.strip().lower()] = value.strip()
        if detailed.get('tunnel') and detailed.get('status'):
            return {
                'name': detailed['tunnel'],
                'type': detailed.get('type', ''),
                'status': detailed['status']
            }

        # tunnelctl status prints a table format:
        # NAME                 TYPE       STATUS     PORTS       ERROR     DEPENDENCIES
        # my-ssh-tunnel        ssh        running    127.0.0.1:1           []
        lines = stdout.strip().splitlines()
        if len(lines) < 2:
            return None
        
        # Simple column parsing
        parts = lines[1].split()
        if len(parts) < 3:
            return None
            
        return {
            'name': parts[0],
            'type': parts[1],
            'status': parts[2]
        }

    def load_tunnel(self, name, definition, persistent=False):
        # tunnelctl load expects a full config file with a 'tunnels' map
        full_config = {
            'tunnels': {
                name: definition
            }
        }
        
        fd, path = tempfile.mkstemp(suffix=".yaml")
        try:
            with os.fdopen(fd, 'w') as f:
                yaml.dump(full_config, f)
            
            args = ['load', path]
            if persistent:
                args.append('--persistent')
            rc, stdout, stderr = self._run_command_raw(args)
            if rc != 0:
                self.module.fail_json(msg="Failed to load tunnel: %s %s" % (stdout, stderr))
        finally:
            os.remove(path)

    def start_tunnel(self, name, wait=True, timeout=30):
        self.run_command(['start', name])
        
        if wait:
            # Wait for healthy
            self._run_command_raw(['wait', name, '--timeout', str(timeout)])

    def stop_tunnel(self, name):
        self.run_command(['stop', name])

    def delete_tunnel(self, name):
        self.run_command(['delete', name])

def main():
    module = AnsibleModule(
        argument_spec=dict(
            name=dict(type='str', required=True),
            state=dict(type='str', default='started', choices=['present', 'absent', 'started', 'stopped']),
            definition=dict(type='dict'),
            bin_path=dict(type='str'),
            socket_path=dict(type='str'),
            persistent=dict(type='bool', default=False),
            wait=dict(type='bool', default=True),
            wait_timeout=dict(type='int', default=30),
        ),
        supports_check_mode=True
    )

    name = module.params['name']
    state = module.params['state']
    definition = module.params['definition']
    persistent = module.params['persistent']
    wait = module.params['wait']
    wait_timeout = module.params['wait_timeout']

    manager = TunneldManager(module)
    current_status = manager.get_status(name)

    result = dict(
        changed=False,
        name=name,
        state=state
    )

    if state == 'absent':
        if current_status:
            if module.check_mode:
                module.exit_json(changed=True)
            manager.delete_tunnel(name)
            result['changed'] = True
        module.exit_json(**result)

    # For present, started, stopped: Ensure tunnel exists
    if not current_status:
        if not definition:
            module.fail_json(msg="Definition is required to create a tunnel")
        
        if module.check_mode:
            module.exit_json(changed=True)
        
        manager.load_tunnel(name, definition, persistent=persistent)
        result['changed'] = True
        # Refresh status after load
        current_status = manager.get_status(name)
        if not current_status:
            module.fail_json(msg="Failed to retrieve status for tunnel %q after loading." % name)
    else:
        # TODO: Implement diffing if definition is provided to handle updates
        pass

    if state == 'started':
        if not current_status:
            module.fail_json(msg="Cannot start tunnel %q: status not found" % name)
        if current_status['status'] != 'running':
            if module.check_mode:
                module.exit_json(changed=True)
            manager.start_tunnel(name, wait=wait, timeout=wait_timeout)
            result['changed'] = True
    elif state == 'stopped':
        if not current_status:
            module.exit_json(**result) # Already absent?
        if current_status['status'] != 'stopped':
            if module.check_mode:
                module.exit_json(changed=True)
            manager.stop_tunnel(name)
            result['changed'] = True

    module.exit_json(**result)

if __name__ == '__main__':
    main()
