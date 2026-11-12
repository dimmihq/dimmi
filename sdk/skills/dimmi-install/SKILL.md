---

name: dimmi-install
description: Add the dimmi customer messenger to a web app (React, Next.js, Vue, Nuxt, Angular, Svelte/SvelteKit or plain HTML), verify signed-in users with server-signed identity tokens, and call the dimmi REST API with @dimmi/api. Use when someone asks to install dimmi, add the dimmi chat widget or messenger, identify users in dimmi, or call the dimmi API.
---

# Install dimmi in a project

## 1. Find out what you need

- The messenger app id, `wgt_...` (dimmi dashboard, Settings > Messenger), and the dimmi host (cloud address or self-hosted URL). Ask for them if they are not in the project's env or config. Put them in env vars, e.g. `NEXT_PUBLIC_DIMMI_APP_ID`, `VITE_DIMMI_APP_ID`.
- The framework: read `package.json`. Choose by dependency: `next` → Next.js; `nuxt` → Nuxt; `@angular/core` → Angular; `@sveltejs/kit` or `svelte` → Svelte; `vue` → Vue; `react` → React; otherwise plain HTML.
- Whether the app has signed-in users and which auth library (Clerk, Auth.js, Supabase, Better Auth, custom session).

Use the project's own package manager (look for `pnpm-lock.yaml`, `yarn.lock`, `bun.lockb`, `package-lock.json`).

## 2. Add the messenger once, at the app root

| Framework        | Package                         | Where                                                                                                   |
| ---------------- | ------------------------------- | ------------------------------------------------------------------------------------------------------- |
| React            | `@dimmi/react`                  | wrap the root in `<DimmiProvider appId host identity>`                                                  |
| Next.js          | `@dimmi/nextjs`                 | `<DimmiProvider>` in `app/layout.tsx`, identity from a server action                                    |
| Vue 3            | `@dimmi/vue`                    | `app.use(createDimmi({ appId, host }))` in `main.ts`                                                    |
| Nuxt 3           | `@dimmi/vue`                    | `plugins/dimmi.client.ts` (must end in `.client.ts`)                                                    |
| Angular 17+      | `@dimmi/angular`                | `provideDimmi({ appId, host })` in `app.config.ts`                                                      |
| Svelte/SvelteKit | `@dimmi/svelte`                 | `onMount(() => initDimmi({ appId, host }))` in the root `+layout.svelte`                                |
| Anything else    | `@dimmi/browser` or the snippet | `loadDimmi({ appId, host })` or `<script async src="<host>/widget/v1/loader.js" data-app-id="wgt_...">` |

Each package's README in `node_modules/@dimmi/<name>/README.md` has a complete example. Do not load the messenger twice (no snippet plus package).

## 3. Verify signed-in users (when the app has accounts)

Sign on the server, never in the browser:

1. `@dimmi/server` (or `@dimmi/nextjs/server`) with `DIMMI_IDENTITY_SECRET` and `DIMMI_IDENTITY_KID` in server env (never `NEXT_PUBLIC_`/`VITE_`).
2. Expose the token:
   - Next.js: `dimmi.config.ts` with `defineDimmi({ identify: clerk() })` (or `authjs`, `supabase`, `betterAuth` adapters, or your own `identify`), a `"use server"` action returning `identityToken(config)`, passed as `identity`.
   - Express: `app.get("/api/dimmi/token", dimmiIdentity((req) => req.user && { id: req.user.id, email: req.user.email }))` from `@dimmi/server/express`. Hono: `@dimmi/server/hono`.
   - SvelteKit, Remix, Astro, Nuxt (Nitro), workers: `createIdentityHandler(identify)` from `@dimmi/server` as a GET route.
3. In the browser pass `identity: async () => (await (await fetch("/api/dimmi/token")).json()).token`, or `identityUrl: "/api/dimmi/token"`.
4. After a sign-in without a page load call `identify()`; in the sign-out handler call `shutdown()`.

## 4. Useful calls

`show()`, `hide()`, `toggle()`, `showNewMessage("text")`, `update({ appearance: { position: "left", primary_color: "#0f766e" } })`, `onUnreadCountChange(fn)` / `useUnreadCount()`. For a strict CSP pass `nonce` and allow the dimmi host in `script-src` and `connect-src`. Behind a proxy pass `apiBase`.

`trackEvent()` is wired but the messenger does not run it yet; record events from the server with `@dimmi/api`: `api.POST("/contact-events", { body: { ... } })`.

## 5. Server-side API

```ts
import { createDimmiApi, paginate, unwrap } from "@dimmi/api";
const api = createDimmiApi({
  host: process.env.DIMMI_HOST!,
  token: process.env.DIMMI_TOKEN!,
  workspace: process.env.DIMMI_WORKSPACE,
});
const contact = await unwrap(api.GET("/contacts/{contact}", { params: { path: { contact: id } } }));
for await (const c of paginate(api, "/contacts")) {
  /* ... */
}
```

## 6. Check your work

- Build the app in production mode and load a page: a `#dimmi-messenger` element appears and the launcher shows.
- With identity on, the network shows `POST <host>/widget/v1/sessions` answering 201 after sign-in.
- Server rendering (Next.js, Nuxt, Angular SSR, SvelteKit) must not touch `window`; all dimmi packages are safe to import there.
- The browser console has no `[dimmi]` errors. `identity_endpoint_invalid` means your token route did not answer `{"token": ...}` JSON.
