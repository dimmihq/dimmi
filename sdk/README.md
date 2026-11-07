# dimmi SDK packages

Packages developers install in their own apps. Each folder is one npm package under the `@dimmi` scope.

| Folder   | Package         | What it does                                                                                                                  |
| -------- | --------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `server` | `@dimmi/server` | Signs identity tokens on any server: Fetch API handler, Express and Hono routes. No dependencies.                             |
| `nextjs` | `@dimmi/nextjs` | The messenger component for Next.js, a server action for the token and adapters for Clerk, Auth.js, Supabase and Better Auth. |

Their tests, type checks and lint run with the browser app's (`pnpm test`, `pnpm typecheck`, `pnpm lint`).

## Publishing

Publishing is done by hand by an owner of the `@dimmi` npm organization:

```sh
cd sdk/server && npx tsc -p tsconfig.build.json && npm publish
cd ../nextjs && npx tsc -p tsconfig.build.json && npm publish
```

Build and publish `@dimmi/server` first: `@dimmi/nextjs` builds against its types and depends on it.
