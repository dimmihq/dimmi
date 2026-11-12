<!-- Paste into your project's AGENTS.md or CLAUDE.md. -->
## dimmi messenger

- The dimmi messenger is installed once at the app root with `@dimmi/<framework>` (`react`, `nextjs`, `vue`, `angular`, `svelte`) or `@dimmi/browser`. Never add the loader script a second time.
- App id and host come from env: `DIMMI_APP_ID` / `DIMMI_HOST` (framework-prefixed for the browser).
- Identity tokens are signed only on the server with `@dimmi/server` (or `@dimmi/nextjs/server`) using `DIMMI_IDENTITY_SECRET` and `DIMMI_IDENTITY_KID`. Never expose those to the browser.
- Call `identify()` after sign-in without a reload and `shutdown()` on sign-out.
- Server-side dimmi API calls use `@dimmi/api` (`createDimmiApi`, `paginate`, `unwrap`), never hand-written fetch.
- Full guide: https://github.com/dimmihq/dimmi/tree/main/sdk/skills/dimmi-install/SKILL.md
