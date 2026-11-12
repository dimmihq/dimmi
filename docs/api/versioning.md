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
6. Optionally, `pnpm changeset` forces a higher SDK version level or adds a release note. Versions are otherwise decided at release (below).

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
- Announce in the changelog, and optionally in an SDK changeset.

## Shipping a break anyway

Only with a new API version, or, rarely, a reviewed exception:

- Add an entry to `contracts/breaking-changes.json` with `id` and `operation` (or `event`) from the `make api-compat` output, a `reason`, `deprecated` and `sunset` dates, and `approved_by`.
- The next platform release raises a major version of every SDK that exposes the API. An SDK whose own surface breaks needs a major changeset; `make sdk-compat` blocks it otherwise.

## Releases and baselines

- The baseline is the previous platform release: `make api-compat` compares with the contracts at the newest `vX.Y.Z` tag (prereleases are skipped); no copy is kept in the tree. CI also compares against the base commit.
- After a release, remove the `contracts/breaking-changes.json` entries it shipped. `make api-compat` lists entries that match nothing since the last release.

## SDK releases

SDKs ship with the platform. Pushing a `vX.Y.Z` tag runs `release.yml` and `.github/workflows/sdk-release.yml`. A prerelease tag (`v0.2.0-rc.1`) releases no SDK. There is no approval step.

`scripts/sdk-release-plan.ts` decides each SDK's release by comparing the tag with the previous platform release:

| Change since the previous release | SDK release |
| --- | --- |
| Never published | First release, at its manifest version |
| API contract and the SDK's source unchanged | None |
| Only the SDK's hand-written source changed | Patch |
| API changed additively (no oasdiff break) | Minor, for the SDKs that expose the API: `@dimmi/api`, Go, PHP, Laravel, Python |
| API changed with an approved break | Major, for the SDKs that expose the API |
| An SDK it depends on is released | At least patch, so its range follows |
| A changeset added since the previous release | At least the level it names |

`make sdk-release-plan` (or `TAG=v0.2.0`) prints the plan. Each SDK's previous version is what users already have: the latest on npm, on PyPI (`dimmi-sdk`), or tagged on its mirror.

The run then applies the versions, generates, builds and tests every SDK (`make sdk-check`), checks each against its last published release (`make sdk-compat`; a break fails unless the plan makes it major), and publishes:

- npm: `@dimmi/*`, with provenance.
- Go: `sdk/go`, generated code included, split into the read-only mirror `dimmihq/dimmi-go` and tagged `vX.Y.Z`. That is why the module path is `github.com/dimmihq/dimmi-go`.
- PHP and Laravel: split into `dimmihq/dimmi-php` and `dimmihq/dimmi-laravel`, tagged `vX.Y.Z`, which Packagist follows.
- Python: `dimmi-sdk` on PyPI (import name `dimmi`), through trusted publishing.

Versions are never committed back to `main`. A commit from the workflow would race the pushes to `main` and needs a token that branch protection allows. Instead, the registries and mirrors hold the versions and the next plan reads them from there. The run also records `sdk/<name>/vX.Y.Z` tags on the platform commit, as a record only.
