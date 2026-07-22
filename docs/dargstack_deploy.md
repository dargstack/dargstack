## dargstack deploy

Deploy the stack

### Synopsis

Deploy services to a Docker Swarm stack.

By default, deploys to the development environment. This includes:
- Auto-building images for services with `dargstack.development.build` labels (controlled by `behavior.build.mode`)
- Generating TLS certificates for local development
- Setting up secrets interactively or with defaults
- Validating all stack resources

Use `--environment production` to deploy to production, which:
- Checks out the latest git tag on `environment.production.branch` (or the tag from `--tag`/`environment.production.tag`) in the stack directory before building the compose files
- Blocks deployment if there are uncommitted changes to tracked files in the stack directory
- Requires all environment variables and secrets to be set
- Blocks deployment if default insecure secrets are present
- Includes production-only services

```
dargstack deploy [flags]
```

### Options

```
  -a, --all          deploy the full stack ignoring --profiles and --services filters
      --force        remove the running stack before deploying
  -h, --help         help for deploy
  -t, --tag string   deploy a specific git tag (production only)
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -e, --environment string     environment to operate on: development|production (default "development")
  -f, --format string          output format for compatible commands: table|json (default "table")
  -l, --log-level string       log level: error, warn, info, debug (default "info")
  -n, --no-interaction         disable interactive prompts
  -o, --offline                skip fetching remote resources
      --platform string        target platform for compose overrides (default: auto-detect)
  -p, --profiles strings       activate one or more compose profiles (or set COMPOSE_PROFILES env var); unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

