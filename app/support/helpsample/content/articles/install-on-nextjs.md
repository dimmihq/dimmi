---
title: Install the messenger on Next.js
description: Add the messenger to a Next.js app with the React package, identify signed-in users and keep it out of your bundle until needed.
collection: messenger-install
author: jonas
updated: 2026-10-27
---
The `@dimmi/react` package adds the messenger to a Next.js app in a few lines. It loads after your page is interactive and adds under 2 KB to your bundle; the messenger itself loads from dimmi's CDN.

## Install the package

```bash
npm install @dimmi/react
```

## Add the provider

Add `<Dimmi>` to your root layout, with the app ID from **Settings → Messenger → Install**:

```tsx
import { Dimmi } from "@dimmi/react";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        {children}
        <Dimmi appId="wgt_7f3k" />
      </body>
    </html>
  );
}
```

That's all you need for anonymous visitors. Open your site and the launcher appears in the bottom corner.

## Identify signed-in users

Pass a token signed on your server, as described in [Identify your signed-in users](/articles/identify-your-users):

```tsx
<Dimmi appId="wgt_7f3k" identity={{ token: await dimmiToken(user) }} />
```

## Open the messenger from your own UI

```tsx
"use client";

import { useDimmi } from "@dimmi/react";

export function HelpButton() {
  const dimmi = useDimmi();

  return <button onClick={() => dimmi.show("messages")}>Contact support</button>;
}
```

> [!TIP]
> Route changes are picked up automatically in the App Router, so the messenger always knows which page a customer was on when they wrote to you.
