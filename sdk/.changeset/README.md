# Changesets

Every change to an SDK, or to the API contract the SDKs are generated
from, ships with a changeset: `pnpm changeset` at the repo root.

- `patch`: fixes, regenerated code with no new surface.
- `minor`: new API surface (new endpoints, fields, events, functions).
- `major`: a break. Only after `make sdk-compat` reports it and the owner
  approves; see docs/api/versioning.md.

The PHP, Laravel, Python and Go SDKs are versioned through the private
`package.json` beside them (`@dimmi/php-sdk`, `@dimmi/laravel-sdk`,
`@dimmi/python-sdk`, `@dimmi/go-sdk`); the release turns their versions
into `sdk/<name>/vX.Y.Z` tags and registry releases.
