## dargstack

Docker stack helper CLI

### Synopsis

dargstack - simplified, approachable Docker Swarm stack management.

### Options

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -d, --dry-run                trace all steps without executing
  -e, --environment string     environment to operate on: development|production (default "development")
  -f, --format string          output format for compatible commands: table|json (default "table")
  -h, --help                   help for dargstack
  -n, --no-interaction         disable interactive prompts
  -o, --offline                skip fetching remote resources
  -p, --profiles strings       activate one or more compose profiles; unlabeled services are included unless a 'default' profile is defined
  -s, --services strings       filter to specific services
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack build](dargstack_build.md)	 - Build development Dockerfiles
* [dargstack certify](dargstack_certify.md)	 - Generate TLS certificates
* [dargstack deploy](dargstack_deploy.md)	 - Deploy the stack
* [dargstack document](dargstack_document.md)	 - Generate the stack documentation
* [dargstack initialize](dargstack_initialize.md)	 - Initialize a new dargstack project
* [dargstack inspect](dargstack_inspect.md)	 - Inspect deployed compose snapshots
* [dargstack profiles](dargstack_profiles.md)	 - List discovered deploy profiles
* [dargstack remove](dargstack_remove.md)	 - Remove the deployed stack
* [dargstack secret](dargstack_secret.md)	 - Manage stack secrets
* [dargstack update](dargstack_update.md)	 - Update components
* [dargstack validate](dargstack_validate.md)	 - Validate stack resources

