## dargstack secret

Inspect stack secrets

### Synopsis

Inspect stack secrets.

Without flags, lists all secret names and their file paths.
Use --show to include values (with clipboard support if available).
Use --public-key to derive and display the public key for private_key type secrets.

```
dargstack secret [flags]
```

### Options

```
  -h, --help         help for secret
  -p, --production   use production compose
  -k, --public-key   show public keys for private_key type secrets
  -s, --show         show secret values
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

