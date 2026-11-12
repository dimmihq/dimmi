# @dimmi/react

The dimmi messenger for React 18 and 19. A provider, hooks and a button. Server-rendering safe: the messenger loads in an effect, never during render.

```sh
npm install @dimmi/react
```

## Usage

```tsx
import { DimmiButton, DimmiProvider, useDimmi, useUnreadCount } from "@dimmi/react";

const identity = async () => (await (await fetch("/api/dimmi/token")).json()).token;

export function App() {
  return (
    <DimmiProvider appId="wgt_..." host="https://dimmi.example.com" identity={identity}>
      <Help />
    </DimmiProvider>
  );
}

function Help() {
  const dimmi = useDimmi();
  const unread = useUnreadCount();
  return (
    <>
      <DimmiButton>Chat with us ({unread})</DimmiButton>
      <button onClick={() => dimmi.shutdown()}>Sign out</button>
    </>
  );
}
```

## API

- `<DimmiProvider {...options}>`: loads the messenger once and shares its client. Options are those of `@dimmi/browser`. Keep `identity` and `settings` stable (`useCallback`, `useMemo`, or module scope); a new one is sent again.
- `<Dimmi {...options} />`: the provider without children.
- `useDimmi()`: the typed client (`show`, `hide`, `toggle`, `showNewMessage`, `update`, `identify`, `shutdown`, `trackEvent`, `page`, `onUnreadCountChange`). Works outside a provider too.
- `useUnreadCount()`: the visitor's unread count.
- `useTrackPage(key)`: calls `page()` when `key` (your router's pathname) changes.
- `<DimmiButton action="open" | "toggle" | "new" message?>`: a plain `<button>`.

The bundle starts with `"use client"`, so it works from Next.js server components. For Next.js, `@dimmi/nextjs` adds server-signed identity.

## Regenerating after API changes

This package has no generated layer; it wraps `@dimmi/browser`. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
