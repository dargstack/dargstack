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
