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

## Step 1: Install dargstack v4

**Recommended** (binary download with checksum verification):

On Windows, run this in a POSIX shell (WSL, Git Bash, or Cygwin); it won't work in `cmd.exe` or native PowerShell.

```bash
ARCHIVE="dargstack_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/').tar.gz"
curl -sfL -o "$ARCHIVE" "https://github.com/dargstack/dargstack/releases/latest/download/$ARCHIVE"
curl -sfL https://github.com/dargstack/dargstack/releases/latest/download/checksums.txt | sha256sum -c - --ignore-missing
tar xzf "$ARCHIVE" && rm "$ARCHIVE"
sudo install -d /usr/local/bin && sudo install -m 0755 dargstack /usr/local/bin/ && rm dargstack
```

The last line needs `sudo` because `/usr/local/bin` is root-owned.
To install without root, or in a shell that has no `sudo` (Git Bash, Cygwin), run `mkdir -p "$HOME/.local/bin" && mv dargstack "$HOME/.local/bin/"` instead and make sure that directory is on your `PATH`.

**Alternative** (verified via Go module proxy):

```bash
go install github.com/dargstack/dargstack/v4/cmd/dargstack@latest
```

Remove the old v3 script:

```bash
sudo rm ~/scripts/dargstack
```

### Pinning a version for CI

The `releases/latest` URL above is suitable for local development where you want the newest release automatically.
For CI pipelines, pin to a specific version so that documentation generation and validation output are reproducible:

```bash
VERSION="$(sed -n 's#^FROM ghcr\.io/dargstack/dargstack:##p' Dockerfile.md)"
BASE_URL="https://github.com/dargstack/dargstack/releases/download/v${VERSION}"
ARCHIVE="dargstack_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/').tar.gz"
curl -sfL -o "$ARCHIVE" "${BASE_URL}/$ARCHIVE"
curl -sfL "${BASE_URL}/checksums.txt" | sha256sum -c - --ignore-missing
tar xzf "$ARCHIVE" && rm "$ARCHIVE"
mv dargstack /usr/local/bin/
```

The version is read from a `Dockerfile.md` file at the repository root.
The `.md` extension is intentional: Renovate's docker-manager matches files named `Dockerfile*`, so it will detect and offer pull requests for version bumps.
The file only needs a single `FROM` line:

```dockerfile
FROM ghcr.io/dargstack/dargstack:4.0.0
```

---

## Step 2: Migrate the config file

v3 used a flat `dargstack.env`:

```bash
# dargstack.env (v3)
VERSION=1.2.3
DOMAIN=app.example.com
```

v4 uses a structured `dargstack.yaml` at the root of your stack directory:

```yaml
# dargstack.yaml (v4)
metadata:
  compatibility: ">=4.0.0 <5.0.0"
  name: my-stack # optional; defaults to parent directory name
environment:
  production:
    branch: main # optional
    tag: latest # optional; "latest" or a specific image tag/version
    domain: app.example.com # optional
runtime:
  sudo: auto # optional; "auto" | "always" | "never"
```

Create `dargstack.yaml` at the root of your stack directory (same level as `src/`) and remove `dargstack.env`.

---

## Step 3: Split the monolithic stack.yml into per-service files

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
dargstack.env
```

### v4 structure

```
artifacts/             ← generated files (gitignored)
src/
  development/
    <service>/
      compose.yaml     ← one file per service
    .env
  production/
    <service>/
      compose.yaml     ← only the differences from development
    .env
dargstack.yaml         ← structured config
```

### How to split

For each service defined in `src/development/stack.yml`, create a dedicated directory and compose file.
Move only that service's keys (`services:`, `secrets:`, `volumes:`, `networks:`, `configs:`) into it.

**Before (v3): `src/development/stack.yml`** (excerpt):

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

**After (v4): `src/development/api/compose.yaml`**:

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

**After (v4): `src/development/postgres/compose.yaml`**:

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

> **Note:** The `file:` path in each service's compose is relative to that service's directory.
> v4 rewrites these to point to `artifacts/secrets/` automatically before calling `docker stack deploy`.

### Production overrides

v3 had a `production.yml` spruce overlay and a `production.sed` sed-patch file.
Drop both.
Instead, write only the differences in `src/production/<service>/compose.yaml`:

**Before (v3): `src/production/production.yml`** (excerpt):

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

**After (v4): `src/production/api/compose.yaml`**:

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

You no longer need `production.sed`.
The `#DARGSTACK-REMOVE` sed trick is replaced by the `# dargstack:dev-only` label convention.
Any deploy label ending in `# dargstack:dev-only` is stripped before production deployment.

### Development image names

The development image name format has changed:

| v3                              | v4                                            |
| ------------------------------- | --------------------------------------------- |
| `username/repository_name:dev`  | `stack-name/repository-name:development`      |

### Volumes in production overrides

Spruce replaces lists by default.
When a service uses a named volume in both development and production, the top-level `volumes:` definition belongs in the base (development) layer, and the production overlay only replaces the service's mount list.

**Common mistake:** putting the top-level `volumes:` block in the production overlay.
This replaces the entire top-level `volumes:` section, losing volume definitions from other services.

**Correct pattern:**

`src/development/postgres/compose.yaml` (base layer defines the volume):

