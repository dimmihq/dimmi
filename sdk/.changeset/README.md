# Changesets

Changesets are optional. SDK versions are decided at each platform
release (`vX.Y.Z` tag) by `scripts/sdk-release-plan.ts`; see
docs/api/versioning.md. Add one with `pnpm changeset` at the repo root
only to:

- force a higher level for a package than the plan would pick, such as
  `minor` for a new framework option, or `major` for an approved break
  of an SDK's own surface;
- describe a change for the release notes.

A changeset applies to the first platform release after it is committed.
Changesets are never consumed or deleted by the release; delete old ones
whenever you like.

The PHP, Laravel, Python and Go SDKs are named by the private
`package.json` beside them (`@dimmi/php-sdk`, `@dimmi/laravel-sdk`,
`@dimmi/python-sdk`, `@dimmi/go-sdk`).
