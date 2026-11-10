# Image and Chart Reference

## Image

`sarak.sh/mikluko/kuberex:<version>`, for `linux/amd64` and `linux/arm64`, built on `debian:trixie`.
Each release is also tagged `<major>.<minor>` and `latest`.

### Contents

The image installs these packages:

| Package        | Source                                                            | Version                                   |
| -------------- | ----------------------------------------------------------------- | ----------------------------------------- |
| `rex`          | `snapshot` channel of `https://pkgs.superlogical.com/rex/apt`     | pinned as `REX_VERSION` in `Dockerfile`   |
| `claude-code`  | `stable` channel of `https://downloads.claude.ai/claude-code/apt` | pinned as `CLAUDE_CODE_VERSION`           |
| terminal tools | Debian: `zsh`, `git`, `vim`, `htop`, `mc`, `ripgrep`, `fd-find`, `fzf`, `bat`, `jq` and others | the Debian snapshot the image is built on |

A new Rex or Claude Code version reaches the pod as a new image release.
Every day, the renovate workflow moves the Rex and Claude Code pins and the Debian snapshot to the newest of each, and when any moves, releases a new version carrying the move and anything else under `[Unreleased]` in [`CHANGELOG.md`](../CHANGELOG.md).
The full package list is in the [`Dockerfile`](../Dockerfile).

