# AGENTS.md

Binding rules for anyone, human or agent, changing this repository.

## Never break the API or the SDKs

The REST API (`/api/v1`), the messenger API (`/widget/v1`), webhooks and every SDK in `sdk/` are public contracts. Customers' code runs against them unattended.

- Changes are additive only: new paths, optional parameters, optional request fields, response fields, events.
- Never remove or rename a path, operation id (route name), parameter, field, enum value, schema or webhook event. Never change a type. Never make something optional required.
- Never break an SDK's public surface: keep old functions, types and arguments; add new ones beside them.
- A break ships only in a new API version, or with an approved entry in `contracts/breaking-changes.json` and a major SDK changeset. See `docs/api/versioning.md`.

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
3. `make sdk-check`: generates every SDK, then builds it and runs its unit tests. Hand-written SDK code must keep working against the new contract.
4. `make sdk-compat`: must pass; no SDK surface broke against its last published release. `make sdk-compat-update` records an intended additive change to the TypeScript API reports.
5. `pnpm changeset`: pick every SDK whose code or generated layer changed. `patch` for fixes, `minor` for new API surface, `major` only for an approved break.

`make sdk-changeset-check` confirms the changeset is there. CI (`.github/workflows/contracts.yml`) runs all of these.

## Deprecating

- Routes: `.Deprecated(routing.Deprecation{Since, Sunset, Link})`, sunset at least 12 months out. Responses then carry `Deprecation`, `Sunset` and `Link` headers, and the document marks the operation deprecated.
- Fields: add the replacement beside the old one and keep sending the old one.
- Removal happens only in a new API version, after the sunset.

## Never publish

- Never publish to npm, Packagist or PyPI, never push tags, and never push to the SDK mirrors (`dimmihq/dimmi-go`, `dimmi-php`, `dimmi-laravel`). `.github/workflows/sdk-release.yml` generates, splits and publishes after the owner approves the `sdk-release` environment.
- The last released contract is the newest `api-v*` git tag; `make api-compat` compares with it (or `origin/main` before the first release). Only a release creates that tag (`make api-baseline`).

## Repository conventions

- Commit straight to `main`; no feature branches or pull requests.
- Run only the tests related to a change, never the whole suite; CI runs everything.
- Never run anything that calls a real LLM without the owner's explicit approval for that run.
