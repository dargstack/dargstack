## dargstack document

Generate the stack documentation

### Synopsis

Generate stack documentation.

Creates a README.md in the artifacts directory listing all services
found in compose files, along with YAML comments describing each.
Includes a link to the stack domain and source code repository.

```
dargstack document [flags]
```

### Options

```
  -h, --help   help for document
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

