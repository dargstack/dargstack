## dargstack inspect

Inspect deployed compose snapshots

### Synopsis

Inspect the final composed YAML that was deployed.

Without arguments, shows the latest deployment.

```
dargstack inspect [timestamp] [flags]
```

### Options

```
      --difference           show diff between current and last deployed
      --environment string   environment to inspect (development or production) (default "development")
  -h, --help                 help for inspect
      --list                 list all past deployments
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -f, --format string          output format for compatible commands: table|json (default "table")
  -n, --no-interaction         disable interactive prompts
      --offline                skip fetching remote resources
      --profiles strings       activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

