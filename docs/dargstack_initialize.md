## dargstack initialize

Initialize a new dargstack project

### Synopsis

Initialize a new dargstack project.

Creates a project directory structure with:
- `dargstack.yaml` config file with all options (commented with defaults)
- `src/development` and `src/production` service directories
- `artifacts` directory for generated outputs (docs, certificates, audit logs)

Optionally clone an existing dargstack project from a Git URL instead.

Without arguments, init prompts you for a project name.
With an argument, uses it as the project name or Git URL directly.

Use `--configuration-only` to print a full config template to stdout without creating a project.

```
dargstack initialize [name-or-url] [flags]
```

### Options

```
      --configuration-only   print config template to stdout without creating a project
  -h, --help                 help for initialize
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

* [dargstack](dargstack.md)	 - Docker stack helper CLI

