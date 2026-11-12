# AGENTS.md

Binding rules for anyone, human or agent, changing this repository.

## Never break the API or the SDKs

The REST API (`/api/v1`), the messenger API (`/widget/v1`), webhooks and every SDK in `sdk/` are public contracts. Customers' code runs against them unattended.

- Changes are additive only: new paths, optional parameters, optional request fields, response fields, events.
- Never remove or rename a path, operation id (route name), parameter, field, enum value, schema or webhook event. Never change a type. Never make something optional required.
- Never break an SDK's public surface: keep old functions, types and arguments; add new ones beside them.
- A break ships only in a new API version, or with an approved entry in `contracts/breaking-changes.json`; the next platform release then raises a major version of every SDK that exposes the API. See `docs/api/versioning.md`.

## SDK generated code is never committed

The committed source of truth for every SDK's types is `tests/contracts/api_v1.openapi.json` (and `webhooks_v1.json`). Each SDK's generated layer is gitignored and produced by `make sdk-generate`:

- `sdk/api/src/schema.ts`, `sdk/api/src/paging.ts`
- `sdk/go/api/api.gen.go`, `sdk/go/api/openapi30.json`
- `sdk/php/src/Generated/`
- `sdk/python/src/dimmi/_generated/`

Run `make sdk-generate` after cloning and after every contract change, before building or testing an SDK (`pnpm sdk:build`, `pnpm sdk:test`, `pnpm typecheck` and `pnpm test` generate `sdk/api` themselves when it is missing). Never hand-edit generated code; change the API or the generator.

## After changing routes, requests, resources or events

Run, in order, and commit what they change:

1. `make api-snapshot`: refreshes `tests/contracts/*.json` (the REST, messenger and webhook contracts).
2. `make api-compat`: must pass against the last release.
3. `make sdk-check`: generates every SDK, then builds it and runs its unit tests. Hand-written SDK code must keep working against the new contract. It fails when the PHP client's committed surface (`sdk/php/api-surface.json`) would drop or change an entry; for an intended change run `cd sdk/php && bin/generate --update-surface` and commit the diff.
4. `make sdk-compat`: must pass; no SDK surface broke against its last published release. `make sdk-compat-update` records an intended additive change to the TypeScript API reports.
CI (`.github/workflows/contracts.yml`) runs all of these.

## SDK releases

- SDKs ship with each platform release (`vX.Y.Z` tag, never a prerelease); `.github/workflows/sdk-release.yml` runs on the tag, with no approval step.
- Versions are decided for you (`scripts/sdk-release-plan.ts`, `make sdk-release-plan`): API unchanged and SDK source unchanged → not released; source changed → patch; API changed additively → minor for the SDKs that expose the API; approved break → major; a released dependency → patch for its dependents.
- A changeset (`pnpm changeset`) is optional: add one only to force a higher level for a package, or to say what changed. It applies to the next platform release.
- Versions are never committed: the registries (npm, PyPI) and the mirrors (`dimmi-go`, `dimmi-php`, `dimmi-laravel`) hold them.

## Deprecating

- Routes: `.Deprecated(routing.Deprecation{Since, Sunset, Link})`, sunset at least 12 months out. Responses then carry `Deprecation`, `Sunset` and `Link` headers, and the document marks the operation deprecated.
- Fields: add the replacement beside the old one and keep sending the old one.
- Removal happens only in a new API version, after the sunset.

## Never publish

- Never publish to npm, Packagist or PyPI, never push tags, and never push to the SDK mirrors (`dimmihq/dimmi-go`, `dimmi-php`, `dimmi-laravel`). Only `.github/workflows/sdk-release.yml` generates, splits and publishes, on a platform release tag.
- The last released contract is the one at the newest platform release tag (`vX.Y.Z`); `make api-compat` compares with it. After a release, remove the `contracts/breaking-changes.json` entries it shipped; `make api-compat` lists them as matching nothing.

## Repository conventions

- Commit straight to `main`; no feature branches or pull requests.
- Run only the tests related to a change, never the whole suite; CI runs everything.
- Never run anything that calls a real LLM without the owner's explicit approval for that run.
