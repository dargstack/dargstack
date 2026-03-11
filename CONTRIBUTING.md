# Contributing to dargstack

Thank you for your interest in contributing!

## Vision & Design Principles

dargstack is a **CLI tool and project structure specification** for Docker Swarm stack management. Core principle: **Development-First Composition** — the development environment is the base layer; production is expressed as an incremental overlay.

**Design Tenets:**

| # | Principle | What it means |
|---|-----------|-------------|
| T1 | Optimal DX | Every friction point detected and resolved interactively |
| T2 | Declaration-first | Behavior driven by file structure and compose declarations |
| T3 | Context-aware | CLI infers from environment (git, Docker, OS, directory layout) |
| T4 | Offline-capable | Core operations work without internet; online features degrade gracefully |
| T5 | Cross-platform | Linux, macOS, Windows — no platform-specific workarounds by the user |

**Vision Goals:**
- Reduce cognitive overhead compared to plain `docker stack`
- Maintain minimal command set (deploy, remove, build, validate, document, inspect)
- Enable development as the source of truth with production as a small delta
- Build in audit trails (deployment snapshots) and validation
- Simplify secret management via templating and smart regeneration

## Key Design Decisions

Understanding these decisions will help you make changes that align with dargstack's architecture:

1. **YAML merge uses yaml.v2 for spruce compatibility** — spruce traverses `map[interface{}]interface{}` trees; yaml.v3 produces `map[string]interface{}`. Pipeline: yaml.v2 parse → spruce merge/eval → yaml.v2 marshal → yaml.v3 re-parse for clean output. **Never** parse compose with yaml.v3 before passing to spruce.

2. **Docker Swarm only** — no `docker compose up` path; always `docker stack deploy`. `build:` is handled before deploy (auto-build in dev mode).

3. **Development-first composition** — dev compose is the base; production overlay adds/overrides/prunes.

4. **Compose profiles via deploy labels** — Docker Swarm rejects the standard `profiles:` key. dargstack uses `deploy.labels.dargstack.profiles` (comma-separated). When no `--profile` flag is given: if any service declares a `default` profile, only `default`-profiled services deploy; otherwise all services deploy. If a `default` profile exists, unlabeled services deploy only when profile `unlabeled` is explicitly selected.

5. **Dev-only markers** — lines annotated with `# dargstack:dev-only` are stripped when building production compose.

6. **Service directories** — each service lives in its own directory under `src/{development,production}/<service-name>/` containing a `compose.yaml` plus co-located resources (secrets, configs, Dockerfiles). Each `compose.yaml` is a full, valid Docker Compose document. Relative `file:` paths and `build.context` are resolved to absolute paths during merge.

7. **Environment file merging** — dev `.env` and prod `.env` are merged for production (prod values override). Missing values are prompted; production blocks on missing values.

8. **STACK_DOMAIN** — defaults to `app.localhost`; configurable. TLS cert covers all discovered subdomains.

9. **Smart cert regeneration** — only regenerated if domains changed or cert is within 30 days of expiry.

10. **Image lifecycle** — auto-build in dev via `dargstack.development.build`; pre-pull before production deploy; offer cleanup after production deploy.

11. **`x-dargstack` extension** — top-level compose extension key; `secrets` subkey holds templating metadata.

12. **Secret templating** — `x-dargstack.secrets` with `hint`, `length`, `special_characters`, `template` (`{{ref}}`), and optional `third_party`. Topological sort resolves order.

13. **Audit trail** — each deployment saves a timestamped snapshot to `artifacts/audit-log/`. `inspect` lists, diffs, and displays past snapshots. `--dry-run` traces all steps without deploying.

## Architecture

dargstack is organized into focused packages, each with a clear responsibility:

