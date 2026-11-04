# 02 Widget identity: server helpers and snippets (docs content)

Style: caveman prose, code exact. Single source: `resources/snippets/identity/*.tmpl` render these blocks; `{{.AppID}}`, `{{.Kid}}`, `{{.BaseURL}}` filled by `GET /api/v1/identity-verification/snippets?language=<id>`. Docs site renders same files. CI job `snippets-ci` (language Docker images, separate from `go test`) runs every raw snippet and every helper package example, verifies produced token in-process (`VerifyIdentityToken`), asserts valid and asserts signed-out request returns `{"token": null}`.

Client side (components, tags, hooks) lives in [01 sdk-contract.md](../01-messenger-widget/sdk-contract.md). This file: what server must return, how user maps to token, JS helpers (`@dimmi/nextjs`, `@dimmi/server`), raw snippets for every other stack. Other per-stack packages: later (identity-helper-packages).

## Model

Host app answers one question for the SDK: "who is signed in on this request?" Helper turns answer into short-lived signed token. App code never handles JWTs.

Identity source contract (every helper, every raw snippet):

| Rule | Value |
|---|---|
| Route | `GET /api/dimmi/token` (helpers mount it; Next.js App Router uses package server action instead, same response) |
| Auth | host session as-is (cookie). Bearer-auth SPAs: identity source object sends own header (01 sdk-contract React section) |
| Signed in | `200 {"token": "<jwt>"}` |
| Signed out | `200 {"token": null}`. Never 302 to login, never 401 page (SDK reports `identity_endpoint_invalid`) |
| Headers | `Cache-Control: no-store, private`, `Content-Type: application/json`. No CORS headers (same origin only) |
| Token | HS256 with `DIMMI_IDENTITY_SECRET` (string as-is, no base64 decode); `kid` = `DIMMI_IDENTITY_KID` when set. RS256/EdDSA when `DIMMI_IDENTITY_PRIVATE_KEY` set (`kid` required) |
| Lifetime | `exp = iat + 600` (10 minutes). Minted per request; SDK asks again when needed. Never longer than 1 hour |
| Never | sign in browser; put token in HTML, URL, logs, cache; expose secret via `NEXT_PUBLIC_`/`VITE_`/`PUBLIC_`/`NUXT_PUBLIC_` var; default `email_verified` to true |

Why no inline token in HTML: page caches (CDN, ISR, `expires_in public`), bfcache and Turbo snapshots replay HTML. Replayed token = previous user boots on shared device or other user's page. On-demand fetch costs one same-origin request at idle (widget core is lazy anyway).

## User object

JS helpers take user object, never claims. Raw snippets write claims directly (Claim column).

| JS | Claim | Rule |
|---|---|---|
| `id` | `sub` | required; number converted to string; 1 to 255 chars |
| `email` | `email` | |
| `emailVerified` | `email_verified` | only `true` when your app confirmed address; default false (R3) |
| `name` | `name` | |
| `phone`, `phoneVerified` | `phone`, `phone_verified` | E.164 |
| `avatarUrl` | `avatar_url` | https only |
| `locale`, `timezone` | same | |
| `signedUpAt` | `custom_attributes.signed_up_at` (unix seconds) | Date, epoch seconds or ISO; 18 rule fields read it |
| `company: { id, name, domain, attributes }` | `company {id, name, domain, custom_attributes}` | `id` required inside |
| `attributes` | `custom_attributes` | keys `[a-z0-9_]{1,64}`, max 100, flat scalars, `null` deletes key |

Agent actions (07) bind only to trusted values: put ids your APIs need (`stripe_customer_id`, `account_id`, `plan`) in `attributes` (JWT `custom_attributes`). Claim-written keys become trusted (`contacts.trusted_attribute_keys`); values set from the widget (`visitor`, lead `PATCH /widget/v1/contact`, Intercom shim) are never trusted.

`null`/`undefined`/empty fields omitted from token (no `"email": null` claims). Helpers validate in dev: bad attribute key, nested value, `id` missing: `console.error` / logger with code (`identify_user_invalid`, `attribute_key_invalid`) and fix. Unknown fields: warn `user_field_unknown` ("Put `plan` under `attributes`.").

## Helpers per stack (recommended)

