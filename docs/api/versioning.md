# API versioning and compatibility

The REST API (`/api/v1`), the messenger API (`/widget/v1`), webhooks and every SDK under `sdk/` are public contracts. Customers' code runs against them unattended. We never break them.

## Versions

- An API version is a date, such as `2026-11-01` (`config/app.go`, `API.Version`). It is the `info.version` of the OpenAPI document and the `api_version` of every webhook payload.
- A version is frozen once released. Within a version, changes are **additive only**.
- A breaking change ships only as a new dated version. The old version keeps working, unchanged, for at least 12 months after the new one is released.

## What counts as breaking

Checked automatically by `make api-compat` (oasdiff plus our own webhook rules). Any of these fails CI:

- Removing or renaming a path, method, operation id, query/path parameter, request field or response field (required or optional).
- Changing a field's or parameter's type or format.
- Making an optional request field or parameter required, or adding a new required one.
- Narrowing a request enum, removing a response enum value, or **adding a response enum value** (generated SDKs, such as the Python one, reject unknown values).
- Removing a component schema (SDK type names come from it).
- Removing a webhook event, changing its object, making it named-only, or removing or retyping an envelope field.

Additive and allowed:

- New paths, operations, optional parameters and optional request fields.
- New response fields. Clients must ignore fields they do not know.
- New webhook events and new envelope fields.
- New request enum values.

## How to change the API

1. Change the route, request or resource, additively.
2. `make api-snapshot` refreshes `tests/contracts/*.json`.
3. `make api-compat` checks it against the last release and must pass.
4. `make sdk-check` generates every SDK from the contract, builds it and runs its tests. Generated SDK code is gitignored; only the contract is committed.
5. `make sdk-compat` compares each SDK's public surface with its last published release (npm, the `dimmi-go`/`dimmi-php`/`dimmi-laravel` mirrors, PyPI).
6. `pnpm changeset` records an SDK release note (minor for new API surface).

## Deprecating

Deprecate first, remove only in a new API version.

```go
routing.Get(g, "/old", "things.old", c.things.Old).Deprecated(routing.Deprecation{
	Since:  time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
	Sunset: time.Date(2027, 11, 1, 0, 0, 0, 0, time.UTC), // at least 12 months later
	Link:   "https://docs.trydimmi.com/changelog/things-old",
})
```

- Every response then carries `Deprecation: @<unix time>` (RFC 9745), `Sunset: <HTTP date>` (RFC 8594) and `Link: <...>; rel="deprecation"`.
- The OpenAPI operation is marked `deprecated: true` with `x-sunset`, so generated SDKs flag it.
- For a field, add the replacement beside it and describe the old one as deprecated. Keep sending it.
- Announce in the changelog and in the SDK changeset.

## Shipping a break anyway

Only with a new API version, or, rarely, a reviewed exception:

- Add an entry to `contracts/breaking-changes.json` with `id` and `operation` (or `event`) from the `make api-compat` output, a `reason`, `deprecated` and `sunset` dates, and `approved_by`.
- The SDKs that change shape get a major changeset; `make sdk-compat` blocks it otherwise.

## Releases and baselines

- The last released contracts are the ones at the newest `api-v*` git tag; no copy is kept in the tree. `make api-compat` compares against that tag (against `origin/main` before the first release); CI also compares against the base commit.
- Only a release creates the tag (`make api-baseline`). It also empties `contracts/breaking-changes.json`.
- SDKs release through changesets (`.github/workflows/sdk-release.yml`): a "Version packages" pull request, then publishing behind the protected `sdk-release` environment. The release generates every SDK first. npm and PyPI packages ship the generated code; Go, PHP and Laravel are split, generated code included, into read-only mirrors tagged `vX.Y.Z`: `github.com/dimmihq/dimmi-go` (the Go module path), `dimmihq/dimmi-php` and `dimmihq/dimmi-laravel` (Packagist).
