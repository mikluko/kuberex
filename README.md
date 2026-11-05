# kuberex

kuberex runs a [Rex](https://www.superlogical.com/rex/docs/installation.md) server in a Kubernetes cluster,
so your Rex sessions live on an always-on machine you reach from anywhere on your tailnet.
It is one pod, from the container image `sarak.sh/mikluko/kuberex` and the Helm chart `oci://sarak.sh/mikluko/charts/kuberex`,
released together at the same version for `linux/amd64` and `linux/arm64`.

Each person who connects works as a Linux account of their own, with their own home, through Rex's [login server](https://www.superlogical.com/rex/docs/linux/login-server.md).
The image holds Rex, Claude Code, zsh and a set of terminal tools, and a deployment can add Debian packages of its own.
The pod joins your tailnet itself, so it needs no Ingress, no load balancer and no open port.

## Quick start

This runs kuberex on a Tailscale tailnet, with an account for you and its home on a 10 GiB volume of the cluster's default storage class.

Before you begin:

- a cluster you can install Helm charts into, with a default storage class
- a one-time Tailscale auth key for the node
- your Tailscale login, such as `alice@example.com`
- the Rex app or the `rex` command on the machine you connect from

1. Create the namespace and put the auth key in a Secret:

   ```sh
   kubectl create namespace rex
   kubectl create secret generic rex-tailscale -n rex --from-literal=authkey=<auth-key>
   ```

1. Write `values.yaml` with an account for you:

   ```yaml
   tailscale:
     authKey:
       existingSecret: rex-tailscale
   accounts:
     version: 1
     accounts:
       alice:
         principals: ["tailscale:alice@example.com"]
   ```

   `alice` is the name of your account on the server; `alice@example.com` is your Tailscale login.

1. Install the chart:

   ```sh
   helm install rex oci://sarak.sh/mikluko/charts/kuberex -n rex -f values.yaml
   ```

1. Connect once the pod is running. The node joins as `kuberex`, and serves HTTP on port 80, which the tailnet encrypts:

   ```sh
   rex -S http://kuberex.<tailnet>.ts.net:80 whoami
   ```

   `<tailnet>` is your tailnet's DNS name, shown in the DNS page of the Tailscale admin console.
   In the Rex app, add the same address as a remote host.

## More

- [Run kuberex on a Headscale tailnet](docs/headscale.md)
- [Add accounts for more people](docs/accounts.md)
- [Install extra packages](docs/packages.md)
- [Image and chart reference](docs/reference.md)
- Every chart value is documented in [`chart/values.yaml`](chart/values.yaml).

## Development

kuberex is developed at [sarak.sh/mikluko/kuberex](https://sarak.sh/mikluko/kuberex), where issues and pull requests go.
[github.com/mikluko/kuberex](https://github.com/mikluko/kuberex) is a read-only mirror, and closes issues and pull requests opened there.
Releases are listed in [`CHANGELOG.md`](CHANGELOG.md).

kuberex is released under the [MIT License](LICENSE).
