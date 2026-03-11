## dargstack remove

Remove the deployed stack

### Synopsis

Remove the deployed stack.

Removes all services, networks, and secrets from the Docker Swarm stack.
Use --profiles or --services to remove only a subset of services. Without
those flags the full stack is removed. Use --production to build the compose
from production sources when resolving which services belong to a profile.
Optionally (with --volumes) removes all stack volumes, clearing persistent data.

```
dargstack remove [flags]
```

### Options

```
  -h, --help               help for remove
      --production         remove in production mode
      --profiles strings   remove only services in the given compose profiles
      --services strings   remove only the specified services
      --volumes            also remove stack volumes
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

