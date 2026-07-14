# Architecture & Design

## Vision & Design Principles

dargstack is a **CLI tool and project structure specification** for Docker Swarm stack management. Core principle: **Development-First Composition** — the development environment is the base layer; production is expressed as an incremental overlay.

**Design Tenets:**

| #   | Principle         | What it means                                                             |
| --- | ----------------- | ------------------------------------------------------------------------- |
| T1  | Optimal DX        | Every friction point detected and resolved interactively                  |
| T2  | Declaration-first | Behavior driven by file structure and compose declarations                |
| T3  | Context-aware     | CLI infers from environment (git, Docker, OS, directory layout)           |
| T4  | Offline-capable   | Core operations work without internet; online features degrade gracefully |
| T5  | Cross-platform    | Linux, macOS, Windows — no platform-specific workarounds by the user      |

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

4. **Compose profiles via deploy labels** — Docker Swarm rejects the standard `profiles:` key. dargstack uses `deploy.labels.dargstack.profiles` (comma-separated). When no `--profiles` flag is given: if any service declares a `default` profile, only `default`-profiled services deploy; otherwise all services deploy. If a `default` profile exists, unlabeled services deploy only when profile `unlabeled` is explicitly selected.

5. **Dev-only markers** — lines annotated with `# dargstack:dev-only` are stripped when building production compose.

6. **Service directories** — each service lives in its own directory under `src/{development,production}/<service-name>/` containing a `compose.yaml` plus co-located resources (secrets, configs, Dockerfiles). Each `compose.yaml` is a full, valid Docker Compose document. Relative `file:` paths and `build.context` are resolved to absolute paths during merge.

7. **Environment file merging** — dev `.env` and prod `.env` are merged for production (prod values override). Missing values are prompted; production blocks on missing values.

8. **STACK_DOMAIN** — defaults to `app.localhost`; configurable. TLS cert covers all discovered subdomains.

9. **Smart cert regeneration** — only regenerated if domains changed or cert is within 30 days of expiry.

10. **Image lifecycle** — auto-build in dev via `dargstack.development.build`; pre-pull before production deploy; offer cleanup after production deploy.

11. **`x-dargstack` extension** — top-level compose extension key; `secrets` subkey holds templating metadata.

12. **Secret templating** — `x-dargstack.secrets` with `hint`, `length`, `special_characters`, `template` (`{{ref}}`), and optional `third_party`. Topological sort resolves order.

13. **Audit trail** — each deployment saves a timestamped snapshot to `artifacts/audit-log/`. `inspect` lists, diffs, and displays past snapshots. `--dry-run` traces all steps without deploying.