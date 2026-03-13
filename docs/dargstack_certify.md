## dargstack certify

Generate TLS certificates

### Synopsis

Generate TLS certificates for development.

Creates self-signed certificates for localhost and all service subdomains.
Certificates are stored in artifacts/certificates and must be trusted in your browser or client.

```
dargstack certify [flags]
```

### Options

```
  -h, --help   help for certify
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

