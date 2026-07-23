---
name: dargstack
description: Understand dargstack project structure, spruce operators, secret templating, label conventions, and deploy workflow for AI agents working in dargstack-managed Docker Swarm projects.
metadata:
  dargstack_version: "4.0.0"
---

# Dargstack Agent Skill

## Project Structure

A dargstack project has this layout:

```
project/
├── stack/                          # dargstack project root
│   ├── dargstack.yaml              # project configuration
│   ├── src/
│   │   ├── development/            # base service definitions
│   │   │   ├── <service>/
│   │   │   │   ├── compose.yaml    # full Docker Compose document
│   │   │   │   ├── config.yaml     # file-based config volume
│   │   │   │   └── *.secret        # secret files
│   │   │   └── .env                # environment variables
│   │   └── production/             # production overrides (differences only)
│   │       ├── <service>/
│   │       │   ├── compose.yaml    # spruce deep-merge override
│   │       │   └── config.yaml     # replaces development config
│   │       └── .env                # extends development env vars
│   └── artifacts/                  # generated outputs
│       ├── audit-log/              # deployment snapshots (gitignored)
│       ├── certificates/           # TLS certificates (gitignored)
│       ├── secrets/                # Docker secrets (gitignored)
│       └── docs/                   # generated documentation
├── <service>/                       # service source code (sibling to stack/)
│   ├── Dockerfile
│   └── ...
└── README.md
```

**Key rules:**
- `stack/dargstack.yaml` is the project configuration file
- Each service gets its own directory under `src/<environment>/`
- Service directories contain `compose.yaml` plus any config/secret files
- Production files contain only **differences** from development — they are deep-merged on top
- Service source code lives as a sibling to `stack/`, not inside it

## Spruce Operators

Service files are deep-merged using [spruce](https://github.com/geofffranks/spruce). Production `compose.yaml` files use these operators:

| Operator | Effect |
|----------|--------|
| `(( purge ))` | Remove this key entirely from the merged result |
| `(( append ))` | Append to a list instead of replacing it |
| `(( merge ))` | Deep merge maps recursively |
| `(( omit ))` | Skip this key during merge |
| `(( keep ))` | Keep the original value, ignore the override |
| `(( ref <path> ))` | Reference a value from another part of the document |

**Example — production override:**

```yaml
services:
  api:
    image: ghcr.io/org/api:v1.0.0    # overwrite: pin image tag
    ports: (( purge ))                # remove direct port binding
    deploy:
      labels:
        - (( append ))                # keep dev labels, add new ones
        - "traefik.enable=true"
      replicas: 3                     # overwrite: scale up
```

**Example — development-only lines:**

Lines annotated with `# dargstack:dev-only` in development files are stripped before the production merge.

```yaml
services:
  api:
    deploy:
      labels:
        - some.label=for-development # dargstack:dev-only
```

## Secret Templating

Secrets are defined in `x-dargstack.secrets` within `compose.yaml`:

```yaml
x-dargstack:
  secrets:
    api-key:
      type: random_string
      length: 32
      special_characters: false
    db-password:
      type: template
      template: "postgresql://user:{{secret:api-key}}@host:5432/db"
    signing-key:
      type: private_key
      key_type: ed25519
    external-token:
      type: third_party
      hint: "Get at https://example.com/tokens"
    dev-secret:
      type: insecure_default
      insecure_default: "CHANGE_ME"
```

**Secret types:**
- `random_string` — auto-generated random string (configurable length, special characters)
- `wordlist_word` — auto-generated word from a wordlist
- `private_key` — auto-generated private key (ed25519, rsa, ecdsa)
- `third_party` — requires manual value; `hint` guides the user
- `insecure_default` — uses the provided default value
- `template` — composed from tokens like `{{secret:name}}`, `{{random_string}}`, `{{wordlist_word}}`, `{{private_key}}`

Secret files (`.secret` extension) live alongside `compose.yaml` in the service directory.

## Label Conventions

Special labels in `deploy.labels` control dargstack behavior:

| Label | Purpose |
|-------|---------|
| `dargstack.development.build` | Build context path (relative to `stack/src/development/<service>/`) |
| `dargstack.development.git.ssh` | SSH git URL to clone before building |
| `dargstack.development.git.https` | HTTPS git URL fallback for cloning |
| `dargstack.profiles` | Profile membership (comma-separated) |

Git-cloned repositories go to a sibling directory of `stack/`, named after the repository.

## Profiles

Services can belong to profiles via `dargstack.profiles` label. Deploy with `--profiles <name>`.

- Unlabeled services are always deployed unless any service declares `default`
- If `default` is declared, only `default`-labeled services deploy by default
- Explicit `--profiles` only deploys matching services; use `--profiles unlabeled` to include unlabeled ones

## Deploy Workflow

1. `dargstack deploy` — merges compose files, resolves secrets, builds images, deploys to Docker Swarm
2. `dargstack build` — build development images only
3. `dargstack validate` — validate compose files and resources
4. `dargstack secret generate` — generate secrets from templates
5. `dargstack remove` — remove the deployed stack
6. `dargstack audit` — view deployment history

Use `--environment production` for production deployments. Use `--dry-run` to trace without executing.

## Platform Overrides

Platform-specific compose modifications live under `x-dargstack.platform.<os>` in development `compose.yaml`:

```yaml
x-dargstack:
  platform:
    darwin:
      services:
        cadvisor:
          privileged: false
          volumes:
            - (( prune ))
```

Supported platforms: `darwin`, `linux`, `windows`. Active platform is auto-detected or overridden with `--platform`.