# Blog post spec (`/blog/<slug>`)

Blog posts live as MDX files in `site/content/blog/<slug>.mdx` and prerender to `site/dist/blog/<slug>/index.html`. Every post targets 1 main keyword, opens with a 2 to 3 line direct answer, and includes first-hand proof from dimmi's own codebase, numbers, or product captures.

## Frontmatter schema

Use the exact same keys as `PageMeta` in `site/src/routes.ts`:

```yaml
---
title: "Intercom Fin pricing in 2026: real monthly math"
description: "Intercom Fin bills $0.99 per outcome on top of seat fees. See real monthly bills for 5, 15, and 25 seats next to BYO-key agents."
type: "blog"
keyword: "intercom fin pricing"
answer: "Intercom Fin adds $0.99 per resolved conversation on top of $29 to $132 per seat. A 5-seat team resolving 1,200 chats a month pays $1,333 on Essential."
author: "frantz-kati"
published: "2026-12-01"
updated: "2026-12-01"
status: "published"
reviewedBy: "frantz-kati"
reviewedOn: "2026-12-01"
proof:
  - "shots/usage-dark.webp"
rivals:
  - name: "Intercom"
    checkedOn: "2026-12-01"
    source: "https://www.intercom.com/pricing"
---
```

- **Draft safety:** posts with `status: "draft"` never prerender into `site/dist`, never appear on `/blog`, and never enter `sitemap.xml` or `/blog/rss.xml`.
- **Title and description:** `title` is 60 characters or fewer; `description` is 140 to 155 characters (160 max at the gate).
- **Primary heading (`<h1>`):** never put a rival name in the `<h1>` (`WRITING.md` rule 4).

## Required page elements

1. **Direct answer up top:** 2 to 3 short lines in the first 50 words answering the query with concrete numbers or code. Match `frontmatter.answer`.
2. **Visible author and dates:** show the author name, published date, and updated date in `<main>` (matching JSON-LD and `sitemap.xml` `<lastmod>`).
3. **First-hand proof:** every post adds real evidence built or measured by us (working code, benchmark output, pricing math, or a real dimmi capture listed in `proof`).
4. **Scannable headings:** sentence-case `<h2>` and `<h3>` headings, with at least 1 `<h2>` phrased as a real buyer question followed by a 1 to 2 sentence answer. Long posts (1,200+ words) use 5 or more `<h2>` headings.
5. **Footer actions:** include "Start free" and GitHub star buttons at the bottom of the post.

## Schema and feeds

- **JSON-LD:** emit `BlogPosting` (`headline`, `description`, `datePublished`, `dateModified`, `image`, `author` with `@type: "Person"`) and `BreadcrumbList` (`Home`, `Blog`, post title). Zero `FAQPage` schema.
- **Sitemap and RSS:** `updated` drives `<lastmod>` in `sitemap.xml`. Published posts also appear on `/blog` (newest first) and `/blog/rss.xml`.
- **Changelog boundary:** dimmi's own product changelog lives in dimmi's Changelog on our help center, never as blog posts.

## Internal linking and rollout

- Add 2 to 4 contextual internal links inside `<main>`, with at least 1 link to `/pricing`, `/self-host`, `/tools/*`, or `/vs/*` in the upper 40% of the post.
- Keep anchors descriptive (3 to 7 words) and varied across posts.
- Score at least 65% unique on 5-word normalized shingles and 0 hits on the slop check.
- Ship at most 20 pages per batch (starting at 2 to 3 posts per week within `site/pace.json`), with `reviewedBy: "frantz-kati"` on every published post.
