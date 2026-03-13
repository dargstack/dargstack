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
  -d, --diff         show diff between current and last deployed
  -e, --env string   environment to inspect (development or production) (default "development")
  -h, --help         help for inspect
  -l, --list         list all past deployments
```

### Options inherited from parent commands

```
  -c, --config string    path to stack directory (default: auto-detect)
  -f, --format string    output format for compatible commands: table|json (default "table")
  -n, --no-interaction   disable interactive prompts
  -v, --verbose          verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