```yaml
services:
  postgres:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

`src/production/postgres/compose.yaml` (overlay only changes the mount):

```yaml
services:
  postgres:
    volumes:
      - (( prune ))
      - pgdata:/var/lib/postgresql/data
      - ./backups:/backups
```

The production overlay replaces only `postgres.volumes`.
The top-level `volumes:` section from the base layer is preserved because spruce deep-merges maps.
The same rule applies to `secrets:` and `configs:`.
Define them in the base layer even if only production uses them in their final form.

### Environment files

v3 concatenated `src/development/stack.env` and `src/production/production.env` into `src/production/stack.env` during `derive`.
v4 uses `.env.template` files (tracked in version control) per environment, with corresponding `.env` files (gitignored) holding actual values:

- `src/development/.env.template`: development variable keys (tracked)
- `src/development/.env`: development values (gitignored, auto-created from template)
- `src/production/.env.template`: production override keys (tracked)
- `src/production/.env`: production values (gitignored, auto-created from template)

Rename `src/development/stack.env` to `src/development/.env.template` and `src/production/production.env` to `src/production/.env.template`.
Add `.env` to `src/development/.gitignore` and `src/production/.gitignore`.

---

## Step 4: Migrate secrets

### v3 approach

v3 stored secrets as static files with a `.secret.template` extension in `src/<env>/secrets/**/*.secret.template`.
The deploy script checked whether the corresponding `.secret` file existed and whether it still contained the `UNSET THIRD PARTY SECRET` placeholder.

Secret files were typically committed to the repository or managed manually outside git.

### v4 approach

v4 declares secret generation rules in `x-dargstack.secrets` inside each service's compose file.
Generated values are written to `artifacts/secrets/` (gitignored).
Nothing is committed.

**Migration:**

1. For each secret that was randomly generated (passwords, keys), add an `x-dargstack.secrets` entry with `type: random_string` or `type: private_key`.
   Delete the static secret file: v4 will generate a fresh value.

2. For each secret that required manual input (third-party tokens, API keys), add `type: third_party` and an optional `hint:`.
   Set the value in `artifacts/secrets/<name>` after the first deploy attempt, or provide it when prompted.

3. For composite secrets built from other secrets (e.g. connection URLs), use `type: template`:

   ```yaml
   x-dargstack:
     secrets:
       db-url:
         type: template
         template: "postgresql://postgres:{{secret:postgres-password}}@postgres:5432/mydb"
   ```

4. For secrets that need a human-readable word, use `type: wordlist_word`.

5. For development-only secrets with a hardcoded default, use `type: insecure_default`.

6. Delete `src/development/secrets/` and `src/production/secrets/`, those directories are no longer used.

### Renaming existing production secrets

v4 defines secrets with a flat kebab-case name (e.g. `my-stack-api-key`) while v3 included underscores (e.g. `my-stack_api-key`).
Any secret already created in a production swarm therefore needs to be re-created under its new name before the v4 stack is first deployed.

[`scripts/migrate-secrets.sh`](scripts/migrate-secrets.sh) automates this.
Run it on a swarm manager node before deploying the v4 branch to production.
It derives the new name for each existing secret by replacing underscores with dashes, dumps every old secret's value via a throwaway swarm service, and recreates it under the new name.
It only creates new secrets, it never deletes or modifies the old ones, so remove those manually once the v4 stack is confirmed healthy.

---

## Step 5: Remove the `derive` step

If your CI/CD pipeline or deployment scripts ran `dargstack derive` before `dargstack deploy --production`, remove that step.
v4 performs the merge automatically during deploy.

---

## Step 6: Verify

Run validation against your migrated stack:

```bash
dargstack validate
dargstack validate --production
```

Then do a dry-run deploy to see the merged compose output without touching the daemon:

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

| v3                      | v4                                    |
| ----------------------- | ------------------------------------- |
| `dargstack build`       | `dargstack build`                     |
| `dargstack deploy`      | `dargstack deploy`                    |
| `dargstack derive`      | _(removed: automatic during deploy)_  |
| `dargstack redeploy`    | `dargstack deploy --force`            |
| `dargstack rgen`        | `dargstack document`                  |
| `dargstack rm`          | `dargstack remove`                    |
| `dargstack self-update` | `dargstack update --self`             |
| `dargstack validate`    | `dargstack validate`                  |
| _(none)_                | `dargstack audit`                     |
| _(none)_                | `dargstack certify`                   |
| _(none)_                | `dargstack clone`                     |
| _(none)_                | `dargstack initialize`                |
| _(none)_                | `dargstack inspect`                   |
| _(none)_                | `dargstack profiles`                  |
| _(none)_                | `dargstack secret`                    |

### New `certify` command

dargstack v4 generates certificates for all project services using `mkcert`.
Certificates are stored under `artifacts/certificates`.
You may drop project-specific certificate generation solutions and update certificate mount paths, for example:

| v3                             | v4                                 |
| ------------------------------ | ---------------------------------- |
| `/etc/traefik/acme/traefik.crt`  | `/etc/traefik/acme/localhost.pem`       |
| `/etc/traefik/acme/traefik.key`  | `/etc/traefik/acme/localhost-key.pem`   |

---

## Getting help

- [README](README.md): project structure, configuration, and all commands
- [GitHub Issues](https://github.com/dargstack/dargstack/issues): bug reports and questions
