# syntax=docker/dockerfile:1

# ── base: shared Go module cache ────────────────────────────────────────────
FROM golang:1.26.1-alpine AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download \
    && apk update && apk add git

# ── copy: shared source snapshot ────────────────────────────────────────────
FROM base AS copy
COPY . .

# ── lint ────────────────────────────────────────────────────────────────────
FROM golangci/golangci-lint:v2.12.2-alpine AS lint
WORKDIR /src
COPY --from=copy /src .
RUN golangci-lint run ./...

# ── test ────────────────────────────────────────────────────────────────────
FROM copy AS test
RUN --mount=type=cache,target=/var/cache/apk \
    --mount=type=cache,target=/go/pkg/mod \
    apk update && apk add gcc musl-dev \
    && go test -race -coverprofile=/tmp/coverage.txt -covermode=atomic ./...

# ── build ───────────────────────────────────────────────────────────────────
FROM copy AS build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/dargstack \
      ./cmd/dargstack \
    && /out/dargstack --version

# ── final: minimal scratch image ────────────────────────────────────────────
FROM scratch AS final
COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=lint /src/go.mod /.lint-stage.go.mod
COPY --from=test /src/go.mod /.test-stage.go.mod
COPY --from=build /out/dargstack /usr/local/bin/dargstack
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/dargstack"]
LABEL org.opencontainers.image.source="https://github.com/dargstack/dargstack"
LABEL org.opencontainers.image.description="Docker Swarm stack helper CLI"
