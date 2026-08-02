# syntax=docker/dockerfile:1.7

ARG BUN_IMAGE=oven/bun:1-alpine
ARG GO_IMAGE=golang:1.25.1-alpine
ARG RUNTIME_IMAGE=alpine:3.20

FROM --platform=$BUILDPLATFORM ${BUN_IMAGE} AS frontend-default-builder
WORKDIR /src

COPY web/default/package.json web/default/bun.lock ./web/default/

WORKDIR /src/web/default
RUN --mount=type=cache,id=renewapi-bun-default,target=/root/.bun/install/cache,sharing=locked \
    bun install --frozen-lockfile

COPY web/default ./web/default
COPY VERSION /src/VERSION

WORKDIR /src/web/default
RUN DISABLE_ESLINT_PLUGIN=true VITE_REACT_APP_VERSION="$(cat /src/VERSION)" bun run build

FROM --platform=$BUILDPLATFORM ${BUN_IMAGE} AS frontend-classic-builder
WORKDIR /src

COPY web/classic/package.json web/classic/bun.lock ./web/classic/

WORKDIR /src/web/classic
RUN --mount=type=cache,id=renewapi-bun-classic,target=/root/.bun/install/cache,sharing=locked \
    bun install --frozen-lockfile

COPY web/classic ./web/classic
COPY VERSION /src/VERSION

WORKDIR /src/web/classic
RUN VITE_REACT_APP_VERSION="$(cat /src/VERSION)" bun run build

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS backend-builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown
ARG UPSTREAM_REF=unknown

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH}

WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .
COPY --from=frontend-default-builder /src/web/default/dist ./web/default/dist
COPY --from=frontend-classic-builder /src/web/classic/dist ./web/classic/dist

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test "$(go env GOOS)" = "${TARGETOS}" && \
    test "$(go env GOARCH)" = "${TARGETARCH}" && \
    go build -trimpath -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=${VERSION}'" -o /out/new-api

FROM ${RUNTIME_IMAGE} AS runtime
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown
ARG UPSTREAM_REF=unknown
ARG RUNTIME_IMAGE=alpine:3.20

LABEL org.opencontainers.image.source="https://github.com/alex-ai-dev-lab/renewapi" \
      org.opencontainers.image.description="NewAPI source fork with compatibility and anti-poison profile hardening" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.base.name="${RUNTIME_IMAGE}" \
      org.opencontainers.image.upstream-ref="${UPSTREAM_REF}"

RUN apk add --no-cache ca-certificates tzdata curl shadow su-exec \
    && addgroup -g 1000 newapi \
    && adduser -D -H -u 1000 -G newapi -s /sbin/nologin newapi \
    && mkdir -p /app/logs /data /app/public \
    && chown newapi:newapi /app /app/logs /app/public /data

WORKDIR /data
COPY --from=backend-builder /out/new-api /app/new-api
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/

COPY --chmod=755 <<'EOF' /app/docker-entrypoint.sh
#!/bin/sh
set -eu

APP_USER="${APP_USER:-newapi}"
PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

ensure_identity() {
  current_gid="$(id -g "$APP_USER" 2>/dev/null || echo 1000)"
  current_uid="$(id -u "$APP_USER" 2>/dev/null || echo 1000)"
  if [ "$current_gid" != "$PGID" ]; then
    groupmod -o -g "$PGID" "$APP_USER" 2>/dev/null || true
  fi
  if [ "$current_uid" != "$PUID" ]; then
    usermod -o -u "$PUID" -g "$PGID" "$APP_USER" 2>/dev/null || true
  fi
}

fix_path() {
  path="$1"
  [ -e "$path" ] || mkdir -p "$path"
  if [ "$(id -u)" = "0" ]; then
    chown "$PUID:$PGID" "$path" 2>/dev/null || true
  fi
}

if [ $# -eq 0 ] || [ "${1#-}" != "$1" ]; then
  set -- /app/new-api "$@"
fi

if [ "$(id -u)" = "0" ]; then
  ensure_identity
  fix_path /data
  fix_path /app/logs
  fix_path /app/public
  exec su-exec "$PUID:$PGID" "$@"
fi

exec "$@"
EOF
RUN sed -i 's/\r$//' /app/docker-entrypoint.sh

EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD curl -fsS "http://127.0.0.1:${PORT:-3000}/api/status" >/dev/null || exit 1

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/new-api", "--log-dir", "/app/logs"]
