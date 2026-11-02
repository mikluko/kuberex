# kuberex

kuberex is the wiring that runs the [Superlogical Rex](https://www.superlogical.com/rex/docs/installation.md) server on Kubernetes.
It ships as the container image `sarak.sh/mikluko/kuberex`, for `linux/amd64` and `linux/arm64`.

## What the image holds

The image is built on `debian:trixie` and installs two packages at versions pinned in the `Dockerfile`:

| Package       | Apt channel                                               | Pin                   |
| ------------- | --------------------------------------------------------- | --------------------- |
| `rex`         | `snapshot` at `https://pkgs.superlogical.com/rex/apt`     | `REX_VERSION`         |
| `claude-code` | `stable` at `https://downloads.claude.ai/claude-code/apt` | `CLAUDE_CODE_VERSION` |

A new Rex or Claude Code version reaches the pod as a new image release.

## Run it

The container runs as `rex` (uid and gid 999) and starts the Rex server on the tailnet as an embedded Tailscale node.
It needs no added capabilities, and the server starts on a read-only root filesystem when these paths are writable:

| Path           | Holds                                                   |
| -------------- | ------------------------------------------------------- |
| `/var/lib/rex` | Rex state, the tailnet node state and the home of `rex` |
| `/run/rex`     | the server sockets                                      |

The server takes its tailnet settings from the environment:

| Variable          | Meaning                             |
| ----------------- | ----------------------------------- |
| `TS_HOSTNAME`     | tailnet hostname, default `kuberex` |
| `TS_AUTHKEY_FILE` | file holding a one-time join key    |
| `TS_CONTROL_URL`  | control server, default Tailscale's |
