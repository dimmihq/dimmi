# Compatibility

Every SDK is checked in a real project per supported version.

## Backend

Checked against a running dimmi server. A real-app check signs an identity token, boots the messenger as that user in Chromium, calls the REST API (list conversations, create a contact, start a conversation) and verifies a real signed webhook.

| Language | Framework | Version | Unit tests | Real-app check |
| -------- | --------- | ------- | ---------- | -------------- |
| Go | net/http | 1.22 | pass (14 tests) | pass: HS256 token inline |
| Go | net/http | 1.23 | pass (14 tests) | pass: HS256 via `/dimmi/token` identity URL |
| Go | net/http | 1.24 | pass (14 tests) | pass: ES256 token inline |
| Go | net/http | 1.26 | pass (14 tests) | pass: HS256 token inline |
| PHP | plain PHP (`dimmi/dimmi-php`) | PHP 8.2 | pass (57 tests) | pass |
| PHP | plain PHP (`dimmi/dimmi-php`) | PHP 8.3 | pass (57 tests) | pass |
| PHP | plain PHP (`dimmi/dimmi-php`) | PHP 8.4 | pass (57 tests) | pass |
| PHP | Laravel (`dimmi/dimmi-laravel`) | 10 (PHP 8.2) | pass (19 tests) | pass |
| PHP | Laravel (`dimmi/dimmi-laravel`) | 11 (PHP 8.3) | pass (19 tests) | pass |
| PHP | Laravel (`dimmi/dimmi-laravel`) | 12 (PHP 8.4) | pass (19 tests) | pass |
| Python | FastAPI | 3.10 | pass (65 tests) | pass: HS256 token inline |
| Python | FastAPI | 3.12 | pass (65 tests) | pass: HS256 via `/dimmi/token` identity URL |
| Python | FastAPI | 3.13 | pass (66 tests) | pass: ES256 token inline |

## JavaScript and TypeScript

Checked 2026-11-12 against the local dimmi server (`http://localhost:8080`, app `wgt_01m77nrqt5fbgtg05x46fm0ehx`). Each app came from the framework's own scaffolder, installed the packed `@dimmi/*` tarballs, built in production mode and was served by its production server (or a static server with a `/dimmi-identity` token route). A Chromium check then:

- waits for the messenger to boot;
- signs in: the app fetches a token signed by `@dimmi/server` (Next.js: a server action through `@dimmi/nextjs/server`), and `POST /widget/v1/sessions` answers 201, creating the contact `<app>@sdk.test`;
- calls `update()` and checks the messenger redrew;
- calls `trackEvent()` and `show()`, and checks the panel opened;
- requires no console errors and no failed widget call.

Screenshots are in `tmp/proof/shots/sdk-<framework>-<version>-*.png`. The harness lives in `tmp/sdk-js-matrix` (`matrix.sh`). The apps themselves are not kept in the repository.

| Package | Framework | Version | Node | Build | Real-app check |
| --- | --- | --- | --- | --- | --- |
| `@dimmi/react` | Vite + React | 18.3.1 | 22 | ok | ok |
| `@dimmi/react` | Vite + React | 19.3.1 | 22 | ok | ok |
| `@dimmi/nextjs` | Next.js app router (React 18.3) | 14.2.35 | 22 | ok | ok |
| `@dimmi/nextjs` | Next.js app router (React 19.1) | 15.5.29 | 22 | ok | ok |
| `@dimmi/vue` | Vite + Vue | 3.4.38 | 22 | ok | ok |
| `@dimmi/vue` | Vite + Vue | 3.5.45 | 22 | ok | ok |
| `@dimmi/vue` | Nuxt (SSR, client plugin) | 3.21.11 | 22 | ok | ok |
| `@dimmi/angular` | Angular standalone | 17.3.12 | 22 | ok | ok |
| `@dimmi/angular` | Angular standalone | 18.2.14 | 22 | ok | ok |
| `@dimmi/angular` | Angular standalone, SSR + prerender | 19.2.25 | 22 | ok | ok |
| `@dimmi/svelte` | SvelteKit (Svelte 5.57), prerendered | 2.70.4 | 22 | ok | ok |
| `@dimmi/svelte` | SvelteKit (Svelte 5.57), prerendered | 3.2.0 | 22 | ok | ok |
| `@dimmi/api` + `@dimmi/server` | Node ESM and CJS, `tsc` NodeNext, TS 5.9 | Node 25 | 25 | ok | ok: walked 12 contacts over 3 cursor pages |
| `@dimmi/browser` | plain page (`tmp/proof/sdk.mjs`) | n/a | n/a | ok | ok |

Known gaps, in the messenger rather than the SDKs:

- `trackEvent()` sends `Dimmi("track", ...)`, which the messenger does not run yet; it logs `unknown_command`. Record events server-side with `@dimmi/api` (`POST /contact-events`).
- `onUnreadCountChange()` / `useUnreadCount()` listen for a `dimmi:unread` window event the messenger does not send yet, so they stay at 0.
