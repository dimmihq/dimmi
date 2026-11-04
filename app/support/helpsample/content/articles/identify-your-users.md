---
title: Identify your signed-in users
description: Pass a signed token from your server so the messenger knows who your customers are and keeps their history across devices.
collection: messenger
author: jonas
updated: 2026-10-25
---
When a signed-in customer opens the messenger, they shouldn't have to tell you their email. Identify them, and their conversations follow them across browsers and devices, and your team sees their plan and company next to every message.

## How it works

Your server signs a short-lived token with your workspace's identity secret and hands it to the messenger. dimmi verifies the signature, so nobody can pretend to be one of your customers by editing JavaScript in their browser.

## Sign the token on your server

```ts
import { SignJWT } from "jose";

const secret = new TextEncoder().encode(process.env.DIMMI_IDENTITY_SECRET);

export async function dimmiToken(user: { id: string; email: string; name: string }) {
  return new SignJWT({ email: user.email, name: user.name })
    .setProtectedHeader({ alg: "HS256" })
    .setSubject(user.id)
    .setIssuedAt()
    .setExpirationTime("10m")
    .sign(secret);
}
```

## Hand it to the messenger

```html
<script>
  Dimmi("boot", { identity: { token: "{{ dimmi_token }}" } });
</script>
```

> [!CAUTION]
> Never put your identity secret in code that runs in the browser. Anyone who has it can sign in to the messenger as any of your customers.

## When customers sign out

Call `Dimmi("shutdown")` when a customer signs out. The messenger forgets them on that device and starts a fresh anonymous session, so the next person on a shared computer can't read their conversations.
