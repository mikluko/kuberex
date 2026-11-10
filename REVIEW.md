# Review

What a reviewer of a kuberex pull request checks beyond CI.
CI (`.forgejo/workflows/test.yaml`) runs `go vet` and `go test`, and lints and renders the chart with every `chart/ci/*.yaml`.
Nothing on a pull request builds the image or runs the entrypoint.

## Changelog and releases

- `main` publishes a release whenever the newest heading in `CHANGELOG.md` names a version no tag has, on push and daily at 06:30 UTC.
  A pull request's entry therefore goes under `## [Unreleased]`.
  A version heading belongs only in a pull request whose stated purpose is a release; the renovate workflow cuts one by itself when it moves a pin.
- Every change to what the image or the chart does carries an entry: `Dockerfile`, `rootfs/`, `cmd/`, `internal/`, `chart/`.
  A change to CI or to docs alone carries none.
- Entries use the Keep a Changelog 2.0.0 types and describe the effect for someone running the image or the chart.
- Renaming or removing a chart value, a variable the entrypoint reads, a `kuberexctl` subcommand, or a path accounts or downstream images use is breaking, and its entry starts with `**Breaking:**`.
- A version heading carries the date in UTC; the release check refuses a date later than today in UTC.

## Chart and image in step

The chart and the image are released together at one version; `chart/Chart.yaml` stays at `0.0.0` and the release sets both.
A chart value that feeds the entrypoint lands in the same pull request as the entrypoint code that reads it, together with:

- its default and comment in `chart/values.yaml`
- its validation in `chart/templates/configmap.yaml` when it names accounts
- its variable or mount in `chart/templates/statefulset.yaml`
- an example in `chart/ci/full-values.yaml`
- its row in `docs/reference.md`

## Reference

`docs/reference.md` documents every variable, chart value, `kuberexctl` subcommand and file the image provides.
A pull request that changes one updates it in the same pull request, and every name, path and behaviour the reference states exists in the tree.

## What CI leaves out

- A change to `Dockerfile`, `rootfs/` or the in-pod behaviour of `kuberexctl` shows in its description that the image was built and the changed path was run in it.
- The entrypoint, `rootfs/usr/local/bin/kuberex`, is POSIX sh under `set -eu`, run by dash.
- The entrypoint stays the parent of the login server, runs `kuberexctl stop-servers` once the login server has exited, and exits 0 after a `kuberexctl shutdown`.

## Privilege

The server container runs as root with only the capabilities `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `KILL`, `SETGID` and `SETUID`, and with `allowPrivilegeEscalation: false`.
Anything an account triggers as root goes through `kuberexctl serve`, which admits a request by the caller's uid.
An account Secret stays readable by its account alone.
A pull request that widens any of this says why in its description.

## Pins

The renovate workflow moves `FROM`, `REX_VERSION` and `CLAUDE_CODE_VERSION` in the `Dockerfile` daily.
A pull request that moves one by hand says why.

## Downstream

`sarak.sh/mikluko/rexcity` builds its image `FROM` the newest kuberex release within the hour, with no review in between.
`sarak.sh/saraksh/kube` pins the chart for rexcity and sets its values in `k8s/apps/rexcity`.
A breaking change names, in its description, what each of them has to change.
