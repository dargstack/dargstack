# dargstack

Docker Swarm, made simple. Dev-first deployments, production overlays, built-in audit trail.

dargstack is a **CLI tool and project structure specification** that **reduces Docker Swarm complexity** to a minimal command set. Define your development setup as the base, express production as incremental changes on top.

dargstack does **not** replace `docker stack`. You can interact with `docker stack` on the same stack that you manage with dargstack.

---

The following projects successfully employ dargstack in production:

- [maevsi/stack](https://github.com/maevsi/stack/)
- [dargmuesli/jonas-thelemann_stack](https://github.com/dargmuesli/jonas-thelemann_stack/)
- [flipdot/drinks-touch_stack](https://github.com/flipdot/drinks-touch_stack/)

---

Migrating from v3? See [MIGRATION.md](MIGRATION.md).
Contributing? See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Table of Contents

- [Why dargstack?](#why-dargstack)
- [Install](#install)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
  - [Service Files](#service-files)
  - [Git Cloning](#git-cloning)
  - [Configuration: dargstack.yaml](#configuration-dargstackyaml)
  - [Profiles & Performance](#profiles--performance)
  - [Secret Templating](#secret-templating)
  - [Environment Files](#environment-files)
- [Commands](#commands)

---

## Why dargstack?

Deploying the same app to development and production with Docker Swarm usually means maintaining two nearly identical compose files. Change one thing? Manually copy, edit, hope nothing breaks.

dargstack inverts this: define development as the source of truth, then express production as **changes** on top. One deploy command. One audit trail. Done.

| dargstack                                                      | `docker stack`                                                                |
| -------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| ✅ A single resource and diff specification                    | ❌ Two compose files for dev and prod – risk of configuration drift           |
| ✅ Clear file separation by service                            | ❌ Monolithic compose file – hard to maintain if big                          |
| ✅ Snapshot for every deploy; easy inspect and diff            | ❌ Volatile audit trail – live console tracing only                           |
| ✅ Safer secret management with auto-generation and templating | ❌ Manual secret management – tedious, often insecure defaults                |
| ✅ Development certificates auto-generated                     | ❌ No TLS certificates – out of scope, traffic unencrypted                    |
| ✅ Zero downtime service update motivation                     | ❌ Stop-first update order by default – unreliable availability in production |

## Install

### Recommended — From GitHub Releases

```bash
ARCHIVE="dargstack_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/').tar.gz"
curl -sfL -o "$ARCHIVE" "https://github.com/dargstack/dargstack/releases/latest/download/$ARCHIVE"
curl -sfL https://github.com/dargstack/dargstack/releases/latest/download/checksums.txt | sha256sum -c - --ignore-missing
tar xzf "$ARCHIVE" && rm "$ARCHIVE"
sudo mv dargstack /usr/local/bin/
```

### Alternative — From Source

**Prerequisite** – Go installed, see [go.dev: Download and install](https://go.dev/doc/install).

```bash
go install github.com/dargstack/dargstack/v4/cmd/dargstack@latest
```

Package integrity is enforced by the Go module proxy and the module's `go.sum` lockfile.
Pin to a specific version (e.g., `@v4.1.0`) for a reproducible, auditable install.

## Quick Start

**Prerequisite** – Docker installed, see [docs.docker.com: Install Docker Engine](https://docs.docker.com/engine/install/).

1. Initialize a new dargstack project:

   ```bash
   dargstack initialize
   ```

2. Fill in your service configuration according to the [docker.com: Compose file reference](https://docs.docker.com/reference/compose-file).

3. Deploy:

   ```bash
   cd <project_name>
   dargstack deploy
   ```

Done! 🎉 Your stack is live.

## Core Concepts

Suppose you have an `api` service as part of an `example` project:

```
example/
├── api/                                # The service's source code
│   ├── Dockerfile                      # Dockerfile for the api service
│   └── ...
├── stack/
│   ├── artifacts/                      # Generated files
│   │   ├── audit-log/                  # Deployment snapshots (gitignored)
│   │   ├── certificates/               # Local TLS certificates (gitignored)
│   │   ├── docs/
│   │   │   └── README.md               # Generated stack documentation
│   │   ├── .gitignore
│   │   └── README.md                   # Explains artifacts folder contents
│   ├── src/
│   │   ├── development/
│   │   │   ├── api/
│   │   │   │   ├── compose.yaml        # Full Docker Compose document
│   │   │   │   ├── configuration.toml  # File-based volume mount
│   │   │   │   ├── key.secret          # Secret used by the service
│   │   │   │   └── ...
│   │   │   └── .env                    # Environment variables
│   │   └── production/
│   │       ├── api/
│   │       │   ├── compose.yaml        # YAML deep-merge override
│   │       │   ├── configuration.toml  # File-based override
│   │       │   └── ...
│   │       └── .env                    # Key-based override
│   └── dargstack.yaml                  # Project configuration
└── ...
```

### Service Files

Each service file is a full Docker Compose document — files are deep-merged by [spruce](https://github.com/geofffranks/spruce).
See [github.com: What are all the Spruce operators?](https://github.com/geofffranks/spruce/blob/main/doc/operators.md) for special keywords controlling merge behavior.

```yaml
# src/development/api/compose.yaml
services:
  api:
    image: api:latest
    ports:
      - "3000:3000"
    secrets:
      - api-key
    deploy:
      labels:
        - (( append ))
        - dargstack.development.build=../../../../api
        - traefik.http.routers.api.rule=Host(`api.${STACK_DOMAIN}`)
        - traefik.http.routers.api.tls=true
        - traefik.http.services.api.loadbalancer.server.port=8080
        - some.label=for-development # dargstack:dev-only
    user: (( prune ))

secrets:
  api-key:
    file: ./key.secret

x-dargstack:
  secrets:
    api-key:
      length: 32
      special_characters: false
```

**Key rules:**

- Each service file has a `services:` top-level key
- Secrets, volumes, networks, and configs are declared in the same file as the service that uses them
- Production files contain only **differences** from development:

```yaml
# src/production/api/compose.yaml
services:
  api:
    image: ghcr.io/myorg/api:v1.0.0
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        order: start-first # Zero-downtime (use stop-first for stateful services)
```

### Git Cloning

The `dargstack.development.git.ssh` and `dargstack.development.git.https` labels instruct dargstack to clone a git repository before building a service's Docker image. The repository is cloned to a sibling directory of the stack, named after the repository:

```yaml
services:
  webapp:
    deploy:
      labels:
        dargstack.development.git.ssh: "git@github.com:mystack/webapp.git"
    image: mystack/webapp:development
```

This clones `webapp.git` to a `webapp/` directory next to the stack directory, and automatically sets the build context to that directory. You can override the build context with `dargstack.development.build` to point to a subdirectory:

```yaml
services:
  webapp:
    deploy:
      labels:
        dargstack.development.git.ssh: "git@github.com:mystack/webapp.git"
        dargstack.development.git.https: "https://github.com/mystack/webapp.git"
        dargstack.development.build: "../../../../repository/packages/frontend"
    image: mystack/webapp:development
```

The SSH URL is used as the primary clone URL, with the HTTPS URL as a fallback. Providing both ensures cloning works regardless of network restrictions. The repository is cloned once (on first deploy) and left untouched on subsequent deploys.

### Configuration: dargstack.yaml

```yaml
metadata:
  compatibility: ">=4.0.0 <5.0.0" # required, string (semver range)
  name: my-stack # optional, defaults to parent directory name
  source: # optional
    name: my-repo
    url: https://github.com/org/repo
  external_services: # optional
    status:
      description: Service status dashboard

runtime:
  sudo: auto # optional, `auto` | `always` | `never`
  build:
    mode: always # optional, `always` | `missing`
  deploy:
    volumes:
      prompt: true # optional, prompt to remove volumes on first deploy

environment:
  development:
    domain: app.localhost # optional, defaults to "app.localhost"
    certificate:
      include: [] # optional, domains added to TLS cert
      exclude: [] # optional, domains removed from TLS cert
  production:
    branch: main # optional, defaults to "main"
    domain: app.localhost # optional, defaults to "app.localhost"
    tag: latest # optional, `latest` | string
```

### Profiles & Performance

Deploy named groups of services to save resources:

```yaml
# src/development/adminer/compose.yaml
services:
  adminer:
    image: adminer
    deploy:
      labels:
        dargstack.profiles: db # Only deployed with --profiles db
```

Multiple profiles: `dargstack.profiles: "db,monitoring"` (comma-separated).

If no profile selection is made and any service declares `default`, only services with `dargstack.profiles: default` are deployed; unlabeled services are excluded.
If no profile selection is made and no service declares `default`, all services (including unlabeled) are deployed.

When one or more profiles are explicitly activated with `--profiles`, only services whose `dargstack.profiles` intersect the active profile set are deployed. Services without a `dargstack.profiles` label are deployed only if the special `unlabeled` profile is explicitly activated (for example: `--profiles unlabeled` or `--profiles db --profiles unlabeled`).

### Secret Templating

Define secrets with generation settings and template resolution:

```yaml
x-dargstack:
  secrets:
    postgres-password:
      type: random_string
      length: 32
      special_characters: true
    jwt-signing-key.secret:
      type: private_key
      key_type: ed25519
    external-api-token:
      type: third_party
      hint: "Get yours at https://example.com/settings/tokens"
    dev-only-secret:
      type: insecure_default
      insecure_default: "CHANGE_ME"
    api-db_url:
      type: template
      template: "postgresql://postgres:{{secret:postgres-password}}@postgres:5432/app"
```

`type` controls secret behavior. Supported values: `random_string`, `wordlist_word`, `private_key`, `third_party`, `insecure_default`, `template`. If omitted, the type is inferred from the fields provided:

- `private_key` if `key_type` or `key_size` is set
- `third_party` if `third_party` is set
- `template` if `template` is set
- `insecure_default` if `insecure_default` is set
- `random_string` if `length` or `special_characters` is set

**`random_string` properties:**

- `length` — Random string length (default: `32`)
- `special_characters` — Include special characters (default: `true`; set `false` to opt out)

**`private_key` properties:**

- `key_type` — Key algorithm: `ed25519` (default), `rsa`, `ecdsa`
- `key_size` — Key size: RSA default `2048`; ECDSA `256` (P-256), `384` (P-384), `521` (P-521)

**`third_party` properties:**

- `hint` — Human-readable hint for expected value (shown when the secret is unset)

**`insecure_default` properties:**

- `insecure_default` — Default value used for the secret

**`template` properties:**

- `template` — Template string, supporting the following tokens:

  - `{{secret:<name>}}` (or legacy `{{<name>}}`) — Reference another secret
  - `{{random_string}}`, `{{random_string:<length>}}`, `{{random_string:<length>:<special>}}` — Inline random generation
  - `{{wordlist_word}}` — Inline word generation
  - `{{private_key}}` — Inline private key generation

### Environment Files

`.env` files use `KEY=VALUE` format. During deploy, missing values are prompted. Production blocks on missing values.

## Commands

| Command                                              | Description                            |
| ---------------------------------------------------- | -------------------------------------- |
| [dargstack audit](docs/dargstack_audit.md)           | View deployment audit log              |
| [dargstack build](docs/dargstack_build.md)           | Build development Dockerfiles          |
| [dargstack certify](docs/dargstack_certify.md)       | Generate TLS certificates              |
| [dargstack clone](docs/dargstack_clone.md)           | Clone an existing dargstack project    |
| [dargstack deploy](docs/dargstack_deploy.md)         | Deploy the stack                       |
| [dargstack document](docs/dargstack_document.md)     | Generate the stack documentation       |
| [dargstack initialize](docs/dargstack_initialize.md) | Bootstrap a new dargstack project      |
| [dargstack remove](docs/dargstack_remove.md)         | Remove the deployed stack              |
| [dargstack secret](docs/dargstack_secret.md)         | Manage stack secrets                   |
| [dargstack update](docs/dargstack_update.md)         | Update dargstack to the latest version |
| [dargstack validate](docs/dargstack_validate.md)     | Validate stack resources               |

See [docs/dargstack.md](docs/dargstack.md) for global flags and detailed command documentation.

---

Licensed under [GPLv3](LICENSE).
