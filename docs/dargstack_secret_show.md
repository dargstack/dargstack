## dargstack secret show

Show secret values

### Synopsis

Show secret values.

Displays the current values of all secrets. If a clipboard tool is available
(wl-copy, xclip, xsel, pbcopy, clip), offers an interactive picker to copy
individual keys and values.

If a secret name is provided, only that secret is shown.

Use --type key to derive and display public keys for private_key type secrets
instead of showing stored values.

```
dargstack secret show [name] [flags]
```

### Options

```
  -h, --help          help for show
      --type string   output type: value (secret values) or key (derived public keys) (default "value")
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
```

### SEE ALSO

* [dargstack secret](dargstack_secret.md)	 - Manage stack secrets

