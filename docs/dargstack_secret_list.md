## dargstack secret list

List secret names and file paths

### Synopsis

List all secret names and their file paths.

Without flags, lists all secret names and their file paths in the current stack.

```
dargstack secret list [flags]
```

### Options

```
  -h, --help         help for list
  -p, --production   use production compose
```

### Options inherited from parent commands

```
  -c, --configuration string   path to stack directory (default: auto-detect)
  -f, --format string          output format for compatible commands: table|json (default "table")
  -n, --no-interaction         disable interactive prompts
  -v, --verbose                verbose output
```

### SEE ALSO

* [dargstack secret](dargstack_secret.md)	 - Manage stack secrets

