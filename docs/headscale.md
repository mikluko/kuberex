# Run kuberex on a Headscale Tailnet

This guide shows you how to run kuberex on a tailnet whose control server is [Headscale](https://headscale.net/).

Before you begin:

- the namespace and the Secret from step 1 of the [quick start](../README.md#quick-start), with a pre-auth key from your Headscale server in the Secret
- the `values.yaml` from step 2 of the quick start, with your Headscale login as the principal
- the URL of your Headscale server

1. Add the control server to `values.yaml`:

   ```yaml
   tailscale:
     controlURL: https://<headscale-host>
   ```

1. Install the chart:

   ```sh
   helm install rex oci://sarak.sh/mikluko/charts/kuberex -n rex -f values.yaml
   ```

1. Connect:

   ```sh
   rex -S http://kuberex.<magicdns-domain>:80 whoami
   ```

   `<magicdns-domain>` is the `dns.base_domain` of your Headscale server.

Headscale ACLs decide who reaches the node; the accounts file decides who gets in, and as which account.
