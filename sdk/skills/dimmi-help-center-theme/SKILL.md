---
name: dimmi-help-center-theme
description: Build, preview and publish a custom theme for a dimmi help center with the dimmi CLI and Liquid templates. Use when someone asks to restyle, redesign, brand or customize their dimmi help center, docs site, changelog or feature request pages.
---

# Build a dimmi help center theme

A dimmi help center theme is a folder of Liquid templates, CSS, JavaScript and images. dimmi renders it on its servers. You write the folder on the person's computer with the `dimmi` CLI, preview it with their real help center content, and publish it when they say so.

Rules for the whole job:

- **Never ask for a key, token or password in the chat.** `dimmi login` opens a page in their browser; the sign-in goes to the system keychain.
- **Never read the saved keys** (keychain, the `dimmi` config folder). Every command reads them itself.
- **Publish only when the person says so** in their own words. Publishing changes what their readers see right away.
- Pass `--json` to every command and read each line's `event` and `message`.
- Read the docs pages as Markdown when you need facts: `https://trydimmi.com/docs/help-center/themes.md`, `/docs/help-center/theme-reference.md` and `/docs/help-center/theme-variables.md`.

## 1. Get the CLI and sign in

If `dimmi version` fails, install the binary as the `dimmi-install` skill or the docs describe, then:

```bash
dimmi login --json
```

Self-hosted dimmi: add `--url https://support.example.com`. Tell the person a page opened in their browser and to sign in there.

## 2. Start from the default theme

```bash
dimmi theme init --json acme-theme
```

The folder now holds the default theme. It looks exactly like their help center today. Change only what the person asks for; a file you delete falls back to the default theme, so keep the folder small:

- Colors and type: edit or add `assets/theme.css`, linked in `layouts/theme.liquid` with `{{ 'theme.css' | asset_url | stylesheet_tag }}`.
- One page: edit its file in `templates/`, such as `templates/home.liquid`.
- Shared pieces (header, footer, cards): edit files in `partials/` and `layouts/theme.liquid`.

Keep these when you rewrite markup:

- `{{ platform_assets }}`, `{{ seo_tags }}` and `{{ messenger_snippet }}` in the layout's `<head>`.
- `{{ form_token_field }}` inside every `<form method="post">`. Posts without it are refused.
- Interface words through `t`: `{{ 'search_placeholder' | t }}`, so every language keeps working. Never hard-code English labels.
- Links from variables (`article.url`, `help_center.search_url`) or the `url` filter, never a hard-coded host.

## 3. Preview while you work

Run it in the background; it keeps running:

```bash
dimmi theme preview --json --no-browser acme-theme
```

The `theme_preview` event's `data.url` is the address. Give it to the person to open. Pages reload on every save. A `theme_problem` event, and the page itself, names a mistake with its file and line: fix it before anything else.

Check every page you touched: home, a collection, an article, search, `/does-not-exist` for the 404, and, if their help center uses them, `/changelog` and `/feedback`. Check a narrow window too.

## 4. Publish when they say so

```bash
dimmi theme publish --json acme-theme
```

dimmi checks the whole theme again. If it refuses, every `theme_problem` names a file and line, and nothing changed for readers. On success, `theme_published` gives the version number and id (`thm_...`).

`--upload-only` stores the version without making it live. To go back to the default theme or an earlier version, the person can call `PUT /api/v1/help-centers/{help_center}/live-theme` with `{"theme_id": null}` or the version's id.

## Limits to keep in mind

- 1,000 files, 5 MB a file, 25 MB a theme.
- A page render stops after 2 seconds or 2,000,000 steps; avoid loops inside loops over large lists.
- Scripts run only on the help center's own address (`*.dimmi.help` or a custom domain), never under the app's `/hc/` path. Load outside fonts, scripts or images only from origins listed under `content_security_policy` in `theme.json`.
