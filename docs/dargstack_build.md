## dargstack build

Build development Dockerfiles

### Synopsis

Build service Docker images.
Builds Dockerfiles for services with a `dargstack.development.build` or `dargstack.development.git.ssh`/`dargstack.development.git.https` label in their compose definition.

The `dargstack.development.build` label takes precedence over `dargstack.development.git.ssh`/`dargstack.development.git.https`.
Each service must have a Dockerfile in the build context directory.

Without arguments, lists available services and prompts you to select which to build.
With service names as arguments, builds only those services.

Images are tagged as `<stack>/<service>:development`.

```
dargstack build [service...] [flags]
```

### Options

```
  -h, --help   help for build
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
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

