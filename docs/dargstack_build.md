## dargstack build

Build development Dockerfiles

### Synopsis

Build service Docker images.

Builds Dockerfiles for services with a dargstack.development.build label in their compose definition.
Each service must have a Dockerfile in the build context directory.

Without arguments, lists available services and prompts you to select which to build.
With service names as arguments, builds only those services.

Images are tagged as <stack>/<service>:development.

```
dargstack build [service...] [flags]
```

### Options

```
  -h, --help   help for build
```

### Options inherited from parent commands

```
      --config string    path to stack directory (default: auto-detect)
      --format string    output format for compatible commands: table|json (default "table")
      --no-interaction   disable interactive prompts
      --verbose          verbose output
```

### SEE ALSO

* [dargstack](dargstack.md)	 - Docker stack helper CLI

