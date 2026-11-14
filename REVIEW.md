# Review

What a reviewer of a kuberex pull request checks beyond CI.
CI (`.forgejo/workflows/test.yaml`) runs `go vet` and `go test`, lints and renders the chart with every `chart/ci/*.yaml`, and builds the image for amd64 and arm64.
Nothing on a pull request runs the image or its entrypoint.

## Changelog and releases

- `main` publishes a release whenever the newest heading in `CHANGELOG.md` names a version no tag has, on push and daily at 06:30 UTC.
  A pull request's entry therefore goes under `## [Unreleased]`.
  A version heading belongs only in a pull request whose stated purpose is a release; Renovate's pull request, which moves the pins, is one.
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

Renovate moves the Debian `FROM`, `GO_VERSION`, `REX_VERSION` and `CLAUDE_CODE_VERSION` in the `Dockerfile`, and with them the `go` directive of `go.mod` within its minor, in a pull request it merges itself once every check of the test workflow has passed (`renovate.json`).
That pull request is a release: Renovate writes its changelog entry and version heading with the commands `renovate.json` gives as `postUpgradeTasks`, which run the changelog tool `go.mod` names as a tool, and the merge starts the release workflow.
The heading is dated the day Renovate cut it, and the release workflow publishes nothing dated another day than its merge, so one left open past midnight UTC needs its heading's date moved to the day of the merge before it is merged by hand.
A pull request that moves one by hand says why.

## Downstream

`sarak.sh/mikluko/rexcity` builds its image `FROM` the newest kuberex release within the hour, with no review in between.
`sarak.sh/saraksh/kube` pins the chart for rexcity and sets its values in `k8s/apps/rexcity`.
A breaking change names, in its description, what each of them has to change.
