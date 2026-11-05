# Install Extra Packages

This guide shows you how to have kuberex install Debian packages that the image does not hold, such as a tool from your own apt repository.

The pod installs them with `apt-get` at every start, before anyone can connect, so a start takes as long as the download, and the pod does not start while a package cannot be fetched.

1. To install from Debian, list the packages in `values.yaml`:

   ```yaml
   apt:
     packages:
       - postgresql-client
       - tmux
   ```

   To install from another repository, add its source and its signing key as well.
   The key's file name is what `Signed-By` points to in `/etc/apt/keyrings`:

   ```yaml
   apt:
     sources:
       example: |
         Types: deb
         URIs: https://apt.example.com/debian
         Suites: trixie
         Components: main
         Signed-By: /etc/apt/keyrings/example.asc
     keyrings:
       example.asc: |
         -----BEGIN PGP PUBLIC KEY BLOCK-----
         ...
         -----END PGP PUBLIC KEY BLOCK-----
     packages:
       - example-tool
   ```

1. Apply it:

   ```sh
   helm upgrade rex oci://sarak.sh/mikluko/charts/kuberex -n rex -f values.yaml
   ```

   The pod restarts and installs the packages. Its log shows the `apt-get` output.

A package entry is passed to `apt-get install` as it stands, so `example-tool=1.2.0` pins a version.
