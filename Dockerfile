# syntax=docker/dockerfile:1.2
#########################################################
# BUILD IMAGE
#########################################################

# hadolint ignore=DL3006,DL3049
FROM golang:1.17 AS build
##ARG GOPRIVATE="*gitlab.com/mintel/*"
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=secret,id=netrc,dst=/root/.netrc \
    --mount=type=cache,target=/go/src,sharing=locked \
    --mount=type=cache,target=/go/pkg,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    go mod download
COPY . ./
RUN --mount=type=cache,target=/go/src,sharing=locked \
    --mount=type=cache,target=/go/pkg,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    go build ./cmd/syncer

#########################################################
# TEST IMAGE
# Rebuild the Go binary as source files change.
#########################################################

# hadolint ignore=DL3049
FROM build AS test

##ENV GOPRIVATE="*gitlab.com/mintel/*"
##ENV EBTAILER_PORT=80

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Install golangci-lint.
RUN { curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v1.41.1; } \
    && ln -s /go/bin/golangci-lint /usr/local/bin/golangci-lint

# hadolint ignore=DL3008
RUN --mount=type=cache,target=/var/cache/apt \
    --mount=type=cache,target=/var/lib/apt \
    apt-get update -y && apt-get install -y --no-install-recommends \
        jq \
        rsync

# Make the module cache a perminent part of the image.
# hadolint ignore=DL3006,DL3018,DL3019
RUN --mount=type=cache,id=/go/src,target=/gotmp/src,sharing=locked \
    --mount=type=cache,id=/go/pkg,target=/gotmp/pkg,sharing=locked \
    rsync -ra /gotmp/ /go/

RUN go install github.com/githubnemo/CompileDaemon@v1.3.0
CMD ["CompileDaemon", "-include", "*.go", "-include", "*.html", "-include", "go.*", "--command=./cmd/syncer"]

#########################################################
# RELEASE IMAGE
#########################################################

# hadolint ignore=DL3006,DL3007
FROM gcr.io/distroless/base:latest AS release

USER root

##ENV EBTAILER_PORT=8080
##EXPOSE 8080

COPY --from=build --chown=root:root /app/cmd/syncer /app/cmd/syncer
ENTRYPOINT ["/app/cmd/syncer"]

ARG GIT_DESCRIPTION
ARG BUILD_DATE
ARG VCS_REF
LABEL org.opencontainers.image.authors="Jaye Doepke <jdoepke@mintel.com>, Spencer Huff <shuff@mintel.com>" \
      org.opencontainers.image.title="grafana-local-syncer" \
      org.opencontainers.image.description="Sync dashboard files on disk to a local grafana instance." \
      org.opencontainers.image.url="https://github.com/mintel/grafana-local-sync" \
      org.opencontainers.image.source="git@github.com:mintel/grafana-local-sync.git" \
      org.opencontainers.image.version="$GIT_DESCRIPTION" \
      org.opencontainers.image.vendor="Mintel Group Ltd." \
      org.opencontainers.image.licences="MIT" \
      org.opencontainers.image.created="$BUILD_DATE" \
      org.opencontainers.image.revision="$VCS_REF"

#########################################################
# DEFAULT IMAGE
# Build the release image by default.
#########################################################

FROM release
