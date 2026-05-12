# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Improved project README with quick start, architecture overview, and comparison table.
- Ansible collection installation instructions via `ansible-galaxy`.
- FQCN examples in Ansible documentation.

## [0.1.0] - 2025-05-11

### Added

- Initial release of `tunneld` daemon and `tunnelctl` CLI.
- Declarative YAML configuration for SSH and Kubernetes port-forward tunnels.
- DAG-based dependency management with topological startup ordering.
- Automatic cascading stop and restart of dependent tunnels.
- TCP health checks with configurable intervals and timeouts.
- gRPC API for programmatic tunnel control.
- Bash completion for both `tunneld` and `tunnelctl`.
- Custom Ansible modules (`tunneld_tunnel`, `tunneld_info`).
- Multi-arch Docker images (`amd64`, `arm64`) published to GHCR.
- Debian package (`.deb`) and ZIP archive generation.
- systemd service sample.
- CI/CD pipelines for build, test, and release.
- SSH key management via `tunnelctl key` commands.

[unreleased]: https://github.com/BrunoSilvaFreire/tunneld/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/BrunoSilvaFreire/tunneld/releases/tag/v0.1.0
