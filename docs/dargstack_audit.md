## dargstack audit

View deployment audit log

### Synopsis

View the final composed YAML that was deployed.

Without arguments, shows the latest deployment.

```
dargstack audit [timestamp] [flags]
```

### Options

```
      --difference           show diff between current and last deployed
      --environment string   environment to audit (development or production) (default "development")
  -h, --help                 help for audit
      --list                 list all past deployments
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -f, --format string          output format for compatible commands: table|json (default "table")
  -l, --log-level string       log level: error, warn, info, debug (default "info")
  -n, --no-interaction         disable interactive prompts
  -o, --offline                skip fetching remote resources
  -p, --profiles strings       activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI