# @dimmi/nextjs

The dimmi messenger for Next.js, with your signed-in users verified. You tell dimmi who is signed in through your auth library; a server action signs a short-lived token for them, and the messenger never trusts anything the browser says about who someone is.

```sh
npm install @dimmi/nextjs
```

Create a secret in dimmi under **Settings > Identity verification**:

```sh
DIMMI_IDENTITY_SECRET=dmi_idsec_...
DIMMI_IDENTITY_KID=isk_...
```

## 1. Say who is signed in

```ts
// dimmi.config.ts
import { defineDimmi } from "@dimmi/nextjs/config";
import { clerk } from "@dimmi/nextjs/clerk";

export default defineDimmi({ identify: clerk() });
```

Adapters: `clerk()`, `authjs(auth)`, `supabase()`, `betterAuth(auth)`. Each takes `extend` to add attributes or mark an email verified, or return `null` to treat the user as signed out. Any other auth works with your own function:

```ts
export default defineDimmi({
  identify: async () => {
    const user = await getCurrentUser();
    return user && { id: user.id, email: user.email, emailVerified: user.emailVerified };
  },
});
```

## 2. A server action for the token

```ts
// app/dimmi-token.ts
"use server";
import config from "@/dimmi.config";
import { identityToken } from "@dimmi/nextjs/server";

export async function dimmiToken() {
  return identityToken(config);
}
```

## 3. The messenger

```tsx
// app/layout.tsx
import { Dimmi } from "@dimmi/nextjs";
import { dimmiToken } from "./dimmi-token";

<Dimmi appId="wgt_..." host="https://your-dimmi" identity={dimmiToken} />;
```

After your app signs someone in or out without a page load, call `refreshDimmiIdentity()`; on sign-out you can also call `dimmiLogout()`.

## Routes instead of a server action

App Router: `export const GET = createTokenRoute(config);` in `app/api/dimmi/token/route.ts`.
Pages Router: `export default createPagesHandler(config);` in `pages/api/dimmi/token.ts`.
Then use the plain snippet with `data-identity-url="/api/dimmi/token"`.

## Auth providers without a server step

If you use Clerk, Auth0, Supabase or another provider that publishes a key set, you can skip signing entirely: add the provider's JWKS address under **Settings > Identity verification** and pass the provider's own session token to the messenger.
