# Rex server overlay on the base machine image.
#
# Build:  container build --secret id=rex-apt,src=rex-apt.conf -t machine-rex .
#
# rex-apt.conf is the netrc-format apt credential for pkgs.superlogical.com.
# It is mounted only for the build and never written to an image layer, so
# the machine cannot fetch rex updates itself: rebuild and re-create instead.

FROM sarak.sh/mikluko/machine-debian:13.20261027.0

ARG DEBIAN_FRONTEND=noninteractive

RUN --mount=type=cache,target=/var/lib/apt,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    apt-get update -yq && \
    apt-get install -y \
        dumb-init locales curl wget jq less vim \
        procps iproute2 iputils-ping dnsutils unzip xz-utils file

COPY --chmod=0644 rex-archive-keyring.gpg /etc/apt/keyrings/rex-archive-keyring.gpg
COPY rex.sources /etc/apt/sources.list.d/rex.sources
COPY rex.pref /etc/apt/preferences.d/rex
COPY --chmod=0644 claude-code.asc /etc/apt/keyrings/claude-code.asc
COPY claude-code.sources /etc/apt/sources.list.d/claude-code.sources

RUN --mount=type=cache,target=/var/lib/apt,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=secret,id=rex-apt,target=/etc/apt/auth.conf.d/rex.conf,mode=0600 \
    apt-get update -yq && \
    apt-get install -y --allow-downgrades rex claude-code && \
    install -d -o rex -g rex -m 0750 /run/rex

# Mirrors rex.service: the runtime dir holds the socket and is where rex
# commands in the server's terminals look for it. Go aborts on a removed
# GODEBUG value, so none is inherited.
ENV XDG_RUNTIME_DIR=/run/rex \
    REX_RUNTIME_DIR=/run/rex \
    GODEBUG= \
    SHELL=/bin/bash

USER rex
WORKDIR /var/lib/rex

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/usr/bin/rex", "server", "run", \
     "--listen", "/run/rex/server.sock", \
     "--state-dir", "/var/lib/rex", \
     "--socket-mode", "0660", \
     "--log-format", "text"]
