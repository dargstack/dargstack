## dargstack deploy

Deploy the stack

### Synopsis

Deploy services to a Docker Swarm stack.

By default, deploys to the development environment. This includes:
- Auto-building images for services with dargstack.development.build labels (unless behavior.build.skip is set)
- Generating TLS certificates for local development
- Setting up secrets interactively or with defaults
- Validating all stack resources

use --production to deploy to production, which:
- Requires all environment variables and secrets to be set
- Blocks deployment if default insecure secrets are present
- Pre-pulls images before deployment
- Includes production-only services

Use --profiles to activate specific compose profiles.
Use --services to deploy only selected services.
Use --dry-run to preview all steps without deploying.
Use --list-profiles to print discovered profiles and exit.
Use --list-secrets to print resolved secrets and exit.
Use --secrets-only to run secret setup only without deploying.
Use --tag (production only) to deploy a specific git tag.

```
dargstack deploy [flags]
```

### Options

```
      --dry-run            trace all steps without deploying
  -h, --help               help for deploy
      --list-profiles      list discovered deploy profiles and exit
      --list-secrets       list resolved secrets and exit
  -p, --production         deploy in production mode
      --profiles strings   activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
      --secrets-only       run secret setup only without deploying
      --services strings   deploy only these services (comma-separated)
      --tag string         deploy a specific git tag (production only)
```

### Options inherited from parent commands

```
      --config string    path to stack directory (default: auto-detect)
      --format string    output format for compatible commands: table|json (default "table")
      --no-interaction   disable interactive prompts
      --verbose          verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

