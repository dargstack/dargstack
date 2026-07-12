## dargstack remove

Remove the deployed stack

### Synopsis

Remove the deployed stack.

Removes all services, networks, and secrets from the Docker Swarm stack.
Use `--profiles` or `--services` to remove only a subset of services. Without
those flags the full stack is removed. Use `--environment production` to build the compose
from production sources when resolving which services belong to a profile.
Optionally (with `--volumes`) removes all stack volumes, clearing persistent data.

```
dargstack remove [flags]
```

### Options

```
  -h, --help      help for remove
      --volumes   also remove stack volumes
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -e, --environment string     environment to operate on: development|production (default "development")
  -f, --format string          output format for compatible commands: table|json (default "table")
  -n, --no-interaction         disable interactive prompts
      --offline                skip fetching remote resources
      --profiles strings       activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