| Stack | Package | Identity defined in | Route |
|---|---|---|---|
| Next.js App Router | `@dimmi/nextjs` | `dimmi.config.ts` `identify` (adapter or function) | none (server action) |
| Next.js Pages Router | `@dimmi/nextjs` | same | `pages/api/dimmi/token.ts` one-line re-export |
| Express / Hono | `@dimmi/server/express`, `@dimmi/server/hono` | handler argument | yours, `/api/dimmi/token` |
| Any Fetch-API server (React Router v7 / Remix resource route, SvelteKit `+server.ts`, Astro endpoint, Nuxt via `toWebRequest`, Workers, Bun, Deno) | `@dimmi/server` `createIdentityHandler` | handler argument | yours, `/api/dimmi/token` |
| Rails, Laravel, plain PHP, Django, FastAPI, Go, Fastify, Supabase Edge Function | package below, raw snippet stays for those who skip it | package or raw snippet | yours, `/api/dimmi/token` |
| Native app, WebView | none on device: app's backend signs with same helper or snippet, app passes token through identity provider ([20](../20-native-sdks/api-mapping.md)) or `#dimmi-identity` fragment | server side | yours |

Install text per stack: 01 sdk-contract.md "Install by stack". Low-level signer for custom flows: `signIdentityToken(user)` (`@dimmi/server`). Same rules as table above.

### Next.js auth adapters (launch set)

```ts
// dimmi.config.ts
import { defineDimmi } from '@dimmi/nextjs/config';
import { clerk } from '@dimmi/nextjs/clerk';

export default defineDimmi({
  identify: clerk(),
});
```

| | `clerk()` | `authjs(auth)` | `supabase()` | `betterAuth(auth)` |
|---|---|---|---|---|
| Import | `@dimmi/nextjs/clerk` | `@dimmi/nextjs/authjs` | `@dimmi/nextjs/supabase` | `@dimmi/nextjs/better-auth` |
| Reads | `auth()`, then `currentUser()` (profile cached in process 5 min per user id, LRU 1,000: Clerk Backend API rate limits) | `await auth()` (your `@/auth` export) | `createServerClient(...).auth.getUser()` (server-verified, never `getSession()`) | `auth.api.getSession({ headers: await headers() })` |
| `id` | `user.id` | `session.user.id` (missing: `identify_user_invalid`, fix: "expose user.id in session callback") | `user.id` | `session.user.id` |
| `email` | `primaryEmailAddress.emailAddress` | `session.user.email` | `user.email` | `user.email` |
| `emailVerified` | `primaryEmailAddress.verification.status === 'verified'` | `false` (session has no flag; set via `extend`) | `user.email_confirmed_at != null` | `user.emailVerified` |
| `name` | `fullName` | `session.user.name` | `user_metadata.full_name ?? user_metadata.name` | `user.name` |
| `avatarUrl` | `imageUrl` | `session.user.image` | `user_metadata.avatar_url` | `user.image` |
| `signedUpAt` | `createdAt` | | `created_at` | `createdAt` |
| `company` | active org `{ id: orgId }` | | | active organization `{ id }` (organization plugin) |
| Env | Clerk's own | Auth.js own | `NEXT_PUBLIC_SUPABASE_URL` + `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY` (fallback `_ANON_KEY`) | Better Auth own |

Not at launch: Firebase Auth (no server session by default; custom `identify` with session cookie + `firebase-admin`), WorkOS AuthKit, Kinde, Lucia (custom `identify`, 6 lines). Adapters work in App Router actions and Pages Router API routes.

Every adapter takes `extend` (merged over mapped user; return `null` to treat as signed out):

```ts
import { defineDimmi } from '@dimmi/nextjs/config';
import { authjs } from '@dimmi/nextjs/authjs';
import { auth } from '@/auth';

export default defineDimmi({
  identify: authjs(auth, {
    extend: (session) => ({
      emailVerified: session.user.emailVerified === true,
      attributes: { plan: session.user.plan },
    }),
  }),
});
```

## `@dimmi/server` API

```ts
import { createIdentityHandler, signIdentityToken, createPortalLink } from '@dimmi/server';

// (request: Request) => Promise<Response>: implements the identity source contract
const handler = createIdentityHandler(async (request) => {
  const user = await getUser(request);
  return user && { id: user.id, email: user.email, emailVerified: user.emailVerified, name: user.name };
});

// low level: string token, 10 minutes, kid from env
const token = await signIdentityToken({ id: 'user_42', email: 'ada@acme.com', emailVerified: true });
```

