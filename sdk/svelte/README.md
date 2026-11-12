# @dimmi/svelte

The dimmi messenger for Svelte 4 and 5 and SvelteKit 2 and 3: a store and an action.

```sh
npm install @dimmi/svelte
```

## Usage

```svelte
<!-- src/routes/+layout.svelte -->
<script lang="ts">
  import { onMount } from "svelte";
  import { initDimmi } from "@dimmi/svelte";

  let { children } = $props();
  onMount(() => initDimmi({ appId: "wgt_...", host: "https://dimmi.example.com" }));
</script>

{@render children()}
```

```svelte
<script lang="ts">
  import { dimmiAction, unreadCount, useDimmi } from "@dimmi/svelte";
  const dimmi = useDimmi();
</script>

<button use:dimmiAction={"open"}>Chat with us ({$unreadCount})</button>
<button use:dimmiAction={{ message: "Billing question" }}>Ask about billing</button>
<button onclick={() => dimmi.shutdown()}>Sign out</button>
```

## API

- `initDimmi(options)`: loads the messenger in the browser and answers its client. Does nothing on the server.
- `useDimmi()`: the typed client.
- `unreadCount`: a readable store.
- `dimmiAction(node, "open" | "toggle" | "close" | { message })`: opens the messenger on click.

Sign identity tokens in a `+server.ts` route with `createIdentityHandler` from `@dimmi/server`.

## Regenerating after API changes

This package has no generated layer; it wraps `@dimmi/browser`. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
