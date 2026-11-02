# Ferndesk competitor research

Research date: 2026-11-02 (Nov 2026). Subject: Ferndesk (ferndesk.com). Lens: Dimmi first users (vibe coders, solo devs, early adopters).

Tags: [unverified] = single source or conflicting sources. [3P] = third-party or competitor-written claim.

Doc links below use `ferndesk.com/help/<slug>`; every help article also serves raw Markdown at `<slug>.md` (index: https://ferndesk.com/help/llms.txt).

## TL;DR

- Ferndesk is an AI help center, not a support desk. Tagline "The help center that never goes stale" (https://ferndesk.com). Agent "Fern" reads GitHub, support tickets, Slack, Linear, videos, live app; drafts new and updated articles; human approves every change.
- No inbox, no live chat, no human agent seat. Widget does self-serve search plus AI answers, then hands off to Intercom, Crisp, Help Scout, Zendesk, Pylon, Plain, or email form (https://ferndesk.com/help/using-ferndesk-with-other-chat-platforms-f5hri). Ferndesk is complement to helpdesk, and competitor to its docs module.
- Solo founder Wilson Wilson (ex-Senja), founded 2025, first hire Jan 2026 (https://render.com/customers/ferndesk, https://ferndesk.com/help/ferndesk-for-chrome-keqoq). "100+ software teams" (homepage). Product Hunt #2 of day, 2026-09-30, 386 upvotes (https://www.producthunt.com/products/ferndesk).
- Pricing: one plan. Pro $149/month ($1,490/year), 5 editor seats, 1,000 AI conversations, unlimited PR reviews, 5 languages. Early-stage $75/month year one (under $1M ARR). Enterprise from $399/month (SAML, SCIM, 2,500 conversations). Add-ons: 500 conversations $25, seat $10, language $10, workspace $39 (https://ferndesk.com/help/ferndesk-pricing-and-plans-k5ojj). 7-day trial, no free plan.
- Strongest pieces: merged-PR drafting with sensitivity setting, claim-level verification of every published article against code/site/tickets, scheduled audits from tickets + failed searches + feedback + commits, Playwright-style screenshot recapture by logging into app, 22-platform importer with redirects, workspace MCP plus public reader MCP, llms.txt, subfolder hosting.
- Weak spots for our audience: $149 flat entry (no free tier), closed source, cloud only, model hidden behind OpenRouter (no BYOK), no SOC 2 yet, no own conversations layer, Discord only via forums, docs live in Ferndesk DB not user repo.
- Dimmi opening: Ferndesk proves "docs maintain themselves from merged PRs" sells to indie SaaS at $75-$149/month for docs alone. Dimmi can bundle same loop with inbox plus agent, self-hosted, BYOK, docs-as-code.

## Company and market status

| Item | Value |
|---|---|
| Founder | Wilson Wilson, founder (https://ferndesk.com/blog/how-to-price-your-ai-saas-2-mistakes-we-made, https://www.producthunt.com/products/ferndesk). Previously Senja (same blog) [unverified detail] |
| Founded | 2025 (https://render.com/customers/ferndesk). Changelog archive starts July 2025 [unverified] |
| Team | Ran as "one-man show" for about six months; Victor joined as employee #2, Jan 2026 (https://ferndesk.com/help/ferndesk-for-chrome-keqoq). Current size not stated |
| Funding | None found [unverified] |
| Customers | "Trusted by 100+ software teams"; logos Metricool, Zeffy, Andri, PixelFlow, SEO Gets; case studies RightMessage, ISMS Copilot, GummySearch, Youform, We Are Distributed, Client Portal, Duplika, Mallow, Cap (https://ferndesk.com, https://ferndesk.com/customers) |
| Infra | Render, nine services, Docker plus Node, Valkey; moved from Railway (https://render.com/customers/ferndesk) |
| AI vendors | OpenRouter (generation, embeddings, rerank), Google Vertex AI (embeddings), Turbopuffer (search index) (https://ferndesk.com/help/security-overview-hmx06) |
| Compliance | Not SOC 2. Type I expected Oct/Nov 2026, Type II Q4 2026 or Q1 2027. No trust center (https://ferndesk.com/help/security-overview-hmx06) |
| Product Hunt | Launched 2026-09-30, #2 of day, 386 upvotes, 5.0 from 2 reviews (https://www.producthunt.com/products/ferndesk) |
| HN | No Show HN thread found |

Timeline (changelog https://ferndesk.com/help/changelog; dates from entry pages):
- 2025-08-20 Zendesk and Crisp sync. 2025-09-12 "Introducing Fern", audits replace support trends, pricing simplified to $59 base plus $0.50 per published article. 2025-10-22 subfolder hosting. 2025-11-27 recommendations inbox and signals (code plus support).
- 2025-12-10 AI answers widget. 2026-01-15 automated screenshots, Email Fern, llms.txt. 2026-01-30 Chrome extension. 2026-03-24 Fern V2 (whole help center structure), OpenAPI docs, REST API.
- 2026-04-27 auto-draft from merged PRs. 2026-05-14 Slack. 2026-05-20 video uploads, side-by-side diffs. 2026-06-03 user groups, article visibility. 2026-06-09 translation glossary. 2026-06-19 escalation to human support.
- 2026-08-04 MCP, public API rework, webhooks. 2026-08-10 changelogs drafted from PRs. 2026-09-07 article verification, dark mode. 2026-09-12 GraphQL API references. 2026-09-30 Product Hunt.

Cadence: roughly one substantial release every two to three weeks from team of one to few.

## Pricing

| Plan | Price | Included | Key adds |
|---|---|---|---|
| Pro | $149/month or $1,490/year | 5 editor seats, 1,000 AI conversations/month, unlimited PR reviews, 5 languages, 1 workspace | Fern drafting, PR screening, audits, verification, screenshots, translations, custom domain, /help subfolder, white label, private help center, reader SSO (OIDC), magic links, API, MCP |
| Early-stage | $75/month year one ($745 prepaid) | Same as Pro | Under $1M ARR, under 10 people; full price month 13 |
| Enterprise | From $399/month, sales | 2,500 AI conversations/month | SAML SSO, SCIM, custom integrations, DPA, security questionnaires, invoicing |

Sources: https://ferndesk.com/pricing, https://ferndesk.com/help/ferndesk-pricing-and-plans-k5ojj.

- Add-ons (https://ferndesk.com/help/buy-additional-usage-bc25m): 500 AI conversations $25 ($0.05 each); 50 PR reviews $25; editor seat $10/month; language $10/month; extra workspace (separate help center) $39/month. Not purchasable during trial.
- Units: AI conversation counted once per conversation that produces generated answer (https://ferndesk.com/help/what-are-ai-answers-and-pr-reviews-x9ep0). At zero, assistant shows fallback; help center stays live (https://ferndesk.com/help/what-happens-when-i-run-out-of-ai-answers-or-pr-reviews-so5ls).
- Every workspace member counts as billed seat since 2026-06-19 (https://ferndesk.com/help/seamless-escalation-to-human-support-s15ey). No free viewer-only seats.
- Trial: 7 days, no card; editing paused at expiry, content kept (https://ferndesk.com/help/what-happens-when-your-trial-ends-w56no).
- History: per-article-drafted, then edit credits, then $0.50 per published article, then Solo/Startup/Business tiers, then single base plan (https://ferndesk.com/blog/how-to-price-your-ai-saas-2-mistakes-we-made). Legacy Startup $39 and Scale $99 tiers grandfathered (https://ferndesk.com/help/ferndesk-pricing-and-plans-k5ojj; prices per PixTeller [3P] https://pixteller.com/blog/ferndesk-review-2026-the-ai-powered-help-center-that-actually-maintains-itself-536). Entry price roughly quadrupled for new customers in 2026.

Solo dev math: $75/month year one, then $149. Covers docs plus AI answers only; still need separate helpdesk (Crisp, Help Scout, Intercom) for conversations. Dimmi self-host plus BYOK covers both.

## Feature inventory

### Help center

| Capability | Status | Evidence |
|---|---|---|
| Hosted help center | Yes | https://ferndesk.com/help/overview-tg7gf |
| Custom domain | Yes, Pro | https://ferndesk.com/help/connect-a-custom-domain-to-your-help-center-zgigc |
| Subfolder hosting (`/help`) via Cloudflare or Vercel | Yes | https://ferndesk.com/help/put-your-docs-on-a-sub-folder-with-cloudflare-713ia, https://ferndesk.com/help/put-your-docs-on-a-sub-folder-with-vercel-wot0l |
| Themes / branding | Yes: colors, custom fonts, light/dark logos, custom head/body code, white label | https://ferndesk.com/help/how-to-customize-your-help-center-cf4wn, https://ferndesk.com/help/recommendations-inbox-and-signals-k2n1w |
| Multiple help centers | Yes, one per workspace, $39/month each | https://ferndesk.com/help/manage-multiple-workspaces-xvgyu |
| Locales / translations | Yes, AI translation into 15 languages, glossary, per-language guidance, 5 included | https://ferndesk.com/help/ai-translations-overview-zrv2b |
| Search | Yes, combined articles plus API reference, natural language, typo tolerant | https://ferndesk.com/help/search-your-help-center-dw7tq, https://ferndesk.com/help/mcp-support-and-public-api-rework-sb147 |
| AI answers on help center | Yes, with citations | https://ferndesk.com/help/how-ai-answers-work-ufxew |
| llms.txt plus per-article `.md` | Yes, automatic | https://ferndesk.com/help/seo-geo-aeo-optimization-for-your-help-center-ru32t |
| Reader actions: copy as Markdown, open in ChatGPT/Claude, ask AI | Yes | https://ferndesk.com/help/fern-v2-sl6v9 |
| MCP server for docs (end readers) | Yes, toggle; tools search, read, `ask_assistant`; not on subfolder domains yet | https://ferndesk.com/help/let-readers-use-your-help-center-in-ai-tools-83x1a |
| API | Yes, scoped keys | https://ferndesk.com/help/manage-api-keys-25y55 |
| SEO | Yes: SSR, auto sitemap, meta and OG images, redirects | https://ferndesk.com/help/seo-geo-aeo-optimization-for-your-help-center-ru32t, https://ferndesk.com/help/manage-help-center-redirects-ro1im |
| Article revision history and restore | Yes | https://ferndesk.com/help/restore-a-previous-article-version-t1ig9 |
| Product-version docs (v1/v2 switcher) | Unclear. Docusaurus import handles versioned sites; no version picker documented | https://ferndesk.com/help/mcp-support-and-public-api-rework-sb147 |
| Review / approval queue for AI edits | Yes. Fern never auto-publishes; task threads "Ready to review", side-by-side diff, "Not relevant" dismiss | https://ferndesk.com/help/automatic-pr-drafts-km5ce, https://ferndesk.com/help/video-uploads-and-revamped-editor-5h3vs |
| Private help center, user groups, per-section access | Yes, Pro. Magic link, OIDC, shared password, sign in with your product, JWT identify | https://ferndesk.com/help/access-control-4j7cg, https://ferndesk.com/help/jwt-authentication-5b7kd |
| Article visibility: hidden, unlisted, AI-assistant-only | Yes | https://ferndesk.com/help/user-groups-and-article-visibility-7j4ob |
| API reference docs (OpenAPI, GraphQL, try-it playground) | Yes | https://ferndesk.com/help/api-references-graphql-support-and-a-refreshed-playground-9j9cx |
| Analytics: views, votes, missed searches, feedback | Yes | https://ferndesk.com/help/using-help-center-analytics-to-improve-your-content-qyjcy |
| Export (ZIP: articles, drafts, translations) | Yes | https://ferndesk.com/help/export-your-help-center-data-kmctz |
| Docs-as-code (content in user repo, PR review in GitHub) | No. GitHub access read-only; never writes back | https://ferndesk.com/help/security-overview-hmx06 |

### Auto-updating docs

| Capability | Status | Evidence |
|---|---|---|
| GitHub merged PRs trigger draft | Yes. Production branch per repo; revert PRs skipped; sensitivity Strict / Balanced / Eager; guidance up to 4,000 chars | https://ferndesk.com/help/automatic-pr-drafts-km5ce |
| Code research for any task | Yes, multi-repo, branch-scoped, no persistent code index | https://ferndesk.com/help/github-integration-qqglv, https://ferndesk.com/help/security-overview-hmx06 |
| Linear | Yes, context source; @Fern or assign issue to Fern creates task | https://ferndesk.com/help/how-to-use-fern-in-linear-issues-l4ojz |
| Support tickets as source | Yes: Intercom, Zendesk, Help Scout, Crisp, HubSpot, Plain, Pylon, Missive, BoldDesk, Fernand, Gmail, Featurebase, custom JSON endpoint | https://ferndesk.com/integrations, https://ferndesk.com/help/custom-support-endpoint-bvhje |
| Slack | Yes: @Fern in thread makes task; selected channels feed support trends | https://ferndesk.com/help/slack-integration-b6an3 |
| Discord | Yes, forum channels only (up to 3,000 closed threads) | https://ferndesk.com/help/discord-integration-us1cg, https://ferndesk.com/help/discord-integration-evzrs |
| Changelog as source | Yes, plus Fern drafts changelog entries from merged PRs and checks for stale articles | https://ferndesk.com/help/changelogs-are-here-47487 |
| Video as source | Yes, uploads or YouTube/Loom/Wistia/Vimeo | https://ferndesk.com/help/how-fern-creates-articles-zqy2e |
| Email Fern (per-workspace inbound address; Zapier/n8n) | Yes | https://ferndesk.com/help/email-fern-with-tasks-and-requests-kkgkr |
| How drafts proposed | Task thread on Fern page with PR metadata, summary, draft; "Review & edit changes"; ask Fern for revisions in thread | https://ferndesk.com/help/automatic-pr-drafts-km5ce |
| Cold start from website / codebase | Yes, "Fern can bootstrap a help center from your website and connected product sources" | https://ferndesk.com/help/what-is-ferndesk-llazr |
| Cold start from past tickets | Yes via audits (lookback up to one year) | https://ferndesk.com/help/how-fern-audits-your-help-center-vk8jz |
| Import from other help centers | Yes, 22 platforms incl. Intercom, Zendesk, Help Scout, Crisp, HubSpot, GitBook, Mintlify, Document360, Docusaurus, Featurebase, Plain, Slite; redirects preserved; no API keys needed | https://ferndesk.com/help/migrate-your-help-center-to-ferndesk-enryy |
| Content gap detection from support | Yes: audits cluster tickets, failed searches, unanswered AI queries, negative feedback | https://ferndesk.com/help/how-fern-audits-your-help-center-vk8jz, https://ferndesk.com/help/content-gap-detection-358st |
| Scheduled audits | Yes, weekly / biweekly / monthly | https://ferndesk.com/help/scheduled-audits-m5cij |
| Stale-article detection | Yes. Verification checks each claim against code, website, API refs, tickets; tabs Needs review / Up to date / Can't verify; 30-day freshness; continuous mode | https://ferndesk.com/help/verify-your-help-center-articles-0mv7r |
| Screenshot capture and recapture | Yes (beta): Fern logs into app with stored login steps, incl. magic-link via generated inbox; annotates; no Google/GitHub OAuth | https://ferndesk.com/help/dashboard-integration-1uhd0, https://ferndesk.com/help/magic-link-and-email-authentication-for-screenshots-q6sxw |
| Auto-publish | No, never | https://ferndesk.com/help/how-fern-creates-articles-zqy2e |

### AI agent / chat (customer-facing)

| Capability | Status | Evidence |
|---|---|---|
| Widget | Yes: script SDK, search, articles, AI assistant, What's new, contact; widget / sidebar / modal / inline modes; React example; JS events | https://ferndesk.com/help/self-service-widget-overview-vao8v, https://ferndesk.com/help/ferndesk-sdk-reference-14bi5 |
| Live chat | No native. Hands off to Intercom, Crisp, Help Scout, Zendesk, Pylon, Plain widgets | https://ferndesk.com/help/using-ferndesk-with-other-chat-platforms-f5hri |
| Inbox | No. Contact form emails support address | https://ferndesk.com/help/configure-widget-settings-vds71 |
| Human handoff | Yes: AI decides / always visible / off; AI-drafted handoff summary prefilled in chat platform composer | https://ferndesk.com/help/ai-escalation-and-handoff-behavior-i02iu |
| Email channel (agent answers email) | No | https://ferndesk.com/help/what-is-the-ai-assistant-4rrkd (help center and widget only) |
| Slack / Discord / Telegram / WhatsApp answering customers | No. Slack and Discord are read-only sources and task triggers | https://ferndesk.com/help/slack-integration-b6an3, https://ferndesk.com/help/discord-integration-us1cg |
| BYOK / model choice | No. OpenRouter-routed, model undisclosed | https://ferndesk.com/help/security-overview-hmx06 |
| Citations | Yes, plus up to two article images and one walkthrough video link | https://ferndesk.com/help/how-ai-answers-work-ufxew |
| Configuration | Tone, emoji, answer length, 4,000-char instructions, 20 off-limits topics; same config for help center, widget, MCP | https://ferndesk.com/help/customize-the-ai-assistant-c56e1 |
| Playground / test mode | No dedicated mode documented; test by asking live assistant | https://ferndesk.com/help/ai-escalation-and-handoff-behavior-i02iu |
| Analytics | Yes: conversations, escalations, coverage (full / partial / none / irrelevant), cited articles, "Ask Fern to draft an article" per row | https://ferndesk.com/help/using-help-center-analytics-to-improve-your-content-qyjcy |
| Agent actions (Stripe, HTTP tools) | No | Not in docs index https://ferndesk.com/help/llms.txt |
| Reply drafting for support agents | Yes, Chrome extension: type `!!` in any reply box to insert article link, short answer, or Fern-drafted reply | https://ferndesk.com/help/search-and-draft-replies-with-the-popover-ugwbj |

### Other modules

| Capability | Status | Evidence |
|---|---|---|
| Changelog | Yes: Fern drafts from PRs, new/improved/fixed/breaking tags, reactions, RSS, full-screen in-app announcement, widget feed | https://ferndesk.com/help/changelogs-are-here-47487, https://ferndesk.com/help/changelogs-uxh2l |
| Feedback board | No. Ferndesk's own feedback.ferndesk.com runs on Featurebase (listed subprocessor) | https://ferndesk.com/help/security-overview-hmx06 |
| Roadmap | No | Not in docs index https://ferndesk.com/help/llms.txt |
| Announcements | Partial: changelog modal and widget prompts only | https://ferndesk.com/help/announce-changelog-updates-in-the-widget-l0a8t |
| Surveys | No (only article thumbs up/down plus comments) | https://ferndesk.com/help/using-help-center-analytics-to-improve-your-content-qyjcy |
| Webhooks | Yes, signed, retried; article updates, feedback | https://ferndesk.com/help/mcp-support-and-public-api-rework-sb147 |
| Public API | Yes: articles, drafts, revisions (with diff), translations, collections, sections, changelogs, analytics, feedback, assistant conversations, users, tasks, webhooks | https://ferndesk.com/help/llms.txt |
| CLI | No | Not in docs index https://ferndesk.com/help/llms.txt |
| Chrome extension | Yes, docs sidebar, chat with docs, edit from anywhere | https://ferndesk.com/help/ferndesk-for-chrome-keqoq |
| Free tools (SEO funnel) | OpenAPI validator, docs-debt quiz, savings calculator | https://ferndesk.com/tools/openapi-validator, https://ferndesk.com/savings-calculator |

### Platform

| Item | Status | Evidence |
|---|---|---|
| Open source | No (no repo or license found) | https://ferndesk.com |
| Self-host | No; hosted on Render | https://render.com/customers/ferndesk |
| Team SSO | SAML plus SCIM Enterprise only | https://ferndesk.com/pricing |
| Reader SSO | OIDC, magic links, JWT on Pro | https://ferndesk.com/help/access-control-4j7cg |
| Data handling | Code not stored or indexed; temp workspaces; ticket PII redacted; deleted 30 days after termination | https://ferndesk.com/help/security-overview-hmx06 |

### Agent-native

- Workspace MCP over OAuth, no API key. Search, read, create/update/publish drafts, conversations, analytics, user groups. Guides for Claude, Claude Code (`claude mcp add --transport http ferndesk <url>`), Codex CLI, Cursor, ChatGPT (https://ferndesk.com/help/use-ferndesk-with-ai-tools-xnmq8, https://ferndesk.com/help/connect-claude-code-ihxmp).
- Changes from AI tools land as drafts under user's own permissions (https://ferndesk.com/help/mcp-support-and-public-api-rework-sb147).
- Public reader MCP per help center, with `ask_assistant` (https://ferndesk.com/help/let-readers-use-your-help-center-in-ai-tools-83x1a).
- Fern reachable from Linear, Slack, email, Chrome, Cmd+K. Tasks API to trigger Fern programmatically (https://ferndesk.com/help/llms.txt).
- No CLI, no docs-in-repo sync.

## Self-updating docs deep dive

Three automations under Settings > Automations (https://ferndesk.com/help/automatic-pr-drafts-km5ce):
1. Automatic PR drafts. Webhook on merge to production branch. Fern decides if change is customer-facing (UI flows, endpoints, errors, config), applies sensitivity, then drafts. Skipped merges logged as "No documentation changes needed" under Automations filter, with reason. Each merged PR consumes one PR review (unlimited on Pro).
2. Scheduled audits. One thread per run, findings grouped "Reduce your support volume", "Missing essentials", "Fill your search gaps", "Address negative feedback". Each finding: reason, proposed plan, evidence count, expandable tickets/queries/commits. Buttons "Draft with Fern", "Edit prompt first", "Dismiss" (https://ferndesk.com/help/how-fern-audits-your-help-center-vk8jz).
3. Continuous verification. Claim-by-claim check: "What's wrong" list quotes article text, suggested fix, evidence; "Draft the fix" then "Publish changes"; "Not an issue" suppresses repeat. Email to every member when run finds drift; clean runs silent (https://ferndesk.com/help/verify-your-help-center-articles-0mv7r).

Prompted tasks: Fern researches (help center, tickets, code, attachments incl. PDF/DOCX with OCR, web), shows todo plan, drafts, waits for review. Global drafting and research instructions plus "About you" memories (https://ferndesk.com/help/how-fern-creates-articles-zqy2e, https://ferndesk.com/help/recommendations-inbox-and-signals-k2n1w).

Fern V2 restructures whole help center (sections, collections, API refs) as staged update (https://ferndesk.com/help/fern-v2-sl6v9). Translations follow automatically after publish; Fern edits primary language only (https://ferndesk.com/help/ai-translations-overview-zrv2b).

Gaps seen: drafts and review live inside Ferndesk, not GitHub PRs; no auto-publish option even for trivial edits; screenshot login is brittle text instructions with stored demo credentials; no customer conversation layer to close loop ("your issue is fixed") itself.

## Onboarding and DX for solo dev

- Signup, email confirm, five-minute onboarding: import existing help center by URL or bootstrap from website plus GitHub (https://ferndesk.com/help/how-to-sign-up-a10kl, https://ferndesk.com/help/what-is-ferndesk-llazr).
- Widget: one script from `static.ferndesk.com/dist/sdk.js`; `Ferndesk('open' | 'openAssistant' | 'openContact')`; JWT identify (`sub`, `email`, `exp`, `iat`), widget and help center must share exact origin (https://ferndesk.com/help/ferndesk-sdk-reference-14bi5, https://ferndesk.com/help/jwt-authentication-5b7kd). No npm package documented.
- Strengths: OAuth MCP in one command, scoped API keys, signed webhooks, llms.txt everywhere, Markdown for every page, subfolder hosting guides for Cloudflare and Vercel.
- Friction: $149 after first year; trial 7 days, no add-ons during trial; still need separate helpdesk; custom support endpoint cannot be authenticated (https://ferndesk.com/help/custom-support-endpoint-bvhje).

## What users love, what they hate

Love:
- Evergreen docs from ticket history; subfolder hosting Zendesk would not allow (Juan Manuel Zolezzi, Product Hunt https://www.producthunt.com/products/ferndesk).
- Polished UI; audits against product changes vs static tools like Docusaurus, Mintlify, Fumadocs (Product Hunt review).
- Time saved: "easily saves me 20h a month" (Tristan Roth, ISMS Copilot, https://ferndesk.com/help/recommendations-inbox-and-signals-k2n1w); "20+ hours per week" (We Are Distributed, https://ferndesk.com/help/fern-v2-sl6v9). Vendor-published.
- RightMessage keeps 30,000-word help center current (https://ferndesk.com/help/introducing-fern-ljgan).

Hate / complaints:
- Early ask: auto-draft on merge instead of button (Product Hunt review); since shipped April 2026.
- Missing dark mode (shipped Sep 2026).
- Smaller ecosystem, compliance immaturity, still needs human review [3P PixTeller].
- Tiny review base (2 on Product Hunt). No HN or Reddit threads found. Sentiment mostly vendor-curated (https://love.ferndesk.com/).

## How they market

- Category creation: "self-updating help center", "documentation drift" (https://ferndesk.com/self-updating-help-center, https://ferndesk.com/blog/documentation-drift).
- Large SEO library: 13 `/compare/*` pages (Mintlify, Zendesk, Intercom, GitBook, Document360, Help Scout, others), roughly 100 listicle and review posts, `/answers` pages aimed at AI-answer engines (https://ferndesk.com/sitemap.xml).
- Migration as wedge: one-click import from 22 platforms, URLs preserved (https://ferndesk.com/migration).
- Founder-voice changelog emails, build in public, indie founder network (Senja, Bento founders named Fern) (https://ferndesk.com/help/introducing-fern-ljgan).
- Early-stage 50% discount, affiliate program, wall of love, savings calculator.

## Dimmi: must-have at launch

Only items Ferndesk sets bar on. Each line: why, then bar.

1. **Docs agent that drafts from merged PRs, with review queue.** Ferndesk sells this alone at $149/month. Bar: merge webhook, customer-facing filter with sensitivity knob, skipped-PR log with reason, diff review, never auto-publish by default.
2. **Cold start from repo plus website.** Empty docs kills agent. Bar: one command or button drafts full help center from repo and marketing site into review queue in minutes.
3. **Importer for common help centers by public URL with redirects.** Bar: Intercom, Zendesk, Help Scout, Crisp, GitBook, Mintlify at minimum; no API keys; redirects kept.
4. **llms.txt, per-article Markdown, copy-as-Markdown, open-in-ChatGPT/Claude.** Table stakes now. Bar: automatic, also on self-host.
5. **MCP server (team) plus public reader MCP.** Ferndesk ships both over OAuth. Bar: one-line `claude mcp add`, drafts land in review queue, reader `ask` tool uses same agent config.
6. **Subfolder hosting.** Indie SEO lever Ferndesk markets hard. Bar: documented Cloudflare Worker, Vercel rewrite, Next.js middleware recipes.

## Dimmi: must ship after MVP (strict priority)

1. **Claim-level verification of published articles.** Ferndesk's newest and most defensible feature; turns "docs rot" into checklist.
2. **Gap audits from conversations, failed searches, unanswered agent queries.** Dimmi owns conversations natively, so signal better than Ferndesk's synced copy.
3. **Changelog drafted from merged PRs with in-app announcement.** Ferndesk ships; pairs with loop closing.
4. **Screenshot recapture from live app.** Ferndesk beta; strong demo moment.
5. **AI translations with glossary.** Cheap under BYOK.
6. **Chrome-style reply assist and Linear/Slack @agent task triggers.** Nice once core loop works.

## Standout ideas beyond Ferndesk

- **Docs-as-code PRs.** Ferndesk keeps docs in its DB and GitHub access is read-only. Dimmi opens PR in user repo with doc change next to code change; review in GitHub.
- **One product, not two.** Ferndesk needs Intercom/Crisp/Help Scout for conversations. Dimmi: inbox, agent, docs, changelog in one self-hosted app; handoff stays inside.
- **Close loop with customer.** Ferndesk sees ticket but cannot reply. Dimmi links conversation to PR; on merge, docs updated and asking customers messaged.
- **Pre-merge docs check.** GitHub check on open PR: "this PR makes 3 articles wrong", draft attached. Ferndesk only acts after merge.
- **BYOK, zero markup, visible cost.** Ferndesk charges $0.05 per AI conversation over 1,000 and hides model. Dimmi shows tokens per conversation; local models on self-host.
- **Open verification rules.** Publish how claims verified and what "Can't verify" means; trust play against closed agent.

## Open questions and unverified

- Team size today, funding [unverified].
- Exact legacy tier prices and when Pro replaced them [3P PixTeller only].
- Which LLMs behind OpenRouter; whether model changes over time.
- Product-version doc switcher support.
- Whether bootstrap-from-website runs automatically in onboarding or needs prompted task.
- SOC 2 Type I delivery (promised Oct/Nov 2026).
- Widget bundle size; npm package.
- Verification accuracy and false-positive rate: none published.
- Not trialed (read-only research).

## Sources

1. https://ferndesk.com
2. https://ferndesk.com/pricing
3. https://ferndesk.com/integrations
4. https://ferndesk.com/migration
5. https://ferndesk.com/customers
6. https://ferndesk.com/sitemap.xml
7. https://ferndesk.com/self-updating-help-center
8. https://ferndesk.com/help
9. https://ferndesk.com/help/llms.txt
10. https://ferndesk.com/help/changelog
11. https://ferndesk.com/help/what-is-ferndesk-llazr
12. https://ferndesk.com/help/how-to-sign-up-a10kl
13. https://ferndesk.com/help/ferndesk-pricing-and-plans-k5ojj
14. https://ferndesk.com/help/what-are-ai-answers-and-pr-reviews-x9ep0
15. https://ferndesk.com/help/buy-additional-usage-bc25m
16. https://ferndesk.com/help/what-happens-when-i-run-out-of-ai-answers-or-pr-reviews-so5ls
17. https://ferndesk.com/help/what-happens-when-your-trial-ends-w56no
18. https://ferndesk.com/help/how-to-invite-your-team-90x2f
19. https://ferndesk.com/help/how-fern-creates-articles-zqy2e
20. https://ferndesk.com/help/automatic-pr-drafts-km5ce
21. https://ferndesk.com/help/how-fern-audits-your-help-center-vk8jz
22. https://ferndesk.com/help/scheduled-audits-m5cij
23. https://ferndesk.com/help/verify-your-help-center-articles-0mv7r
24. https://ferndesk.com/help/content-gap-detection-358st
25. https://ferndesk.com/help/recommendations-inbox-and-signals-k2n1w
26. https://ferndesk.com/help/what-is-the-ai-assistant-4rrkd
27. https://ferndesk.com/help/how-ai-answers-work-ufxew
28. https://ferndesk.com/help/ai-escalation-and-handoff-behavior-i02iu
29. https://ferndesk.com/help/customize-the-ai-assistant-c56e1
30. https://ferndesk.com/help/self-service-widget-overview-vao8v
31. https://ferndesk.com/help/configure-widget-settings-vds71
32. https://ferndesk.com/help/ferndesk-sdk-reference-14bi5
33. https://ferndesk.com/help/using-ferndesk-with-other-chat-platforms-f5hri
34. https://ferndesk.com/help/custom-support-endpoint-bvhje
35. https://ferndesk.com/help/use-ferndesk-with-ai-tools-xnmq8
36. https://ferndesk.com/help/let-readers-use-your-help-center-in-ai-tools-83x1a
37. https://ferndesk.com/help/connect-claude-code-ihxmp
38. https://ferndesk.com/help/manage-api-keys-25y55
39. https://ferndesk.com/help/seo-geo-aeo-optimization-for-your-help-center-ru32t
40. https://ferndesk.com/help/migrate-your-help-center-to-ferndesk-enryy
41. https://ferndesk.com/help/ai-translations-overview-zrv2b
42. https://ferndesk.com/help/changelogs-uxh2l
43. https://ferndesk.com/help/github-integration-qqglv
44. https://ferndesk.com/help/linear-integration-vybpq
45. https://ferndesk.com/help/how-to-use-fern-in-linear-issues-l4ojz
46. https://ferndesk.com/help/slack-integration-b6an3
47. https://ferndesk.com/help/discord-integration-us1cg
48. https://ferndesk.com/help/search-and-draft-replies-with-the-popover-ugwbj
49. https://ferndesk.com/help/email-fern-with-tasks-and-requests-kkgkr
50. https://ferndesk.com/help/security-overview-hmx06
51. https://ferndesk.com/help/restore-a-previous-article-version-t1ig9
52. https://ferndesk.com/help/manage-multiple-workspaces-xvgyu
53. https://ferndesk.com/help/dashboard-integration-1uhd0
54. https://ferndesk.com/help/magic-link-and-email-authentication-for-screenshots-q6sxw
55. https://ferndesk.com/help/access-control-4j7cg
56. https://ferndesk.com/help/jwt-authentication-5b7kd
57. https://ferndesk.com/help/export-your-help-center-data-kmctz
58. https://ferndesk.com/help/using-help-center-analytics-to-improve-your-content-qyjcy
59. https://ferndesk.com/help/changelog/ferndesk-is-live-on-product-hunt-uk0ub
60. https://ferndesk.com/help/changelog/api-references-graphql-support-and-a-refreshed-playground-9j9cx
61. https://ferndesk.com/help/changelog/verify-article-accuracy-a-refreshed-ferndesk-and-dark-mode-blnya
62. https://ferndesk.com/help/changelogs-are-here-47487
63. https://ferndesk.com/help/mcp-support-and-public-api-rework-sb147
64. https://ferndesk.com/help/seamless-escalation-to-human-support-s15ey
65. https://ferndesk.com/help/user-groups-and-article-visibility-7j4ob
66. https://ferndesk.com/help/video-uploads-and-revamped-editor-5h3vs
67. https://ferndesk.com/help/auto-draft-from-merged-prs-1ro1l
68. https://ferndesk.com/help/fern-v2-sl6v9
69. https://ferndesk.com/help/ferndesk-for-chrome-keqoq
70. https://ferndesk.com/help/automated-screenshots-qocpj
71. https://ferndesk.com/help/ai-answers-bhq5x
72. https://ferndesk.com/help/introducing-fern-ljgan
73. https://ferndesk.com/help/live-chat-integrations-lkl1l
74. https://ferndesk.com/blog/how-to-price-your-ai-saas-2-mistakes-we-made
75. https://render.com/customers/ferndesk [3P, vendor case study]
76. https://www.producthunt.com/products/ferndesk
77. https://pixteller.com/blog/ferndesk-review-2026-the-ai-powered-help-center-that-actually-maintains-itself-536 [3P]
78. https://love.ferndesk.com/
