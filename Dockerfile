# Rex server image for Kubernetes.
#
# Build:  container build -t kuberex .

# The Go that builds kuberexctl and that the image ships.
# renovate: datasource=docker depName=golang extractVersion=^(?<version>\d+\.\d+\.\d+)-trixie$
ARG GO_VERSION=1.27.4

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-trixie AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -o /out/ ./cmd/kuberexctl

FROM golang:${GO_VERSION}-trixie AS go

FROM debian:trixie-20261104

# Rex and Claude Code versions the image ships. Rex's snapshot index keeps
# only its newest builds, so a stale REX_VERSION stops the image building.
# renovate: datasource=deb depName=rex registryUrl=https://pkgs.superlogical.com/rex/apt?suite=snapshot&components=main&binaryArch=amd64
ARG REX_VERSION=0.0.1794632741-g1555b2
# renovate: datasource=deb depName=claude-code registryUrl=https://downloads.claude.ai/claude-code/apt/stable?suite=stable&components=main&binaryArch=amd64
ARG CLAUDE_CODE_VERSION=2.1.311-1

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
        build-essential git openssh-client nodejs npm \
        curl wget jq less vim procps iproute2 iputils-ping dnsutils unzip xz-utils file \
        htop mc ripgrep fd-find fzf tree bat rsync zip ncdu

COPY apt/keyrings/ /etc/apt/keyrings/
COPY apt/sources.list.d/ /etc/apt/sources.list.d/

RUN --mount=type=cache,target=/var/lib/apt,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    apt-get update -yq && \
    apt-get install -y --no-install-recommends \
        "rex=$REX_VERSION" "claude-code=$CLAUDE_CODE_VERSION"

COPY --from=go /usr/local/go/ /usr/local/go/
RUN ln -s ../go/bin/go ../go/bin/gofmt /usr/local/bin/

COPY rootfs/ /
COPY --from=build /out/kuberexctl /usr/local/bin/
RUN echo '. /etc/zsh/kuberex-claude.zsh' >>/etc/zsh/zshrc

# rex commands look for the login server's sockets in the runtime dir. Go
# aborts on a removed GODEBUG value, so none is inherited.
ENV REX_RUNTIME_DIR=/run/rex \
    GODEBUG= \
    SHELL=/usr/bin/zsh

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/usr/local/bin/kuberex"]
