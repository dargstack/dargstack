# syntax=docker/dockerfile:1

# ── base: shared Go module cache ────────────────────────────────────────────
FROM golang:1.26.1-alpine AS base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# ── lint ────────────────────────────────────────────────────────────────────
FROM golangci/golangci-lint:2.11.3-alpine AS lint
WORKDIR /src
COPY --from=base /go /go
COPY . .
RUN golangci-lint run ./...

# ── copy: shared source snapshot ────────────────────────────────────────────
FROM base AS copy
COPY . .

# ── test ────────────────────────────────────────────────────────────────────
FROM copy AS test
RUN go test -race -coverprofile=/tmp/coverage.txt -covermode=atomic ./...

# ── build ───────────────────────────────────────────────────────────────────
FROM copy AS build
RUN CGO_ENABLED=0 go build -trimpath -o /out/dargstack ./cmd/dargstack

# ── final: smoke-test the binary ────────────────────────────────────────────
FROM alpine:3.23.3 AS final
COPY --from=build /out/dargstack /usr/local/bin/dargstack
RUN dargstack --help
