## dargstack schema

Print the dargstack.yaml JSON Schema

### Synopsis

Print the JSON Schema for dargstack.yaml to stdout.

The schema can be used for IDE autocomplete and validation.

For IDE integration, save the schema locally and configure your editor:

  dargstack schema --save

Then point your editor's YAML language server to the saved file, or add
a $schema field to your dargstack.yaml:

  $schema: "file:///home/user/.local/share/schemas/dargstack.json"

```
dargstack schema [flags]
```

### Options

```
  -h, --help                                                    help for schema
      --save string[="~/.local/share/schemas/dargstack.json"]   Save schema to a file for IDE integration
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
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

