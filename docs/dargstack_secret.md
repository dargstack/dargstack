## dargstack secret

Manage stack secrets

### Synopsis

Manage stack secrets.

List, inspect, generate, and check the status of secrets defined in your stack.

Use 'dargstack secret generate' to create secrets from x-dargstack.secrets templates.
Use 'dargstack secret show' to view secret values (with clipboard support if available).
Use 'dargstack secret show --type key' to derive public keys from private_key type secrets.
Use 'dargstack secret status' to check which secrets are set, missing, or hold placeholders.

### Options

```
  -h, --help   help for secret
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
* [dargstack secret generate](dargstack_secret_generate.md)	 - Generate secrets from x-dargstack.secrets templates
* [dargstack secret show](dargstack_secret_show.md)	 - Show secret values
* [dargstack secret status](dargstack_secret_status.md)	 - Show secret status

