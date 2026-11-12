# CLAUDE.md

@AGENTS.md

The rules in AGENTS.md are binding. Above all: never break the API or the SDKs, and after any change to routes, requests, resources or events run `make api-snapshot`, `make api-compat`, `make sdk-check`, `make sdk-compat` and add a changeset (`pnpm changeset`). SDK generated code is never committed: `make sdk-generate` produces it. Never publish anything.
