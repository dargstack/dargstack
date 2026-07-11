## dargstack secret status

Show secret status

### Synopsis

Show which secrets are set, missing, or hold placeholder values.

Displays the status of each secret:
  set        - has a real value on disk
  placeholder - holds a third-party placeholder value
  missing    - no file exists on disk

```
dargstack secret status [flags]
```

### Options

```
  -h, --help         help for status
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

