# Alternatives roundup spec (`/alternatives` and `/open-source-intercom`)

Alternatives pages help buyers shortlist 4 to 7 real tools for one clear switching pain, such as per-seat pricing or closed-source lock-in. Ship an alternatives page only when a broad roundup adds real choices that a single `/vs/<rival>` page cannot cover.

## When to ship

- Ship `/vs/<rival>` first when buyers are comparing dimmi head-to-head against 1 named tool.
- Ship `/alternatives` or `/open-source-intercom` when buyers search for a shortlist of several options and want trade-offs side by side.
- Register the route in `site/src/routes.ts` with `PageMeta` (`type: "alternatives"`), `reviewedBy: "frantz-kati"`, and `rivals` entries for every named product.

## Metadata and SEO

- **Title:** clear category or switching query plus benefit, 60 characters or fewer.
- **Description:** state the top pick, the switching pain, and 1 real number in 140 to 160 characters.
- **Primary heading (`<h1>`):** keep competitor names out of the `<h1>` (`WRITING.md` rule 4). Use a buyer-outcome headline ("Open-source customer support platforms compared") instead of naming a rival in `<h1>`.
- **Schema:** `WebPage` (with `author` as `@type: "Person"`, `datePublished`, `dateModified`, `reviewedBy`) and `BreadcrumbList`. Zero `FAQPage` schema.

## Required structure

1. **Direct answer in the first 40 words:** name the top pick, state who it fits best, and summarize the switching triggers covered below. Match `meta.answer`.
2. **How we picked:** list the 3 to 5 factual criteria used to compare every tool (seat pricing, AI model billing, license, self-hosting, migration path).
3. **Summary table:** 1 row per tool with starting price, AI pricing unit, open-source license, self-hosting support, and best-fit team.
4. **Self-contained tool blocks (4 to 7 tools):** give every tool the exact same structure so search engines and AI answers never pin one vendor's drawback onto another:
   - **Verdict:** 1 plain sentence naming what the tool does best.
   - **Best for:** concrete team size, stack, or budget.
   - **Strengths:** 2 to 3 specific capabilities checked against live docs.
   - **Trade-offs:** 1 to 2 honest limits or billing catches.
   - **Pricing:** checked starting price, AI fee, source link, and `checkedOn` date.
5. **Real product proof:** at least 1 real dimmi capture listed in `meta.proof` and stored on disk. Never copy competitor logos or UI screenshots.

## Ranking and honesty rules

- Place dimmi where it honestly belongs for the buyer persona on the page. Never rank dimmi first for a persona it does not serve well.
- Check every listed tool's pricing page and license on the PR day and record `checkedOn` and `source` in `meta.rivals`.
- State dimmi's real limits clearly (launch date, no SOC 2 or ISO 27001 yet, smaller app marketplace).

## Uniqueness and internal links

- Score at least 65% unique on 5-word normalized shingles against other `/alternatives` pages, `/`, and `/pricing`.
- Link out from `<main>` to `/pricing`, `/self-host`, and every matching `/vs/<rival>` page that exists.
- Link back to the roundup from `/vs` and relevant blog posts.
