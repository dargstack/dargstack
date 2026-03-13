# Migration Guide: dargstack v3 → v4

This guide helps you migrate an existing dargstack v3 (Bash) project to v4 (Go).

---

## Overview of changes

| Area              | v3                                               | v4                                                                                       |
| ----------------- | ------------------------------------------------ | ---------------------------------------------------------------------------------------- |
| Runtime           | Bash script                                      | Compiled Go binary                                                                       |
| Compose structure | Single `stack.yml` per environment               | One `compose.yaml` per service                                                           |
| Production merge  | `derive` via sed + optional spruce merge         | Automatic deep-merge via spruce on deploy                                                |
| Secrets           | `.secret.template` files in `src/<env>/secrets/` | `x-dargstack.secrets` in compose files, generated to `artifacts/secrets/`                |
| Config file       | `dargstack.env` key-value file                   | `dargstack.yaml` structured config                                                       |
| Spruce            | Invoked via `docker run gfranks/spruce`          | Integrated via Go library (`github.com/geofffranks/spruce`), no external binary required |

---

## Step 1 — Install dargstack v4

**Recommended** (verified via Go module proxy):

```bash
go install github.com/dargstack/dargstack/v4/cmd/dargstack@latest
```

**Alternative** (binary download — verify the checksum on the [Releases page](https://github.com/dargstack/dargstack/releases) before use):

```bash
curl -sL https://github.com/dargstack/dargstack/releases/latest/download/dargstack_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/').tar.gz | tar xz
sudo mv dargstack /usr/local/bin/
```

Remove the old v3 script:

```bash
sudo rm /usr/local/bin/dargstack   # or wherever you installed it
```

---

## Step 2 — Migrate the config file

v3 used a flat `dargstack.env`:

```bash
# dargstack.env (v3)
VERSION=1.2.3
DOMAIN=app.example.com
```

v4 uses a structured `dargstack.yaml` at the root of your stack directory:

```yaml
# dargstack.yaml (v4)
compatibility: ">=4.0.0 <5.0.0"
name: my-stack # optional; defaults to parent directory name
production:
  branch: main # optional
  tag: latest # optional; "latest" or a specific image tag/version
  domain: app.example.com # optional
sudo: auto # optional; "auto" | "always" | "never"
```

Create `dargstack.yaml` at the root of your stack directory (same level as
`src/`) and remove `dargstack.env`.

---

## Step 3 — Split the monolithic stack.yml into per-service files

### v3 structure

```
src/
  development/
    stack.yml          ← all services in one file
    stack.env
  production/
    stack.yml          ← derived by `dargstack derive`
    production.yml     ← optional spruce overlay
    production.env
    production.sed     ← optional sed patches
```

### v4 structure

```
src/
  development/
    <service>/
      compose.yaml     ← one file per service
    .env
  production/
    <service>/
      compose.yaml     ← only the differences from development
    .env
```

### How to split

For each service defined in `src/development/stack.yml`, create a dedicated
directory and compose file. Move only that service's keys (`services:`,
`secrets:`, `volumes:`, `networks:`, `configs:`) into it.

**Before (v3) — `src/development/stack.yml`** (excerpt):

```yaml
services:
  api:
    image: api:latest
    ports:
      - "3000:3000"
    secrets:
      - api-key
  postgres:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres-password
    secrets:
      - postgres-password

secrets:
  api-key:
    file: ./secrets/api/api-key.secret
  postgres-password:
    file: ./secrets/postgres/postgres-password.secret
```

**After (v4) — `src/development/api/compose.yaml`**:

```yaml
services:
  api:
    image: api:latest
    ports:
      - "3000:3000"
    secrets:
      - api-key

secrets:
  api-key:
    file: ./key.secret

x-dargstack:
  secrets:
    api-key:
      type: random_string
      length: 32
```

**After (v4) — `src/development/postgres/compose.yaml`**:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/postgres-password
    secrets:
      - postgres-password

secrets:
  postgres-password:
    file: ./password.secret

x-dargstack:
  secrets:
    postgres-password:
      type: random_string
```

> **Note:** The `file:` path in each service's compose is relative to that
> service's directory. v4 rewrites these to point to `artifacts/secrets/`
> automatically before calling `docker stack deploy`.

### Production overrides

v3 had a `production.yml` spruce overlay and a `production.sed` sed-patch file.
Drop both. Instead, write only the differences in
`src/production/<service>/compose.yaml`:

**Before (v3) — `src/production/production.yml`** (excerpt):

```yaml
services:
  api:
    image: ghcr.io/myorg/api:v1.0.0
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        order: start-first
```

**After (v4) — `src/production/api/compose.yaml`**:

```yaml
services:
  api:
    image: ghcr.io/myorg/api:v1.0.0
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        order: start-first
```

You no longer need `production.sed`. The `#DARGSTACK-REMOVE` sed trick is
replaced by the `# dargstack:dev-only` label convention — any deploy label
ending in `# dargstack:dev-only` is stripped before production deployment.

### Environment files

v3 concatenated `src/development/stack.env` and `src/production/production.env`
into `src/production/stack.env` during `derive`. v4 uses independent `.env`
files per environment:

- `src/development/.env` — development variables
- `src/production/.env` — production overrides (key=value pairs that override development values)

Rename `src/development/stack.env` → `src/development/.env` and
`src/production/production.env` → `src/production/.env`.

---

## Step 4 — Migrate secrets

### v3 approach

v3 stored secrets as static files with a `.secret.template` extension in
`src/<env>/secrets/**/*.secret.template`. The deploy script checked whether the
corresponding `.secret` file existed and whether it still contained the
`UNSET THIRD PARTY SECRET` placeholder.

Secret files were typically committed to the repository or managed manually
outside git.

### v4 approach

v4 declares secret generation rules in `x-dargstack.secrets` inside each
service's compose file. Generated values are written to `artifacts/secrets/`
(gitignored). Nothing is committed.

**Migration:**

1. For each secret that was randomly generated (passwords, keys), add an
   `x-dargstack.secrets` entry with `type: random_string` or `type: private_key`.
   Delete the static secret file — v4 will generate a fresh value.

2. For each secret that required manual input (third-party tokens, API keys), add
   `type: third_party` and an optional `hint:`. Set the value in
   `artifacts/secrets/<name>` after the first deploy attempt, or provide it when
   prompted.

3. For composite secrets built from other secrets (e.g. connection URLs), use
   `type: template`:

   ```yaml
   x-dargstack:
     secrets:
       db-url:
         type: template
         template: "postgresql://postgres:{{secret:postgres-password}}@postgres:5432/mydb"
   ```

4. Delete `src/development/secrets/` and `src/production/secrets/` — those
   directories are no longer used.

---

## Step 5 — Remove the `derive` step

If your CI/CD pipeline or deployment scripts ran `dargstack derive` before
`dargstack deploy --production`, remove that step. v4 performs the merge
automatically during deploy.

---

## Step 6 — Verify

Run validation against your migrated stack:

```bash
dargstack validate
dargstack validate --production
```

Then do a dry-run deploy to see the merged compose output without touching the
daemon:

```bash
dargstack deploy --dry-run
dargstack deploy --production --dry-run
```

If both look correct, deploy:

```bash
dargstack deploy
```

---

## Quick reference: command renames

| v3                                    | v4                                                          |
| ------------------------------------- | ----------------------------------------------------------- |
| `dargstack deploy`                    | `dargstack deploy`                                          |
| `dargstack deploy --production <tag>` | `dargstack deploy --production` (tag from `dargstack.yaml`) |
| `dargstack redeploy`                  | `dargstack deploy --re`                                     |
| `dargstack derive`                    | _(removed — automatic during deploy)_                       |
| `dargstack rm`                        | `dargstack remove`                                          |
| `dargstack build`                     | `dargstack build`                                           |
| `dargstack rgen`                      | `dargstack document`                                        |
| `dargstack validate`                  | `dargstack validate`                                        |
| `dargstack self-update`               | `dargstack update --self`                                   |
| _(none)_                              | `dargstack init`                                            |
| _(none)_                              | `dargstack certify`                                         |
| _(none)_                              | `dargstack inspect`                                         |

---

## Getting help

- [README](README.md) — project structure, configuration, and all commands
- [GitHub Issues](https://github.com/dargstack/dargstack/issues) — bug reports
  and questions
