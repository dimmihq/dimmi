# @dimmi/server

Sign dimmi identity tokens on your server, so your signed-in users are verified in the messenger. Runs on Node 20+, Bun, Deno, workers and Vercel edge, with no dependencies.

```sh
npm install @dimmi/server
```

Set `DIMMI_IDENTITY_SECRET` (and `DIMMI_IDENTITY_KID`) from Settings > Identity verification. For RS256, ES256 or EdDSA set `DIMMI_IDENTITY_PRIVATE_KEY` and `DIMMI_IDENTITY_KID` instead.

```ts
import { createIdentityHandler, signIdentityToken } from "@dimmi/server";

const token = await signIdentityToken({ id: user.id, email: user.email, name: user.name });

// A Fetch handler for GET /api/dimmi/token: SvelteKit, Remix, Astro, Nuxt, workers, Bun, Deno.
export const GET = createIdentityHandler(async (request) => currentUser(request));
```

```ts
import { dimmiIdentity } from "@dimmi/server/express";
app.get(
  "/api/dimmi/token",
  dimmiIdentity((req) => req.user ?? null),
);

import { dimmiIdentity as honoIdentity } from "@dimmi/server/hono";
app.get(
  "/api/dimmi/token",
  honoIdentity((c) => c.get("user") ?? null),
);
```

The browser packages take the token through their `identity` option, or `identityUrl` pointed at that route.

## Regenerating after API changes

This package has no generated layer. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
