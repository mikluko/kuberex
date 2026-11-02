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

## Deploy it with Helm

The chart `oci://sarak.sh/mikluko/charts/kuberex` is released with the image, at the same version, and runs one server pod as above.
`values.yaml` in [`chart/`](chart/) documents every value; these are the ones a deployment sets:

| Value                              | Meaning                                                      |
| ---------------------------------- | ------------------------------------------------------------ |
| `stateVolume`                      | volume source of `/var/lib/rex`, default an `emptyDir`       |
| `tailscale.hostname`               | `TS_HOSTNAME`                                                |
| `tailscale.controlURL`             | `TS_CONTROL_URL`                                             |
| `tailscale.authKey.existingSecret` | Secret with the join key, mounted as `TS_AUTHKEY_FILE`       |
| `networkPolicy.enabled`            | CiliumNetworkPolicy: egress to DNS, kube-apiserver and world |
| `nodeSelector`, `tolerations`      | placement of the pod                                         |

```sh
helm install rex oci://sarak.sh/mikluko/charts/kuberex --version 0.4.0 \
  --set tailscale.authKey.existingSecret=rex-tailscale \
  --set stateVolume.hostPath.path=/srv/rex
```
