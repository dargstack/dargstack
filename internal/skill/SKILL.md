---
name: dargstack
description: Understand dargstack project structure, spruce operators, secret templating, label conventions, and deploy workflow for AI agents working in dargstack-managed Docker Swarm projects.
metadata:
  dargstack_version: "4.21.0"
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
│   │   │   ├── compose.yaml        # optional shared base layer (merged into all services)
│   │   │   ├── <service>/
│   │   │   │   ├── compose.yaml    # full Docker Compose document
│   │   │   │   ├── config.yaml     # file-based config volume
│   │   │   │   └── *.secret        # secret files (compose file: convention)
│   │   │   └── .env.template       # env keys (tracked) -> copied to .env (gitignored)
│   │   └── production/             # production overrides (differences only)
│   │       ├── compose.yaml        # optional shared base layer
│   │       ├── <service>/
│   │       │   ├── compose.yaml    # spruce deep-merge override
│   │       │   └── config.yaml     # replaces development config
│   │       └── .env.template       # extends development env vars (tracked)
│   └── artifacts/                  # generated outputs
│       ├── audit-log/              # deployment snapshots (gitignored)
│       ├── certificates/           # TLS certificates (gitignored)
│       ├── secrets/                # Docker secrets (gitignored)
│       ├── configs/                # derived configs, e.g. public keys (gitignored)
│       └── docs/                   # generated documentation
├── <service>/                       # service source code (sibling to stack/)
│   ├── Dockerfile
│   └── ...
└── README.md
```

**Key rules:**
- `stack/dargstack.yaml` is the project configuration file
- Each service gets its own directory under `src/<environment>/`; an optional root `src/<environment>/compose.yaml` is a shared base layer merged first (lowest precedence), then per-service files in name order
- Production files contain only **differences** from development: they are deep-merged on top
- All `secrets.<n>.file:` and derived `configs.<n>.file:` are rewritten to `artifacts/{secrets,configs}/<n>` (gitignored). The real values live there, not in the service dir
- In path values, `~~` expands to the stack root and `~~~` to its parent (e.g. `~~~api` for a sibling service dir)
- Service source code lives as a sibling to `stack/`, not inside it

## Configuration: dargstack.yaml

```yaml
metadata:
  compatibility: ">=4.0.0 <5.0.0"   # required semver range; a CLI outside it offers a self-update
  name: my-stack                    # defaults to the parent dir of stack/
  source: { name: my-repo, url: https://github.com/org/repo }  # link in generated docs
  external_services: { status: { description: "..." } }
runtime:
  sudo: auto                 # auto | always | never
  build: { mode: always }    # always | missing (skip build if image exists)
  deploy: { volumes: { prompt: true } }  # prompt to drop volumes on first deploy
  skill: { install: auto }   # auto | once | off
environment:
  development: { domain: app.localhost, certificate: { include: [], exclude: [] } }
  production:  { branch: main, domain: app.localhost, tag: v1.0.0 }  # tag pins; unset = latest
```

`.env.template` (tracked) is copied to `.env` (gitignored) on first deploy; missing values are prompted, and production blocks on missing values.

## Spruce Operators

Service files are deep-merged using [spruce](https://github.com/geofffranks/spruce).
Production `compose.yaml` files use these operators:

| Operator | Effect |
|----------|--------|
| `(( prune ))` | Remove this key entirely from the merged result |
| `(( append ))` | Append to a list instead of replacing it |
| `(( merge ))` | Deep merge maps recursively |
| `(( omit ))` | Skip this key during merge |
| `(( keep ))` | Keep the original value, ignore the override |
| `(( ref <path> ))` | Reference a value from another part of the document |

**Example: production override:**

```yaml
services:
  api:
    image: ghcr.io/org/api:v1.0.0    # overwrite: pin image tag
    ports: (( prune ))                # remove direct port binding
    deploy:
      labels:
        - (( append ))                # keep dev labels, add new ones
        - "traefik.enable=true"
      replicas: 3                     # overwrite: scale up
```

**Example: development-only lines:**

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
    dashboard-password:
      type: random_string
      length: 24
    dashboard-auth:
      type: basic_auth
      username: admin
      source: dashboard-password
```

**Secret types:**
- `random_string`: auto-generated random string (length default 32; `special_characters` default true)
- `wordlist_word`: auto-generated word (BIP39 wordlist)
- `private_key`: auto-generated key (ed25519 default, rsa ≥2048, ecdsa P-256/384/521)
- `third_party`: requires manual value; `hint` guides the user
- `insecure_default`: uses the provided default value
- `basic_auth`: a bcrypt htpasswd line (`username:hash`) of the plaintext in the `source` secret (≤72 bytes); `username` may reference a secret; the hash is built once and not refreshed
- `template`: composed from tokens: `{{secret:name}}` (or legacy `{{name}}`), `{{random_string[:len[:special]]}}`, `{{wordlist_word}}`, `{{private_key}}`

`type` is optional; when omitted it is inferred from the fields (e.g. `key_type`/`key_size`→`private_key`, `username`→`basic_auth`, `source`→`public_key`, `length`→`random_string`).

Compose declares `secrets.<name>.file:` (conventionally `./<name>.secret` in the service dir); dargstack rewrites every such path to `artifacts/secrets/<name>` (gitignored, mode 0600) and generates/reads the real value there.

## Config Derivation

A public key is not a secret.
When a stack needs the public half of a `private_key` secret (e.g. for a peer to verify signatures), derive it as a plain Docker config via `x-dargstack.configs` instead of storing it as another secret:

```yaml
x-dargstack:
  secrets:
    signing-key:
      type: private_key
      key_type: ed25519
  configs:
    signing-key-pub:
      type: public_key
      source: signing-key

secrets:
  signing-key:
    file: ./secrets/signing-key.secret
configs:
  signing-key-pub:
    file: ./secrets/signing-key-pub.pub

services:
  api:
    secrets: [signing-key]
    configs: [signing-key-pub]
```

`dargstack secret generate` (and deploy) derives the value and writes it to `artifacts/configs/<name>` (gitignored, world-readable); the compose `configs.<name>.file:` path is rewritten to point there, exactly as for secrets.
`source` must name a `private_key` secret; `public_key` is currently the only supported config type.

## Config Content Hash Suffixes

Optional convention around Docker's no-content-update rule (a content change with the same config name fails `docker stack deploy`): name the config `<base>-<hash>`, first 8 hex of the SHA-256 of its file content.
Changed content gets a new name; the deploy creates the new config object and dargstack prunes the old one afterwards.
After editing a config's content, run `dargstack config rotate <name>` (with or without the current suffix); it renames the config in the source compose files, never the content file.
Only suffixed names are validated and pruned (opt-in per config); derived configs get the suffix at deploy time and keep the bare name in the repo.
Per-environment content differences need per-environment config names; rotate warns on drift.

## Label Conventions

Special labels in `deploy.labels` control dargstack behavior:

| Label | Purpose |
|-------|---------|
| `dargstack.development.build` | Build context path (relative to `stack/src/development/<service>/`); takes precedence over the git labels |
| `dargstack.development.git.ssh` | SSH git URL to clone before building (auto-sets build context to the clone) |
| `dargstack.development.git.https` | HTTPS git URL fallback for cloning |
| `dargstack.profiles` | Profile membership (comma-separated) |

Git-cloned repositories go to a sibling directory of `stack/`, named after the repository.

## Profiles

Services can belong to profiles via `dargstack.profiles` label.
Deploy with `--profiles <name>`.

- Unlabeled services are always deployed unless any service declares `default`
- If `default` is declared, only `default`-labeled services deploy by default
- Explicit `--profiles` only deploys matching services; use `--profiles unlabeled` to include unlabeled ones
- A service belongs to multiple profiles via one comma-separated label value, e.g. `dargstack.profiles=default,backend`

## Commands

| Command | Purpose |
|---------|---------|
| `dargstack deploy` | merge, generate secrets/certs, clone+build dev images, deploy to Swarm |
| `dargstack build [svc...]` | build dev images, tagged `<stack>/<svc>:development` |
| `dargstack validate` | validate compose + resources (including config content hash suffixes) |
| `dargstack certify` (=`tls`) | generate self-signed dev TLS certs in `artifacts/certificates` |
| `dargstack secret show\|generate\|status` | show values, generate from templates, report set/missing/placeholder |
| `dargstack config rotate <name>` | recompute a config's content hash suffix after its content changed |
| `dargstack remove` (=`rm`) | remove the stack; `--volumes` also drops volumes; `--profiles`/`--services` for a subset |
| `dargstack audit [ts]` | show the last composed YAML deployed; `--list`, `--difference` |
| `dargstack document` (=`docs`) | generate `artifacts/docs/README.md` from compose + comments |
| `dargstack profiles` | list discovered profiles |
| `dargstack initialize` (=`init`) | bootstrap a project; `--configuration-only`, `--target` |
| `dargstack clone [url]` | clone an existing project; `--target` |
| `dargstack update` | self-update the binary |
| `dargstack schema` | print/save the `dargstack.yaml` JSON Schema (`--save`) |
| `dargstack skill install\|update\|uninstall\|status` | manage this skill (`--project` for project-local) |

`inspect` is a deprecated alias of `audit`.

**Global flags:** `-e/--environment` (development|production; or `DARGSTACK_ENVIRONMENT`), `-d/--dry-run`, `--platform`, `-p/--profiles` (or `COMPOSE_PROFILES`; `--profiles unlabeled` includes unlabeled services), `-s/--services`, `-c/--configuration`, `-f/--format` (table|json), `-n/--no-interaction`, `-o/--offline`, `-l/--log-level`.
**Deploy flags:** `-a/--all` (ignore profile/service filters), `--force` (remove stack first), `-t/--tag` and `--major` (production).

**Production deploy** (`-e production`) additionally:
- checks out the latest git tag on `environment.production.branch` (or `-t`/`environment.production.tag`) in the stack dir; blocks on uncommitted tracked changes
- rewrites `file:` secrets to `external: true`, strips `dargstack.development.*` labels, and remaps dev bind-mount paths to mirrored prod paths
- requires all env vars and secrets set; blocks on `insecure_default`/third-party placeholders
- includes production-only services; blocks a major version bump unless `--major`

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

Supported platforms: `darwin`, `linux`, `windows`.
Active platform is auto-detected or overridden with `--platform`.

## Recursive Skill Improvement

If this skill does not provide enough information to complete a task, and you resort to source code analysis, documentation review, or other means to understand dargstack's behavior:

1. **After your investigation**, improve this skill by contributing the newly discovered, relevant information back to the dargstack repository.
2. Base additions on what you found in dargstack's source code, not on assumptions or local project specifics.
3. Focus on generalizable knowledge: CLI flags not documented here, merge behaviors, error conditions, internal conventions, or any mechanism not covered above.
4. Open a pull request against the dargstack repository editing `internal/skill/SKILL.md`: add new sections or extend existing ones.
5. Bump the `dargstack_version` in the frontmatter if the information applies to a newer version.

This way the skill shipped with dargstack improves through use, and all future users benefit.
