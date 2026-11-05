# kuberex

kuberex is the wiring that runs the [Superlogical Rex](https://www.superlogical.com/rex/docs/installation.md) server on Kubernetes.
It ships as the container image `sarak.sh/mikluko/kuberex`, for `linux/amd64` and `linux/arm64`.
Development happens at [sarak.sh/mikluko/kuberex](https://sarak.sh/mikluko/kuberex); [github.com/mikluko/kuberex](https://github.com/mikluko/kuberex) is a read-only mirror that closes issues and pull requests opened there.

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

The embedded node serves HTTPS with certificates from the control server, which Headscale does not issue.
With `TS_SOCKET` set, the server instead serves plain HTTP on the tailnet through a `tailscaled` that is already on it, and ignores the three variables above:

| Variable                    | Meaning                                                             |
| --------------------------- | ------------------------------------------------------------------- |
| `TS_SOCKET`                 | LocalAPI socket of that `tailscaled`                                |
| `REX_TAILNET_PORT`          | tailnet port, default `80`                                          |
| `REX_ALLOW_TAILNET_MEMBERS` | `true` admits every member the ACLs let through, not only the owner |

`REX_ALLOW_TAILNET_MEMBERS` applies to the embedded node too. A node joined with a tagged key has no owner, so it admits nobody without it.

### Login server

With `REX_LOGIN_SERVER_ACCOUNTS` set to an accounts file, the container runs as root and starts [`rex login-server`](https://www.superlogical.com/rex/docs/linux/login-server.md) on the tailnet through `TS_SOCKET` instead.
Each caller gets the server of the account the file maps it to, running as that account; `REX_ALLOW_TAILNET_MEMBERS` does not apply.

Every account the file names, but `root` and `"*"`, is created at each start, since `/etc` is not kept, so the root filesystem has to be writable.
It gets `zsh` and a home in `/home/<name>`; a home that exists keeps the uid that owns it, and a new one takes the next free uid from 1000.
The chart mounts `home/` of the state volume at `/home`, so homes outlive the pod.
The container needs the capabilities `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `KILL`, `SETGID` and `SETUID`, and opens `/run/rex` to every account for traversal.
A client picks an account with `http://<account>@<host>`; without one, the file's defaults decide.

## Deploy it with Helm

The chart `oci://sarak.sh/mikluko/charts/kuberex` is released with the image, at the same version, and runs one server pod as above.
`values.yaml` in [`chart/`](chart/) documents every value; these are the ones a deployment sets:

| Value                              | Meaning                                                      |
| ---------------------------------- | ------------------------------------------------------------ |
| `stateVolume`                      | volume source of `/var/lib/rex`, default an `emptyDir`       |
| `tailscale.hostname`               | `TS_HOSTNAME`                                                |
| `tailscale.controlURL`             | `TS_CONTROL_URL`                                             |
| `tailscale.authKey.existingSecret` | Secret with the join key                                     |
| `tailscale.allowTailnetMembers`    | `REX_ALLOW_TAILNET_MEMBERS`                                  |
| `tailscale.sidecar.enabled`        | joins through a `tailscaled` sidecar instead, see below      |
| `loginServer.enabled`              | runs the login server, see below; needs the sidecar          |
| `loginServer.accounts`             | the accounts file                                            |
| `networkPolicy.enabled`            | CiliumNetworkPolicy: egress to DNS, kube-apiserver and world |
| `nodeSelector`, `tolerations`      | placement of the pod                                         |

```sh
helm install rex oci://sarak.sh/mikluko/charts/kuberex --version 0.4.0 \
  --set tailscale.authKey.existingSecret=rex-tailscale \
  --set stateVolume.hostPath.path=/srv/rex
```

With `tailscale.sidecar.enabled`, the pod runs `tailscaled` from `tailscale/tailscale` as a sidecar in userspace networking, keeping its node state in `/var/lib/rex/tailscaled`.
The sidecar joins the tailnet with the join key and the control server, and the server listens on the tailnet at `http://<hostname>:<tailscale.sidecar.port>`:

```sh
helm install rex oci://sarak.sh/mikluko/charts/kuberex \
  --set tailscale.controlURL=https://headscale.example.com \
  --set tailscale.authKey.existingSecret=rex-tailscale \
  --set tailscale.allowTailnetMembers=true \
  --set tailscale.sidecar.enabled=true
```

With `loginServer.enabled`, the same pod runs the login server; the accounts file decides who gets in, and as which account:

```yaml
tailscale:
  sidecar:
    enabled: true
loginServer:
  enabled: true
  accounts:
    version: 1
    accounts:
      alice:
        principals: ["tailscale:alice@example.com"]
```
