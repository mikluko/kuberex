# kuberex

kuberex is the wiring that runs the [Superlogical Rex](https://www.superlogical.com/rex/docs/installation.md) server on Kubernetes.
It ships as the container image `sarak.sh/mikluko/kuberex`.
The image holds no Rex binary: at pod start, `kuberex install` installs the `rex` package from the Rex apt repository,
and `kuberex run` starts the server on the tailnet.

## Provide the Rex repository credential

The Rex apt repository at `pkgs.superlogical.com` admits only Rex testers, so the pod needs your repository credential.

1. Log in with Discord on [Installing on Linux](https://www.superlogical.com/rex/docs/linux/installation.md).
   The page then shows the install commands, with your user name and password for `pkgs.superlogical.com`.
1. Write the credential to a file named `auth.conf`, in the netrc format apt reads:

   ```
   machine pkgs.superlogical.com login <user-name> password <password>
   ```

   Replace `<user-name>` and `<password>` with the values from the Rex page.
1. Create a Secret from the file, in the namespace the pod runs in:

   ```shell
   kubectl create secret generic rex-apt --from-file=auth.conf -n <namespace>
   ```

1. Mount the Secret at `/etc/rex-apt` in the container that runs `kuberex install`.
   The default value of `REX_APT_AUTH` is `/etc/rex-apt/auth.conf`; set it to read the file from another path.

apt reads the file as its own `_apt` user, so the file must be world-readable.
The default mode of a Secret volume, 0644, is enough.
The credential stays on the mount: `kuberex install` passes its path to apt and never copies it into the installed system.
