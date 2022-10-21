# DargStack

A template for Docker stack project layouts.
Bootstrap it from [github.com/dargmuesli/dargstack_template](https://github.com/dargmuesli/dargstack_template)!

> **This template solves the problem of separated development and production environments in the otherwise well-defined, containerized software development process.
> It focuses on the development configuration, derives the production configuration from it and makes deployments a breeze!**


## Table of Contents

1. **[Installation Example](#installation-example)**
1. **[Skeleton](#skeleton)**
1. **[Helper Script](#helper-script)**
1. **[Configuration](#configuration)**
1. **[Example Projects](#example-projects)**


## Installation Example

When using bash, you could setup this script as an executable the following way:

```bash
mkdir ~/scripts/ \
    && wget https://raw.githubusercontent.com/dargmuesli/dargstack/master/src/dargstack -O ~/scripts/dargstack \
    && chmod +x ~/scripts/dargstack \
    && echo 'export PATH="$PATH:$HOME/scripts/"' >> ~/.bashrc \
    && . ~/.bashrc
```

Feel free to deviate from this example and use your personal preference!

### Info for Apple users

The bash used on macOS is of version 3.x which has no [globstar](https://www.gnu.org/software/bash/manual/html_node/The-Shopt-Builtin.html) installed.
To run dargstack you need to install a newer version of bash from i.e. [brew](https://brew.sh/) and set the new bash as the one to use in the [dargstack shebang](https://github.com/dargmuesli/dargstack/blob/master/src/dargstack#L1).

A second problem that arises after that is that parameters couldn't be read. This comes from the getopt function which is a different one on macOS than on usual linux distros and does not support long options with two hyphens. More info on that [here](https://en.wikipedia.org/wiki/Getopt#Extensions)

To solve this problem the gnu-getopt version of getopt has to be installed. As the bash it can be installed from [brew](https://formulae.brew.sh/formula/gnu-getopt#default). If you don't wish to add the getopt to the path just change the ```getopt``` command to the new installed getopt by adding the installation directory in front of the getopt command. An example can be seen in the dargstack_macos file. In that case the changes have to be done to line 472 and 484.

This arises another problem with the parameters which will be tracked in [#48](https://github.com/dargmuesli/dargstack/issues/48)
For now it is not possible to use dargstack on mac unfortunately.


## Skeleton

The essential idea of this template.
Read the full and detailed skeleton specification at [./README-skeleton.md](./README-skeleton.md).


## Helper Script

Requires sudo >= 1.8.21 due to usage of the extended --preserve-env list syntax.
That means the minimum supported Debian version is `buster`.

```
DargStack template helper script.

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
    -u, --url <url>           The URL to clone from. May include the substrings <owner> and <name> that are replaced by their corresponding value that is inferred from the DargStack directory structure. Usable with modules: deploy.
```


## Configuration

A few setup strategies for the development environment have proven themselves useful, e.g. running a local dns server.


### DNS

Within development of a DargStack, access to the web apps should be routed via the locally resolved domain `[project_name].test` and its subdomains.
Therefore one needs to configure the local DNS resolution to make this address resolvable.
This can either be done by simply adding this domain and all subdomains to the operation system's hosts file or by setting up a local DNS server.
An advantage of the latter method is that subdomain wildcards can be used and thus not every subdomain needs to be defined separately.

Here is an example configuration for [dnsmasq](https://en.wikipedia.org/wiki/Dnsmasq) that uses the local DNS server on top of the router's advertised DNS server:

<details>
  <summary><b>Instructions for Arch Linux</b></summary>

  `/etc/dnsmasq.conf`
  ```env
  # Files to read resolv configuration from.
  conf-file=/etc/dnsmasq-openresolv.conf
  resolv-file=/etc/dnsmasq-resolv.conf

  # Limit to machine-wide requests.
  listen-address=::1,127.0.0.1

  # Wildcard DNS.
  address=/.test/127.0.0.1

  # Enable logging (systemctl status dnsmasq).
  #log-queries
  ```

  `/etc/NetworkManager/NetworkManager.conf`
  ```env
  [main]

  # Don't touch `/etc/resolv.conf`.
  rc-manager=resolvconf
  ```

  `/etc/resolvconf.conf`
  ```env
  # Limit to machine-wide requests.
  name_servers="::1 127.0.0.1"

  # Files to output resolv configuration to.
  dnsmasq_conf=/etc/dnsmasq-openresolv.conf
  dnsmasq_resolv=/etc/dnsmasq-resolv.conf
  ```

  Then run `sudo resolvconf -u`!
</details>

<details>
  <summary><b>Instructions for Ubuntu & Debian</b></summary>

  `/etc/dnsmasq.conf`
  ```env
  # Files to read resolv configuration from.
  resolv-file=/etc/resolvconf/resolv.conf.d/original

  # Limit to machine-wide requests.
  listen-address=::1,127.0.0.1

  # Wildcard DNS.
  address=/.test/127.0.0.1

  # Enable logging (systemctl status dnsmasq).
  #log-queries
  ```

  `/etc/NetworkManager/NetworkManager.conf`
  ```env
  [main]

  # Don't touch `/etc/resolv.conf`.
  rc-manager=resolvconf
  systemd-resolved=false # for Ubuntu and Debian
  ```

  `/etc/resolvconf/resolv.conf.d/head`
  ```env
  nameserver=::1
  nameserver=127.0.0.1
  ```

  ---

  If on [WSL](https://docs.microsoft.com/en-us/windows/wsl/install):

  `/etc/wsl.conf`
  ```env
  [network]
  generateResolvConf = false

  [boot]
  command="dpkg-reconfigure --frontend=noninteractive resolvconf && resolvconf -u && service docker start && service dnsmasq start && service resolvconf start"
  ```
</details>


## Example Projects

- [dargmuesli/dargstack-example](https://github.com/dargmuesli/dargstack-example/)
- [dargmuesli/dargstack-example_stack](https://github.com/dargmuesli/dargstack-example_stack/)
- [dargmuesli/jonas-thelemann_stack](https://github.com/dargmuesli/jonas-thelemann_stack/)
- [flipdot/drinks-touch_stack](https://github.com/flipdot/drinks-touch_stack/)
- [maevsi/maevsi_stack](https://github.com/maevsi/maevsi_stack/)