| Package | Responsibility |
|---------|-----------------|
| `cmd/dargstack/` | Entry point and CLI bootstrapping |
| `internal/cli/` | Cobra commands: deploy, rm, build, certificates, docs, validate, inspect, update --self |
| `internal/compose/` | Spruce-based YAML deep merge, profile filtering, env file helpers |
| `internal/config/` | `dargstack.yaml` parsing, stack directory detection, semver compatibility |
| `internal/docker/` | Docker SDK client for queries; CLI executor for `docker stack deploy/rm` |
| `internal/tls/` | Domain-aware certificate generation with smart regeneration |
| `internal/resource/` | Secret, config, Dockerfile, certificate validation and docs generation |
| `internal/secret/` | Template resolution (`{{ref}}`), topological sort, random generation |
| `internal/audit/` | Deployment snapshots in `artifacts/audit-log/`, listing, diffing |

## Development Setup

**Prerequisite** – Git installed, see [git-scm.com: Install](https://git-scm.com/).

```bash
# Clone the repository
git clone https://github.com/dargstack/dargstack.git
cd dargstack

# Build
go build ./cmd/dargstack

# Run tests
go test -race ./...

# Lint (requires golangci-lint v2)
golangci-lint run ./...
```

## Code Style

- Run `golangci-lint run ./...` before submitting — CI enforces zero issues.
- Use `gofmt` and `goimports` for formatting.
- Error strings should not be capitalized (per Go conventions).
- Check all error return values (enforced by `errcheck`).

## Testing

- Tests live alongside source files as `*_test.go`.
- Use `t.TempDir()` for filesystem tests.
- Run with `-race` to detect data races.
- Docker-dependent tests are in CI's integration-test job.

## Documentation

- Cobra command documentation lives in `/docs`
- Update the LLM-ready markdown files with `go run ./internal/tools/docgen`

## Implementation Guides

### Adding a CLI command

1. Create `internal/cli/<command>.go` with a `*cobra.Command` (Short, Long, RunE)
2. Register in `internal/cli/root.go` `init()` via `rootCmd.AddCommand()`
3. Add help text and flag definitions in `init()`
4. Regenerate docs with `go run ./internal/tools/docgen`
5. Update [README.md](README.md) commands table
6. Add test coverage

### Adding a CLI flag

1. Define in `internal/cli/<command>.go` using `Flags().StringVar(...)` or similar
2. Register in `init()` of that command file
3. Pass value to the underlying package (e.g. `filter.go` for profiles)
4. Regenerate docs with `go run ./internal/tools/docgen`
5. Update [README.md](README.md) Commands and Global Flags tables
6. Add test coverage

### Extending resource validation

1. Add check function to `internal/resource/validate.go` (follow `validateSecrets()`/`validateConfigs()` pattern)
2. Return `Issue` structs with `Severity`, `Resource`, and `Description`
3. Call from `Validate()` in the same file
4. Add tests in `validate_test.go`

### Extending secret templating

1. Add field to `Template` struct in `internal/secret/secret.go`
2. Update `ExtractTemplates()` to parse the new field
3. If the new field creates dependencies, update `TopologicalSort()`
4. Document the new field in [README.md](README.md)
5. Add test in `secret_test.go`

### Modifying config fields

1. Update `Config` struct in `internal/config/config.go` (e.g. `ProductionConfig.Tag` maps to `production.tag`)
2. Add defaults in `applyDefaults()`
3. Update [README.md](README.md) `Configuration: dargstack.yaml` section

### Changing TLS behavior

1. Edit `internal/tls/tls.go`
2. Update `ExtractDomains()` for new domain sources
3. Update tests in `internal/tls/tls_test.go`

### Compose merge pipeline (CRITICAL — do not break)

**DO NOT** parse compose with yaml.v3 then pass to spruce — it fails silently because spruce requires `map[interface{}]interface{}` but yaml.v3 produces `map[string]interface{}`.

Always follow this sequence:

1. Parse with `yaml.v2.Unmarshal()` → produces `map[interface{}]interface{}`
2. Pass to spruce for merge/eval
3. Marshal with `yaml.v2.Marshal()`
4. Re-parse with yaml.v3 for clean, type-safe output

## Pull Requests

1. Fork the repository and create a feature branch.
2. Make your changes with tests.
3. Ensure `go test -race ./...` and `golangci-lint run ./...` pass.
4. Open a PR with a clear description of the change.

## Releasing

Releases are automated via goreleaser on tag push:

```bash
git tag v1.0.0
git push origin v1.0.0
```
