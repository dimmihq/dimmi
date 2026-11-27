# Publish gate spec

Every new page on `trydimmi.com` must pass `scripts/publish-gate.ts` in CI before a PR can merge. The gate checks `site/dist` after the build and blocks low-value, duplicated, unverified, or stale pages.

## How the gate runs

CI runs the gate inside the `site` job in `.github/workflows/main.yml`, right after `scripts/check-site-paths.sh site/dist`:

```bash
node scripts/publish-gate.ts site/dist --baseline https://trydimmi.com/sitemap.xml
```

The command writes a machine-readable report to `site/dist/publish-gate.json` and uploads it as a CI artifact on every run. Each check prints `PASS`, `WARN`, or `FAIL`. Any `FAIL` exits non-zero unless `--report-only` is passed.

## Required checks

| Check               | Rule                                                                                                                                                                          | Result on miss                                  |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------- |
| Sign-off            | Content pages carry `meta` with `status: "published"`, `reviewedBy: "frantz-kati"`, and valid dates (`published`, `updated`, `reviewedOn`). Drafts never ship in `site/dist`. | `FAIL`                                          |
| Direct answer       | `meta.answer` appears in the first 50 words of `<main>`.                                                                                                                      | `FAIL`                                          |
| Unique content      | 5-word shingles of `<main>` text (with numbers and rival names swapped for `<NUM>` and `<RIVAL>`) score at least 65% unique against same-type pages, `/`, and `/pricing`.     | `FAIL` (`WARN` only on `/docs/api-reference/*`) |
| Template budget     | Marketing pages keep shared shingle text at 300 words or fewer. `/vs/*` pages also need at least 900 unique words.                                                            | `FAIL`                                          |
| Title tag           | Every page has a unique `<title>` of 60 characters or fewer.                                                                                                                  | `FAIL`                                          |
| Meta description    | Every page has a unique `<meta name="description">` of 140 to 160 characters.                                                                                                 | `FAIL`                                          |
| Dates and author    | HTML, JSON-LD (`datePublished`, `dateModified`), and `sitemap.xml` `<lastmod>` match `meta.updated`. Author in JSON-LD is `@type: "Person"`.                                  | `FAIL`                                          |
| Schema types        | Top-level JSON-LD uses only `Organization`, `WebSite`, `SoftwareApplication`, `WebPage`, `BreadcrumbList`, `BlogPosting`, or `WebApplication`. `FAQPage` never ships.         | `FAIL`                                          |
| Primary heading     | Exactly 1 `<h1>` per page with zero rival names in the `<h1>` text (`WRITING.md` rule 4).                                                                                     | `FAIL`                                          |
| Proof and images    | `meta.proof` paths exist on disk. Every `<img>` carries non-empty `alt`, `width`, and `height`.                                                                               | `FAIL`                                          |
| Internal links      | At least 3 outbound internal links in `<main>`, at least 3 inbound links from `<main>` for `/pricing` and `/vs/*`, and zero broken internal links.                            | `FAIL`                                          |
| Rival freshness     | Every `meta.rivals` item carries `checkedOn` and `source`. Older than 45 days before `meta.updated` fails; older than 90 days before today warns.                             | `FAIL` / `WARN`                                 |
| Slop check          | Zero em dashes, en dashes, curly quotes, spaced dashes, binary contrasts, or banned filler words in `<main>` prose.                                                           | `FAIL`                                          |
| Canonical constants | SDK count matches `sdk/`, free tier is `100` conversations, and any self-host install command matches `README.md`.                                                            | `FAIL`                                          |
| Rollout pace        | New URLs against the live sitemap stay within `site/pace.json` limits.                                                                                                        | `FAIL` (`WARN` if baseline is unreachable)      |

## Rollout pace

`site/pace.json` controls how fast new pages go live:

```json
{
  "mode": "start",
  "maxPerRelease": 20,
  "maxPerWeek": 5
}
```

Keep `mode` at `"start"` until Google indexes the first wave of comparison and content pages cleanly. Raise the cap only after checking Google Search Console indexing and click data.

## Legacy allow-list policy

Pre-existing marketing pages that need separate cleanup PRs sit in `site/publish-gate-allowlist.json` with a review date and reason. Allow-listed items log `WARN` instead of `FAIL`.

Never allow-list these hard checks:

- `faq-schema` (`FAQPage` JSON-LD always fails)
- `duplicate-title` and `duplicate-description`
- `signoff` and `h1-rival`
- `proof` on new content pages

Remove each route from `site/publish-gate-allowlist.json` in the PR that fixes its underlying issue.

## Sign-off policy

Only `frantz-kati` may appear in `meta.reviewedBy` for published pages. Automated accounts, CI bots, and unapproved reviewer handles fail both `site/src/routes.test.ts` and `scripts/publish-gate.ts`.

PRs that touch route `meta`, `site/src/pages/vs/`, `site/src/vs/`, `site/src/data/rivals.ts`, or `site/content/` must complete the Content pages checklist in `.github/PULL_REQUEST_TEMPLATE.md` before merge.
