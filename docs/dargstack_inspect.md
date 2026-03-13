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
      --diff         show diff between current and last deployed
      --env string   environment to inspect (development or production) (default "development")
  -h, --help         help for inspect
      --list         list all past deployments
```

### Options inherited from parent commands

```
      --config string    path to stack directory (default: auto-detect)
      --format string    output format for compatible commands: table|json (default "table")
      --no-interaction   disable interactive prompts
      --verbose          verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

