## dargstack remove

Remove the deployed stack

### Synopsis

Remove the deployed stack.

Removes all services, networks, and secrets from the Docker Swarm stack.
Optionally (with --volumes) removes all stack volumes, clearing persistent data.

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
      --config string    path to stack directory (default: auto-detect)
      --format string    output format for compatible commands: table|json (default "table")
      --no-interaction   disable interactive prompts
      --verbose          verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

