# Docs and API reference SEO spec (`/docs/*`)

Every documentation page and generated API reference page in `site/dist/docs/` must carry a unique `<title>` and a unique `<meta name="description">`. Generated `/docs/api-reference/*` pages receive a `WARN` instead of a `FAIL` on the 65% shingle uniqueness check because REST endpoints share parameter and response phrasing.

## Title and description rules

- **Unique title:** every `/docs/*` page, including every `/docs/api-reference/*` endpoint page, must have a unique `<title>` of 60 characters or fewer. Duplicate titles fail the publish gate.
- **Unique description:** every `/docs/*` page must have a unique `<meta name="description">`. Duplicate or empty descriptions fail the publish gate, including on generated API reference pages.
- **One `<h1>`:** every docs page has exactly 1 `<h1>` with zero rival names in the heading text.

## Uniqueness policy

| Section                                           | 65% shingle check     | Duplicate title or description |
| ------------------------------------------------- | --------------------- | ------------------------------ |
| Hand-written guides (`/docs/*`)                   | `FAIL` below 65%      | `FAIL`                         |
| Generated API reference (`/docs/api-reference/*`) | `WARN` only below 65% | `FAIL`                         |

The generator for `/docs/api-reference/*` reads endpoint summaries and descriptions from `tests/contracts/api_v1.openapi.json`. When a new endpoint is added to the contract, give its OpenAPI `summary` and `description` specific, distinct wording so its generated title and meta description stay unique.

## Sitemap dates and schema

- **Hand-written docs:** `scripts/generate-sitemap.ts` sets `<lastmod>` from the last git commit touching the page file in `docs/src/content/docs/`.
- **API reference:** `scripts/generate-sitemap.ts` sets `<lastmod>` from the last git commit touching `tests/contracts/api_v1.openapi.json`.
- **Schema:** emit valid `BreadcrumbList` or `WebPage` JSON-LD only. Never emit `FAQPage` schema on docs pages.

## Internal linking and quality

- Keep every internal link working across both `/docs/*` and marketing routes (`scripts/publish-gate.ts` checks every link in `site/dist/docs/` when docs are built).
- Add contextual links from relevant guides (such as quickstart, self-hosting, migration, and SDK guides) to `/pricing`, `/self-host`, and matching `/vs/*` pages.
- Follow `docs/WRITING.md` and `WRITING.md` for all hand-written docs prose: zero em dashes, zero curly quotes, and zero filler adverbs.
