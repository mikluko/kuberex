# Rex server image for Kubernetes.
#
# Build:  container build -t kuberex .
#
# Rex is not in the image: `kuberex install` puts it, and any packages the
# deployment names, into overlays at pod start, so the rex apt credential is a
# runtime secret. See rootfs/usr/local/sbin/kuberex.

FROM debian:trixie-20261027

ARG DEBIAN_FRONTEND=noninteractive

# postinst of the rex package keeps an existing account; a fixed uid keeps the
# ownership of the host directory under /var/lib/rex stable across releases.
RUN groupadd --system --gid 999 rex && \
    useradd --system --uid 999 --gid rex --home-dir /var/lib/rex --shell /usr/sbin/nologin rex && \
    install -d -o rex -g rex -m 0750 /var/lib/rex

RUN --mount=type=cache,target=/var/lib/apt,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    apt-get update -yq && \
    apt-get install -y --no-install-recommends \
        ca-certificates dumb-init util-linux libpam0g locales \
        build-essential git openssh-client golang nodejs npm \
        curl wget jq less vim procps iproute2 iputils-ping dnsutils unzip xz-utils file

# The rex source for `kuberex install`, added after the build's own apt run:
# it needs the credential the build does not have.
COPY apt/keyrings/ /etc/apt/keyrings/
COPY apt/sources.list.d/ /etc/apt/sources.list.d/
COPY apt/preferences.d/ /etc/apt/preferences.d/
COPY rootfs/ /

# The runtime dir holds the socket and is where rex commands in the server's
# terminals look for it. Go aborts on a removed GODEBUG value, so none is
# inherited.
ENV XDG_RUNTIME_DIR=/run/rex \
    REX_RUNTIME_DIR=/run/rex \
    GODEBUG= \
    SHELL=/bin/bash

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/usr/local/sbin/kuberex"]
CMD ["run"]
