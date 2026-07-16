## dargstack clone

Clone an existing dargstack project

### Synopsis

Clone an existing dargstack project from a Git URL.

Supports https://, git@, git://, and ssh:// URLs.
Without arguments, prompts for a Git URL.
By default, clones into a subdirectory of the current directory named after the repository.

Use --target to specify a different directory for the clone.

```
dargstack clone [url] [flags]
```

### Options

```
  -h, --help            help for clone
      --target string   target directory for the clone (default: inferred from URL)
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
  -p, --profiles strings       activate one or more compose profiles (or set COMPOSE_PROFILES env var); unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

