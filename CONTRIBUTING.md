# Contributing to dargstack

Thank you for your interest in contributing!

## Quick Start

**Prerequisites**

- Go — see [go.dev: Download and install](https://go.dev/doc/install)
- Git — see [git-scm.com: Install](https://git-scm.com/)
- golangci-lint v2 — see [golangci-lint.run: Local Installation](https://golangci-lint.run/docs/welcome/install/local/)

```bash
# Clone and build
git clone https://github.com/dargstack/dargstack.git
cd dargstack
go build ./cmd/dargstack

# Run tests
go test -race ./...

# Lint
golangci-lint run ./...
```

## Development Workflow

Run the binary during development with `go run ./cmd/dargstack` instead of rebuilding.

Pull requests should:

1. Fork the repository and create a feature branch.
2. Include tests for new behavior.
3. Pass `go test -race ./...` and `golangci-lint run ./...`.
4. Have a clear description of the change and its motivation.

## Code Style

- Run `golangci-lint run ./...` before submitting — CI enforces zero issues.
- Use `gofmt` and `goimports` for formatting.
- Error strings should not be capitalized (per Go conventions).
- Check all error return values (enforced by `errcheck`).

## Testing

- Tests live alongside source files as `*_test.go`.
- Use `t.TempDir()` for filesystem tests.
- Run with `-race` to detect data races.
- Docker-dependent tests run in CI's integration-test job.

## Architecture

dargstack is organized into focused packages, each with a clear responsibility:

| Package              | Responsibility                                                                           |
| -------------------- | ---------------------------------------------------------------------------------------- |
| `cmd/dargstack/`     | Entry point and CLI bootstrapping                                                        |
| `internal/cli/`      | Cobra commands: deploy, rm, build, certificates, docs, validate, inspect, secret, update |
| `internal/compose/`  | Spruce-based YAML deep merge, profile filtering, env file helpers                        |
| `internal/config/`   | `dargstack.yaml` parsing, stack directory detection, semver compatibility                |
| `internal/docker/`   | Docker SDK client for queries; CLI executor for `docker stack deploy/rm`                 |
| `internal/tls/`      | Domain-aware certificate generation with smart regeneration                              |
| `internal/resource/` | Secret, config, Dockerfile, certificate validation and docs generation                   |
| `internal/secret/`   | Template resolution (`{{ref}}`), topological sort, random generation                     |
| `internal/audit/`    | Deployment snapshots in `artifacts/audit-log/`, listing, diffing                         |

## Implementation Reminders

- **CLI changes** — register new commands in `internal/cli/root.go`; regenerate docs with `go run ./internal/tools/docgen`
- **Config changes** — add defaults in `applyDefaults()` in `internal/config/config.go`
- **Secret templating changes** — update `TopologicalSort()` if the change introduces new dependencies
- **TLS changes** — update `ExtractDomains()` in `internal/tls/tls.go` if new domain sources are added
- **Compose merge** — never parse compose with yaml.v3 before spruce; always use yaml.v2 for the merge pipeline

## Design Decisions

See [docs/architecture.md](docs/architecture.md) for vision, design tenets, and all key design decisions.

## Getting Help

- [README.md](README.md) — usage documentation and examples
- [GitHub Issues](https://github.com/dargstack/dargstack/issues) — bug reports and feature requests