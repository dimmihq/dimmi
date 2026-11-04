# 01 Messenger widget: SDK public contract

Style: caveman prose, code exact. Contract only. Frontend team implements (own JSX runtime and controllers, Shadow DOM, `resources/widget`, [client.md](client.md); packages in `sdk/` workspace). `tests/e2e/widget_protocol_test.go` exercises same handshake. Server identity helpers + raw per-language snippets: [02 snippets.md](../02-widget-identity/snippets.md). HTTP wire (`/widget/v1`, snake_case bodies) unchanged by this file; only SDK surface defined here. Anything published here is breaking to change: additive only inside v1.

## Before / after

Before (spec draft, rejected):

```tsx
'use client';
import { DimmiMessenger } from '@dimmi/react';
export function Support({ signedIn }: { signedIn: boolean }) {
  return (
    <DimmiMessenger
      appId="wgt_01jbd2k7m3q8r5t6v9w0x1y2z3"
      userJwt={signedIn ? () => fetch('/api/dimmi/token').then(r => r.json()).then(j => j.token) : null}
    />
  );
}
```

Plus hand-written `app/api/dimmi/token/route.ts` (40 lines of `jose`). Problems: id in code, JWT vocabulary in app code, fetch chain, signed-in ternary, `'use client'` wrapper, API route to maintain.

After (Next.js App Router + Clerk). `npx dimmi init` writes all of it:

```tsx
// app/layout.tsx
import { Dimmi } from '@dimmi/nextjs';

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        {children}
        <Dimmi />
      </body>
    </html>
  );
}
```

```ts
// dimmi.config.ts
import { defineDimmi } from '@dimmi/nextjs/config';
import { clerk } from '@dimmi/nextjs/clerk';

export default defineDimmi({
  identify: clerk(),
});
```

```ts
// next.config.ts
import type { NextConfig } from 'next';
import { withDimmi } from '@dimmi/nextjs/plugin';

const nextConfig: NextConfig = {};

export default withDimmi(nextConfig);
```

```bash
# .env.local
DIMMI_APP_ID=wgt_01jbd2k7m3q8r5t6v9w0x1y2z3
DIMMI_IDENTITY_SECRET=dmi_idsec_...
```

Open from anywhere, no JS, works in server components:

```tsx
<button data-dimmi="open">Help</button>
```

No API route, no fetch, no `'use client'`, no token in code, no id in code. Signed-in users verified, signed-out users anonymous, sign-out and account switch handled by SDK.

## Design rules (binding for every package)

