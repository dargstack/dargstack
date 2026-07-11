## dargstack secret

Manage stack secrets

### Synopsis

Manage stack secrets.

List, inspect, generate, and check the status of secrets defined in your stack.

Use 'dargstack secret generate' to create secrets from x-dargstack.secrets templates.
Use 'dargstack secret show' to view secret values (with clipboard support if available).
Use 'dargstack secret show --type key' to derive public keys from private_key type secrets.
Use 'dargstack secret status' to check which secrets are set, missing, or hold placeholders.

### Options

```
  -h, --help   help for secret
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -f, --format string          output format for compatible commands: table|json (default "table")
  -n, --no-interaction         disable interactive prompts
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI
* [dargstack secret generate](dargstack_secret_generate.md)	 - Generate secrets from x-dargstack.secrets templates
* [dargstack secret list](dargstack_secret_list.md)	 - List secret names and file paths
* [dargstack secret show](dargstack_secret_show.md)	 - Show secret values
* [dargstack secret status](dargstack_secret_status.md)	 - Show secret status

