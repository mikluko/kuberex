# Run kuberex on a Headscale Tailnet

This guide shows you how to run kuberex on a tailnet whose control server is [Headscale](https://headscale.net/).

Rex's built-in Tailscale node serves HTTPS with a certificate from the control server, which Headscale does not issue.
On Headscale, the pod runs `tailscaled` as a sidecar instead, and the server listens on the tailnet over plain HTTP, which the tailnet encrypts.

Before you begin:

- the namespace and the Secret from step 1 of the [quick start](../README.md#quick-start), with a pre-auth key from your Headscale server in the Secret
- the URL of your Headscale server

1. Install the chart with the sidecar on:

   ```sh
   helm install rex oci://sarak.sh/mikluko/charts/kuberex -n rex \
     --set tailscale.controlURL=https://<headscale-host> \
     --set tailscale.authKey.existingSecret=rex-tailscale \
     --set tailscale.sidecar.enabled=true
   ```

   If the key was made with tags, add `--set tailscale.allowTailnetMembers=true`.
   A tagged node has no owner, and only the owner is let in by default; with it set, your Headscale ACLs decide who reaches the node.

1. Connect over HTTP on port 80:

   ```sh
   rex -S http://kuberex.<magicdns-domain>:80 whoami
   ```

   `<magicdns-domain>` is the `dns.base_domain` of your Headscale server.

To give each person their own account on the server instead, see [Give each person an account of their own](login-server.md).
The sidecar's settings are listed in the [reference](reference.md#chart).
