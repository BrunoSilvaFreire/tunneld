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
  server_address:
    description: The gRPC address of the tunneld server.
    type: str
author:
  - Gemini CLI
'''

EXAMPLES = r'''
- name: Ensure an SSH tunnel is running
  tunneld_tunnel:
    name: my-ssh-tunnel
    state: started
    definition:
      enabled: true
      type: ssh
      ssh:
        user: bruno
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
        self.server_address = module.params.get('server_address')

    def run_command(self, args):
        cmd = [self.bin_path]
        if self.server_address:
            cmd.extend(['--server', self.server_address])
        cmd.extend(args)
        return self.module.run_command(cmd)

    def get_status(self, name):
        rc, stdout, stderr = self.run_command(['status', name])
        if rc != 0:
            if "not found" in stderr.lower():
                return None
            self.module.fail_json(msg="Failed to get tunnel status: %s" % stderr)
        
        # Parse status output. Example:
        # NAME                 TYPE       STATUS     DEPENDENCIES
        # my-ssh-tunnel        ssh        running    []
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

    def load_tunnel(self, name, definition):
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
            
            rc, stdout, stderr = self.run_command(['load', path])
            if rc != 0:
                self.module.fail_json(msg="Failed to load tunnel: %s" % stderr)
        finally:
            os.remove(path)

    def start_tunnel(self, name):
        rc, stdout, stderr = self.run_command(['start', name])
        if rc != 0:
            self.module.fail_json(msg="Failed to start tunnel: %s" % stderr)

    def stop_tunnel(self, name):
        rc, stdout, stderr = self.run_command(['stop', name])
        if rc != 0:
            self.module.fail_json(msg="Failed to stop tunnel: %s" % stderr)

    def delete_tunnel(self, name):
        rc, stdout, stderr = self.run_command(['delete', name])
        if rc != 0:
            self.module.fail_json(msg="Failed to delete tunnel: %s" % stderr)

def main():
    module = AnsibleModule(
        argument_spec=dict(
            name=dict(type='str', required=True),
            state=dict(type='str', default='started', choices=['present', 'absent', 'started', 'stopped']),
            definition=dict(type='dict'),
            bin_path=dict(type='str'),
            server_address=dict(type='str'),
        ),
        supports_check_mode=True
    )

    name = module.params['name']
    state = module.params['state']
    definition = module.params['definition']

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
        
        manager.load_tunnel(name, definition)
        result['changed'] = True
        # Refresh status after load
        current_status = manager.get_status(name)
    else:
        # TODO: Implement diffing if definition is provided to handle updates
        pass

    if state == 'started':
        if current_status['status'] != 'running':
            if module.check_mode:
                module.exit_json(changed=True)
            manager.start_tunnel(name)
            result['changed'] = True
    elif state == 'stopped':
        if current_status['status'] != 'stopped':
            if module.check_mode:
                module.exit_json(changed=True)
            manager.stop_tunnel(name)
            result['changed'] = True

    module.exit_json(**result)

if __name__ == '__main__':
    main()