Claude Code neither checks for updates nor updates itself, and sends no telemetry, error reports or other nonessential traffic: `/etc/claude-code/managed-settings.d/kuberex.json` sets `DISABLE_UPDATES`, `DISABLE_AUTOUPDATER`, `DISABLE_TELEMETRY`, `DISABLE_ERROR_REPORTING` and `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` for every account.
Feature-flag fetching is off with them, so [Remote Control](https://code.claude.com/docs/en/remote-control.md#requirements) is unavailable.
An account receiving [server-managed settings](https://code.claude.com/docs/en/server-managed-settings.md) skips this file; see [Claude Code Sessions](#claude-code-sessions).

### Entrypoint

The entrypoint starts [`rex login-server`](https://www.superlogical.com/rex/docs/linux/login-server.md) on the tailnet through a `tailscaled` already on it.
It must run as root, with the capabilities `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `KILL`, `SETGID` and `SETUID`, on a writable root filesystem.
It reads:

| Variable                    | Meaning                                                                          |
| --------------------------- | -------------------------------------------------------------------------------- |
| `REX_LOGIN_SERVER_ACCOUNTS` | path of the login server's [accounts file](https://www.superlogical.com/rex/docs/linux/login-server.md#accounts-file); required |
| `TS_SOCKET`                 | LocalAPI socket of the `tailscaled`; required                                     |
| `REX_TAILNET_PORT`          | tailnet port the login server serves plain HTTP on; default `80`                  |
| `KUBEREX_APT`               | directory of apt additions; none when unset                                       |
| `KUBEREX_SECRETS`           | directory of [account Secrets](#account-secrets); none when unset                 |
| `KUBEREX_SERVICES`          | [services file](#services); none when unset                                       |
| `KUBEREX_SHUTDOWN_ACCOUNTS` | accounts, separated by spaces, that may [shut the container down](#shutdown); none when unset |
| `KUBEREX_KEEP_SERVERS`      | accounts, separated by spaces, whose Rex servers are [kept running](#kept-servers); none when unset |

At each start, in this order, it:

1. copies `$KUBEREX_APT/sources/*.sources` to `/etc/apt/sources.list.d` and `$KUBEREX_APT/keyrings/*` to `/etc/apt/keyrings`, and, when `$KUBEREX_APT/packages` is not empty, runs `apt-get update` and `apt-get install` with its words as arguments
1. creates every account the accounts file names, except `root` and `"*"`, with zsh and a home in `/home/<account>`; a home that exists keeps the uid that owns it, and a new home takes the next free uid from 1000
1. makes `/run/kuberex` owned by root with mode `0755`, and removes `/run/kuberex/ready` and `/run/kuberex/shutdown` left from its last start
1. copies the [account Secrets](#account-secrets) in `$KUBEREX_SECRETS`
1. opens `/run/rex` to every account for traversal
1. makes `/tmp` owned by root with mode `1777`, and `/tmp/user/<uid>` with mode `0700` for every account it created
1. creates `/run/kuberex/ready`, which [account containers](#account-containers) wait for
1. starts the Rex servers of the accounts in `$KUBEREX_KEEP_SERVERS`, waits for them to accept connections, and [keeps them running](#kept-servers)
1. starts the services in `$KUBEREX_SERVICES`, beside the next step
1. starts the login server with its runtime root in `/run/rex`, and passes it the signals the container receives
1. starts `kuberexctl serve` for the accounts in `$KUBEREX_SHUTDOWN_ACCOUNTS`
1. once the login server has exited, stops keeping servers running, then stops every account's Rex server with `kuberexctl stop-servers`, so each records a clean shutdown, and syncs the filesystems
1. exits with code 0 after a [shutdown](#shutdown), and otherwise with the login server's exit code

Each account's server runs in a PAM session, where `pam_tmpdir` sets `TMPDIR` and `TMP` to `/tmp/user/<uid>`, a directory only the account can open.

A client picks an account with `http://<account>@<host>:<port>`; without one, the file's `defaults` decide.

### Account Secrets

`$KUBEREX_SECRETS` holds a directory per account, and in it a directory per Secret, as `<account>/<secret>/<key>`.
At each start every key is copied to `/run/kuberex/secrets/<account>/<secret>/<key>`, owned by the account with mode `0400`.
`/run/kuberex/secrets/<account>` opens only to the account, and `/run/kuberex/secrets` lists nothing to accounts.
The copies are made at start, so a changed Secret reaches the account at the next start.
The chart mounts `/run/kuberex` from an in-memory volume shared with the [account containers](#account-containers), where an account reads its Secrets at the same paths.

### Services

A service is a command that starts with the pod, in a Rex session of an account's own server, where anyone connecting as that account sees it.
The services file is a JSON array of services, each with these fields:

| Field     | Meaning                                                                        |
| --------- | ------------------------------------------------------------------------------ |
| `account` | the account to run as; required                                                |
| `label`   | the label of the Rex session; required                                         |
| `command` | the command and its arguments, run through the account's login shell environment; required |
| `cwd`     | the directory to start in; the account's home when empty                       |

`kuberexctl services` starts each account's server the way the login server does, which then serves connections to that account from the same server.
A session that already carries the label, which the server restored from before the restart with a bare shell in it, is replaced by a new one running the command.
The new session starts once the old one is gone, which may take up to two minutes for a session with many processes; a service whose old session outlives that is not started.
A command that exits is not restarted, and its terminal stays open with its last output.

### Kept Servers

The login server starts an account's Rex server when a client connects as the account, and that server stops once no sessions or control connections remain.
An account in `$KUBEREX_KEEP_SERVERS` has its server started before the services and the login server, by `kuberexctl keep-servers`, which does not stop it when empty and starts it again within about two seconds whenever it is gone.
The login server serves that account's clients from the same server.

### Account Containers

An account container is a container of the pod, beside the server container, that runs one command as an account.
It runs the chart's image, or an image built on it, as root with the capabilities `CHOWN`, `SETGID` and `SETUID`, and shares the server container's `/home`, `/run/rex`, `/run/kuberex` and `/tmp`.
Its command is:

```sh
kuberexctl run --account <account> [--cwd <dir>] [--wait-server] -- <command> [<arg>...]
```

`kuberexctl run`:

1. waits for `/run/kuberex/ready`, so for the server container to have set the accounts up
1. creates the account in the container's own `/etc/passwd`, with the uid and gid that own `/home/<account>`, unless it is there with that uid
1. takes the account's uid and gid, with no other group and no capability
1. with `--wait-server`, waits for the account's Rex server to accept connections
1. runs the command directly, not through a shell, in `--cwd` or the account's home

The command gets the container's environment, with `HOME`, `USER`, `LOGNAME`, `SHELL` set for the account, `TMPDIR` and `TMP` set to `/tmp/user/<uid>`, `REX_RUNTIME_DIR` set to the account's runtime directory in `/run/rex`, so `rex` reaches the account's server, and `REX_AUTOSTART` set to `false`, so `rex` never starts a server in the account container.
The command's account Secrets are in `/run/kuberex/secrets/<account>`.

When the server container ends, for a [shutdown](#shutdown) or otherwise, the account containers keep running, and every account's Rex server is gone until the server container has started again.

### Shutdown

An account in `$KUBEREX_SHUTDOWN_ACCOUNTS` shuts the server container down with:

```sh
kuberexctl shutdown
```

It exits 0 once the shutdown is accepted and prints `kuberexctl: shutting down the container`.
It exits 1 when the account is not allowed, the container is already shutting down, or shutdown is not enabled.
About a second after accepting, `kuberexctl serve`, running as root, records the shutdown in `/run/kuberex/shutdown` and sends `SIGTERM` to the login server, which stops admitting clients and exits.
The [entrypoint](#entrypoint) then stops every account's Rex server with `rex server stop`, so the server records a clean shutdown and restores its sessions at the next start; each terminal's processes, the caller included, get a hangup.
It syncs the filesystems and ends the container with code 0, whatever the login server's exit code.

`kuberexctl` guarantees only that the container ends.
Whether it starts again is the pod's restart policy: the chart's StatefulSet has `Always`, so Kubernetes starts the container again, which runs the [entrypoint](#entrypoint) from the beginning; the services start again, and the volumes, the pod and its `tailscaled` stay as they were.
Kubernetes delays a container that starts again within ten minutes of its last start, from 10 seconds up to 5 minutes, whatever its exit code.

`kuberexctl serve` listens on `/run/kuberex/control.sock` and admits a request by the uid of the process making it.

### Claude Code Sessions

Rex restores an account's sessions, windows and panes when its server starts again, on the first connection after the pod restarts, but each pane comes back as a new shell.
A Claude Code session that was running in a pane is resumed there: at the shell's first prompt, after its startup files, the shell runs `claude --resume <session>` in the directory the session started in.

- Only an interactive session is resumed: not `claude -p`, and not a session started from inside another Claude Code session.
- A session quit with `/exit` or by logging out is not resumed.
- The flags `claude` was started with are not passed again; set defaults such as the model or the permission mode in `~/.claude/settings.json`.

Claude Code hooks in `/etc/claude-code/managed-settings.d/kuberex.json` run `kuberexctl claude`, which keeps the session of each pane in `~/.local/state/kuberex/claude`.
When an account receives [server-managed settings](https://code.claude.com/docs/en/server-managed-settings.md) from its organization, Claude Code skips managed settings files, these hooks with them.
The same hooks in `~/.claude/settings.json` bring sessions back:

```json
{
  "hooks": {
    "SessionStart": [
      { "hooks": [{ "type": "command", "command": "/usr/local/bin/kuberexctl claude record", "async": true }] }
    ],
    "SessionEnd": [
      { "hooks": [{ "type": "command", "command": "/usr/local/bin/kuberexctl claude forget" }] }
    ]
  }
}
```

## Chart

`oci://sarak.sh/mikluko/charts/kuberex`, released with the image at the same version.
It runs one StatefulSet of one replica, a ConfigMap and a ServiceAccount.
The pod runs `tailscaled` from `tailscale/tailscale` as a sidecar in userspace networking, with its state in `/var/lib/rex/tailscaled`.
[`chart/values.yaml`](../chart/values.yaml) documents every value; these are the ones a deployment sets:

| Value                              | Effect                                                                     |
| ---------------------------------- | -------------------------------------------------------------------------- |
| `accounts`                         | the accounts file, rendered as JSON; it must name an account               |
| `tailscale.hostname`               | the node's tailnet hostname; default `kuberex`                             |
| `tailscale.controlURL`             | the control server; default Tailscale's                                    |
| `tailscale.authKey.existingSecret` | Secret holding the auth key, under the key `tailscale.authKey.key`, default `authkey` |
| `tailscale.port`                   | `REX_TAILNET_PORT`; below 1024 the pod sets `net.ipv4.ip_unprivileged_port_start` to it |
| `apt.sources`                      | apt sources, file name without `.sources` to deb822 content                |
| `apt.keyrings`                     | keyrings for them, file name in `/etc/apt/keyrings` to content            |
| `apt.packages`                     | `apt-get install` arguments                                                |
| `accountSecrets`                   | account to a list of Secret names in the release's namespace, each mounted at `/etc/kuberex/secrets/<account>/<secret>` as an [account Secret](#account-secrets); each account must be named in `accounts` |
| `services`                         | the [services](#services), as a list; each `account` must be named in `accounts` |
| `shutdownAccounts`                 | accounts that may [shut the container down](#shutdown); each must be named in `accounts` |
| `keepServers`                      | accounts whose Rex servers are [kept running](#kept-servers), as `KUBEREX_KEEP_SERVERS`; each must be named in `accounts` |
| `accountContainers`                | the [account containers](#account-containers), as a list of `name`, `account` (named in `accounts`) and `command`, and optionally `cwd`, `image` (`repository`, `tag` and `pullPolicy`, each defaulting to `image`'s), `env` (over `env`), `resources` and `volumeMounts` (of `extraVolumes`); one whose account is in `keepServers` passes `--wait-server` |
| `shareProcessNamespace`            | one process namespace for the pod's containers, so an account container sees the processes of its account's Rex sessions |
| `persistence.size`                 | size of the PersistentVolumeClaim `state-<release>-0` the StatefulSet creates for `/var/lib/rex`; default `10Gi` |
| `persistence.storageClassName`     | storage class of that claim; default the cluster's                         |
| `stateVolume`                      | volume source of `/var/lib/rex` in place of that claim, rendered as given, such as `hostPath` |
| `networkPolicy.enabled`            | a CiliumNetworkPolicy allowing egress only to kube-dns, the kube-apiserver and outside the cluster |
| `extraVolumes`                     | volumes added to the pod, rendered as given                                |
| `extraVolumeMounts`                | mounts added to the server's container, rendered as given                  |

The chart mounts `home/` of the state volume at `/home`, so homes outlive the pod.
A change to `accounts` or `apt` restarts the pod.
The claim outlives the release: `helm uninstall` leaves it, and a new install of the same release name takes it back.
A StatefulSet cannot change its claim template, so changing `persistence` on an installed release needs the StatefulSet deleted with `kubectl delete statefulset <release> --cascade=orphan` first.
