# DargStack

A template for Docker stack project layouts.
Bootstrap it from [github.com/dargmuesli/dargstack_template](https://github.com/dargmuesli/dargstack_template)!

This template solves the problem of separated development and production environments in the otherwise well-defined, containerized software development process.
It focuses on the development configuration, derives the production configuration from it and makes deployments a breeze!


## Table of Contents

1. **[Skeleton](#skeleton)**
1. **[Helper Script](#helper-script)**
1. **[Configuration](#configuration)**
1. **[Example Projects](#example-projects)**


## Skeleton

The essential idea of this template.
Read the full and detailed skeleton specification at [./README-skeleton.md](./README-skeleton.md).


## Helper Script

Requires sudo >= 1.8.21 due to usage of the extended --preserve-env list syntax.
That means the minimum supported Debian version is `buster`.

```
usage: dargstack <module> <options>

modules
    build [sibling]           Builds the main project or the specified sibling, tagged as dev. Only for development.
    deploy                    Deploys a Docker project either from a full local development clone of the project or, with the --production parameter provided, by doing a sparse Git checkout containing only the production configuration. In the latter case derive is executed first and the existence of required environment variables is checked before deployment starts.
    derive                    Derives a ./production/stack.yml from ./development/stack.yml.
    rgen                      Generate the README.
    rm                        Removes the stack.
    self-update               Updates the helper script.

options
    -h, --help                Display this help. Usable with modules: all.
    -p, --production <tag>    Execute in production mode. Version must equal a tag name or latest. Usable with modules: deploy.
    -u, --url <url>           The URL to clone from. May include the substrings <owner> and <name> that are replaced by their corresponding value that is inferred from the DargStack directory structure. Usable with modules: deploy.
```


## Configuration

A few setup strategies for the development environment have proven themselves useful, e.g. running a local dns server.


### DNS

Within development of a DargStack, access to the web apps should be routed via the locally resolved domain `[project_name].test` and its subdomains.
Therefore one needs to configure the local DNS resolution to make this address resolvable.
This can either be done by simply adding this domain and all subdomains to the operation system's hosts file or by settings up a local DNS server.
An advantage of the latter method is that subdomain wildcards can be used and thus not every subdomain needs to be defined separately.

Here is an example configuration for [dnsmasq](https://en.wikipedia.org/wiki/Dnsmasq) on [Arch Linux](https://www.archlinux.org/) that uses the local DNS server on top of the router's advertised DNS server:

`/etc/dnsmasq.conf`
```conf
# Use NetworkManager's resolv.conf
resolv-file=/run/NetworkManager/resolv.conf

# Limit to machine-wide requests
listen-address=127.0.0.1

# Wildcard DNS
address=/.test/127.0.0.1

# Enable logging (systemctl status dnsmasq)
#log-queries
```

`/etc/NetworkManager/NetworkManager.conf`
```conf
[main]

# Don't touch /etc/resolv.conf
rc-manager=unmanaged
```


## Example Projects

- [dargmuesli/dargstack-example](https://github.com/dargmuesli/dargstack-example/)
- [dargmuesli/dargstack-example_stack](https://github.com/dargmuesli/dargstack-example_stack/)
- [dargmuesli/jonas-thelemann_stack](https://github.com/dargmuesli/jonas-thelemann_stack/)
- [dargmuesli/randomwinpicker_stack](https://github.com/dargmuesli/randomwinpicker_stack/)
- [flipdot/drinks-touch_stack](https://github.com/flipdot/drinks-touch_stack/)
