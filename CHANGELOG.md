# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
A PATCH release with only a base image bump is cut automatically.

## [Unreleased]

## [0.2.0] - 2026-10-31

### Added

- `kuberex install`, for an init container, installs Rex from apt into
  overlays on `/usr`, `/etc` and `/var` kept on an emptyDir at `/ov`.
  `REX_APT_AUTH` names the file with the rex apt credential (default
  `/etc/rex-apt/auth.conf`); `REX_VERSION` pins its version (default newest).
- `KUBEREX_PACKAGES` lists apt package specs `kuberex install` adds after Rex,
  such as `claude-code=2.1.309-1`.
- Apt sources and keyrings mounted under `/etc/kuberex/apt/sources.list.d/` and
  `/etc/kuberex/apt/keyrings/` are added before the install; credentials under
  `/etc/kuberex/apt/auth.conf.d/` are used where they are mounted.
- `kuberex run` mounts the same overlays and starts the Rex server as `rex`
  with no capabilities.
- The server joins a tailnet as `TS_HOSTNAME` (default `kuberex`), enrolling
  with the join key in `TS_AUTHKEY_FILE` on the control server in
  `TS_CONTROL_URL` (default Tailscale's).
- `git`, Go, Node.js, `build-essential` and `openssh-client` in the image.

### Changed

- **Breaking:** the image is `sarak.sh/mikluko/kuberex`, published for
  `linux/amd64` and `linux/arm64`.
- **Breaking:** the image is built on `debian:trixie-20261027` instead of
  `machine-debian`, without systemd or unbound.
- **Breaking:** Rex and Claude Code are no longer in the image; Claude Code
  comes only through `KUBEREX_PACKAGES`. The pod needs
  `hostUsers: false`, `CAP_SYS_ADMIN`, an unconfined AppArmor profile and the
  init container running `kuberex install`.
- The `rex` account has uid and gid 999.

## [0.1.0] - 2026-10-30

### Added

- Image `sarak.sh/mikluko/machine-rex`, built on
  `sarak.sh/mikluko/machine-debian:13.20261027.0`, published as `0.1.0`,
  `0.1` and `latest`.
- The Rex server starts as the container command under `dumb-init`, as the
  `rex` account, listening on `/run/rex/server.sock`.
- Claude Code from Anthropic's stable apt channel, at `/usr/bin/claude`.
