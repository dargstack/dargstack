## dargstack certify

Generate TLS certificates

### Synopsis

Generate TLS certificates for development.

Creates self-signed certificates for `localhost` and all service subdomains.
Certificates are stored in `artifacts/certificates` and must be trusted in your browser or client.

```
dargstack certify [flags]
```

### Options

```
  -h, --help   help for certify
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

