# DargStack Skeleton

The template' directory and file tree documentation.

Note that `Stack Project/production/secrets/` *should* and `Stack Project/production/stack.yml` *must* not be part of a repository that follows the DargStack template.
They are still mentioned though to explain their roles within the template.


## Table of Contents

1. **[Main Project](#main-project)**
1. **[Stack Project](#stack-project)**
    1. **[development/](#development)**
        1. **[certificates/](#certificates)**
        1. **[secrets/](#development-secrets)**
        1. **[stack.yml](#development-stackyml)**
    1. **[production/](#production)**
        1. **[backup/](#backup)**
        1. **[configurations/](#configurations)**
        1. **[data/](#data)**
        1. **[secrets/](#production-secrets)**
        1. **[production.sed](#productionsed)**
        1. **[production.yml](#productionyml)**
        1. **[stack.env.template](#stackenvtemplate)**
        1. **[stack.yml](#production-stackyml)**

## Main Project

The main project folder should at least contain a `Dockerfile` in addition to the main service's source.


## Stack Project

The stack project is separated into the fundamental development configuration and the deriving production configuration.
An IDE configuration file *may* connect those two directories so that they can be configured together.


### development/

The development directory contains the fundamental stack configuration.
Similar to "mobile first", the DargStack template works "development first".
The modifications needed for production are incremental to the configuration defined for development.


#### certificates/

A DargStack will most likely serve web apps and thus should make use of encryption certifices for HTTPS/SSL.
Those can easily be generated using the [new-certificates.sh](https://gist.github.com/Dargmuesli/538a2c382c009f4620803679c8172c9d) script.
The root certificate of those self-signed certificates needs to be imported in your browser.

For production real certificates must be used.
[Traefik](https://traefik.io/) can fetch those from [Let's Encrypt](https://letsencrypt.org/).


<div id="development-secrets" />

#### secrets/

Confidential data, like usernames and passwords, need to be accessible as [Docker secrets](https://docs.docker.com/engine/swarm/secrets/) to keep them out of the source code.
These files, which contain the passwords' values, need to exist inside the `[project-name]_stack/development/secrets/` directory.


<div id="development-stackyml" />

#### stack.yml

This file defines the complete stack, containing all services you deem necessary for development.
Simply [deploy the development stack](https://docs.docker.com/engine/reference/commandline/stack_deploy/) using `dargstack deploy`!
You can use the variable `${STACK_DOMAIN}` within this file, which is set to the project's name + `.test` automatically.


### production/

This directory contains data, like raw data or environment variables, that is needed for production as well as files, which define incremental changes to the development configuration that shall result in the production configuration.


#### backup/

The output of your backup services, which create regular backups of your data volumes, *can* be placed in this directory temporarily.
This allows you to set up a method like a cron job that moves the dumped backup data to a different device.


#### configurations/

This directory stores configuration files for your services.


#### data/

This directory stores data for your serivces, like SQL dumps of database schemes, which shall be imported to your database on its first start.


<div id="production-secrets" />

#### secrets/

Don't use password files for production as it's done for development. Use the `docker secret create` command instead.

Data like environment variables, which set passwords, or configuration files, which include secret keys belong, should be provided as Docker secrets.
In case the stack contains services which do not support Docker secrets yet, the data *can* be placed in this directory.
In favor of efforts to adapt Docker secret usage this practise is discouraged though.

PowerShell on Windows may add a carriage return at the end of strings piped to the command.
A workaround can be that you create secrets from temporary files that do not contain a trailing newline.
hey can be written using:

```PowerShell
"secret data" | Out-File secret_name -NoNewline
```

When done, shred those files!


#### production.sed

Replacement patterns for [sed](https://linux.die.net/man/1/sed) can be used to tweak the production stack derivation's output.
When specified in `production.sed` as one sed pattern per line, the helper script will apply them one by one.


#### production.yml

This file is merged with the development stack configuration file by [spruce](https://github.com/geofffranks/spruce).
Setting the same field with a different value here will override the development value, which is in particular useful for specifying a version tag for services that previously had the `:dev` tag.
For more advanced modifications, like list manipulation, refer to [spruce's documentation](https://github.com/geofffranks/spruce/tree/master/doc).


#### stack.env.template

You *may* need to clone a `[project-name]_stack/production/stack.env.template` file to a sibling `stack.env` file and specify the included environment variables.

`stack.env` contains environment variables for the production stack file itself.


<div id="production-stackyml" />

#### stack.yml

This file does not exist in the bare DargStack skeleton.
Utilize the helper script [dargstack](https://github.com/dargmuesli/dargstack_template/blob/master/dargstack) for deployment.
It derives `[project-name]_stack/production/stack.yml` from `[project-name]_stack/development/stack.yml` and deploys the latter automatically.
