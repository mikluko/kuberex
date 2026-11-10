# Writes /candidates: for rex and claude-code, the version apt would install
# from the channels apt/sources.list.d names, on amd64 and on arm64, one line
# "<package> <arch> <version>" each, "(none)" where there is none. Built by the
# renovate workflow with apt/ as the context and --no-cache, so the indexes are
# read afresh.

ARG BASE=debian:trixie
FROM $BASE AS resolve
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && \
    apt-get install -qq -y --no-install-recommends ca-certificates >/dev/null && \
    rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*
COPY keyrings/ /etc/apt/keyrings/
COPY sources.list.d/ /etc/apt/sources.list.d/
RUN for arch in amd64 arm64; do \
      [ "$arch" = "$(dpkg --print-architecture)" ] || dpkg --add-architecture "$arch"; \
    done && \
    apt-get update -qq && \
    for pkg in rex claude-code; do \
      for arch in amd64 arm64; do \
        candidate=$(apt-cache policy "$pkg:$arch" | sed -n 's/^ *Candidate: //p'); \
        echo "$pkg $arch ${candidate:-(none)}"; \
      done; \
    done >/candidates

FROM scratch
COPY --from=resolve /candidates /
