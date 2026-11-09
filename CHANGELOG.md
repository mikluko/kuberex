# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.19.0] - 2026-11-09

### Added

- Restart: an account in chart value `restartAccounts`, passed as
  `KUBEREX_RESTART_ACCOUNTS`, runs `kuberex-control restart` to restart the
  server container. Every account's Rex server stops cleanly, so its sessions
  come back, and the container exits with code 0 for Kubernetes to start it
  again with its services.

## [0.18.0] - 2026-11-09

### Changed

- Rex is `0.0.1794237812-g75d0b3`.
- Claude Code is `2.1.311-1`.

## [0.17.0] - 2026-11-09

### Changed

- Claude Code no longer checks for updates, prints "Update available!" or
  sends telemetry, error reports and other nonessential traffic: the managed
  settings in `/etc/claude-code/managed-settings.d/kuberex.json` set
  `DISABLE_UPDATES`, `DISABLE_AUTOUPDATER`, `DISABLE_TELEMETRY`,
  `DISABLE_ERROR_REPORTING` and `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`.
  Remote Control is unavailable with them.

## [0.16.0] - 2026-11-09

### Added

- Chart values `extraVolumes` and `extraVolumeMounts`, which add volumes to
  the pod and mount them in the server's container.
- Account Secrets: chart value `accountSecrets` mounts Secrets per account,
  and the entrypoint copies their keys through `KUBEREX_SECRETS` to
  `/run/kuberex/secrets/<account>/<secret>/<key>`, readable only by that
  account.

## [0.15.0] - 2026-11-09

### Added

- Services: commands that start with the pod, each in a labelled Rex session
  of an account's own server, from chart value `services` through
  `KUBEREX_SERVICES` and `kuberex-services`. A session restored from before
  the restart under the same label is replaced.

## [0.14.0] - 2026-11-09

### Added

- A Claude Code session running in a Rex pane when the pod stops is resumed
  in that pane once Rex restores it, through `kuberex-claude` and Claude Code
  hooks in `/etc/claude-code/managed-settings.d/kuberex.json`.

## [0.13.0] - 2026-11-09

### Changed

- Base image is `debian:trixie-20261104`.
- Rex is `0.0.1794111130-g9bcf6a`.

## [0.12.1] - 2026-11-05

### Fixed

- `TMPDIR` and `TMP` are set: the entrypoint makes `/tmp/user/<uid>` for
  every account, which `pam_tmpdir` cannot create with the account's uid.

## [0.12.0] - 2026-11-05

### Added

- Each account gets a private temporary directory, `/tmp/user/<uid>`, as
  `TMPDIR` and `TMP`, from `libpam-tmpdir`.

### Fixed

- `/tmp` is owned by root with mode `1777`, so an account can no longer
  delete or replace another account's files there.

## [0.11.0] - 2026-11-05

### Added

- `KUBEREX_APT`, a directory of apt sources, keyrings and packages installed
  at every start, and chart values `apt.sources`, `apt.keyrings` and
  `apt.packages` that fill it.

### Changed

- **Breaking:** the login server is the only mode. The container runs as root
  and needs `REX_LOGIN_SERVER_ACCOUNTS` and `TS_SOCKET`, and the chart always
  runs the `tailscaled` sidecar.
- **Breaking:** chart value `loginServer.accounts` is now `accounts`, and must
  name an account; `tailscale.sidecar.port`, `.image` and `.resources` are now
  `tailscale.port`, `tailscale.image` and `tailscale.resources`.

### Removed

- **Breaking:** the single server as `rex` and Rex's built-in Tailscale node,
  with `TS_HOSTNAME`, `TS_AUTHKEY_FILE`, `TS_CONTROL_URL` and
  `REX_ALLOW_TAILNET_MEMBERS`.
- **Breaking:** chart values `loginServer.enabled`,
  `tailscale.sidecar.enabled` and `tailscale.allowTailnetMembers`.

## [0.10.0] - 2026-11-05

### Added

