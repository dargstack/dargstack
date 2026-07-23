## dargstack skill install

Install the dargstack agent skill

```
dargstack skill install [flags]
```

### Options

```
  -h, --help   help for install
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
      --project                use project-local .agents/skills/ instead of global ~/.agents/skills/
  -s, --services strings       filter to specific services
```

### SEE ALSO

* [dargstack skill](dargstack_skill.md)	 - Manage the dargstack AI agent skill

