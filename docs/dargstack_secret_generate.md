## dargstack secret generate

Generate secrets from x-dargstack.secrets templates

### Synopsis

Generate secrets from x-dargstack.secrets templates.

Reads secret templates from the compose file and generates values for any
missing secrets. Auto-generatable types (random_string, wordlist_word,
private_key, insecure_default, template) are created automatically.
Third-party secrets require manual values.

In production mode (--environment production), validates that third-party secrets do not
hold placeholder values and blocks if they do.

In non-interactive mode (--no-interaction), auto-generates what it can and
warns about secrets that still need values.

```
dargstack secret generate [flags]
```

### Options

```
  -h, --help   help for generate
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
  -p, --profiles strings       activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack secret](dargstack_secret.md)	 - Manage stack secrets

