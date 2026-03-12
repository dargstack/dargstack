# dargstack

Docker Swarm, made simple. Dev-first deployments, production overlays, built-in audit trail.

dargstack is a **CLI tool and project structure specification** that **reduces Docker Swarm complexity** to a minimal command set. Define your development setup as the base, express production as incremental changes on top.

dargstack does **not** replace `docker stack`. You can interact with `docker stack` on the same stack that you manage with dargstack.

---

The following projects successfully employ dargstack in production:

- [maevsi/stack](https://github.com/maevsi/stack/)
- [dargmuesli/jonas-thelemann_stack](https://github.com/dargmuesli/jonas-thelemann_stack/)
- [flipdot/drinks-touch_stack](https://github.com/flipdot/drinks-touch_stack/)

## Why dargstack?

Deploying the same app to development and production with Docker Swarm usually means maintaining two nearly identical compose files. Change one thing? Manually copy, edit, hope nothing breaks.

dargstack inverts this: define development as the source of truth, then express production as **changes** on top. One deploy command. One audit trail. Done.

| dargstack                                                      | `docker stack`                                                                 |
| -------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| ✅ A single resource and diff specification                    | ❌ Two compose files for dev and prod – risk of configuration drift            |
| ✅ Clear file separation by service                            | ❌ Monolithic compose file – hard to maintain if big                           |
| ✅ Snapshot for every deploy; easy inspect and diff            | ❌ Volatile audit trail – live console tracing only                            |
| ✅ Safer secret management with auto-generation and templating | ❌ Manual secret management – tedious, often insecure defaults                 |
| ✅ Development certificates auto‑generated                     | ❌ No TLS certificates – out of scope, traffic unencrypted                     |
| ✅ Zero downtime service update motivation                     | ❌ Stop-first update order by default – unreliable availability in production |

## Install

### Recommended — From Source

**Prerequisite** – Go installed, see [go.dev: Download and install](https://go.dev/doc/install).

```bash
go install github.com/dargstack/dargstack/v4/cmd/dargstack@latest
```

Package integrity is enforced by the Go module proxy and the module's `go.sum` lockfile.
Pin to a specific version (e.g. `@v4.1.0`) to get a reproducible, auditable install.

### Alternative — From GitHub Releases

> **Security note:** Binary downloads do not include checksum verification in the snippet below.
> Before moving the binary to your PATH, verify the SHA-256 checksum published on the
> [Releases page](https://github.com/dargstack/dargstack/releases), or prefer `go install` above.

```bash
curl -sL https://github.com/dargstack/dargstack/releases/latest/download/dargstack_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/').tar.gz | tar xz
sudo mv dargstack /usr/local/bin/
```

## Quick Start

**Prerequisite** – Docker installed, see [docs.docker.com: Install Docker Engine](https://docs.docker.com/engine/install/).

1. Initialize a new dargstack project:

   ```bash
   dargstack init
   ```

2. Fill in your service configuration according to the [docker.com: Compose file reference](https://docs.docker.com/reference/compose-file).

3. Then deploy:

   ```bash
   cd <project_name>
   dargstack deploy
   ```

Done! 🎉 Your project is live now.

## Project Structure

Suppose you have an `api` service as part of an `example` project. Your project structure would look like this:

```
example/
├── api/                                # The service's source code
│   ├── Dockerfile                      # Dockerfile for the api service
:   :
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
│   │   │   │   ├── configuration.toml  # File-based volume mount by the service
│   │   │   │   ├── key.secret          # Secret used by the service
│   │   │   :   :
│   │   │   └── .env                    # Environment variables
│   │   └── production/
│   │       ├── api/
│   │       │   ├── compose.yaml        # Yaml deep-merge override
│   │       │   ├── configuration.toml  # File-based override
│   │       :   :
│   │       └── .env                    # Key-based override
│   └── dargstack.yaml                  # Project configuration
:
```

### Service Files

Each service file is a full Docker Compose document — files are deep-merged by [spruce](https://github.com/geofffranks/spruce).

```yaml
# src/development/api/compose.yaml
services:
  api:
    image: api:latest
    ports:
      - "3000:3000"
    secrets:
      - api-key.secret
    deploy:
      labels:
        - dargstack.development.build=../../../../api
        - traefik.http.routers.api.rule=Host(`api.${STACK_DOMAIN}`)
        - traefik.http.routers.api.tls=true
        - traefik.http.services.api.loadbalancer.server.port=8080
        - some.label=for-development # dargstack:dev-only

secrets:
  api-key.secret:
    file: ./key.secret

x-dargstack:
  secrets:
    api-key.secret:
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
        order: start-first # Zero-downtime updates
```

### Configuration: dargstack.yaml

```yaml
compatibility: ">=4.0.0 <5.0.0" # required, string (semver range)
name: my-stack # optional, defaults to parent directory name
production:
  branch: main # optional, string
  tag: latest # optional, `latest` | string
  domain: app.localhost # optional, string
sudo: auto # optional, `auto` | `always` | `never`
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
    postgres-password.secret:
      type: random_string
      # length defaults to 32, special_characters defaults to true
    jwt-signing-key.secret:
      type: private_key
    external-api-token.secret:
      type: third_party
      hint: "Get yours at https://example.com/settings/tokens"
    dev-only-secret.secret:
      type: insecure_default
      insecure_default: "CHANGE_ME"
    api-db_url.secret:
      type: template
      template: "postgresql://postgres:{{secret:postgres-password.secret}}@postgres:5432/app"
```

- `type` — Secret behavior. Supported values: `random_string`, `word`, `private_key`, `third_party`, `insecure_default`, `template`
- `hint` — Human-readable hint for expected value (shown for `third_party` secrets when unset)
- `length` — Random string length for `type: random_string` (default: `32`)
- `special_characters` — Include special characters for `type: random_string` (default: `true`; set `false` to opt out)
- `insecure_default` — Default value used for `type: insecure_default`
- `template` — Template string for `type: template`

Template tokens:

- `{{secret:<name>}}` (or legacy `{{<name>}}`) — Reference another secret
- `{{random}}`, `{{random:<length>}}`, `{{random:<length>:<special>}}` — Inline random generation
- `{{word}}` — Inline word generation
- `{{private_key}}` — Inline private key generation

### Environment Files

`.env` files use `KEY=VALUE` format. During deploy, missing values are prompted. Production blocks on missing values.

## Commands

Go to [docs: dargstack](docs/dargstack.md) for detailed command documentation.

| Command                                              | Description                            |
| ---------------------------------------------------- | -------------------------------------- |
| [dargstack build](docs/dargstack_build.md)           | Build development Dockerfiles          |
| [dargstack certify](docs/dargstack_certify.md)       | Generate TLS certificates              |
| [dargstack deploy](docs/dargstack_deploy.md)         | Deploy the stack                       |
| [dargstack document](docs/dargstack_document.md)     | Generate the stack documentation       |
| [dargstack initialize](docs/dargstack_initialize.md) | Initialize a new dargstack project     |
| [dargstack inspect](docs/dargstack_inspect.md)       | Inspect deployed compose snapshots     |
| [dargstack remove](docs/dargstack_remove.md)         | Remove the deployed stack              |
| [dargstack update](docs/dargstack_update.md)         | Update dargstack to the latest version |
| [dargstack validate](docs/dargstack_validate.md)     | Validate stack resources               |
