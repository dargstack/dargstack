## dargstack skill

Manage the dargstack AI agent skill

### Synopsis

Manage the dargstack AI agent skill.

The skill teaches AI agents about dargstack conventions: project structure,
spruce operators, secret templating, label semantics, and deploy workflow.

Install the skill globally (~/.agents/skills/dargstack/) or project-local
(.agents/skills/dargstack/) with --project.

### Options

```
  -h, --help      help for skill
      --project   use project-local .agents/skills/ instead of global ~/.agents/skills/
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
* [dargstack skill install](dargstack_skill_install.md)	 - Install the dargstack agent skill
* [dargstack skill status](dargstack_skill_status.md)	 - Show the status of the installed dargstack agent skill
* [dargstack skill uninstall](dargstack_skill_uninstall.md)	 - Uninstall the dargstack agent skill
* [dargstack skill update](dargstack_skill_update.md)	 - Update the dargstack agent skill to the current bundled version

