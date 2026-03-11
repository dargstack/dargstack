## [4.0.0-beta.5](https://github.com/dargstack/dargstack/compare/v4.0.0-beta.4...v4.0.0-beta.5) (2026-03-11)

### Bug Fixes

* **version:** fall back to build info ([86c05a0](https://github.com/dargstack/dargstack/commit/86c05a06e3af1af0d63953899fb97730a9fb93cd))

## [4.0.0-beta.4](https://github.com/dargstack/dargstack/compare/v4.0.0-beta.3...v4.0.0-beta.4) (2026-03-11)

### Bug Fixes

* **go:** add v4 path suffix ([60c23a9](https://github.com/dargstack/dargstack/commit/60c23a9d61313ab881f29fe3b073921353e79ae1))

## [4.0.0-beta.3](https://github.com/dargstack/dargstack/compare/v4.0.0-beta.2...v4.0.0-beta.3) (2026-03-11)

### Bug Fixes

* **ci:** drop deprecated goreleaser properties ([7af5ee4](https://github.com/dargstack/dargstack/commit/7af5ee475646a5e8717d7f96d6545fac8d17bfa2))

## [4.0.0-beta.2](https://github.com/dargstack/dargstack/compare/v4.0.0-beta.1...v4.0.0-beta.2) (2026-03-11)

### Bug Fixes

* **release:** use semantic release over tag workflow ([41dfaa8](https://github.com/dargstack/dargstack/commit/41dfaa84f674a4e68fc74dc24238fdbad8661230))

## [4.0.0-beta.1](https://github.com/dargstack/dargstack/compare/v3.0.0...v4.0.0-beta.1) (2026-03-11)

### ⚠ BREAKING CHANGES

* drop v3

### Features

* add audit log for deployment tracking ([31a6ed8](https://github.com/dargstack/dargstack/commit/31a6ed8f85bdf1c6d1250b4f13e598634f98426a))
* add build, certify, inspect, update, and document commands ([62f58bb](https://github.com/dargstack/dargstack/commit/62f58bb050b53d60e2016b7d9b35e05c6b60bab2))
* add CLI root command with shared flags and helpers ([edaf55b](https://github.com/dargstack/dargstack/commit/edaf55b8d4d35b6c008c48fc9493fdee73ae7b3e))
* add compose file merging with spruce and path resolution ([f50f456](https://github.com/dargstack/dargstack/commit/f50f4564ca948f900ad32a9148c1ae040d3c5622))
* add compose profile and service filtering ([c04a441](https://github.com/dargstack/dargstack/commit/c04a4417a81792d43d2c8fc3438359940248f875))
* add config and stack directory detection ([200a7d3](https://github.com/dargstack/dargstack/commit/200a7d32987f33904fd9a98ba074cae3a61e0d99))
* add deploy command with secret setup, profiles, and dry-run support ([9f910bf](https://github.com/dargstack/dargstack/commit/9f910bf791dae83bfc3e3f3ec4dfe9830c5c2862))
* add docgen tool and auto-generated CLI reference docs ([69cc61e](https://github.com/dargstack/dargstack/commit/69cc61e46c7c09f1216492667e099a056c926f6b))
* add docker client, executor, swarm, and stack management ([1ae2894](https://github.com/dargstack/dargstack/commit/1ae28942abc7b993e4d45f9cbaf10cfd91c3760f))
* add init command to scaffold a new dargstack project ([758e3f5](https://github.com/dargstack/dargstack/commit/758e3f5898cdc39f8f5d3bcc0c9e70afaa6df13f))
* add interactive terminal prompt helpers ([e070672](https://github.com/dargstack/dargstack/commit/e0706724e3533dfbe95ce631ac27eaf332123ac3))
* add remove command with targeted service and volume removal ([2998296](https://github.com/dargstack/dargstack/commit/29982963316cd92fef391672de6cc4955b11b395))
* add resource validation and documentation generation ([0c4f8c2](https://github.com/dargstack/dargstack/commit/0c4f8c2707057af81d295d5834ef2d9820599a9c))
* add secret templating, generation, and topological resolution ([a486dd8](https://github.com/dargstack/dargstack/commit/a486dd880fcb0b8fff5eb1d2965a3d9ca74bd08c))
* add self-update mechanism via GitHub releases ([c2f5617](https://github.com/dargstack/dargstack/commit/c2f5617fe87d236bc8c0049ca7af3b4002403f1c))
* add TLS certificate retrieval and management ([1d57a1d](https://github.com/dargstack/dargstack/commit/1d57a1d56a8a4213757ee25b36df7067854b6c2d))
* add validate command for compose config verification ([962ad43](https://github.com/dargstack/dargstack/commit/962ad43a110ed9b75f947fe560b7bd13829e0a7d))
* drop v3 ([373106a](https://github.com/dargstack/dargstack/commit/373106aa21f70fe9e06123a9be1f6170f1717cea))

### Bug Fixes

* implement feedback ([d120968](https://github.com/dargstack/dargstack/commit/d12096880ffdb917524f756bbc51957ac5dacbfa))

## [3.0.0](https://github.com/dargstack/dargstack/compare/2.5.1...3.0.0) (2025-12-07)

### ⚠ BREAKING CHANGES

* **deploy:** switch domain to `app.localhost` for development
* **git:** switch from master to main

### Features

* **deploy:** switch domain to `app.localhost` for development ([91440cb](https://github.com/dargstack/dargstack/commit/91440cbb198b938c1aa9906c88a40199e4d77f13))
* **git:** switch from master to main ([9d10c10](https://github.com/dargstack/dargstack/commit/9d10c10f9c12f6f4619c270327fae56f3ce4f724))

## [2.5.1](https://github.com/dargstack/dargstack/compare/2.5.0...2.5.1) (2025-03-19)


### Bug Fixes

* **configuration:** include in sparse checkout ([1899ac5](https://github.com/dargstack/dargstack/commit/1899ac5c5cb9889ed665069e3b98580338ade75c))

# [2.5.0](https://github.com/dargstack/dargstack/compare/2.4.1...2.5.0) (2025-03-18)


### Features

* add configuration support ([ac43b10](https://github.com/dargstack/dargstack/commit/ac43b10ce477d0948fe57c64a005b7162c61a2c7))

## [2.4.1](https://github.com/dargstack/dargstack/compare/2.4.0...2.4.1) (2025-03-18)


### Bug Fixes

* improve unset third party secret clarity ([27e9e2e](https://github.com/dargstack/dargstack/commit/27e9e2e0c79c907d3b6fd10324add713e26ebaf5))

# [2.4.0](https://github.com/dargstack/dargstack/compare/2.3.0...2.4.0) (2025-03-18)


### Features

* warn for unset third party secrets ([09dc452](https://github.com/dargstack/dargstack/commit/09dc45290615ccc409bfc5ddcecb8220682852fe))

# [2.3.0](https://github.com/dargstack/dargstack/compare/2.2.3...2.3.0) (2025-03-11)


### Features

* allow deploy only from stack directory ([c84690a](https://github.com/dargstack/dargstack/commit/c84690af17c027a7ec4d1361fe59a569a8c324a0))
* change name casing ([13dc886](https://github.com/dargstack/dargstack/commit/13dc886010d7106343d746b66926655ccf7117f7))

## [2.2.3](https://github.com/dargstack/dargstack/compare/2.2.2...2.2.3) (2025-02-05)


### Bug Fixes

* allow space in environment file variables ([bfea172](https://github.com/dargstack/dargstack/commit/bfea1728fee14944fab2ee68418d35609ee6519d))

## [2.2.2](https://github.com/dargstack/dargstack/compare/2.2.1...2.2.2) (2024-09-15)


### Bug Fixes

* **options:** correct advertise address typo ([db396b1](https://github.com/dargstack/dargstack/commit/db396b19c866c160615190f475141f10207fba69))

## [2.2.1](https://github.com/dargstack/dargstack/compare/2.2.0...2.2.1) (2023-03-27)


### Bug Fixes

* use ghcr.io registry ([f94a8b2](https://github.com/dargstack/dargstack/commit/f94a8b26644367258b09f4ae6ce974781dc6522c))

# [2.2.0](https://github.com/dargstack/dargstack/compare/2.1.0...2.2.0) (2023-03-27)


### Features

* migrate to new organization name ([dc52ae9](https://github.com/dargstack/dargstack/commit/dc52ae91cd2ea835bf06b10594a27494acc9863e))

# [2.1.0](https://github.com/dargmuesli/dargstack/compare/2.0.3...2.1.0) (2023-01-24)


### Features

* add license ([4bbe914](https://github.com/dargmuesli/dargstack/commit/4bbe914ae83cae2f97bf2509a459c303c362908b))

## [2.0.3](https://github.com/dargmuesli/dargstack/compare/2.0.2...2.0.3) (2023-01-03)


### Bug Fixes

* **package:** use prepare ([58c8ced](https://github.com/dargmuesli/dargstack/commit/58c8cedddfe251e79a97b79fded02718cf9d50f6))


### Reverts

* Revert "fix(deps): add pinst" ([77af3b8](https://github.com/dargmuesli/dargstack/commit/77af3b82e3b2c26c8fa316767a71c63695508c3f))

## [2.0.2](https://github.com/dargmuesli/dargstack/compare/2.0.1...2.0.2) (2023-01-03)


### Bug Fixes

* **deps:** add pinst ([cf1bb3d](https://github.com/dargmuesli/dargstack/commit/cf1bb3da83fdb5c1bb7d4294579ece2792879a4d))

## [2.0.1](https://github.com/dargmuesli/dargstack/compare/2.0.0...2.0.1) (2022-12-27)


### Bug Fixes

* **package:** migrate to module ([359fcea](https://github.com/dargmuesli/dargstack/commit/359fceaae976727380f0da1a8248c21e3dd8b6bb))

# [2.0.0](https://github.com/dargmuesli/dargstack/compare/1.3.0...2.0.0) (2022-11-08)


### Bug Fixes

* **ci:** add npm token ([23773f4](https://github.com/dargmuesli/dargstack/commit/23773f4c183c33438d27c3b338493f887a2b27cb))


* feat!: use localhost for development ([690a5f8](https://github.com/dargmuesli/dargstack/commit/690a5f811866ff332f221b11099e2faebad16cdf))


### BREAKING CHANGES

* Using a custom stack domain requires a parameter now!
It is not required anymore to setup a local dns server.

# [2.0.0-alpha.1](https://github.com/dargmuesli/dargstack/compare/1.3.0...2.0.0-alpha.1) (2022-10-27)


* feat!: use localhost for development ([690a5f8](https://github.com/dargmuesli/dargstack/commit/690a5f811866ff332f221b11099e2faebad16cdf))


### BREAKING CHANGES

* Using a custom stack domain requires a parameter now!
It is not required anymore to setup a local dns server.

# [1.3.0](https://github.com/dargmuesli/dargstack/compare/1.2.3...1.3.0) (2022-10-27)


### Bug Fixes

* correct getopt path ([bddc8fb](https://github.com/dargmuesli/dargstack/commit/bddc8fb67fa9519d639387b26299e46e98cec4f4))


### Features

* merge macos fixes into source script and docs ([63ad5fd](https://github.com/dargmuesli/dargstack/commit/63ad5fd49116d8a74ad9d862c664977b1dc99dd4))

## [1.2.3](https://github.com/dargmuesli/dargstack/compare/1.2.2...1.2.3) (2022-09-10)


### Bug Fixes

* preserve STACK_DOMAIN ([1a578df](https://github.com/dargmuesli/dargstack/commit/1a578df471b714de66814e65063b298da7fdd2e1))

## [1.2.2](https://github.com/dargmuesli/dargstack/compare/1.2.1...1.2.2) (2022-09-10)


### Bug Fixes

* **self-update:** trap function ([af2735e](https://github.com/dargmuesli/dargstack/commit/af2735eab3216c68fdc0e620d193c048ee144479))

## [1.2.1](https://github.com/dargmuesli/dargstack/compare/1.2.0...1.2.1) (2022-09-10)


### Bug Fixes

* **self-update:** improve logic ([0a0594d](https://github.com/dargmuesli/dargstack/commit/0a0594d39995a598c9134a9de25a84d5378a1a55))

# [1.2.0](https://github.com/dargmuesli/dargstack/compare/1.1.3...1.2.0) (2022-09-10)


### Features

* make function variables local ([81c5669](https://github.com/dargmuesli/dargstack/commit/81c5669a304311b1b6e8a5fbc0ea5c8d1c4ad078))

## [1.1.3](https://github.com/dargmuesli/dargstack/compare/1.1.2...1.1.3) (2022-09-07)


### Bug Fixes

* **dargstack:** remove leftovers ([d02869c](https://github.com/dargmuesli/dargstack/commit/d02869c8b7dade2ace074fbdf7d9d9560b14b634))

## [1.1.2](https://github.com/dargmuesli/dargstack/compare/1.1.1...1.1.2) (2022-09-07)


### Bug Fixes

* ensure swarm for development too ([31e7309](https://github.com/dargmuesli/dargstack/commit/31e7309a7b93a35687f14cf971843dbf9ba70ed0))

## [1.1.1](https://github.com/dargmuesli/dargstack/compare/1.1.0...1.1.1) (2022-09-07)


### Bug Fixes

* preserve environment variables for development ([807900b](https://github.com/dargmuesli/dargstack/commit/807900b61fa2763925af3d51125818ce4de93eb2))

# [1.1.0](https://github.com/dargmuesli/dargstack/compare/1.0.0...1.1.0) (2022-09-07)


### Features

* preserve environment variables for users outside docker group ([e55e83f](https://github.com/dargmuesli/dargstack/commit/e55e83f0662399d4a6154a01babce2619168a203))

## [0.8.1](https://github.com/dargmuesli/dargstack/compare/0.8.0...0.8.1) (2021-11-16)


### Bug Fixes

* **template:** allow check of templates with values ([038c058](https://github.com/dargmuesli/dargstack/commit/038c05835f7c4c47addb9aed2a22cba85bbbcdd1))

# [0.8.0](https://github.com/dargmuesli/dargstack/compare/0.7.2...0.8.0) (2021-10-27)


### Bug Fixes

* minor tweaks ([db9ff9b](https://github.com/dargmuesli/dargstack/commit/db9ff9baa6381dc6ab787bca5b5943ae9a32edcb))
* shellcheck ([3ebdf9d](https://github.com/dargmuesli/dargstack/commit/3ebdf9d460760381b19f40caf2eb68a0e071fa0a))


### Features

* warn for unset development secrets ([b721237](https://github.com/dargmuesli/dargstack/commit/b721237e74c5e2b820c96f6b583b207d8c89d343))

## [0.7.2](https://github.com/dargmuesli/dargstack/compare/0.7.1...0.7.2) (2021-02-25)


### Bug Fixes

* correct removal name ([3c8a1e3](https://github.com/dargmuesli/dargstack/commit/3c8a1e39aeee082a6cb772c2f745e595b7cf158d))

## [0.7.1](https://github.com/dargmuesli/dargstack/compare/0.7.0...0.7.1) (2021-01-20)


### Bug Fixes

* naming ([268d7e6](https://github.com/dargmuesli/dargstack/commit/268d7e628c5e1614e15f19da6810944312fb45ba))

# [0.7.0](https://github.com/dargmuesli/dargstack/compare/0.6.0...0.7.0) (2021-01-17)


### Features

* remove dangling images only if stack is running ([98095b2](https://github.com/dargmuesli/dargstack/commit/98095b2c919f0c3405b5c48e7cfd76dd62df58f1)), closes [#6](https://github.com/dargmuesli/dargstack/issues/6)

# [0.6.0](https://github.com/dargmuesli/dargstack/compare/0.5.3...0.6.0) (2021-01-17)


### Features

* wait for docker rm & add redeploy command ([92f5a19](https://github.com/dargmuesli/dargstack/commit/92f5a190a55d48ac237065d822fca3207cf50b54)), closes [#8](https://github.com/dargmuesli/dargstack/issues/8)

## [0.5.3](https://github.com/dargmuesli/dargstack/compare/0.5.2...0.5.3) (2020-12-09)


### Bug Fixes

* correct environment variable check order ([2860cec](https://github.com/dargmuesli/dargstack/commit/2860cec5f70f2103147b9e9ad812bba4d66385c8))

## [0.5.2](https://github.com/dargmuesli/dargstack/compare/0.5.1...0.5.2) (2020-12-09)


### Bug Fixes

* derive environment variables ([1797bbc](https://github.com/dargmuesli/dargstack/commit/1797bbca50564fe1d59bb42bc36c537a9b7f16a6))
* remove incorrect exclamation mark ([04bfd2f](https://github.com/dargmuesli/dargstack/commit/04bfd2fe260da6b344229aa75cb2313c7b0eaaa7))

## [0.5.1](https://github.com/dargmuesli/dargstack/compare/0.5.0...0.5.1) (2020-11-10)


### Bug Fixes

* correct docker command ([cbadb8f](https://github.com/dargmuesli/dargstack/commit/cbadb8f190b6986a1eba6bed2733f4ec603790a5))

# [0.5.0](https://github.com/dargmuesli/dargstack/compare/0.4.0...0.5.0) (2020-11-06)


### Features

* remove stopped containers ([7352a2a](https://github.com/dargmuesli/dargstack/commit/7352a2a1373d533ec59fffca84bfbd78a5bd790e))

# [0.4.0](https://github.com/dargmuesli/dargstack/compare/0.3.1...0.4.0) (2020-11-06)


### Features

* ask to prune images ([f0c8c5c](https://github.com/dargmuesli/dargstack/commit/f0c8c5cf5a9f9c5bb802f52b08f1047a03c321af))

## [0.3.1](https://github.com/dargmuesli/dargstack/compare/0.3.0...0.3.1) (2020-11-03)


### Bug Fixes

* **deploy:** preserve environment variables only when file exists ([79e937c](https://github.com/dargmuesli/dargstack/commit/79e937c447c7023e8db8ffe0248fc945e2543b19))

# [0.3.0](https://github.com/dargmuesli/dargstack/compare/0.2.1...0.3.0) (2020-10-12)


### Features

* add validation ([c082724](https://github.com/dargmuesli/dargstack/commit/c082724cb30f216b60f66b3209e32f7b56a655dd))

## [0.2.1](https://github.com/dargmuesli/dargstack/compare/0.2.0...0.2.1) (2020-10-04)


### Bug Fixes

* disable shellcheck word splitting warning ([e49c584](https://github.com/dargmuesli/dargstack/commit/e49c5840a40df664bcf20f23b7ff0df75454af88))

# [0.2.0](https://github.com/dargmuesli/dargstack/compare/0.1.0...0.2.0) (2020-09-28)


### Features

* adapt source directory ([67382db](https://github.com/dargmuesli/dargstack/commit/67382dbc4195db64436ebde853f8ad09b81ca3d0))
