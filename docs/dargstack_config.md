## dargstack config

Manage stack configs

### Synopsis

Manage stack configs.

Docker only allows label updates on existing config objects: a content update with the same
name fails the deploy. dargstack works around this with an optional content hash suffix
convention: a config named `<base>-<hash>` carries the first 8 hex characters of the
SHA-256 of its file content. When the content changes, the name changes with it, so the
deploy creates a new config object instead of failing, and the old one is pruned afterwards.

The convention is opt-in per config: `dargstack validate` only polices names that
already carry a suffix, and `dargstack deploy` only prunes suffix-named orphans.

Use 'dargstack config rotate <name>' to recompute the suffix after the config content changed.

### Options

```
  -h, --help   help for config
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
* [dargstack config rotate](dargstack_config_rotate.md)	 - Recompute a config's content hash suffix

