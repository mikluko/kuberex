# Rex server image for Kubernetes.
#
# Build:  container build -t kuberex .

FROM --platform=$BUILDPLATFORM golang:1.27.2-trixie AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -o /out/kuberex-accounts ./cmd/kuberex-accounts

FROM debian:trixie-20261027

# Rex and Claude Code versions the image ships. Rex's snapshot index keeps
# only its newest builds, so a stale REX_VERSION stops the image building.
ARG REX_VERSION=0.0.1793645657-gd018d2
ARG CLAUDE_CODE_VERSION=2.1.309-1

ARG DEBIAN_FRONTEND=noninteractive

# postinst of the rex package keeps an existing account; a fixed uid keeps the
# ownership of the host directory under /var/lib/rex stable across releases.
RUN groupadd --system --gid 999 rex && \
    useradd --system --uid 999 --gid rex --home-dir /var/lib/rex --shell /usr/bin/zsh rex && \
    install -d -o rex -g rex -m 0750 /var/lib/rex /run/rex

RUN --mount=type=cache,target=/var/lib/apt,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    apt-get update -yq && \
    apt-get install -y --no-install-recommends \
        ca-certificates dumb-init util-linux libpam0g libpam-tmpdir locales zsh \
        build-essential git openssh-client golang nodejs npm \
        curl wget jq less vim procps iproute2 iputils-ping dnsutils unzip xz-utils file \
        htop mc ripgrep fd-find fzf tree bat rsync zip ncdu

COPY apt/keyrings/ /etc/apt/keyrings/
COPY apt/sources.list.d/ /etc/apt/sources.list.d/

RUN --mount=type=cache,target=/var/lib/apt,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    apt-get update -yq && \
    apt-get install -y --no-install-recommends \
        "rex=$REX_VERSION" "claude-code=$CLAUDE_CODE_VERSION"

COPY rootfs/ /
COPY --from=build /out/kuberex-accounts /usr/local/bin/kuberex-accounts

# rex commands look for the login server's sockets in the runtime dir. Go
# aborts on a removed GODEBUG value, so none is inherited.
ENV REX_RUNTIME_DIR=/run/rex \
    GODEBUG= \
    SHELL=/usr/bin/zsh

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/usr/local/bin/kuberex"]
