# @dimmi/nextjs

The dimmi messenger for Next.js 14 and 15 (app router), with your signed-in users verified through a server action. It re-exports everything in `@dimmi/react`.

```sh
npm install @dimmi/nextjs
```

```ts
// dimmi.config.ts
import { defineDimmi } from "@dimmi/nextjs/config";
import { clerk } from "@dimmi/nextjs/clerk"; // or authjs, supabase, better-auth, or your own identify

export default defineDimmi({ identify: clerk() });
```

```ts
// app/actions.ts
"use server";
import { identityToken } from "@dimmi/nextjs/server";
import config from "@/dimmi.config";

export async function dimmiIdentity() {
  return identityToken(config);
}
```

```tsx
// app/layout.tsx
import { DimmiProvider } from "@dimmi/nextjs";
import { dimmiIdentity } from "./actions";

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>
        <DimmiProvider appId="wgt_..." host="https://dimmi.example.com" identity={dimmiIdentity}>
          {children}
        </DimmiProvider>
      </body>
    </html>
  );
}
```

`<Dimmi appId host identity />` is the same without children. In client components use `useDimmi()`, `useUnreadCount()` and `<DimmiButton>`. After signing someone in or out without a page load, call `refreshDimmiIdentity()`; on sign-out, `dimmiLogout()`.

Set `DIMMI_IDENTITY_SECRET` and `DIMMI_IDENTITY_KID` on the server. Pages-router apps use `createPagesHandler(config)`; route handlers `createTokenRoute(config)`.

## Regenerating after API changes

This package has no generated layer. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
