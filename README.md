# dargstack

Dargstack addresses the challenge of maintaining separate development and production environments within an otherwise well-structured, containerized software development workflow.
It prioritizes development configuration, derives production configurations from it, and simplifies deployments!

This repository contains the dargstack script.
If you're looking for guidance on initiating and running a project with dargstack, refer to the [template documentation](./docs/template.md).
To bootstrap your project using the dargstack template, visit [dargstack/dargstack_template](https://github.com/dargstack/dargstack_template).

The following projects showcase dargstack in action:

- [maevsi/stack](https://github.com/maevsi/stack/)
- [dargmuesli/jonas-thelemann_stack](https://github.com/dargmuesli/jonas-thelemann_stack/)
- [flipdot/drinks-touch_stack](https://github.com/flipdot/drinks-touch_stack/)

You can explore a minimal setup example here:

- [dargstack/dargstack-example](https://github.com/dargstack/dargstack-example/)
- [dargstack/dargstack-example_stack](https://github.com/dargstack/dargstack-example_stack/)

## Table of Contents

1. **[Installation](#installation)**
2. **[Configuration Options](#configuration-options)**

## Installation

Dargstack requires `sudo >= 1.8.21` due to its use of the extended `--preserve-env` list syntax.
The minimum supported Debian version is `buster`.

To set up the script as an executable using Bash, follow these steps:

```bash
mkdir ~/scripts/ \
    && wget https://raw.githubusercontent.com/dargstack/dargstack/master/src/dargstack -O ~/scripts/dargstack \
    && chmod +x ~/scripts/dargstack \
    && echo 'export PATH="$PATH:$HOME/scripts"' >> ~/.bashrc \
    && . ~/.bashrc
```

Feel free to adjust this setup to match your preferences!

### macOS Installation Notes

1. The `getopt` utility on macOS [differs from its Linux counterpart](https://en.wikipedia.org/wiki/Getopt#Extensions) as it does not support long options with two hyphens.
   To resolve this, install GNU `getopt`:

   ```sh
   brew install gnu-getopt
   ```

   Dargstack will automatically detect `getopt` under `/opt/homebrew/opt/gnu-getopt/bin/getopt`.

2. macOS ships with Bash version 3.x, which does not support [globstars](https://www.gnu.org/software/bash/manual/html_node/The-Shopt-Builtin.html).
   To run dargstack, install a newer version of Bash via [Homebrew](https://brew.sh/):

   ```sh
   brew install bash
   ```

   You must **always** use the newly installed Bash version to invoke dargstack. To simplify this, consider adding an alias to your [`~/.bashrc`](https://wiki.ubuntuusers.de/alias/):

   ```sh
   /opt/homebrew/Cellar/bash/5.2.2/bin/bash dargstack
   # or
   echo "alias dargstack='/opt/homebrew/Cellar/bash/5.2.2/bin/bash dargstack'" >> ~/.bashrc
   ```


## Configuration Options

```
Dargstack template helper script.

usage: dargstack <module> <options>

modules
    build [sibling]           Builds the main project or the specified sibling, tagged as dev. Only for development.
    deploy                    Deploys a Docker project either from a full local development clone of the project or, with the --production parameter provided, by doing a sparse Git checkout containing only the production configuration. In the latter case derive is executed first and the existence of required environment variables is checked before deployment starts.
    derive                    Derives a ./production/stack.yml from ./development/stack.yml.
    rgen                      Generate the README.
    rm                        Removes the stack.
    self-update               Updates the helper script.
    validate                  Checks for an up-2-date README.

options
    -a, --advertise-addr      The address Docker Swarm advertises.
    -h, --help                Display this help. Usable with modules: all.
    -o, --offline             Do not try to update the checkout
    -p, --production <tag>    Execute in production mode. Version must equal a tag name or latest. Usable with modules: deploy.
    -u, --url <url>           The URL to clone from. May include the substrings <owner> and <name> that are replaced by their corresponding value that is inferred from the dargstack directory structure. Usable with modules: deploy.
```
