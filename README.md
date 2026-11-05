# kuberex

kuberex runs a [Rex](https://www.superlogical.com/rex/docs/installation.md) server in a Kubernetes cluster,
so your Rex sessions live on an always-on machine you reach from anywhere on your tailnet.
It is one pod, from the container image `sarak.sh/mikluko/kuberex` and the Helm chart `oci://sarak.sh/mikluko/charts/kuberex`,
released together at the same version for `linux/amd64` and `linux/arm64`.

The image holds Rex, Claude Code, zsh and a set of terminal tools.
The pod joins your tailnet itself, so it needs no Ingress, no load balancer and no open port.

## Quick start

This runs kuberex on a Tailscale tailnet, with Rex's built-in Tailscale node and its state on a PersistentVolumeClaim.

Before you begin:

- a cluster you can install Helm charts into
- a one-time Tailscale auth key, made without tags so the node belongs to you;
  only the node's owner can connect to it
- the Rex app or the `rex` command on the machine you connect from

1. Create the namespace and put the auth key in a Secret:

   ```sh
   kubectl create namespace rex
   kubectl create secret generic rex-tailscale -n rex --from-literal=authkey=<auth-key>
   ```

1. Create a volume for the server's state and your home:

   ```sh
   kubectl apply -n rex -f - <<'YAML'
   apiVersion: v1
   kind: PersistentVolumeClaim
   metadata:
     name: rex-state
   spec:
     accessModes: [ReadWriteOnce]
     resources:
       requests:
         storage: 10Gi
   YAML
   ```

1. Install the chart:

   ```sh
   helm install rex oci://sarak.sh/mikluko/charts/kuberex -n rex \
     --set tailscale.authKey.existingSecret=rex-tailscale \
     --set stateVolume.persistentVolumeClaim.claimName=rex-state
   ```

1. Connect once the pod is running. The node joins as `kuberex`:

   ```sh
   rex -S https://kuberex.<tailnet>.ts.net whoami
   ```

   `<tailnet>` is your tailnet's DNS name, shown in the DNS page of the Tailscale admin console.
   In the Rex app, add the same address as a remote host.

## More

- [Run kuberex on a Headscale tailnet](docs/headscale.md)
- [Give each person an account of their own](docs/login-server.md)
- [Image and chart reference](docs/reference.md)
- Every chart value is documented in [`chart/values.yaml`](chart/values.yaml).

## Development

kuberex is developed at [sarak.sh/mikluko/kuberex](https://sarak.sh/mikluko/kuberex), where issues and pull requests go.
[github.com/mikluko/kuberex](https://github.com/mikluko/kuberex) is a read-only mirror, and closes issues and pull requests opened there.
Releases are listed in [`CHANGELOG.md`](CHANGELOG.md).

kuberex is released under the [MIT License](LICENSE).
