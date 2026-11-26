# @dimmi/server

Sign dimmi identity tokens on your server, so your signed-in users are verified in the messenger. Runs on Node 20+, Bun, Deno, workers and Vercel edge, with no dependencies.

```sh
npm install @dimmi/server
```

Set `DIMMI_IDENTITY_SECRET` from Settings > Identity verification. `DIMMI_IDENTITY_KID` is optional for that HS256 secret: without it dimmi tries each of your secrets. For RS256, ES256 or EdDSA set `DIMMI_IDENTITY_PRIVATE_KEY` and `DIMMI_IDENTITY_KID` (required) instead.

Map your user to the fields dimmi knows: `id`, `email`, `emailVerified`, `name`, `phone`, `phoneVerified`, `avatarUrl`, `locale`, `timezone`, `signedUpAt`, `company`, `companies` (up to 50 more) and `attributes`. Anything else goes under `attributes`, with snake_case keys that start with a letter; any other field throws `user_field_unknown`, so never pass your user object as is.

```ts
import { createIdentityHandler, signIdentityToken } from "@dimmi/server";

const token = await signIdentityToken({ id: user.id, email: user.email, name: user.name });

// identify reads your session and returns a user or null.
async function identify(request: Request) {
  const user = await currentUser(request);
  return user ? { id: user.id, email: user.email, name: user.name, attributes: { plan: user.plan } } : null;
}

// A Fetch handler for GET /api/dimmi/token: (request: Request) => Promise<Response>.
const handler = createIdentityHandler(identify);
```

Next.js route handlers, Workers (`fetch(request)`), Bun (`Bun.serve({ fetch: handler })`) and Deno (`Deno.serve(handler)`) call it with a `Request`, so they take it as is:

```ts
// Next.js, app/api/dimmi/token/route.ts
export const GET = handler;
```

The other frameworks hand over an event or context, so pass its request:

```ts
// SvelteKit, src/routes/api/dimmi/token/+server.ts
export const GET = ({ request }) => handler(request);

// Astro, src/pages/api/dimmi/token.ts
export const GET = ({ request }) => handler(request);

// Remix and React Router resource routes: loader args carry the request
export const loader = ({ request }) => handler(request);

// Nuxt (h3), server/api/dimmi/token.get.ts
import { toWebRequest } from "h3";
export default defineEventHandler((event) => handler(toWebRequest(event)));
```

Express and Hono have their own adapters:

```ts
import { dimmiIdentity } from "@dimmi/server/express";
app.get(
  "/api/dimmi/token",
  dimmiIdentity((req) => (req.user ? { id: req.user.id, email: req.user.email, name: req.user.name } : null)),
);

import { dimmiIdentity as honoIdentity } from "@dimmi/server/hono";
app.get(
  "/api/dimmi/token",
  honoIdentity((c) => {
    const user = c.get("user");
    return user ? { id: user.id, email: user.email, name: user.name } : null;
  }),
);
```

The browser packages take the token through their `identity` option, or `identityUrl` pointed at that route.

## Regenerating after API changes

This package has no generated layer. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
