## dargstack secret status

Show secret status

### Synopsis

Show which secrets are set, missing, or hold placeholder values.

Displays the status of each secret:
  set        - has a real value on disk
  placeholder - holds a third-party placeholder value
  missing    - no file exists on disk

```
dargstack secret status [flags]
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -e, --environment string     environment to operate on: development|production (default "development")
  -f, --format string          output format for compatible commands: table|json (default "table")
  -n, --no-interaction         disable interactive prompts
  -o, --offline                skip fetching remote resources
  -p, --profiles strings       activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack secret](dargstack_secret.md)	 - Manage stack secrets