1. **Ids and secrets live in env.** `DIMMI_APP_ID` (public), `DIMMI_IDENTITY_SECRET` (server only). Code never contains `wgt_` or `dmi_idsec_` strings. Framework packages read env; prop overrides exist for multi-widget apps only.
2. **Identity declared once, server side, as a user object.** `identify()` returns `{ id, email, emailVerified, name, ... }` or `null`. App code never says JWT, claim, `sub`, sign. Mapping to claims in 02 snippets.md.
3. **Token signed on server, fetched by SDK on demand, never inlined into HTML.** Inline tokens leak through CDN/page caches, bfcache, Turbo snapshots (previous user's token boots after sign-out). Source = Next server action or same-origin identity URL. Token in memory only.
4. **One line per app, no provider.** Component or tag mounts widget. Hooks and helpers talk to the page singleton (`window.Dimmi`), so they work in any component, island, portal, second React root, or non-React code.
5. **Same verbs on every stack.** `open`, `close`, `toggle`, `show(view)`, `startConversation`, `track`, `update`, `identify`, `logout`, `on`. Script tag, `@dimmi/browser`, React, Next all share names and payloads.
6. **Never silent.** Every misconfiguration gives one console line with code, plain sentence, fix, docs link. Server helpers log same codes server side.
7. **Static stays static.** Mount components never read request data during render (no `cookies()`/`headers()` in layout): marketing pages keep static/ISR rendering; identity resolved after hydration.
8. **SSR-safe, lazy, tiny.** Import never touches `window` at module scope. Calls on server are no-ops (dev warn `called_on_server`). npm packages ship loader only; UI core always loads from `apiBase` (widget updates without customer redeploy).

## Packages

| Package | Runtime | Exports | Client cost (gz) |
|---|---|---|---|
| script tag `/widget/v1/loader.js` | browser | global `Dimmi`; auto-boot from `data-*` attributes | at most 3 KB |
| `@dimmi/browser` | browser, SSR-safe import | `dimmi` (typed singleton), `boot()`, types. Same code as loader | at most 3 KB (same loader) |
| `@dimmi/react` | React 18/19 | `<Dimmi />`, `useDimmi()`, `useDimmiEvent()` | +1 KB over browser |
| `@dimmi/nextjs` | Next.js 14.2+ (App + Pages Router) | `<Dimmi />` (server component), `useDimmi`, `useDimmiEvent`; `/config` (`defineDimmi`), `/plugin` (`withDimmi`), `/server` (`identityToken`, `portalLink`), `/portal-sso` (14 SSO route), adapters `/clerk`, `/authjs`, `/supabase`, `/better-auth`; `/pages` (Pages Router component), `/pages-api` (Pages Router token route) | +1.5 KB over browser; server code zero client bytes |
| `@dimmi/server` | Node 18+, Bun, Deno, Workers, Vercel Edge (Web Crypto) | `createIdentityHandler()` (Fetch `Request -> Response`), `signIdentityToken()`, `createPortalLink()`, `createPortalSsoHandler()` (14); subpaths `/express`, `/hono` | n/a (server) |

Per-stack packages join these in 02 tasks M6: `@dimmi/sveltekit`, `@dimmi/nuxt`, `@dimmi/vue`, `@dimmi/astro`, gem `dimmi`, composer `dimmi/laravel` + `dimmi/dimmi`, pip `dimmi`, Go `github.com/dimmihq/dimmi-go`, `@dimmi/server/fastify`, `@dimmi/server/supabase`, `@dimmi/react/supabase` (02 snippets.md "Per-stack packages"). Until each ships: script tag or `@dimmi/browser` + raw identity snippet or `@dimmi/server` Fetch handler. Native and headless: `@dimmi/headless`, `@dimmi/react-native`, Swift `Dimmi`, Maven `com.trydimmi:dimmi-android` ([20](../20-native-sdks/README.md)). Per-stack packages: later (identity-helper-packages).

Rules:
- React Router v7 / Remix use `@dimmi/react` + `@dimmi/server` (no separate package: route changes go through `history`, already detected).
- All JS packages: ESM + CJS, `sideEffects: false`, full `.d.ts`, `exports` map with `react-server`/`browser`/`node`/`worker` conditions. No runtime deps except framework peers. Server entry imports `server-only` where framework supports it.
- `@dimmi/nextjs` ships one `'use server'` module (token action). Precedent: `@clerk/nextjs` ships `'use server'` in package. `withDimmi` adds package to `transpilePackages`.
- Every package sends `sdk: {name, version}` on `POST /widget/v1/sessions` (`dimmi-browser`, `dimmi-react`, `dimmi-nextjs`, others per package).
- Retired names (never publish): `@dimmi/messenger`, `@dimmi/next`, `@dimmi/svelte`, `@dimmi/widget`, `DimmiMessenger`, boot option `user_jwt`.

## Environment

| Var | Where | Required | Notes |
|---|---|---|---|
| `DIMMI_APP_ID` | server env (Next reads it server side; other server-rendered stacks render it into the script tag themselves) | yes | `wgt_...`, public. Vite SPAs: `VITE_DIMMI_APP_ID` (client must see it at build). Next also accepts `NEXT_PUBLIC_DIMMI_APP_ID` (fallback, not needed) |
| `DIMMI_IDENTITY_SECRET` | server only | for identity | `dmi_idsec_...`, used as-is. Helpers refuse to start signing when same value found in any `NEXT_PUBLIC_`/`VITE_`/`PUBLIC_`/`NUXT_PUBLIC_` var (error `identity_secret_public`, `withDimmi` fails build) |
| `DIMMI_IDENTITY_KID` | server only | no | `isk_...`; set as JWT `kid`. Absent: server tries every active HS256 secret (02) |
| `DIMMI_IDENTITY_PRIVATE_KEY` | server only | no | PEM PKCS#8 for RS256/EdDSA (02 uploaded public key). Requires `DIMMI_IDENTITY_KID`. Both this and secret set: error `identity_key_conflict` |
| `DIMMI_API_BASE` | server env | no | self-host URL or proxy path. Default `https://app.trydimmi.com` |
| `DIMMI_PORTAL_URL` | server only | no | feedback portal base for `createPortalLink`, `portalLink`, `/api/dimmi/sso` return check (14) |

## Install by stack (exact text served by install endpoint)

`GET /api/v1/widget-settings/{widget}/install?framework=<id>` renders these blocks (templates `resources/widget/install/<id>.tmpl`; `{{.AppID}}` filled in env lines only). Framework ids equal 09 `dimmi init` ids. Server-side identity variants and adapters: 02 snippets.md.

### Next.js App Router (`nextjs-app`), golden path

```bash
npm i @dimmi/nextjs
```

Files exactly as in "Before / after". Without auth library or before identity is set up, skip `dimmi.config.ts`: widget runs anonymous.

How it works:
- `<Dimmi />` = synchronous server component. Reads `DIMMI_APP_ID`, `DIMMI_API_BASE` and `dimmi.config.ts` options (appearance, context). Never reads request data, so layout stays static. Renders tiny client component + passes token server action.
- `withDimmi(nextConfig, options?)`: aliases `dimmi:config` to `./dimmi.config.{ts,js,mjs}` (root, then `src/`) for webpack and Turbopack (`turbopack.resolveAlias`); sets build flag so `<Dimmi />` knows plugin present; optional `tunnel` rewrite (proxy section); fails build on `identity_secret_public`.
- Token action (`'use server'`, inside package): runs `identify()` (adapters use `auth()`/`headers()`/`cookies()` inside action, request-scoped), signs 10 minute token with `DIMMI_IDENTITY_SECRET`, returns `{ token: string | null }`. Same-origin enforced by Next action Origin check. Never cached.
- Client calls action at idle (in parallel with config GET; widget core is lazy anyway, so no visible latency) and on identity check triggers (Identity sources section).
- `output: 'export'` (no server): server actions unavailable. `<Dimmi />` runs anonymous, warns `server_actions_unavailable` with fix (identity URL on separate backend).

Custom identity (any session library, own DB):

```ts
// dimmi.config.ts
import { defineDimmi } from '@dimmi/nextjs/config';
import { getSession } from '@/lib/session';

export default defineDimmi({
  async identify() {
    const session = await getSession();
    if (!session) return null;
    return {
      id: session.user.id,
      email: session.user.email,
      emailVerified: session.user.emailVerifiedAt != null,
      name: session.user.name,
      company: { id: session.org.id, name: session.org.name },
      attributes: { plan: session.org.plan },
    };
  },
});
```

Use anywhere in client components:

```tsx
'use client';
import { useDimmi } from '@dimmi/nextjs';

export function UpgradeHelp() {
  const { startConversation, unreadCount } = useDimmi();
  return (
    <button onClick={() => startConversation('I have a question about upgrading')}>
      Ask us {unreadCount > 0 && `(${unreadCount})`}
    </button>
  );
}
```

`defineDimmi` options:

| Option | Type | Default | Notes |
|---|---|---|---|
| `identify` | `() => Awaitable<DimmiUser \| null \| undefined \| false>` | none (anonymous) | falsy = signed out. Thrown error: logged `identify_failed`, treated as signed out |
| `appearance` | `DimmiAppearance` | dashboard settings | per-site override, see Types |
| `context` | `DimmiContext` | `{}` | static page context (e.g. `app_version`) |
| `identityCheck` | `'navigation' \| 'load'` | `'navigation'` | `load`: check only on page load, focus and `identify()` (saves one action call per client navigation) |

Escape hatch without plugin (monorepo tooling that blocks aliases): pass own action.

```ts
// app/dimmi-token.ts
'use server';
import { identityToken } from '@dimmi/nextjs/server';
import { clerk } from '@dimmi/nextjs/clerk';

export const dimmiToken = identityToken(clerk());
```

```tsx
<Dimmi identity={dimmiToken} />
```

### Next.js Pages Router (`nextjs-pages`)

```tsx
// pages/_app.tsx
import type { AppProps } from 'next/app';
import { Dimmi } from '@dimmi/nextjs/pages';

export default function App({ Component, pageProps }: AppProps) {
  return (
    <>
      <Component {...pageProps} />
      <Dimmi />
    </>
  );
}
```

```ts
// pages/api/dimmi/token.ts
export { default } from '@dimmi/nextjs/pages-api';
```

Same `dimmi.config.ts` + `withDimmi`; adapters detect Pages API context (Clerk `getAuth(req)`, etc.). App id inlined at build by `withDimmi` (`env` entry), so `DIMMI_APP_ID` still enough.

### React SPA: Vite, CRA, TanStack Router (`react`)

```bash
npm i @dimmi/react
```

```tsx
// src/App.tsx
import { Dimmi } from '@dimmi/react';
import { useAuth } from './auth';

export default function App() {
  const { user } = useAuth();
  return (
    <>
      <Routes />
      <Dimmi
        appId={import.meta.env.VITE_DIMMI_APP_ID}
        identityUrl="/api/dimmi/token"
        userId={user?.id}
      />
    </>
  );
}
```

- `identityUrl`: same-origin GET endpoint (backend below). Cookies sent (`credentials: 'same-origin'`). Vite dev: covered by usual `server.proxy['/api']`.
- `userId`: who app thinks is signed in. Hint only, never sent to Dimmi, never trusted. Change triggers token fetch; `null` after a value signs widget out at once (no wait for next check). Omit and SDK still checks on load, navigation, focus.
- No backend yet: drop `identityUrl` and `userId`, widget anonymous.

Backend, Express:

```ts
// server/index.ts
import express from 'express';
import { identityHandler } from '@dimmi/server/express';

const app = express();

app.get('/api/dimmi/token', identityHandler((req) => req.user && {
  id: req.user.id,
  email: req.user.email,
  emailVerified: req.user.emailVerified,
  name: req.user.name,
}));
```

Backend, Hono (Workers, Bun, Node):

```ts
import { Hono } from 'hono';
import { identityHandler } from '@dimmi/server/hono';
import type { User } from './auth';

const app = new Hono<{ Variables: { user: User | null } }>();

app.get('/api/dimmi/token', identityHandler((c) => {
  const user = c.get('user');
  return user && { id: user.id, email: user.email, name: user.name };
}));
```

Anything else speaking Fetch API: `createIdentityHandler(identify)` from `@dimmi/server` returns `(request: Request) => Promise<Response>`. Fastify and non-JS backends: 02 raw snippets.

Bearer-token auth (Supabase, Firebase: no session cookie). Pass identity source object instead of URL (`@dimmi/react/supabase` wraps this, 02 snippets.md):

```tsx
// src/App.tsx (Supabase)
import { Dimmi } from '@dimmi/react';
import { supabase } from './supabase';

const supabaseIdentity = {
  getToken: async () => (await supabase.functions.invoke('dimmi-token')).data?.token ?? null,
  subscribe: (onChange: () => void) => {
    const { data } = supabase.auth.onAuthStateChange(() => onChange());
    return () => data.subscription.unsubscribe();
  },
};

export default function App() {
  return (
    <>
      <Routes />
      <Dimmi appId={import.meta.env.VITE_DIMMI_APP_ID} identity={supabaseIdentity} />
    </>
  );
}
```

Edge Function `dimmi-token`: 02 raw edge (`jose`) snippet; verify user first with `supabase.auth.getUser()` from `Authorization` header, signed out = `{"token": null}`. Secret: `supabase secrets set DIMMI_IDENTITY_SECRET=...`.

### React Router v7 / Remix (`react-router`)

```tsx
// app/root.tsx
import { Dimmi } from '@dimmi/react';
import { Links, Meta, Outlet, Scripts, ScrollRestoration, useRouteLoaderData } from 'react-router';

export function Layout({ children }: { children: React.ReactNode }) {
  const data = useRouteLoaderData<typeof loader>('root');
  return (
    <html lang="en">
      <head>
        <Meta />
        <Links />
      </head>
      <body>
        {children}
        <Dimmi
          appId={import.meta.env.VITE_DIMMI_APP_ID}
          identityUrl="/api/dimmi/token"
          userId={data?.user?.id}
        />
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}
```

```ts
// app/routes/api.dimmi.token.ts
import { createIdentityHandler } from '@dimmi/server';
import { getUser } from '~/auth.server';
import type { Route } from './+types/api.dimmi.token';

const handler = createIdentityHandler(async (request) => {
  const user = await getUser(request);
  return user && { id: user.id, email: user.email, emailVerified: user.emailVerified, name: user.name };
});

export const loader = ({ request }: Route.LoaderArgs) => handler(request);
```

Remix v2: same, imports from `@remix-run/react`.

### Other stacks (`sveltekit`, `nuxt`, `vue`, `astro`, `rails`, `laravel`, `django`, `fastapi`, `go`)

Package path replaces this text once 02 M6 ships each package. Install text = script tag (`html` block below, or `@dimmi/browser` in Vite SPAs) in root layout + identity route from 02 snippets.md.

| Id | Tag goes in | Identity route `GET /api/dimmi/token` |
|---|---|---|
| `sveltekit` | `src/app.html` or `+layout.svelte` `<svelte:head>` | `src/routes/api/dimmi/token/+server.ts`: `export const GET = ({ request }) => handler(request)` with `@dimmi/server` `createIdentityHandler` |
| `nuxt` | `app.head.script` in `nuxt.config.ts` | `server/api/dimmi/token.get.ts`: `createIdentityHandler` via `toWebRequest(event)` |
| `vue` | `index.html` or `@dimmi/browser` `boot()` in `main.ts` | backend: any `@dimmi/server` handler (React section) |
| `astro` | base layout `<script is:inline>` tag | `src/pages/api/dimmi/token.ts` (server output): `export const GET = ({ request }) => handler(request)`. Static site: no identity, anonymous widget |
| `rails` | `application.html.erb` before `</body>` (add CSP nonce yourself) | 02 raw Rails (ruby-jwt) controller |
| `laravel` | root Blade layout | 02 raw PHP snippet in `routes/web.php` (no `auth` middleware) |
| `django` | `templates/base.html` | 02 raw Python (PyJWT) Django view |
| `fastapi` | Jinja base template | 02 raw Python (PyJWT) FastAPI route |
| `go` | base `html/template` | 02 raw Go (golang-jwt) handler |

All use `data-identity-url="/api/dimmi/token"`. SPA navigation (Turbo, Inertia, Livewire, view transitions) detected via `history` patch.

### Plain HTML or any backend (`html`)

```html
<script async src="https://app.trydimmi.com/widget/v1/loader.js" data-app-id="{{.AppID}}"></script>
```

With identity endpoint on your server (02 snippets.md: any language, about 20 lines):

```html
<script
  async
  src="https://app.trydimmi.com/widget/v1/loader.js"
  data-app-id="{{.AppID}}"
  data-identity-url="/api/dimmi/token"
></script>
```

Call API before loader runs (optional queue stub, one line):

```html
<script>window.Dimmi=window.Dimmi||function(){(Dimmi.q=Dimmi.q||[]).push(arguments)}</script>
```

Self-host: replace `https://app.trydimmi.com` with own base URL (generator fills from `app.url`).

### First-party proxy (ad-blockers, strict networks)

Next.js: `withDimmi(nextConfig, { tunnel: true })` adds rewrite `/_dimmi/:path*` to API base and sets `apiBase: '/_dimmi'`. Other stacks: rewrite yourself, pass `apiBase` / `data-api-base="/_dimmi"`.

```js
// any proxy: route these to https://app.trydimmi.com
/_dimmi/widget/*        /_dimmi/connection/uni_sse   (SSE: proxy must stream, not buffer)
```

Proxy that buffers responses (no streaming): SDK falls back to polling automatically (no first SSE frame in 10 s).

### Intercom shim (16 switchers)

Loader installs `window.Intercom` (function + queue `Intercom.q`) when config `features.intercom_shim` or boot option `intercomShim: true`. Existing Intercom snippet calls keep working while customer moves to Dimmi. Mapping table owned by 16 (`16-importers/sources.md` "Intercom shim mapping"); SDK names below are binding.

| Intercom call | Dimmi |
|---|---|
| `boot(settings)` | `boot({ appId, userId: user_id, visitor: { name, email } })` (app id from Dimmi tag; `settings.app_id` ignored). `user_id` -> `userId` hint + unverified `claimed_user_id` (02 R5); `visitor` = unverified hints. Identity source configured: verified identity wins. `company`, `created_at`, custom attributes ignored (browser never writes contact attributes, 02 R8): one `intercom_attributes_ignored` info pointing to `identify()` attributes |
| `update(attrs)` | `update({})` (ping); `user_id` changed -> `identify()`; other attrs ignored (same rule as boot) |
| `shutdown` | `logout()` (Intercom shutdown = clear visitor session on sign-out) |
| `show`, `hide`, `showMessages` | `open()`, `close()`, `show('messages')` |
| `showNewMessage(text?)` | `startConversation(text)` |
| `showArticle(id)` | `GET /widget/v1/help/articles?filter[intercom_id]=<id>` -> `show('help', {article})`; 404: `show('help')` + `article_not_found` warn |
| `onShow(fn)`, `onHide(fn)`, `onUnreadCountChange(fn)` | `on('open')`, `on('close')`, `on('unread_count_changed', ({count}) => fn(count))` |
| `getVisitorId()` | `visitorId` |
| `trackEvent`, `startTour`, `startSurvey`, `startChecklist`, other | no-op + `intercom_method_unsupported` info |
| `user_hash` setting | ignored + `intercom_user_hash_ignored` warn (16 open question 1) |

Never verifies identity (verified only through identity sources). Never writes contact attributes. Real Intercom loader present: no overwrite, `intercom_shim_conflict` warn. Shim code lazy chunk `intercom-shim` (under 2 KB gz), loaded only when enabled.

### Portal SSO helpers (14 feedback portal)

Server-only; binding text in 02 snippets.md "Feedback portal SSO links". Portal token = 02 identity JWT with `aud: "dimmi-portal"` (portal surface rejects widget tokens; widget rejects portal-only tokens: 02 `token_audience_mismatch`), same identity secret/key as widget, `exp = iat + 300`, random `nonce` (single use), claims from `identify()` user object. Token minted on click by host SSO route, travels only in URL fragment: `{DIMMI_PORTAL_URL}/feedback/sso#token=<jwt>&return_to=/feedback/posts/12`. Never in page HTML, query or logs.

| Helper | Package | Does |
|---|---|---|
| `portalLink({ returnTo? })` | `@dimmi/nextjs/server` | returns host link `/api/dimmi/sso?return_to=<path>` (no token) |
| `GET` route `/api/dimmi/sso` | `@dimmi/nextjs/portal-sso` (`app/api/dimmi/sso/route.ts`: `export { GET } from '@dimmi/nextjs/portal-sso'`) | recommended portal `sso_login_url`. Signed in (`identify()` user): mint, 302 to portal fragment URL. Signed out: 302 to host login (`loginUrl`, default `/login`) with return back to route |
| `createPortalLink({ returnTo? })` | `@dimmi/server` | same link shape as `portalLink` for non-Next servers |
| `createPortalSsoHandler(identify, { loginUrl })` | `@dimmi/server` (Fetch handler; `/express`, `/hono` adapters) | same route contract as Next |

`return_to`: relative portal path, or absolute URL under `DIMMI_PORTAL_URL` (portal "Sign in" button sends absolute; helper converts to path); anything else ignored (open redirect guard). Other stacks (no package at MVP, later: identity-helper-packages): 02 snippets.md raw route, same contract.

## Identity sources

How SDK gets token. Exactly one per page; set by framework package or boot option.

| Source | Set by | Request |
|---|---|---|
| server action | `@dimmi/nextjs` App Router | `{ token }` from package action |
| identity URL | `identityUrl` / `data-identity-url` (script tag on any stack) | `GET <url>`, `credentials: 'same-origin'`, `Accept: application/json`, `cache: 'no-store'` |
| identity function | `identity: () => Promise<string \| null>` | your code |
| identity source object | `identity: { getToken, subscribe? }` (e.g. Supabase, React section) | your code; `subscribe(onChange)` pushes auth changes |
| none | | anonymous visitor |

Endpoint contract (02 snippets.md binding): `200 {"token": "<jwt>"}` signed in, `200 {"token": null}` signed out, `Cache-Control: no-store, private`, no CORS headers. Anything else (redirect to login page, HTML, 404, non-JSON) = signed out + `console.error` `identity_endpoint_invalid` naming status and content type.

When SDK asks (deduped, one in flight):
- Page load: after config GET starts, before handshake (handshake waits for answer, max 5 s; timeout = signed out for this load).
- Client navigation (history change, debounce 1 s): every navigation while identified (fast sign-out detection); at most once per 30 s while anonymous. `identityCheck: 'load'` disables.
- Tab visible again after 60 s hidden.
- `userId` hint change; `subscribe` callback; `window.Clerk` session change when Clerk present (feature detect `Clerk.addListener`, no import).
- `identify()` call.
- 401 `session_expired`/`token_expired` from Dimmi.

What SDK does with answer (01 README "Boot handshake" binding):
- Page load: always one handshake request per README table (identified resume needs fresh token, 02 R6).
- Mid-session check, token for same user as live session (SHA-256 of decoded `sub` equals `external_id_hash`), claims hash unchanged, session not within 1 h of expiry: nothing (no Dimmi request).
- Mid-session check, same user, claims changed or near expiry: `PATCH /widget/v1/session`.
- Token, other user or anonymous stored: `POST /widget/v1/sessions` with `previous_token`.
- `null`, stored identified session: `DELETE /widget/v1/session`, wipe, remount anonymous (02 R6).
- Source error (network, 5xx, action rejected): on page load treated as `null` (fail closed: shared device never keeps previous user); mid-session keeps state, retries on next trigger, `console.warn` `identity_source_failed`.

Token lifetime: helpers sign `exp = now + 10 minutes` (fetched on demand, so short is free). Identified session lifetime 24 h (02) extended by any later token. Tokens never stored (memory only), never logged, never put in URL.

## Client API

### `useDimmi()` (React, Next), `dimmi` (`@dimmi/browser`)

Same object everywhere. React: `useSyncExternalStore` over page singleton; component re-renders on state change only. Methods stable (safe to destructure, safe in deps arrays). Server render: state defaults, methods no-op.

```tsx
'use client';
import { useDimmi, useDimmiEvent } from '@dimmi/react';

export function Support() {
  const { ready, isOpen, unreadCount, show, toggle, track } = useDimmi();

  useDimmiEvent('message_received', ({ conversationId }) => {
    track('support_reply_seen', { conversationId });
  });

  return (
    <nav>
      <button onClick={toggle} disabled={!ready}>{isOpen ? 'Close' : 'Support'} ({unreadCount})</button>
      <button onClick={() => show('changelog')}>What's new</button>
      <button onClick={() => show('feedback', { board: 'ideas' })}>Request a feature</button>
    </nav>
  );
}
```

### Views (`show(view, options?)`)

| View | Options | Opens | Owner |
|---|---|---|---|
| `home` | | panel home | 01 |
| `messages` | | conversation list | 01 |
| `conversation` | `{ id: 'conv_...' }` | thread (own only; other: list + "not found" toast) | 01 |
| `help` | `{ article?: 'art_...', query?: string }` | Help tab, article or search | 08 |
| `changelog` | `{ entry?: 'cle_...' }` | "What's new" tab or entry | 13 |
| `feedback` | `{ post?: number, board?: string }` | Feedback tab, post or board | 14 |
| `roadmap` | | roadmap inside Feedback tab | 14 |
| `announcement` | `{ id: 'ann_...' }` | announcement (bypasses triggers, keeps audience) | 15 |
| `survey` | `{ id: 'svy_...' }` | survey | 15 |

View disabled in workspace (tab off, entity not live, not in audience): no-op + `console.warn` `view_unavailable` with reason. Unknown view: `console.error` `unknown_view`. Lazy chunk per view loads on first `show`.

### Methods and state

| Member | Type | Does |
|---|---|---|
| `ready` | `boolean` | core loaded, config fetched |
| `isOpen` | `boolean` | panel open |
| `view` | `DimmiViewName \| null` | current panel view |
| `unreadCount` | `number` | unread team messages (+ changelog dot handled separately: `changelogUnread`) |
| `changelogUnread` | `number` | unread "What's new" entries (13) |
| `identity` | `'pending' \| 'anonymous' \| 'verified' \| 'failed'` | `failed` = token rejected (console has reason) |
| `visitorId` | `string \| null` | `wss_` id, null before first write |
| `open()` | `Promise<void>` | open at last view (default `home`) |
| `close()`, `toggle()` | `Promise<void>` | |
| `show(view, options?)` | `Promise<void>` | table above |
| `startConversation(message?, options?)` | `Promise<void>` | composer with draft; `options.context` attached to conversation (`source`), e.g. `{ announcement_id }` (15). Never sends by itself |
| `track(event, properties?)` | `void` | client-only (15): no network, no storage. Matches cached engage items with `trigger.on = track_event`. Name `[a-z0-9_.:-]{1,64}`; properties flat scalars, max 1 KB, ignored. Invalid name: `console.error` `invalid_event_name` |
| `update(options)` | `void` | `{ context?, locale?, appearance? }`; merges, sends ping when session exists |
| `identify()` | `Promise<IdentityStatus>` | re-ask identity source now (after login without navigation) |
| `logout()` | `Promise<void>` | `DELETE /widget/v1/session`, wipe `dimmi:{app}:*`, close realtime stream, wipe memory, remount anonymous. Never shows previous history. Usually unnecessary (sign-out detected) |
| `on(event, handler)` | `() => void` | returns unsubscribe |

### Declarative attributes (no JS, any stack, server components)

| Attribute | Click does |
|---|---|
| `data-dimmi="open"` / `"close"` / `"toggle"` | method |
| `data-dimmi="show"` + `data-dimmi-view="changelog"` (+ `data-dimmi-id="cle_..."`) | `show(view, {id/entry/post/article})`; id mapped to view's option |
| `data-dimmi="message"` + `data-dimmi-message="Hi, ..."` | `startConversation(message)` |

SDK sets `data-dimmi-unread="3"` on `data-dimmi` elements (CSS badge: `[data-dimmi-unread]::after { content: attr(data-dimmi-unread) }`) and `aria-expanded`. Event delegation on `document`, one listener. Replaces old `launcher_selector`. Hide default launcher: `appearance: { launcher: false }`.

### Global API (script tag, non-React code)

`Dimmi(command, ...args)` queues until loader runs, then calls method of same name. After loader: `Dimmi.open()`, `Dimmi.show('changelog')` etc. return Promises; `Dimmi.ready` Promise resolves on `ready`. `@dimmi/browser` exports same object typed as `dimmi`.

```js
Dimmi('show', 'feedback', { post: 12 });
Dimmi('track', 'export_completed');
Dimmi('on', 'engage_action', ({ eventName }) => {
  if (eventName === 'open_upgrade') openUpgradeDialog();
});
```

Lifecycle commands (framework packages call these; apps rarely do): `boot(options)`, `shutdown()` (unmount UI, close realtime stream, keep storage; not logout). Second `boot` same `appId` = `update` (warn `boot_twice` only when not from framework package). Other `appId` = shutdown then boot. React StrictMode double mount: unmount defers shutdown one macrotask; remount cancels it.

### Boot options (`boot()`, `<Dimmi>` props)

Framework components accept same names as props (camelCase everywhere in JS; wire stays snake_case).

| Option | Type | Default | Notes |
|---|---|---|---|
| `appId` | `string` | required (framework packages read env) | `wgt_...`. Script tag: `data-app-id` |
| `apiBase` | `string` | script origin / `DIMMI_API_BASE` | proxy path or self-host URL. `data-api-base` |
| `identityUrl` | `string` | none | see Identity sources. `data-identity-url` |
| `identity` | `() => Promise<string \| null>` or `{ getToken, subscribe? }` | none | wins over `identityUrl` |
| `userId` | `string \| number \| null` | undefined | hint only (Identity sources) |
| `visitor` | `{ name?, email? }` | none | unverified hints for anonymous visitors (02 R5). Wire field `user` |
| `context` | `DimmiContext` | `{}` | page context, max 4 KB, 30 keys, flat scalars |
| `locale` | `string` | browser | BCP 47 |
| `appearance` | `DimmiAppearance` | dashboard | per-page override; never persisted |
| `trackRoutes` | `boolean` | `true` | SPA route detection |
| `engage` | `boolean` | `true` | `false`: no announcements/surveys on this page (15) |
| `intercomShim` | `boolean` | widget setting `intercom_shim` | install `window.Intercom` shim (16 switchers). Script tag: `data-intercom-shim` |
| `testToken` | `string` | none | dashboard preview only (`POST /widget/v1/sessions {test_token}`) |
| `transferToken` | `string` | none | hosted page only, from `#dimmi-transfer` fragment |
| `mode` | `'launcher' \| 'page'` | `'launcher'` | `page`: full viewport, no launcher (hosted page). Script tag: `data-mode` |
| `debug` | `boolean` | dev: `true` | verbose logs (never tokens). Dev = `NODE_ENV !== 'production'` in packages; script tag: `localhost`, `127.0.0.1`, `*.localhost`, `*.test` |

### Events

Names snake_case strings; payload keys camelCase. No message bodies, no answer values (privacy on host analytics).

| Event | Payload | Owner |
|---|---|---|
| `ready` | `{ appId, visitorId, identity }` | 01 |
| `open` / `close` | `{ view }` / `{}` | 01 |
| `view_changed` | `{ view }` | 01 |
| `unread_count_changed` | `{ count }` | 01 |
| `message_received` | `{ conversationId, messageId, authorType: 'member' \| 'agent' \| 'system' }` | 01 |
| `message_sent` | `{ conversationId, messageId }` | 01 |
| `conversation_started` | `{ conversationId }` | 01 |
| `identity_changed` | `{ status: 'anonymous' \| 'verified' \| 'signed_out' \| 'failed', userHash: string \| null }` | 01/02 (`userHash` = SHA-256 hex of `sub`) |
| `changelog_unread_changed` | `{ count }` | 13 |
| `announcement_shown` / `announcement_dismissed` | `{ id }` | 15 |
| `announcement_clicked` | `{ id, ctaId }` | 15 |
| `engage_action` | `{ id, ctaId, eventName }` (CTA `sdk_event`) | 15 |
| `survey_shown` | `{ id }` | 15 |
| `survey_completed` | `{ id, responseId }` | 15 |
| `error` | `{ code, message, docsUrl }` (same as console) | 01 |

### Deep links (fragment, read on boot and `hashchange`, then removed via `history.replaceState`)

| Fragment | Opens |
|---|---|
| `#dimmi-conversation=conv_...` | own conversation; not own or no session: "Reply to the email to continue" (offline email link, 01) |
| `#dimmi-open=<view>` | any view without id (`changelog`, `feedback`, `roadmap`, `help`, `messages`) (14) |
| `#dimmi-announcement=ann_...` / `#dimmi-survey=svy_...` | 15 |

### Types (published in every JS package)

```ts
export type DimmiScalar = string | number | boolean | null;
export type DimmiContext = Record<string, DimmiScalar>;

export interface DimmiUser {
  id: string | number;
  email?: string | null;
  /** True only when your app confirmed the address. Default false. */
  emailVerified?: boolean;
  name?: string | null;
  phone?: string | null;
  phoneVerified?: boolean;
  avatarUrl?: string | null;
  locale?: string | null;
  timezone?: string | null;
  signedUpAt?: Date | number | string | null;
  company?: {
    id: string | number;
    name?: string;
    domain?: string;
    attributes?: Record<string, DimmiScalar>;
  } | null;
  attributes?: Record<string, DimmiScalar>;
}

export type Identify<Ctx = void> = (
  ctx: Ctx,
) => DimmiUser | null | undefined | false | Promise<DimmiUser | null | undefined | false>;

export interface DimmiAppearance {
  theme?: 'light' | 'dark' | 'auto';
  primaryColor?: `#${string}`;
  position?: 'left' | 'right';
  offset?: { x?: number; y?: number };
  launcher?: boolean;
  zIndex?: number;
  headerColor?: `#${string}`;
  buttonColor?: `#${string}`;
  sound?: boolean;
}

