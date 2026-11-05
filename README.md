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

The embedded node serves HTTPS with certificates from the control server, which Headscale does not issue.
With `TS_SOCKET` set, the server instead serves plain HTTP on the tailnet through a `tailscaled` that is already on it, and ignores the three variables above:

| Variable                    | Meaning                                                             |
| --------------------------- | ------------------------------------------------------------------- |
| `TS_SOCKET`                 | LocalAPI socket of that `tailscaled`                                |
| `REX_TAILNET_PORT`          | tailnet port, default `80`                                          |
| `REX_ALLOW_TAILNET_MEMBERS` | `true` admits every member the ACLs let through, not only the owner |

`REX_ALLOW_TAILNET_MEMBERS` applies to the embedded node too. A node joined with a tagged key has no owner, so it admits nobody without it.

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
