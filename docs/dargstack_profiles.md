## dargstack profiles

List discovered deploy profiles

### Synopsis

List profiles discovered from compose service definitions.

Profiles are defined via the dargstack.profiles label on services.
Use --env production to list profiles from the production compose stack.

```
dargstack profiles [flags]
```

### Options

```
  -h, --help   help for profiles
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