export interface DimmiViews {
  home: {};
  messages: {};
  conversation: { id: string };
  help: { article?: string; query?: string };
  changelog: { entry?: string };
  feedback: { post?: number; board?: string };
  roadmap: {};
  announcement: { id: string };
  survey: { id: string };
}
export type DimmiViewName = keyof DimmiViews;

export type IdentityStatus = 'pending' | 'anonymous' | 'verified' | 'failed';

export interface DimmiEvents {
  ready: { appId: string; visitorId: string | null; identity: IdentityStatus };
  open: { view: DimmiViewName };
  close: {};
  view_changed: { view: DimmiViewName };
  unread_count_changed: { count: number };
  message_received: { conversationId: string; messageId: string; authorType: 'member' | 'agent' | 'system' };
  message_sent: { conversationId: string; messageId: string };
  conversation_started: { conversationId: string };
  identity_changed: { status: 'anonymous' | 'verified' | 'signed_out' | 'failed'; userHash: string | null };
  changelog_unread_changed: { count: number };
  announcement_shown: { id: string };
  announcement_dismissed: { id: string };
  announcement_clicked: { id: string; ctaId: string };
  engage_action: { id: string; ctaId: string; eventName: string };
  survey_shown: { id: string };
  survey_completed: { id: string; responseId: string };
  error: { code: string; message: string; docsUrl: string };
}

