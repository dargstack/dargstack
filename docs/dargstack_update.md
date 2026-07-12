## dargstack update

Update components

### Synopsis

Update dargstack and related components.

```
dargstack update [flags]
```

### Options

```
  -h, --help   help for update
      --self   update dargstack itself
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -e, --environment string     environment to operate on: development|production (default "development")
  -f, --format string          output format for compatible commands: table|json (default "table")
  -n, --no-interaction         disable interactive prompts
      --offline                skip fetching remote resources
      --profiles strings       activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

