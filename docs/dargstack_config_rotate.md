## dargstack config rotate

Recompute a config's content hash suffix

### Synopsis

Recompute a config's content hash suffix after its content changed.

Reads the config's file content, computes the first 8 hex characters of its SHA-256, and
renames the config to `<base>-<hash>` in every source compose file: the top-level
configs key and all service and shared-definition references. The name may be given with
or without the current suffix.

If the config's file is a dargstack artifact under `artifacts/configs/` (a derived
config), the artifact file is renamed to match the new name.

```
dargstack config rotate <name> [flags]
```

### Options

```
  -h, --help   help for rotate
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

* [dargstack config](dargstack_config.md)	 - Manage stack configs

