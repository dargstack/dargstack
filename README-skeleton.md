# DargStack Skeleton

DargStack template's directory and file tree documentation.


## Project Structure

1. **[Main Project](#main-project)**
1. **[Stack Project](#stack-project)**
    1. **[development/](#development)**
        1. **[[resource-folders]](#resource-folders)**
        1. **[secrets/](#secrets)**
        1. **[stack.yml](#stackyml)**
    1. **[production/](#production)**
        1. **[[resource-folders]](#resource-folders)**
        1. **[production.sed](#productionsed)**
        1. **[production.yml](#productionyml)**
        1. **[stack.env.template](#stackenvtemplate)**
        1. **stack.yml¹**

¹ Generated automatically and ignored by the VCS.


## Main Project

The main project folder must contain a `Dockerfile` in addition to the main service's source.

This `Dockerfile` should be structured so that the `FROM` instruction is tagged with `AS development` so that the DargStack script's `build` command builds the main project's development version (unless the `-p, --production` flag is passed).


## Stack Project

The stack project is separated into the fundamental development configuration and the production derivation configuration.
The full production configuration is created from the development and the production derivation configuration by the DargStack script.

---

### Resource Folders

If a stack requires resources that match one of the following folder's descriptions, the *should* be placed in the corresponding folder.
There is no need for all folders to exist.

Every service *should* have its resource placed in a subfolder of each resource folder.
The subfolder's name *should* be name of the corresponding service as used in the `stack.yml`.

1. **backups/**

    If the stack contains a service that backs up Docker volumes to the filesystem, the target directory *should* be this folder.
    Another backup step *should* be set up to move all backups to a secondary location.

1. **certificates/**

    A DargStack will most likely serve web apps and thus *should* make use of encryption certificates for HTTPS/SSL.
    Those can easily be generated using [FiloSottile/mkcert](https://github.com/FiloSottile/mkcert).

    Real certificates *must* be used for production.
    For example, [Traefik](https://traefik.io/) can fetch those from [Let's Encrypt](https://letsencrypt.org/) automatically.

1. **configurations/**

    This directory stores configuration files for your services.
    Configuration files, which include secrets, *must* be treated as secrets!

1. **data/**

    This directory stores data for your services.
    An example are SQL files, containing database schemes, which are imported by your database on its first start.

---

### development/

The development directory contains the fundamental stack configuration.
Similar to "mobile first", the DargStack template works "development first".
The modifications needed for production are incremental to the configuration defined for development.


#### secrets/

Confidential data, like usernames and passwords, needs to be accessible as [Docker secrets](https://docs.docker.com/engine/swarm/secrets/) to keep it out of the source code or environment configuration.
Configuration files, which include secrets, *must* be treated as secrets too!
The development configuration *can* use the contents of files as source for the secrets' values.

Secret files *must not* be used for production though.
Use the `docker secret create` command instead.
PowerShell on Windows may add a carriage return at the end of strings piped to the command.
A workaround can be that you create secrets from temporary files that do not contain a trailing newline.
hey can be written using:

```PowerShell
"secret data" | Out-File secret_name -NoNewline
```

When done, shred those files!


#### stack.yml

This file defines the full stack, containing all services you deem necessary for development.
Simply [deploy the development stack](https://docs.docker.com/engine/reference/commandline/stack_deploy/) using `dargstack deploy`!
You can use the variable `${STACK_DOMAIN}` within this file, which sets the TLD to `localhost` automatically.


### production/

This directory contains resources that are needed for production as well as files, which define incremental changes to the development configuration that shall result in the full production configuration.


#### production.sed

Replacement patterns for [sed](https://linux.die.net/man/1/sed) can be used to tweak the production stack derivation's output.
When specified in `production.sed` as one sed pattern per line, the helper script will apply them one by one.


#### production.yml

This file is merged with the development stack configuration file by [spruce](https://github.com/geofffranks/spruce).
Setting the same field with a different value here will override the development value, which is in particular useful for specifying a version tag for services that previously had the `:dev` tag.
For more advanced modifications, like list manipulation, refer to [spruce's documentation](https://github.com/geofffranks/spruce/tree/master/doc).


#### stack.env.template

The `stack.env.template` file defines environment variables that are used in the the production `stack.yml`.
The DargStack script clones the `stack.env.template` to a sibling `stack.env` file into which the environment variables' values *must* be filled.
