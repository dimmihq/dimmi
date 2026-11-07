# @dimmi/server

Verify your signed-in users in the dimmi messenger. Your server answers one question, "who is signed in on this request?", and this package turns the answer into a short-lived signed token. No JWT code in your app, no dependencies, and it runs on Node, Bun, Deno, Cloudflare Workers and the Vercel edge.

```sh
npm install @dimmi/server
```

Create a secret in dimmi under **Settings > Identity verification** and put it in your server's environment:

```sh
DIMMI_IDENTITY_SECRET=dmi_idsec_...
DIMMI_IDENTITY_KID=isk_...
```

## Any Fetch API server

React Router and Remix resource routes, SvelteKit `+server.ts`, Astro endpoints, Workers, Bun and Deno:

```ts
import { createIdentityHandler } from "@dimmi/server";

export const GET = createIdentityHandler(async (request) => {
  const user = await getUser(request);
  return user && { id: user.id, email: user.email, emailVerified: user.emailVerified, name: user.name };
});
```

## Express

```ts
import { dimmiIdentity } from "@dimmi/server/express";

app.get(
  "/api/dimmi/token",
  dimmiIdentity((req) => req.user && { id: req.user.id, email: req.user.email }),
);
```

## Hono

```ts
import { dimmiIdentity } from "@dimmi/server/hono";

app.get(
  "/api/dimmi/token",
  dimmiIdentity((c) => {
    const user = c.get("user");
    return user && { id: user.id, email: user.email };
  }),
);
```

Then point the messenger at the route:

```html
<script src="https://your-dimmi/widget/v1/loader.js" data-app-id="wgt_..." data-identity-url="/api/dimmi/token" async></script>
```

## The user object

| Field                                                               | Notes                                                                                      |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ |
| `id`                                                                | Required. Your user's id, string or number.                                                |
| `email`, `emailVerified`                                            | `emailVerified: true` only when your app confirmed the address.                            |
| `name`, `phone`, `phoneVerified`, `avatarUrl`, `locale`, `timezone` | Optional.                                                                                  |
| `signedUpAt`                                                        | Date, epoch seconds or ISO string.                                                         |
| `company`                                                           | `{ id, name, domain, attributes }`.                                                        |
| `attributes`                                                        | Flat values under snake_case keys, at most 100. Your AI agent's actions can rely on these. |

## Options

`createIdentityHandler(identify, options)` and `signIdentityToken(user, options)` take `secret`, `kid`, `privateKey` (PKCS #8 PEM for RS256, ES256 or EdDSA), `expiresIn` (seconds, default 600, at most 3600), `audience` and `nonce`.

Never sign tokens in the browser, put them in your HTML or cache the answer: the route always answers `Cache-Control: no-store, private`.
