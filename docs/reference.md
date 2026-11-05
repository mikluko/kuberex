# Image and Chart Reference

## Image

`sarak.sh/mikluko/kuberex:<version>`, for `linux/amd64` and `linux/arm64`, built on `debian:trixie`.
Each release is also tagged `<major>.<minor>` and `latest`.

### Contents

The image installs these packages:

| Package         | Source                                                                | Version                                  |
| --------------- | --------------------------------------------------------------------- | ---------------------------------------- |
| `rex`           | `snapshot` channel of `https://pkgs.superlogical.com/rex/apt`         | pinned as `REX_VERSION` in `Dockerfile`  |
| `claude-code`   | `stable` channel of `https://downloads.claude.ai/claude-code/apt`     | pinned as `CLAUDE_CODE_VERSION`          |
| terminal tools  | Debian: `zsh`, `git`, `vim`, `htop`, `mc`, `ripgrep`, `fd-find`, `fzf`, `bat`, `jq` and others | the Debian snapshot the image is built on |

A new Rex or Claude Code version reaches the pod as a new image release.
The full package list is in the [`Dockerfile`](../Dockerfile).

### User and paths

The container runs as `rex`, uid and gid 999, with the login shell zsh.
It needs no added capabilities, and runs on a read-only root filesystem when these paths are writable:

| Path           | Holds                                                  |
| -------------- | ------------------------------------------------------ |
| `/var/lib/rex` | Rex state, the tailnet node state, the home of `rex`   |
| `/run/rex`     | the server sockets                                     |

### Environment

The server joins the tailnet as Rex's built-in Tailscale node and serves HTTPS at `https://<hostname>.<tailnet>.ts.net`:

| Variable          | Meaning                                         |
| ----------------- | ----------------------------------------------- |
| `TS_HOSTNAME`     | tailnet hostname; default `kuberex`             |
| `TS_AUTHKEY_FILE` | file holding a one-time auth key                |
| `TS_CONTROL_URL`  | control server; default Tailscale's             |

With `TS_SOCKET` set, the server serves plain HTTP on the tailnet through a `tailscaled` already on it, and ignores the three variables above:

| Variable           | Meaning                                    |
| ------------------ | ------------------------------------------ |
| `TS_SOCKET`        | LocalAPI socket of that `tailscaled`       |
| `REX_TAILNET_PORT` | tailnet port; default `80`                 |

In both modes only the node's owner is let in, unless:

| Variable                    | Meaning                                                 |
| --------------------------- | ------------------------------------------------------- |
| `REX_ALLOW_TAILNET_MEMBERS` | `true` lets in every tailnet member the ACLs let through |

### Login server

With `REX_LOGIN_SERVER_ACCOUNTS` set to the path of an [accounts file](https://www.superlogical.com/rex/docs/linux/login-server.md#accounts-file), the container starts `rex login-server` on the tailnet through `TS_SOCKET` instead of one server as `rex`.
`REX_ALLOW_TAILNET_MEMBERS` does not apply; the accounts file decides who gets in.

- It must run as root, with the capabilities `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `KILL`, `SETGID` and `SETUID`, on a writable root filesystem.
- At each start, it creates every account the file names, except `root` and `"*"`, with zsh and a home in `/home/<account>`.
- A home that exists keeps the uid that owns it; a new home takes the next free uid from 1000.
- `/run/rex` is opened to every account for traversal.

A client picks an account with `http://<account>@<host>`; without one, the file's `defaults` decide.

## Chart

`oci://sarak.sh/mikluko/charts/kuberex`, released with the image at the same version.
It runs one Deployment of one replica with the `Recreate` strategy, and a ServiceAccount.
[`chart/values.yaml`](../chart/values.yaml) documents every value; these map onto the image:

| Value                              | Effect                                                                     |
| ---------------------------------- | -------------------------------------------------------------------------- |
| `stateVolume`                      | volume source of `/var/lib/rex`, rendered as given; default an `emptyDir`  |
| `tailscale.hostname`               | `TS_HOSTNAME`                                                              |
| `tailscale.controlURL`             | `TS_CONTROL_URL`                                                           |
| `tailscale.authKey.existingSecret` | Secret holding the auth key, under the key `tailscale.authKey.key`, default `authkey` |
| `tailscale.allowTailnetMembers`    | `REX_ALLOW_TAILNET_MEMBERS`                                                |
| `tailscale.sidecar.enabled`        | runs `tailscaled` from `tailscale/tailscale` as a sidecar in userspace networking, with its state in `/var/lib/rex/tailscaled`, and sets `TS_SOCKET` |
| `tailscale.sidecar.port`           | `REX_TAILNET_PORT`; below 1024 the pod sets `net.ipv4.ip_unprivileged_port_start` to it |
| `loginServer.enabled`              | runs the login server as above; needs `tailscale.sidecar.enabled`          |
| `loginServer.accounts`             | the accounts file, rendered as JSON                                        |
| `networkPolicy.enabled`            | a CiliumNetworkPolicy allowing egress only to kube-dns, the kube-apiserver and outside the cluster |

With `loginServer.enabled`, the chart mounts `home/` of the state volume at `/home`, so homes outlive the pod.