export interface DimmiIdentitySource {
  getToken(): Promise<string | null>;
  subscribe?(onChange: () => void): () => void;
}

export interface DimmiOptions {
  appId: string;
  apiBase?: string;
  identityUrl?: string;
  identity?: (() => Promise<string | null>) | DimmiIdentitySource;
  userId?: string | number | null;
  visitor?: { name?: string; email?: string };
  context?: DimmiContext;
  locale?: string;
  appearance?: DimmiAppearance;
  trackRoutes?: boolean;
  engage?: boolean;
  intercomShim?: boolean;
  testToken?: string;
  debug?: boolean;
}

export interface Dimmi {
  readonly ready: boolean;
  readonly isOpen: boolean;
  readonly view: DimmiViewName | null;
  readonly unreadCount: number;
  readonly changelogUnread: number;
  readonly identity: IdentityStatus;
  readonly visitorId: string | null;
  open(): Promise<void>;
  close(): Promise<void>;
  toggle(): Promise<void>;
  show<V extends DimmiViewName>(
    view: V,
    ...options: {} extends DimmiViews[V] ? [options?: DimmiViews[V]] : [options: DimmiViews[V]]
  ): Promise<void>;
  startConversation(message?: string, options?: { context?: DimmiContext }): Promise<void>;
  track(event: string, properties?: Record<string, DimmiScalar>): void;
  update(options: { context?: DimmiContext; locale?: string; appearance?: DimmiAppearance }): void;
  identify(): Promise<IdentityStatus>;
  logout(): Promise<void>;
  on<E extends keyof DimmiEvents>(event: E, handler: (payload: DimmiEvents[E]) => void): () => void;
}
```

`DimmiUser` to JWT claims mapping: 02 snippets.md "User object" (raw snippets write claims directly).

## `dimmi init` result (09 owns command; this is required output)

Detects framework + auth library from manifests, creates widget/secret via API, writes files, prints 3 lines. Next.js App Router + Clerk, `npx dimmi init --dry-run`:

```diff
--- a/package.json
+++ b/package.json
@@ -11,6 +11,7 @@
   "dependencies": {
     "@clerk/nextjs": "^6.34.0",
+    "@dimmi/nextjs": "^1.0.0",
     "next": "16.0.1",
     "react": "19.2.0",
     "react-dom": "19.2.0"
--- /dev/null
+++ b/dimmi.config.ts
@@ -0,0 +1,6 @@
+import { defineDimmi } from '@dimmi/nextjs/config';
+import { clerk } from '@dimmi/nextjs/clerk';
+
+export default defineDimmi({
+  identify: clerk(),
+});
--- a/next.config.ts
+++ b/next.config.ts
@@ -1,7 +1,8 @@
 import type { NextConfig } from 'next';
+import { withDimmi } from '@dimmi/nextjs/plugin';
 
 const nextConfig: NextConfig = {
   /* config options here */
 };
 
-export default nextConfig;
+export default withDimmi(nextConfig);
--- a/app/layout.tsx
+++ b/app/layout.tsx
@@ -1,5 +1,6 @@
 import type { Metadata } from 'next';
 import { ClerkProvider } from '@clerk/nextjs';
+import { Dimmi } from '@dimmi/nextjs';
 import './globals.css';
 
 export const metadata: Metadata = {
@@ -16,6 +17,7 @@ export default function RootLayout({
       <html lang="en">
         <body>
           {children}
+          <Dimmi />
         </body>
       </html>
     </ClerkProvider>
--- a/.env.local
+++ b/.env.local
@@ -1,2 +1,4 @@
 NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY=pk_test_...
 CLERK_SECRET_KEY=sk_test_...
+DIMMI_APP_ID=wgt_01jbd2k7m3q8r5t6v9w0x1y2z3
+DIMMI_IDENTITY_SECRET=dmi_idsec_****************
--- /dev/null
+++ dimmi.json
@@ -0,0 +1 @@
+{ "$schema": "https://trydimmi.com/schemas/dimmi.json", "host": "https://app.trydimmi.com", "workspace_id": "ws_01ja...", "app_id": "wgt_01jbd2k7m3q8r5t6v9w0x1y2z3", "framework": "nextjs-app", "auth": "clerk" }
```

Secret value masked in diff and terminal, written only to gitignored env file. Output (human):

```
Dimmi installed: Next.js + Clerk, 5 files (dimmi.config.ts, next.config.ts, app/layout.tsx, .env.local, dimmi.json).
Run `npm run dev`, open the site, send a message. Signed-in users show verified in the inbox.
Inbox: https://app.trydimmi.com/w/acme/inbox
```

Auth detection to identify line (codemod):

| Detected dep | Writes | Import of user instance |
|---|---|---|
| `@clerk/nextjs` | `identify: clerk()` | none |
| `next-auth` 5 / `@auth/nextjs` | `identify: authjs(auth)` | `import { auth } from '@/auth'` (path from file exporting `NextAuth(`) |
| `@supabase/ssr` | `identify: supabase()` | none (reads `NEXT_PUBLIC_SUPABASE_URL` + `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY`, falls back to `..._ANON_KEY`) |
| `better-auth` | `identify: betterAuth(auth)` | `import { auth } from '@/lib/auth'` (file calling `betterAuth(`) |
| none / unknown | config with commented `identify` stub returning `null` | widget anonymous; summary line 2 says "Add identify() in dimmi.config.ts for verified users" |

Codemod rules for this contract: one-line inserts carry no `dimmi:start/end` markers (re-run detects `<Dimmi` / `withDimmi(` / config file); wraps `export default X` in `next.config.*` only when exactly one default export of identifier or object; already wrapped by other plugin (`withSentryConfig(nextConfig)`): wraps inner identifier (`withSentryConfig(withDimmi(nextConfig))`).

## SPA route detection

- Patch `history.pushState`/`replaceState`; listen `popstate`, `hashchange`. Debounce 1 s. Ping only when session exists, else buffer last 10 pages.
- `@dimmi/nextjs` also reports route from router (`usePathname`) because App Router can bypass patched history.
- Route change feeds: page ping, engage URL triggers (15), identity check (Identity sources).
- Query string sent; server strips secret-like keys. Fragment never sent.

## Storage and tabs

| Key (localStorage, host origin) | Value | Owner |
|---|---|---|
| `dimmi:{app}:session` | `{ token, id, contact_id, kind, external_id_hash, claims_hash, expires_at }` (widget session token `dmi_ws_`, never identity JWT) | 01 |
| `dimmi:{app}:outbox` | up to 20 unsent messages `{client_id, conversation_id, body, attachment_ids, created_at}` | 01 |
| `dimmi:{app}:pages` | buffered pages before session | 01 |
| `dimmi:{app}:changelog_seen` | anonymous "What's new" cursor | 13 |
| `dimmi:{app}:feedback` | widget feedback session token (`Dimmi-Feedback-Session` header) | 14 |
| `dimmi:{app}:engage` | anonymous receipts mirror, 50 newest | 15 |

- No cookies. Works with third-party cookies blocked. Inside third-party iframe: storage partitioned by browser: separate visitor per top-level site.
- `localStorage` throws (sandboxed iframe, storage disabled): memory-only, `console.warn` `storage_unavailable`, ask email on first message regardless of `collect_email`.
- `external_id_hash` = SHA-256 of `sub`; `claims_hash` = SHA-256 of canonical claims minus `iat`/`exp`/`nonce` (skip-PATCH check). Never raw ids.
- `logout()`, sign-out detection, `session_revoked`: wipe all `dimmi:{app}:*`.
- Multi-tab: every tab own SSE stream (server cap 10 per session; 11th disconnects oldest, that tab polls). `storage` event: session token change or removal in one tab resets others (logout everywhere on device). Outbox entry owned by tab that wrote it; other tabs skip entries newer than 30 s.
- Safari ITP: storage removed after 7 days without visit. Anonymous lead becomes new lead (thread continues by email if given). Identified users unaffected (token restores history).

## Network behavior

- Load order: loader (async, zero work) -> `requestIdleCallback` (timeout 2 s) -> core + `GET /widget/v1/apps/{app}/config` + identity source in parallel -> handshake. Anonymous visitor without stored session and no identity source: config only.
- Boot handshake: [README](README.md) "Boot handshake" decision table, binding. SDK picks row from stored session + identity source answer (Identity sources). `test_token`: `POST /widget/v1/sessions {app_id, test_token}`.
- 401 `session_expired`/`token_expired` on identified session: ask identity source, `PATCH /widget/v1/session {user_jwt}` (admits expired identified token), retry once. No source: drop to anonymous, `console.warn` `identity_refresh_missing`.
- 401 `session_revoked`/`session_invalid`: wipe storage, remount anonymous.
- Invalid token (401 `token_*`): continue anonymous + `console.error` with 02 message and stack-specific fix (mode `optional`; `required` hides widget for that visitor, 02 validations.md).
- Realtime: centrifuge uni-SSE via plain `EventSource` (no centrifuge-js, about 1 KB gz). `POST /widget/v1/realtime-tokens` (bearer) returns 60 s connect token, then `new EventSource(realtime.url + '?cf_connect=' + encodeURIComponent(JSON.stringify({token, subs: {[channel]: {recover: true, offset, epoch}}})))`. Server subscribes stream to `widget_contact:{ctc}`; SDK never names other channels. Visitor session token never in URL. Typing, read state, sends stay REST (stream is receive-only).
- Reconnect: on `error` close EventSource (built-in retry would reuse expired token URL), backoff 1, 2, 4, 8, 15, 30 s with jitter, fresh token, reopen with last `offset`/`epoch`. 2 failed connects in 30 s or no first frame in 10 s: poll `GET /widget/v1/updates` every `polling_interval_ms` (15 s when hidden), retry SSE every 5 min.
- Resume: connect reply `recovered: true` replays missed publications (history 10 min); `recovered: false` (or after polling): refetch conversations + `/updates?since=<cursor>`; dedupe by event `id` and message `id`.
- Attachments: `POST /widget/v1/attachments`, PUT to `upload.url`, `PATCH /widget/v1/attachments/{id} {status: "uploaded"}` before send.
- Read receipts `PATCH .../read-state`; typing `POST .../typing-indicators`; "Talk to a human" `POST .../handoffs`.
- Outbox: send now; network error or 5xx keeps entry, retry 1 s to 60 s backoff, flush on `online`. Same `client_id` across retries. 4xx (except 401/408/429) drops entry, shows inline error.
- Engage (15): lists fetched after handshake when config `engage.active`/`anonymous`; events batched to `POST /widget/v1/engage-events`. `track()` never touches network.
- Time: offset from `server_time`; relative times use server clock.

## Loud console errors (never silent)

Browser format: `console.error('[Dimmi] <code>: <sentence> Fix: <stack-specific fix> <docs_url>')`. Once per code per page load. Production keeps sentence + docs link; dev adds fix. Server helpers (`@dimmi/nextjs`, `@dimmi/server`) log same format via `console.error`, once per code per process per 10 min.

Fix text is stack-aware: package knows its stack (Next fix says `.env.local`, Vite fix says `VITE_`).

| Code | Where | Sentence (fix) |
|---|---|---|
| `app_id_missing` | browser, server | "No app id." (Next: "Set DIMMI_APP_ID in .env.local and restart dev server." Vite: "Set VITE_DIMMI_APP_ID." Script: "Add data-app-id.") |
| `app_not_found` | browser | "App id wgt_x not found." ("Copy it from Settings > Messenger > Install or run npx dimmi init. Wrong region or self-host URL?") |
| `origin_not_allowed` | browser | "This site (https://x.com) is not in allowed origins for wgt_x." ("Add it in Settings > Messenger > Security.") |
| `widget_disabled` | browser | "Messenger is turned off for this workspace." |
| `assets_blocked` | browser | "Could not load widget script from https://app.trydimmi.com." ("Ad-blocker or CSP. Use tunnel: true or a proxy.") |
| `csp_blocked` | browser | "Content-Security-Policy blocked <url>." ("Add it to connect-src.") |
| `identity_secret_missing` | server | "DIMMI_IDENTITY_SECRET is not set; every visitor is anonymous." ("Run npx dimmi init or create a secret in Settings > Security > Identity.") |
| `identity_secret_public` | build, server | "DIMMI_IDENTITY_SECRET is exposed in <VAR> (shipped to browsers)." ("Rotate the secret now, keep it in a server-only var.") Build fails (`withDimmi`) |
| `identity_key_conflict` | server | "Both DIMMI_IDENTITY_SECRET and DIMMI_IDENTITY_PRIVATE_KEY are set." |
| `identify_failed` | server | "identify() threw: <message>. Visitor treated as signed out." |
| `identify_user_invalid` | server | "identify() returned a user without id." (Auth.js: "Expose user.id in your session callback.") |
| `identity_endpoint_invalid` | browser | "GET /api/dimmi/token answered 404 / HTML / redirect to /login, expected JSON {token}." ("Return {token: null} when signed out instead of redirecting to login.") |
| `identity_source_failed` | browser warn | "Could not reach identity source (<reason>); will retry." |
| `identity_refresh_missing` | browser warn | "Session expired and no identity source configured; continuing anonymous." |
| `server_actions_unavailable` | build warn | "Static export has no server; identity disabled." ("Use identityUrl on a separate backend.") |
| `next_plugin_missing` | server info (dev) | "dimmi.config.ts not loaded: wrap next.config with withDimmi()." |
| `token_*`, `identity_not_configured` | browser | 02 plain message + hint, rewritten for stack ("Token expired 3 minutes ago. Your server clock may be off." instead of claim talk where possible) |
| `unknown_view` / `view_unavailable` | browser | "show('x'): unknown view." / "show('changelog'): What's new tab is off for this widget." |
| `invalid_event_name` | browser | "track('Bad Name'): use lowercase letters, digits, _ . : -, max 64." |
| `storage_unavailable` | browser warn | |
| `boot_twice` | browser warn | "boot() called twice with same app id; treated as update()." |
| `called_on_server` | dev warn | "dimmi.open() called during server render; ignored." |
| `intercom_shim_conflict` | browser warn | "window.Intercom already defined by Intercom's loader; shim not installed." ("Remove the Intercom snippet.") |
| `intercom_user_hash_ignored` | browser warn | "Intercom user_hash ignored; visitor stays unverified." ("Use a Dimmi identity source: data-identity-url or identityUrl.") |
| `intercom_attributes_ignored` | browser info | "Intercom attributes (company, custom) ignored; send them as identify() attributes from your server." |
| `intercom_method_unsupported` | browser info | "Intercom('<method>') has no Dimmi equivalent; ignored." (once per method) |
| `native_not_allowed` | SDK | "This widget does not accept apps while allowed sites are set." ("Turn on Accept mobile apps in Settings > Messenger.") |
| `page_hidden` | browser debug | "Launcher hidden on this page by show or hide rules." |
| `article_not_found` | browser warn | "show('help', {article}): article not found or not published; opened Help home." |

## Host page requirements

- CSP: `script-src <api_base>`, `connect-src <api_base>` (uni-SSE and REST share API origin; no `wss:`), `img-src <api_base> <attachment bucket host> data:`. No `unsafe-inline`/`unsafe-eval` (styles via constructable stylesheets inside Shadow DOM; CI page with strict CSP). Script tag on server-rendered stacks: add your framework's CSP nonce attribute yourself. Docs list exact values per region/self-host.
- Size: loader at most 3 KB gz; core at most 35 KB gz; lazy chunks at most 20 KB gz each, named budgets: `help`, `changelog` (8 KB, 13), `feedback` (15 KB, 14), `engage-banner` (6 KB), `engage-modal` (10 KB), `engage-survey` (14 KB) (15), `intercom-shim` (2 KB, 16), `dev-context` (3 KB), `voice` (3 KB), `preview` (12 KB, preview only), `markdown`, `emoji`, `attachments`. Engage scheduler at most 1.5 KB inside core. Framework package client deltas per Packages table. Published at `/widget/v1/manifest.json` and docs badge; CI fails on regression.
- SRI pinning: hashed files and `integrity` values in manifest for customers pinning exact version.
- Example apps per install id in `sdk/examples/<id>` (Clerk, Auth.js, Supabase, Better Auth variants for Next). Playwright suite per example: anonymous boot, sign in (soft navigation), account switch, sign out (soft navigation, widget anonymous within one navigation), static page stays static (`next build` output check), `track` triggers survey, `show` every view.
