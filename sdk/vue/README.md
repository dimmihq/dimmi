# @dimmi/vue

The dimmi messenger for Vue 3 and Nuxt 3: a plugin and composables. The messenger loads only in the browser.

```sh
npm install @dimmi/vue
```

## Vue

```ts
import { createDimmi } from "@dimmi/vue";

createApp(App)
  .use(createDimmi({ appId: "wgt_...", host: "https://dimmi.example.com" }))
  .mount("#app");
```

```vue
<script setup lang="ts">
import { useDimmi, useUnreadCount } from "@dimmi/vue";
const dimmi = useDimmi();
const unread = useUnreadCount();
</script>

<template>
  <button @click="dimmi.show()">Chat with us ({{ unread }})</button>
</template>
```

## Nuxt

Register the plugin as client-only, in `plugins/dimmi.client.ts`:

```ts
import { createDimmi } from "@dimmi/vue";

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp.use(createDimmi({ appId: "wgt_...", host: "https://dimmi.example.com" }));
});
```

Components may call `useDimmi()` during server rendering; it answers a client that does nothing there. Sign tokens in a server route with `@dimmi/server`:

```ts
// server/routes/dimmi-token.ts
import { signIdentityToken } from "@dimmi/server";

export default defineEventHandler(async (event) => {
  const user = event.context.user; // your session
  return { token: user ? await signIdentityToken({ id: user.id, email: user.email }) : null };
});
```

## API

- `createDimmi(options)`: the plugin. Options are those of `@dimmi/browser`. Also sets `this.$dimmi`.
- `useDimmi()`: the typed client.
- `useUnreadCount()`: a `Ref<number>`, unsubscribed when the component unmounts.
- `DimmiKey`: the injection key.

## Regenerating after API changes

This package has no generated layer; it wraps `@dimmi/browser`. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