Options (second argument of `createIdentityHandler` and adapter-level `identityHandler`): `secret` (default env), `kid` (default env), `privateKey` (default env), `expiresIn` (seconds, default 600, max 3600), `audience` (optional, `dimmi` | app id | workspace id).

## Raw snippets (no Dimmi package)

Same contract. Use when helper package not wanted or language not covered (raw path stays for any stack). Each: sign function + handler.

### Node.js (no dependencies)

```js
// dimmi-token.js (Node 18+)
import { createHmac } from 'node:crypto';

const encode = (value) => Buffer.from(JSON.stringify(value)).toString('base64url');

export function dimmiToken(user) {
  const now = Math.floor(Date.now() / 1000);
  const header = { alg: 'HS256', typ: 'JWT', kid: process.env.DIMMI_IDENTITY_KID };
  const claims = {
    sub: String(user.id),
    iat: now,
    exp: now + 600,
    email: user.email ?? undefined,
    email_verified: user.emailVerified === true,
    name: user.name ?? undefined,
  };
  const unsigned = `${encode(header)}.${encode(claims)}`;
  const signature = createHmac('sha256', process.env.DIMMI_IDENTITY_SECRET).update(unsigned).digest('base64url');
  return `${unsigned}.${signature}`;
}
```

```js
// Express
app.get('/api/dimmi/token', (req, res) => {
  res.set('Cache-Control', 'no-store, private');
  res.json({ token: req.user ? dimmiToken(req.user) : null });
});
```

`JSON.stringify` drops `undefined` (no `kid` when env unset).

### Edge runtimes (jose: Workers, Deno, Bun, Vercel Edge)

```js
import { SignJWT } from 'jose';

const key = new TextEncoder().encode(process.env.DIMMI_IDENTITY_SECRET);
const kid = process.env.DIMMI_IDENTITY_KID;

export function dimmiToken(user) {
  return new SignJWT({
    email: user.email ?? undefined,
    email_verified: user.emailVerified === true,
    name: user.name ?? undefined,
  })
    .setProtectedHeader({ alg: 'HS256', typ: 'JWT', ...(kid && { kid }) })
    .setSubject(String(user.id))
    .setIssuedAt()
    .setExpirationTime('10m')
    .sign(key);
}
```

### Python (PyJWT)

```bash
pip install "PyJWT>=2"
```

```python
# dimmi_token.py
import os
import time

import jwt


def dimmi_token(user) -> str:
    now = int(time.time())
    claims = {
        "sub": str(user.id),
        "iat": now,
        "exp": now + 600,
        "email": user.email,
        "email_verified": getattr(user, "email_verified", False) is True,
        "name": getattr(user, "name", None),
    }
    kid = os.environ.get("DIMMI_IDENTITY_KID")
    return jwt.encode(
        {k: v for k, v in claims.items() if v is not None},
        os.environ["DIMMI_IDENTITY_SECRET"],
        algorithm="HS256",
        headers={"kid": kid} if kid else None,
    )
```

```python
# FastAPI
from fastapi import Depends
from fastapi.responses import JSONResponse

@app.get("/api/dimmi/token")
def dimmi(user=Depends(optional_user)):
    token = dimmi_token(user) if user else None
    return JSONResponse({"token": token}, headers={"Cache-Control": "no-store, private"})
```

Django: same `dimmi_token` in a view (no `login_required`): `JsonResponse({"token": dimmi_token(request.user) if request.user.is_authenticated else None})` with `response["Cache-Control"] = "no-store, private"`; route `path("api/dimmi/token", dimmi)`.

### Go (golang-jwt v5)

```bash
go get github.com/golang-jwt/jwt/v5
```

```go
package dimmitoken

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID, Email, Name string
	EmailVerified   bool
}

func Sign(u User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":            u.ID,
		"iat":            now.Unix(),
		"exp":            now.Add(10 * time.Minute).Unix(),
		"email_verified": u.EmailVerified,
	}
	if u.Email != "" {
		claims["email"] = u.Email
	}
	if u.Name != "" {
		claims["name"] = u.Name
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	if kid := os.Getenv("DIMMI_IDENTITY_KID"); kid != "" {
		t.Header["kid"] = kid
	}
	return t.SignedString([]byte(os.Getenv("DIMMI_IDENTITY_SECRET")))
}

// Handler serves GET /api/dimmi/token. current returns the signed-in user, if any.
func Handler(current func(*http.Request) (User, bool)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Token *string `json:"token"`
		}
		if u, ok := current(r); ok {
			token, err := Sign(u)
			if err != nil {
				http.Error(w, "dimmi: token signing failed", http.StatusInternalServerError)
				return
			}
			body.Token = &token
		}
		w.Header().Set("Cache-Control", "no-store, private")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
}
```

