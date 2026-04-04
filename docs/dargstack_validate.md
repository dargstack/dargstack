## dargstack validate

Validate stack resources

### Synopsis

Validate stack resources and configuration.

Checks:
- All secrets files referenced in compose definitions exist
- All Dockerfile contexts for services with dargstack.development.build labels are present
- TLS certificates directory exists for development

```
dargstack validate [flags]
```

### Options

```
  -h, --help               help for validate
  -p, --production         validate in production mode
      --profiles strings   activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings   validate specific services only
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