- Chart values `persistence.size` and `persistence.storageClassName`: without
  `stateVolume`, the chart claims a volume for `/var/lib/rex` itself, 10 GiB
  of the default storage class.

### Changed

- **Breaking:** the chart runs a StatefulSet instead of a Deployment. An
  upgrade replaces the pod `<release>-<hash>` with `<release>-0`.
- **Breaking:** without `stateVolume`, `/var/lib/rex` is a
  PersistentVolumeClaim instead of an `emptyDir`.

## [0.9.0] - 2026-11-05

### Changed

- The login server's accounts get their homes in `/home/<name>` instead of
  `/var/lib/rex/home/<name>`; the chart mounts `home/` of the state volume
  there, so an existing home keeps its files. `/var/lib/rex` is no longer
  opened to the accounts.

## [0.8.0] - 2026-11-05

### Added

- With `REX_LOGIN_SERVER_ACCOUNTS` set to an accounts file, the container
  runs as root and starts `rex login-server` on the tailnet through
  `TS_SOCKET`: it creates every account the file names, but `root` and `"*"`,
  with `zsh` and a home in `/var/lib/rex/home`, and serves each caller as the
  account the file maps the caller to.
- Chart values `loginServer.enabled` and `loginServer.accounts`, which run the
  login server as root with the capabilities `CHOWN`, `DAC_OVERRIDE`,
  `FOWNER`, `KILL`, `SETGID` and `SETUID` on a writable root filesystem.

## [0.7.0] - 2026-11-05

### Added

- Terminal tools from Debian: `htop`, `mc`, `ripgrep`, `fd-find` (as
  `fdfind`), `fzf`, `tree`, `bat` (as `batcat`), `rsync`, `zip` and `ncdu`.

## [0.6.0] - 2026-11-05

### Added

- `zsh`, the login shell of `rex`.

### Fixed

- A terminal opened in the server starts `zsh`: the login shell of `rex` was
  `nologin`, so every terminal printed "This account is currently not
  available." and closed.

## [0.5.0] - 2026-11-05

### Added

- With `TS_SOCKET` set, the server serves plain HTTP on the tailnet at port
  `REX_TAILNET_PORT` (default `80`) through the `tailscaled` behind that
  LocalAPI socket, instead of joining as an embedded node; this works with a
  control server that issues no HTTPS certificates, such as Headscale.
- `REX_ALLOW_TAILNET_MEMBERS=true` admits every tailnet member the ACLs let
  through, not only the node's owner.
- Chart values `tailscale.sidecar.*`, running `tailscaled` from
  `tailscale/tailscale:v1.104.4` as a sidecar in userspace networking with
  its state in `/var/lib/rex/tailscaled`, and `tailscale.allowTailnetMembers`.

## [0.4.0] - 2026-11-02

### Added

- Helm chart `oci://sarak.sh/mikluko/charts/kuberex`, released at the
  image's version: one Rex server pod with its tailnet settings, the volume
  of `/var/lib/rex`, a ServiceAccount and an optional CiliumNetworkPolicy
  limiting egress to DNS, the kube-apiserver and outside the cluster.

## [0.3.0] - 2026-11-02

### Added

- Rex `0.0.1793645657-gd018d2` from the Rex `snapshot` apt channel and
  Claude Code `2.1.309-1` from Anthropic's `stable` apt channel, installed
  in the image.

### Changed

- **Breaking:** the container runs as `rex` (uid 999) and its entrypoint
  starts the Rex server directly; the pod needs no init container, no added
  capabilities, no host user namespace setting and no AppArmor override. The
  server starts on a read-only root filesystem when `/run/rex` and
  `/var/lib/rex` are writable.

### Removed

- **Breaking:** `kuberex install`, `kuberex run`, the overlays at `/ov`,
  `REX_APT_AUTH`, `REX_VERSION`, `KUBEREX_PACKAGES` and the apt config mount
  at `/etc/kuberex/apt/`.

### Fixed

- `TS_CONTROL_URL` reaches the server; the tailnet node enrolls with that
  control server.

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
