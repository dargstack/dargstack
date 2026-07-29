# Major Version Guard for Production Deploy

**Date:** 2026-07-29
**Status:** Approved

## Problem

`dargstack deploy --environment production` moves the stack to the latest git tag on the configured branch. If a major version upgrade (e.g., v2.x.x to v3.x.x) is tagged, the deploy will silently upgrade the major version. Major version upgrades may contain breaking changes and should require explicit operator consent.

## Decision

Block production deploys that would advance the major version unless the operator passes `--major`. Only one major version step is allowed at a time (v2 to v3, not v2 to v4).

## Design

### Tag Resolution Flow

The guard lives in `resolveDeployTag()` in `internal/cli/deploy_image.go`.

Current flow:
1. `--tag` flag: unconditional override
2. Config `production.tag`: unconditional override
3. Auto-resolution: `latestGitTag()` finds the most recent tag on the branch

New flow:
1. `--tag` flag: resolve the tag, then check major version against current
2. Config `production.tag`: resolve the tag, then check major version against current
3. Auto-resolution: resolve within current major, or N+1 with `--major`

### Major Version Detection

The current major version is determined by reading the tag at HEAD:
```
git describe --tags --exact-match HEAD
```

If HEAD is not at a tagged commit (branch, detached non-tagged, or no tags), deploy errors with:
```
no tagged commit checked out — use --tag <version> or --major to deploy
```

### Major Version Parsing

`parseMajorVersion(tag string) (int, error)` strips an optional `v` prefix, splits on `.`, and parses the first segment as an integer.

| Input      | Output |
|------------|--------|
| `v1.2.3`   | `1`    |
| `1.2.3`    | `1`    |
| `v0.9.0`   | `0`    |
| `v10.0.0`  | `10`   |
| `latest`   | error  |

### Tag Filtering

`latestGitTag` is modified to accept a target major version. It uses `git tag` with glob patterns matching both `vN.*` and `N.*` prefixes, sorted by version. Results are deduplicated and the first match is returned.

The function still prefers `origin/<branch>` over the local branch.

### `--major` Flag Behavior

`--major` (bool) acts as an authorization gate for any major version change.

| Scenario | Behavior |
|----------|----------|
| Auto-resolution, no `--major` | Latest tag within current major |
| Auto-resolution, with `--major` | Latest tag with major N+1 |
| `--tag v2.1.1` (same major), no `--major` | Allowed |
| `--tag v3.0.0` (new major), no `--major` | Blocked |
| `--tag v3.0.0 --major` | Allowed |
| `--major`, no N+1 tag exists | Error |

### Error Messages

- No tag at HEAD: `no tagged commit checked out — use --tag <version> or --major to deploy`
- Major upgrade without `--major`: `deploying v3.0.0 is a major version upgrade from v2.1.0 — use --major to confirm`
- `--major`, no N+1 tag: `no v3.x.x tag found on branch — version v3 has not been released yet`

### Single-Step Major Upgrades

`--major` only advances by one major version. If current is v2 and v4 exists but v3 does not, `--major` errors. The operator must deploy v3 first.

## Files Changed

- `internal/cli/deploy.go` — add `--major` flag
- `internal/cli/deploy_image.go` — tag resolution, major version guard, `parseMajorVersion`, modified `latestGitTag`
- `internal/cli/deploy_image_test.go` — comprehensive tests for all scenarios
- `internal/schema/schema.go` — no changes needed (flag is CLI-only, not config)

## Testing

- `parseMajorVersion` with prefixed, unprefixed, non-semver, multi-digit inputs
- `latestGitTag` with mixed major versions, verifying filtering by target major
- `resolveDeployTag` blocking cross-major without `--major`
- `resolveDeployTag` allowing cross-major with `--major`
- `--tag` + cross-major blocked without `--major`, allowed with it
- HEAD not at tagged commit errors
- `--major` with no N+1 tag available
- Mixed `v` prefixed and unprefixed tags in same repo