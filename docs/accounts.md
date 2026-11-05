# Add Accounts for More People

This guide shows you how to give another person an account of their own on a running kuberex.

Each account is a Linux account in the pod, with zsh and a home in `/home/<account>`.
The [accounts file](https://www.superlogical.com/rex/docs/linux/login-server.md#accounts-file) under `accounts` in the chart values says which tailnet logins may use which account.

1. Add the account to `accounts.accounts` in `values.yaml`, with the person's tailnet login as its principal:

   ```yaml
   accounts:
     version: 1
     accounts:
       alice:
         principals: ["tailscale:alice@example.com"]
       bob:
         principals: ["tailscale:bob@example.com"]
   ```

1. Apply it:

   ```sh
   helm upgrade rex oci://sarak.sh/mikluko/charts/kuberex -n rex -f values.yaml
   ```

   The pod restarts and creates the new account. Existing homes keep their files and their owner.

1. The person connects with the same address as you.
   A login allowed one account gets it; to pick one of several, put it in the address: `http://bob@kuberex.<tailnet>.ts.net:80`.

The accounts file also takes `defaults` and the `"*"` account; see [Rex's accounts file reference](https://www.superlogical.com/rex/docs/linux/login-server.md#accounts-file).
