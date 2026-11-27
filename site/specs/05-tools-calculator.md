# Interactive calculator spec (`/tools/*`)

Every calculator under `/tools/<slug>` is a standalone TSX page (such as `site/src/pages/tools/intercom-cost-calculator.tsx`) that renders its default calculation in prerendered HTML without JavaScript. Calculators reuse the exact pricing modules in `site/src/pricing/` so numbers never disagree with `/pricing`.

## Metadata and SEO

- **Route:** `/tools/<slug>` in `site/src/routes.ts` with `PageMeta` (`type: "tool"`, `reviewedBy: "frantz-kati"`), plus `/tools /tools/*` in `deploy/k8s/apps/base/edge/Caddyfile`.
- **Canonical URL:** strip all query parameters from `<link rel="canonical">`. Share links like `?seats=5&conv=2000&ai=0.8` hydrate the form inputs while the canonical tag stays `/tools/<slug>`.
- **Title:** target query plus core inputs and year when useful, 60 characters or fewer (for example, `Intercom pricing calculator: seats + Fin cost (2026)`).
- **Description:** 140 to 160 characters naming the inputs, unit price, and checked date.
- **Primary heading (`<h1>`):** never put a rival name in the `<h1>` (`WRITING.md` rule 4).
- **Schema:** `WebApplication` (`applicationCategory: "BusinessApplication"`, `offers.price: "0"`) and `BreadcrumbList`. Zero `FAQPage` schema.

## Calculation and SSR requirements

1. **Prerendered defaults:** compute and render the default monthly and yearly bills during static prerendering so crawlers and no-JS browsers see full numbers in `<main>`.
2. **Shared math:** import `bill()` from `site/src/pricing/prices.ts`, rival pricing from `site/src/data/rivals.ts` / `site/src/pricing/competitors.ts`, and server costs from `site/src/pricing/servers.ts`. Cover parity with unit tests.
3. **2-line answer up top:** place the worked default example in the first 50 words of `<main>` and match `meta.answer`.
4. **Full cost breakdown:**
   - Rival monthly and yearly total, split into seats, AI fees, and add-ons (with list versus promo price toggles and monthly versus annual billing where the rival offers both).
   - dimmi Cloud total for the exact same usage, with and without BYO model keys.
   - dimmi self-hosted cost: `$0` to dimmi plus server cost from `servers.ts`.
   - Plain-English formula directly under the result.
5. **Honesty callout:** when dimmi costs more for the current inputs (such as a small team answering 2,000 manual conversations with zero AI), show a clear note saying so.
6. **What this leaves out:** list excluded line items plainly (such as SMS, phone, WhatsApp per-message fees, email campaigns, tax, and custom enterprise discounts).
7. **Sources and freshness:** show the rival pricing URL and `checkedOn` date from `meta.rivals`.
8. **Real proof:** include a real dimmi usage capture for the default inputs, listed in `meta.proof` and verified on disk.

## Analytics and internal links

- Fire `track("pricing_calc_change", { field })` when a visitor edits an input.
- Link out from `<main>` to `/pricing`, `/self-host`, and the matching `/vs/<rival>` page.
- Score at least 65% unique on 5-word normalized shingles against `/pricing` and other `/tools/*` pages, with 0 slop hits.
