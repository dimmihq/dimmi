# @dimmi/browser

Load the dimmi messenger on any web page, with a typed API and no framework. About 1.2 kB gzipped, zero dependencies, safe to import during server rendering.

```sh
npm install @dimmi/browser
```

## Usage

```ts
import { loadDimmi } from "@dimmi/browser";

const dimmi = loadDimmi({
  appId: "wgt_...", // Settings > Messenger
  host: "https://dimmi.example.com", // your dimmi host, cloud or self-hosted
  identity: async () => (await (await fetch("/api/dimmi/token")).json()).token,
});

dimmi.show();
dimmi.showNewMessage("I need help with billing");
dimmi.update({ appearance: { position: "left", primary_color: "#0f766e" } });
dimmi.identify(); // after your app signs someone in without a page load
dimmi.shutdown(); // on sign-out
```

`loadDimmi` adds `<host>/widget/v1/loader.js` once. Calls made before it loads wait in the same queue the paste-in snippet uses. A second `loadDimmi` call adds no second script.

## Options

| Option        | What it does                                                                                |
| ------------- | ------------------------------------------------------------------------------------------- |
| `appId`       | The messenger's app id, `wgt_...`. Required.                                                |
| `host`        | Where dimmi runs. Required. Self-hosted installs use their own address.                     |
| `apiBase`     | Where the messenger's API calls go, when not `host` (a proxy).                              |
| `nonce`       | CSP nonce for the loader. The messenger passes it on to every script it loads.             |
| `identity`    | Function answering the signed-in user's token from `@dimmi/server`, or `null`.              |
| `identityUrl` | A URL answering `{"token": "<jwt>"}` or `{"token": null}`, instead of `identity`.           |
| `settings`    | Messenger settings applied at boot, like `update()` takes.                                  |

## Client

| Method                           | Messenger command                                                           |
| -------------------------------- | --------------------------------------------------------------------------- |
| `show()` / `hide()` / `toggle()` | `open` / `close` / `toggle`                                                 |
| `showNewMessage(text?)`          | `ask` (or `open` without text)                                              |
| `update(settings)`               | `update`                                                                    |
| `identify(source?)`              | `identify`                                                                  |
| `shutdown()`                     | `logout`                                                                    |
| `trackEvent(name, metadata?)`    | `track`                                                                     |
| `onUnreadCountChange(fn)`        | listens for the `dimmi:unread` window event; returns an unsubscribe         |
| `page()`                         | route-change hook; the messenger already notices address changes on its own |
| `command(name, ...args)`         | any raw `Dimmi(...)` command                                                |
| `ready`                          | resolves when the loader script loaded, rejects when it was blocked         |

`getDimmi()` answers the client for a messenger loaded by the paste-in snippet. `dimmi(name, ...args)` sends one raw command.

`trackEvent` sends the event from the visitor's browser once they have a session (it waits until then). The workspace accepts only event names switched on for the messenger in Settings > Attributes, unless "Accept any event from the messenger" is on; other names log `event_not_allowed`. Browser events can be forged, so record anything a rule must trust server-side with `@dimmi/api` (`POST /contact-events`).

`onUnreadCountChange` fires once at boot and whenever the visitor's unread total changes.

## Server rendering

Every function does nothing when `window` is missing, so importing and calling them on a server is safe.

## Regenerating after API changes

This package has no generated layer. It follows the messenger's `Dimmi(...)` commands in `resources/widget/core.ts`. `pnpm sdk:generate` regenerates every generated SDK file in this repo.
