## dargstack secret show

Show secret values

### Synopsis

Show secret values.

Displays the current values of all secrets. If a clipboard tool is available
(wl-copy, xclip, xsel, pbcopy, clip), offers an interactive picker to copy
individual keys and values.

Use --type key to derive and display public keys for private_key type secrets
instead of showing stored values.

```
dargstack secret show [flags]
```

### Options

```
  -h, --help          help for show
  -p, --production    use production compose
      --type string   output type: value (secret values) or key (derived public keys) (default "value")
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

