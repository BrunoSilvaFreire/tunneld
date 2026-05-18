# Integration TODOs

The SSH and direct k3d kubectl scenarios are intended to pass in the first
distributed integration cut.

The dependency graph and recovery feature files are committed as `@wip`
scenarios because they expose behavior that needs production-side hardening
before they can be made deterministic in CI:

- Dependency graph: `Supervisor.Run` starts tunnels in topological order, but
  `startAndNotify` returns before the upstream tunnel reaches `Running`.
  A passing dependency scenario should wait for each dependency to become
  healthy before starting dependents.
- Recovery: the restart timing depends on SSH keepalive and process exit timing.
  The test should be enabled after restart/degraded-state behavior is made
  deterministic enough for GitHub-hosted runners.
