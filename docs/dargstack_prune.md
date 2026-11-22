## dargstack prune

Remove stopped containers and unused images

### Synopsis

Remove stopped containers and unused images.

Runs the same cleanup that `dargstack deploy` offers after a production deploy, without deploying.
Stopped containers are removed first, so images only they referenced count as unused.
All images not used by a container are removed, not only dangling ones.
Volumes are never touched: use `dargstack remove --volumes` for those.
Asks for confirmation unless `--no-interaction` is set.

```
dargstack prune [flags]
```

### Options

```
  -h, --help   help for prune
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -e, --environment string     environment to operate on: development|production (default from DARGSTACK_ENVIRONMENT if unset) (default "development")
  -f, --format string          output format for compatible commands: table|json (default "table")
  -l, --log-level string       log level: error, warn, info, debug (default "info")
  -n, --no-interaction         disable interactive prompts
  -o, --offline                skip fetching remote resources
      --platform string        target platform for compose overrides (default: auto-detect)
  -p, --profiles strings       activate one or more compose profiles (or set COMPOSE_PROFILES env var); unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

