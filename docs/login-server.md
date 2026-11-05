# Give Each Person an Account of Their Own

This guide shows you how to run kuberex so that each person who connects works as their own Linux account, with their own home, instead of everyone sharing the account `rex`.

kuberex does this with [`rex login-server`](https://www.superlogical.com/rex/docs/linux/login-server.md).
It runs as root, picks an account for each caller from an accounts file, and starts a Rex server as that account.

Before you begin:

- kuberex running with the `tailscaled` sidecar, as in [Run kuberex on a Headscale tailnet](headscale.md); the login server listens on the tailnet only through the sidecar
- the Tailscale login of each person, as `tailscale:<login>`

1. Write the accounts in a values file, one account per person.
   The keys under `accounts.accounts` are the account names:

   ```yaml
   loginServer:
     enabled: true
     accounts:
       version: 1
       accounts:
         alice:
           principals: ["tailscale:alice@example.com"]
         bob:
           principals: ["tailscale:bob@example.com"]
   ```

   The file follows Rex's [accounts file](https://www.superlogical.com/rex/docs/linux/login-server.md#accounts-file) format, so `defaults` and the `"*"` account work too.

1. Apply it:

   ```sh
   helm upgrade rex oci://sarak.sh/mikluko/charts/kuberex -n rex --reuse-values -f accounts.yaml
   ```

   The pod restarts and creates each account, with zsh and a home in `/home/<account>`.

1. Connect. A caller allowed one account gets it:

   ```sh
   rex -S http://kuberex.<magicdns-domain>:80 whoami
   ```

   To pick one of several accounts, put it in the address: `http://bob@kuberex.<magicdns-domain>:80`.

To add a person later, add an account and upgrade again; existing homes keep their files and their owner.
What the login server needs from the pod is listed in the [reference](reference.md#login-server).
