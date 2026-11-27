# Comparison page spec (`/vs/<rival>`)

Every `/vs/<rival>` page is a TSX marketing page in `site/src/pages/vs/<rival>.tsx` backed by verified rival data in `site/src/data/rivals.ts`. It answers a buyer comparing dimmi to one named rival with checked prices, real product captures, and at least 3 honest reasons to pick the rival.

## Architecture and files

- **Page component:** `site/src/pages/vs/<rival>.tsx`, built with shared kit components in `site/src/vs/`.
- **Rival data:** `site/src/data/rivals.ts` holds plans (list and promo prices), add-ons, AI billing unit, `tone`, facts with `source` and `checkedOn`, rival wins, dimmi wins, switching triggers, and team profiles.
- **Pricing parity:** `site/src/pricing/competitors.ts` re-exports from `site/src/data/rivals.ts`, and dimmi prices come from `site/src/pricing/prices.ts` so `/pricing` and `/vs/*` stay in lockstep.
- **Route registration:** add the route with a full `PageMeta` (`type: "vs"`) to `site/src/routes.ts` and wire `/vs /vs/*` in `deploy/k8s/apps/base/edge/Caddyfile`.

## Metadata and SEO

- **Title:** `dimmi vs <Rival>: <the difference>`, 60 characters or fewer.
- **Description:** lead with the verdict and 1 real number, 140 to 160 characters.
- **Primary heading (`<h1>`):** never put the rival name in the `<h1>` (`WRITING.md` rule 4). Put the rival name in the eyebrow, breadcrumb, title tag, and opening verdict.
- **Schema:** `WebPage` (with `author` as `@type: "Person"`, `datePublished`, `dateModified`, `reviewedBy`) and `BreadcrumbList` (`Home`, `Compare`, `dimmi vs <Rival>`). Zero `FAQPage` schema.

## Required sections

1. **Verdict in the first 50 words:** 2 sentences right under the `<h1>`. Sentence 1 states when the rival is the better pick; sentence 2 states when dimmi wins. Match `meta.answer`.
2. **Sources strip:** author, reviewer (`frantz-kati`), `updated` date, and the `checkedOn` date with source link to the rival pricing page.
3. **Unique hero visual:** one custom visual built only for this rival (for example, a Fin `$0.99` meter for Intercom or a stack diagram for Chatwoot), with its default state rendered in prerendered HTML.
4. **Live bill duel:** real math at 1, 5, and 25 seats (or the rival's real billing unit) using `rivals.ts` and `prices.ts`, showing both list and promo prices when both exist. Default results render in static HTML without JavaScript.
5. **Unique rival block:** one interactive block specific to this rival's pricing trap (such as an outcome slider, plan picker, channel picker, or self-host cost builder).
6. **Switching triggers:** 4 to 8 factual rows, each backed by at least 3 public complaints from the last 12 months (G2, Reddit, GitHub Issues, X, or HN) linked in the PR description.
7. **Real product proof:** real dimmi captures (`site/src/shots.ts`) listed in `meta.proof` and verified on disk. Never use competitor logos or screenshots of competitor UIs.
8. **Where the rival wins:** at least 3 verified bullets naming real strengths of the rival and real dimmi gaps (such as missing SOC 2 or ISO 27001 today, shorter marketplace, or mobile SDK gaps).
9. **Team picker:** 4 concrete team profiles with a clear pick, reason, and estimated bill. At least 1 profile picks the rival when that is the honest call.
10. **Migration steps:** concrete export/import path (such as `dimmi migrate` commands for Intercom) stating what moves and what stays behind.
11. **Visible questions and CTA:** 3 to 5 buyer questions rendered as plain HTML (zero `FAQPage` JSON-LD) followed by one migration-focused CTA.

## Uniqueness and honesty gates

- **Uniqueness score:** at least 65% unique on 5-word normalized shingles against every other `/vs/*` page, `/`, and `/pricing`.
- **Word budgets:** at least 900 unique words and at most 300 shared template words per page.
- **Freshness:** check every rival price on the PR day. `checkedOn` fails if more than 45 days older than `meta.updated` and warns after 90 days.
- **Stop rule:** if dimmi loses on every switching trigger for a rival, do not ship the page.

## Internal linking

- Link every `/vs/<rival>` page to `/pricing`, `/self-host`, `/vs` (once 2 or more `/vs` pages exist), and at least 1 sibling `/vs/<other>` page from `<main>`.
- Give every `/vs/<rival>` page at least 3 contextual inbound links from `<main>` on other pages plus a link in the footer Compare column.
