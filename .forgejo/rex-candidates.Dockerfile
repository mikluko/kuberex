# Writes /candidates: the rex version apt would install from the channel
# apt/sources.list.d/rex.sources names, for amd64 then arm64, one per line,
# "(none)" where there is none. Built by the upgrade workflow with apt/ as the
# context and --no-cache, so the index is read afresh.

ARG BASE=debian:trixie
FROM $BASE AS resolve
ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update -qq && \
    apt-get install -qq -y --no-install-recommends ca-certificates >/dev/null
COPY keyrings/ /etc/apt/keyrings/
COPY sources.list.d/rex.sources /etc/apt/sources.list.d/
RUN find /etc/apt/sources.list.d -type f ! -name rex.sources -delete && \
    for arch in amd64 arm64; do \
      [ "$arch" = "$(dpkg --print-architecture)" ] || dpkg --add-architecture "$arch"; \
    done && \
    apt-get update -qq && \
    for arch in amd64 arm64; do \
      apt-cache policy "rex:$arch" | sed -n 's/^ *Candidate: //p'; \
    done >/candidates

FROM scratch
COPY --from=resolve /candidates /
