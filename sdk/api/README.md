# @dimmi/api

A typed client for the dimmi REST API. It is [openapi-fetch](https://openapi-ts.dev/openapi-fetch/) over types that [openapi-typescript](https://openapi-ts.dev) generates from the API's OpenAPI document. Runs anywhere `fetch` does: Node 20+, Bun, Deno, workers and browsers.

```sh
npm install @dimmi/api
```

## Usage

```ts
import { createDimmiApi, paginate, unwrap } from "@dimmi/api";

const api = createDimmiApi({
  host: "https://dimmi.example.com", // or baseUrl: "https://.../api/v1"
  token: process.env.DIMMI_TOKEN!, // a personal or app access token
  workspace: "ws_...", // optional: a token works in its own workspace; sent as Dimmi-Workspace
  autoIdempotency: true, // Idempotency-Key on every POST
});

const { data, error } = await api.GET("/contacts/{contact}", { params: { path: { contact: "ct_..." } } });

const created = await unwrap(api.POST("/contacts", { body: { email: "ada@example.com" } }));

for await (const contact of paginate(api, "/contacts", { params: { query: { limit: "100" } } })) {
  console.log(contact.email);
}
```

## What it adds to openapi-fetch

- `Authorization: Bearer <token>` on every call, and `Dimmi-Workspace` when `workspace` is set. A token works in its own workspace; naming another is refused.
- 429 answers are retried after `Retry-After` (seconds or a date); `maxRetries` (3) and `maxRetryDelay` (30 s) tune it.
- `autoIdempotency` adds an `Idempotency-Key` to POSTs; retries reuse it. Only [some endpoints](https://trydimmi.com/docs/developers/api/idempotency/) read the key. Pass your own per call with `headers: { "Idempotency-Key": key }`.
- `paginate(api, path, init)` walks every item; `pages()` every page. Lists page by `cursor` (`meta.next_cursor`), by `after`, or by `offset` (`meta.total`), whichever the endpoint uses.
- `unwrap(call)` answers `data` or throws a `DimmiApiError` with `status`, `code`, `errors`.
- Types: `paths`, `operations`, `components`, and `Schemas` (`Schemas["Contact"]`).

## Regenerating after API changes

The generated files are `src/schema.ts` (types) and `src/paging.ts` (how each list pages). Neither is committed (.gitignore): `make sdk-generate` writes them from the committed contract, and `pnpm sdk:build`, `pnpm sdk:test` and `pnpm typecheck` write them first when they are missing. `make sdk-check` generates, builds and tests every SDK.

```sh
pnpm sdk:generate                                        # from tests/contracts/api_v1.openapi.json
node sdk/api/scripts/generate.ts http://localhost:8080/api/v1/openapi.json   # from a running server
pnpm sdk:build api
```

The generator runs openapi-typescript through `pnpm dlx` with TypeScript 5, because the repo's TypeScript 7 has no compiler API.