### Ruby on Rails (ruby-jwt)

```ruby
# Gemfile
gem "jwt"
```

```ruby
# app/controllers/dimmi_tokens_controller.rb
class DimmiTokensController < ApplicationController
  skip_before_action :authenticate_user!, raise: false

  def show
    response.headers["Cache-Control"] = "no-store, private"
    render json: { token: current_user && dimmi_token(current_user) }
  end

  private

  def dimmi_token(user)
    now = Time.now.to_i
    claims = {
      sub: user.id.to_s,
      iat: now,
      exp: now + 600,
      email: user.email,
      email_verified: user.try(:confirmed?) == true,
      name: user.try(:name)
    }.compact
    kid = ENV["DIMMI_IDENTITY_KID"]
    JWT.encode(claims, ENV.fetch("DIMMI_IDENTITY_SECRET"), "HS256", kid ? { kid: kid } : {})
  end
end
```

```ruby
# config/routes.rb
get "/api/dimmi/token", to: "dimmi_tokens#show"
```

Rails 8 auth generator: replace `current_user` with `Current.user`.

### PHP (firebase/php-jwt)

```bash
composer require firebase/php-jwt
```

```php
<?php
// public/api/dimmi/token.php
require __DIR__ . '/../../../vendor/autoload.php';

use Firebase\JWT\JWT;

session_start();
header('Content-Type: application/json');
header('Cache-Control: no-store, private');

$user = $_SESSION['user'] ?? null;

if ($user === null) {
    echo json_encode(['token' => null]);
    exit;
}

$now = time();
$claims = array_filter([
    'sub' => (string) $user['id'],
    'iat' => $now,
    'exp' => $now + 600,
    'email' => $user['email'] ?? null,
    'email_verified' => ($user['email_verified'] ?? false) === true,
    'name' => $user['name'] ?? null,
], fn ($value) => $value !== null);

echo json_encode([
    'token' => JWT::encode($claims, getenv('DIMMI_IDENTITY_SECRET'), 'HS256', getenv('DIMMI_IDENTITY_KID') ?: null),
]);
```

Laravel: same claims inside `Route::get('/api/dimmi/token', ...)` in `routes/web.php` (no `auth` middleware: guests get `{"token": null}`), `email_verified` = `$user->hasVerifiedEmail()`.

## Feedback portal SSO links (14)

Portal (`https://{portal}/feedback`) signs users in with same identity secret. Token in URL fragment, never query: `https://{portal}/feedback/sso#token=<jwt>&return_to=/feedback/posts/12`. Help center tokens (18): same mint and fragment, `aud: "dimmi-help"`, `{help_host}/sso#token=<jwt>`, from customer route at `help_centers.login_url`. Portal tokens: `aud: "dimmi-portal"`, `exp = iat + 300`, random `nonce` (single use, 02 step 11).

Links in app pages never embed token (rule above). Link to host route that mints on click:

| Stack | Link helper | SSO route (mints, redirects to portal) |
|---|---|---|
| Next.js | `portalLink({ returnTo: '/roadmap' })` from `@dimmi/nextjs/server` returns `/api/dimmi/sso?return_to=%2Froadmap` | `app/api/dimmi/sso/route.ts`: `export { GET } from '@dimmi/nextjs/portal-sso';` |
| `@dimmi/server` | `createPortalLink({ returnTo })` | `createPortalSsoHandler(identify, { loginUrl })` |
| Other stacks | link to own SSO route | own route: mint with raw snippet + `aud: "dimmi-portal"`, `exp = iat + 300`, random `nonce`, redirect (rules below) |

```tsx
// app/(app)/sidebar.tsx (server component)
import { portalLink } from '@dimmi/nextjs/server';

export function Sidebar() {
  return <a href={portalLink({ returnTo: '/roadmap' })}>Roadmap</a>;
}
```

