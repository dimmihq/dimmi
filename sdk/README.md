# dimmi SDKs

Packages for putting dimmi, the open-source customer messenger, in any app, and for calling its API.

| Package                     | For                                                                                    |
| --------------------------- | -------------------------------------------------------------------------------------- |
| [`@dimmi/browser`](browser) | Any web page. Loader plus typed API, no framework, ~1.2 kB gzipped.                    |
| [`@dimmi/react`](react)     | React 18 and 19: `<DimmiProvider>`, `useDimmi()`, `useUnreadCount()`, `<DimmiButton>`. |
| [`@dimmi/nextjs`](nextjs)   | Next.js 14 and 15, with identity signed in a server action and auth adapters.          |
| [`@dimmi/vue`](vue)         | Vue 3 and Nuxt 3: `createDimmi()` plugin, `useDimmi()`, `useUnreadCount()`.            |
| [`@dimmi/angular`](angular) | Angular 17+: `provideDimmi()`, `DimmiService`. SSR-safe.                               |
| [`@dimmi/svelte`](svelte)   | Svelte and SvelteKit: `initDimmi()`, `unreadCount` store, `use:dimmiAction`.           |
| [`@dimmi/server`](server)   | Signing identity tokens on any JS server; Express and Hono helpers.                    |
| [`@dimmi/api`](api)         | Typed REST client: openapi-fetch over generated types, retries, pagination.            |

Every browser package is a thin wrapper over `@dimmi/browser`, which drives the messenger's `Dimmi(...)` commands. All of them do nothing during server rendering. Backend SDKs for other languages live beside these.

## Install in two steps

1. Add the messenger with your framework's package and your app id (`wgt_...`, Settings > Messenger) and host.
2. To verify signed-in users, sign a token on your server with `@dimmi/server` (or `@dimmi/nextjs/server`) and pass a function answering it as `identity`.

## Develop

```sh
pnpm sdk:build      # builds every package into sdk/*/dist (ESM, CJS, types)
pnpm sdk:test       # unit tests (vitest, jsdom)
pnpm sdk:generate   # regenerates sdk/api from tests/contracts/api_v1.openapi.json
pnpm exec tsc --noEmit -p tsconfig.sdk.json
```

`scripts/build-sdk.ts` bundles with vite, every import external, and fails when a bundle imports a package its `package.json` does not list.

## Regenerating after API changes

Only `@dimmi/api` has a generated layer (`sdk/api/src/schema.ts`, `sdk/api/src/paging.ts`). Both are committed.

```sh
pnpm sdk:generate && pnpm sdk:build api && pnpm sdk:test
```

## Compatibility

`COMPATIBILITY.md` lists the framework versions each package was built and run in.

## Compatibility rules

Public exports and options are never removed or renamed. Deprecate with a JSDoc `@deprecated` and keep the old name working.

## Backend SDKs

| Package | For |
| --- | --- |
| [`github.com/dimmihq/dimmi-go`](go) | Go 1.22+: REST client (oapi-codegen), identity tokens, webhook verification. |
| [`dimmi/dimmi-php`](php) | PHP 8.2+: Guzzle client over a generated operation table, identity tokens, webhooks. |
| [`dimmi/dimmi-laravel`](laravel) | Laravel 10, 11, 12: `Dimmi::` facade, `@dimmiMessenger`, webhook route and events, `dimmi:install`, `dimmi:test`. |
| [`dimmi-sdk`](python) (PyPI, `import dimmi`) | Python 3.10+: httpx client (openapi-python-client), identity tokens, webhooks. |

Each backend SDK sends the token and `Dimmi-Workspace` header, retries 429 and 503 after `Retry-After`, sends an `Idempotency-Key` with every POST, walks cursor and numbered pages, and raises typed errors from `{code, message, errors}`. Identity tokens follow `@dimmi/server` exactly. Webhooks are verified with the official Standard Webhooks libraries.

Regenerate a backend client after API changes (default spec: `tests/contracts/api_v1.openapi.json`):

```sh
cd sdk/go && scripts/generate.sh && go test ./...
cd sdk/php && bin/generate && vendor/bin/phpunit
cd sdk/python && scripts/generate.sh && .venv/bin/pytest   # see sdk/python/README.md for the venv
```
