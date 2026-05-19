#!/usr/bin/python
# -*- coding: utf-8 -*-

from __future__ import (absolute_import, division, print_function)
__metaclass__ = type

DOCUMENTATION = r'''
---
module: tunneld_info
short_description: Gather facts about tunneld tunnels
description:
  - Gathers facts about tunnels managed by tunneld.
options:
  name:
    description: Filter facts for a specific tunnel name.
    type: str
  bin_path:
    description: Explicit path to the C(tunnelctl) binary.
    type: str
  socket_path:
    description: Path to the tunneld Unix socket.
    type: str
author:
  - Gemini CLI
'''

EXAMPLES = r'''
- name: Gather facts about all tunnels
  tunneld_info:

- name: Gather facts about a specific tunnel
  tunneld_info:
    name: my-ssh-tunnel
'''

from ansible.module_utils.basic import AnsibleModule

class TunneldInfo(object):
    def __init__(self, module):
        self.module = module
        self.bin_path = module.params.get('bin_path') or module.get_bin_path('tunnelctl', required=True)
        self.socket_path = module.params.get('socket_path')

    def run_command(self, args):
        cmd = [self.bin_path]
        if self.socket_path:
            cmd.extend(['--socket', self.socket_path])
        cmd.extend(args)
        return self.module.run_command(cmd)

    def get_tunnels(self, name=None):
        args = ['status']
        if name:
            args.append(name)
        
        rc, stdout, stderr = self.run_command(args)
        if rc != 0:
            if name and "not found" in stderr.lower():
                return []
            self.module.fail_json(msg="Failed to get tunnel status: %s" % stderr)
        
        lines = stdout.strip().splitlines()
        if len(lines) < 2:
            return []

        detailed = {}
        for line in lines:
            if ':' not in line:
                continue
            key, value = line.split(':', 1)
            detailed[key.strip().lower()] = value.strip()
        if detailed.get('tunnel') and detailed.get('status'):
            return [{
                'name': detailed['tunnel'],
                'type': detailed.get('type', ''),
                'status': detailed['status'],
                'dependencies': []
            }]
        
        tunnels = []
        for line in lines[1:]:
            parts = line.split()
            if len(parts) >= 3:
                tunnels.append({
                    'name': parts[0],
                    'type': parts[1],
                    'status': parts[2],
                    'dependencies': parts[3] if len(parts) > 3 else []
                })
        return tunnels

def main():
    module = AnsibleModule(
        argument_spec=dict(
            name=dict(type='str'),
            bin_path=dict(type='str'),
            socket_path=dict(type='str'),
        ),
        supports_check_mode=True
    )

    name = module.params.get('name')
    info = TunneldInfo(module)
    tunnels = info.get_tunnels(name)

    module.exit_json(changed=False, tunnels=tunnels)

if __name__ == '__main__':
    main()