SSO route: signed in: 302 to `{DIMMI_PORTAL_URL}/feedback/sso#token=...&return_to=...`. Signed out: 302 to app login (`loginUrl` option, default `/login`) with `return_to` back to SSO route. Portal setting `sso_login_url` = `https://app.acme.com/api/dimmi/sso` (redirect-back when portal visitor clicks "Sign in"). `return_to`: relative portal path, or absolute URL under `DIMMI_PORTAL_URL` (portal "Sign in" button sends absolute; route converts to path); anything else ignored (open redirect guard, same rule as 14).

## Per-stack packages

Same model as `@dimmi/server`: `identify(request) -> user | null`, helper signs 10 minute token, route answers `{"token": ...}` with `Cache-Control: no-store, private`. No package ever accepts a client-side secret. Secret from `DIMMI_IDENTITY_SECRET`.

| Package | Registry | Surface | Reuse |
|---|---|---|---|
| `@dimmi/sveltekit` | npm | `<Dimmi />` Svelte component, `dimmiHandle({identify})` hook mounting `/api/dimmi/token` | `@dimmi/server`, `@dimmi/browser` |
| `@dimmi/nuxt` | npm | Nuxt module `modules: ['@dimmi/nuxt']`, `identify` in server handler, `useDimmi()` composable | same |
| `@dimmi/vue` | npm | plugin + `useDimmi()` composable, browser only | `@dimmi/browser` |
| `@dimmi/astro` | npm | integration, `<Dimmi />`, endpoint helper | same |
| `@dimmi/server/fastify` | npm subpath | Fastify plugin | `@dimmi/server` |
| `@dimmi/server/supabase`, `@dimmi/react/supabase` | npm subpath | Edge Function handler verifying Supabase session; React identity source object | `@dimmi/server`, `jose` |
| `dimmi` | RubyGems | Rails engine: `dimmi_tag` helper, `Dimmi.identity_token(user)`, mounted route, `config.identify` | ruby-jwt |
| `dimmi/dimmi`, `dimmi/laravel` | Packagist | `Dimmi::identityToken($user)`; Laravel service provider, `@dimmi` Blade directive, route | firebase/php-jwt |
| `dimmi` | PyPI | `dimmi.identity_token(user)`, `dimmi.django` view + template tag, `dimmi.fastapi` router | PyJWT |
| `github.com/dimmihq/dimmi-go` | Go module | `dimmi.IdentityToken(user)`, `dimmi.Handler(identify)` net/http | golang-jwt |

All send `sdk.name` via script tag attribute `data-sdk`. All licensed MIT. Names claimed on registries = human step.

## Common mistakes (docs; automatic detection: [validations.md](validations.md))

| Mistake | Symptom code | Fix |
|---|---|---|
| Base64-decoding secret before signing | `token_signature_invalid` + static hint | use string as-is |
| `.env` value with trailing newline or quotes | `token_signature_invalid` + static hint | trim |
| `exp: Date.now() + 600000` | `token_lifetime_too_long` | seconds |
| `company_id` at top level | ignored silently | nest under `company` (helpers: `company: { id }`) |
| `user_id` instead of `sub` | `token_sub_missing` | rename to `sub` (helpers: `id`) |
| Numeric `sub` (`42`) | warning `claim_sub_number` (works) | `String(id)` (helpers convert) |
| `"email": null` claims | warning `claim_email_invalid` | omit empty fields (helpers do) |
| Token endpoint cached (CDN, service worker, `expires_in public`) | `token_expired` for many users, or wrong user identified | `Cache-Control: no-store, private` |
| Token rendered into HTML | stale/wrong user after sign-out, `token_expired` after back navigation | fetch on demand (identity URL or Next action) |
| Endpoint redirects guests to login (Devise `authenticate_user!`, Laravel `auth`, Django `login_required`) | browser `identity_endpoint_invalid` | return `{"token": null}` |
| `email_verified: true` for every user | silent account takeover risk via email merge (R3) | set only from provider's verified flag |
| Secret in client bundle | not detectable server side; helpers and `withDimmi` refuse public env names (`identity_secret_public`); `npx dimmi init` refuses `NEXT_PUBLIC_`/`VITE_` names | rotate secret now |
| `signed_up_at` as non-date string | warning `claim_signed_up_at_invalid` (18 rules ignore it) | Date, epoch seconds or ISO 8601 |
